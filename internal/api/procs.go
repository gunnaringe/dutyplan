package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// Who may call what (2026-11-22, the user: "the roles are org admin and org members + edit and view shared link"), as one
// table: every procedure of the API has a line here, and the interceptor decides from that line and the request's `org`
// and `plan` alone (TestEveryProcedureHasALine fails on a procedure without one, or one whose request lacks the fields).
//
//   - Without an organisation: Health and Me are open; the organisation list is for anyone signed in or with a link;
//     making one needs an account.
//   - In an organisation, a member (admin or member) may do everything except what is `admin` (renaming and deleting the
//     organisation, changing members; invitations are checked where links are made and deleted). Leaving (removing
//     yourself) is for everyone (`self`).
//   - Someone with a link to one plan (a cookie holds the tokens of the links opened in this browser, signed in or not):
//     view = read only, answer = read and the writes marked `answer` (anyone's answers and notes), edit = every write it
//     reaches except the organisation itself (`admin`). It reaches its own plan and what the plan's page needs from the
//     organisation (`link`: people, tasks, sections), never the plan list, other plans, the members or the links.
//
// Anyone else gets 404 (not_found), so nobody learns which organisations exist; 401 signed_out when not signed in, so
// the page can offer to sign in; 403 read_only when a link reaches it but may not write. Without a login (--no-login)
// everything goes through.

type scope int

const (
	scopeNone scope = iota // no organisation
	scopeOrg               // an organisation: the request's `org`
	scopePlan              // a plan: the request's `org` and `plan`
)

type proc struct {
	scope  scope
	write  bool // changes something: rate limited, announced on the stream
	admin  bool // only an admin, never a link
	self   bool // ... except when it is about yourself (leaving)
	link   bool // a link to a plan reaches it (org scope; a plan-scope procedure reaches the link's own plan)
	answer bool // an answer link may make this write
	costly bool // counts in the dearer bucket too (making organisations, links and drafts)
	big    bool // a bulk write: a bigger body
	open   bool // no session needed (scope none)
}

func read(s scope) proc  { return proc{scope: s, link: true} }
func write(s scope) proc { return proc{scope: s, write: true, link: true} }

const svc = "/dutyplan.v1."

// procs is the table. The keys are Connect's procedure paths.
var procs = map[string]proc{
	svc + "AccountService/Health":    {open: true},
	svc + "AccountService/Me":        {open: true},
	svc + "AccountService/ListOrgs":  {},
	svc + "AccountService/CreateOrg": {write: true, costly: true},

	svc + "OrgService/GetOrg":        read(scopeOrg),
	svc + "OrgService/RenameOrg":     {scope: scopeOrg, write: true, admin: true, link: true},
	svc + "OrgService/DeleteOrg":     {scope: scopeOrg, write: true, admin: true, link: true},
	svc + "OrgService/ListMembers":   {scope: scopeOrg},
	svc + "OrgService/SetMemberRole": {scope: scopeOrg, write: true, admin: true},
	svc + "OrgService/RemoveMember":  {scope: scopeOrg, write: true, admin: true, self: true},
	svc + "OrgService/ListLinks":     {scope: scopeOrg},
	svc + "OrgService/CreateLink":    {scope: scopeOrg, write: true, costly: true},
	svc + "OrgService/DeleteLink":    {scope: scopeOrg, write: true},

	svc + "SetupService/ListTasks":           read(scopeOrg),
	svc + "SetupService/CreateTask":          write(scopeOrg),
	svc + "SetupService/UpdateTask":          write(scopeOrg),
	svc + "SetupService/DeleteTask":          write(scopeOrg),
	svc + "SetupService/MoveTask":            write(scopeOrg),
	svc + "SetupService/ListCombinations":    read(scopeOrg),
	svc + "SetupService/SetCombination":      write(scopeOrg),
	svc + "SetupService/SetTaskCombinations": write(scopeOrg),
	svc + "SetupService/ListPeople":          read(scopeOrg),
	svc + "SetupService/CreatePerson":        write(scopeOrg),
	svc + "SetupService/UpdatePerson":        write(scopeOrg),
	svc + "SetupService/MovePerson":          write(scopeOrg),
	svc + "SetupService/SetPersonTask":       write(scopeOrg),
	svc + "SetupService/RemovePersonTask":    write(scopeOrg),
	svc + "SetupService/ListSections":        read(scopeOrg),
	svc + "SetupService/CreateSection":       write(scopeOrg),
	svc + "SetupService/RenameSection":       write(scopeOrg),
	svc + "SetupService/MoveSection":         write(scopeOrg),
	svc + "SetupService/DeleteSection":       write(scopeOrg),
	svc + "SetupService/ListTaskSections":    read(scopeOrg),
	svc + "SetupService/CreateTaskSection":   write(scopeOrg),
	svc + "SetupService/RenameTaskSection":   write(scopeOrg),
	svc + "SetupService/MoveTaskSection":     write(scopeOrg),
	svc + "SetupService/DeleteTaskSection":   write(scopeOrg),

	svc + "PlanService/ListPlans":       {scope: scopeOrg},
	svc + "PlanService/CreatePlan":      {scope: scopeOrg, write: true},
	svc + "PlanService/GetPlan":         read(scopePlan),
	svc + "PlanService/RenamePlan":      write(scopePlan),
	svc + "PlanService/DeletePlan":      write(scopePlan),
	svc + "PlanService/ListEvents":      read(scopePlan),
	svc + "PlanService/AddEvents":       write(scopePlan),
	svc + "PlanService/CopyEvent":       write(scopePlan),
	svc + "PlanService/UpdateEvent":     write(scopePlan),
	svc + "PlanService/DeleteEvent":     write(scopePlan),
	svc + "PlanService/ListPlaces":      read(scopePlan),
	svc + "PlanService/SetPlaces":       write(scopePlan),
	svc + "PlanService/ResetPlaces":     write(scopePlan),
	svc + "PlanService/SetPlacesMany":   {scope: scopePlan, write: true, link: true, big: true},
	svc + "PlanService/ListFields":      read(scopePlan),
	svc + "PlanService/AddField":        write(scopePlan),
	svc + "PlanService/RenameField":     write(scopePlan),
	svc + "PlanService/MoveField":       write(scopePlan),
	svc + "PlanService/FreezeField":     write(scopePlan),
	svc + "PlanService/DeleteField":     write(scopePlan),
	svc + "PlanService/SetFieldValue":   write(scopePlan),
	svc + "PlanService/ListAnswers":     read(scopePlan),
	svc + "PlanService/SetAvailability": {scope: scopePlan, write: true, link: true, answer: true},
	svc + "PlanService/SetNote":         {scope: scopePlan, write: true, link: true, answer: true},
	svc + "PlanService/SetAnswers":      {scope: scopePlan, write: true, link: true, big: true},

	svc + "DraftService/ListCells":       read(scopePlan),
	svc + "DraftService/AddCell":         write(scopePlan),
	svc + "DraftService/DeleteCell":      write(scopePlan),
	svc + "DraftService/ListPools":       read(scopePlan),
	svc + "DraftService/SetPool":         write(scopePlan),
	svc + "DraftService/SetLocks":        {scope: scopePlan, write: true, link: true, big: true},
	svc + "DraftService/SetDraftAside":   write(scopePlan),
	svc + "DraftService/ClearDraftAside": write(scopePlan),
	svc + "DraftService/GetWeights":      read(scopePlan),
	svc + "DraftService/SetWeights":      write(scopePlan),
	svc + "DraftService/StartRun":        {scope: scopePlan, write: true, link: true, costly: true},
	svc + "DraftService/ListRuns":        read(scopePlan),
	svc + "DraftService/GetRun":          read(scopePlan),
	svc + "DraftService/PinRun":          write(scopePlan),
	svc + "DraftService/UnpinRun":        write(scopePlan),
}

// ---- what a request carries ---------------------------------------------------------------------------------------

type ctxKey int

const (
	userKey ctxKey = iota
	grantKey
	tokensKey
	clientKey
)

// userFrom is the person signed in; ok is false without a login (--no-login) or when nobody is.
func userFrom(ctx context.Context) (store.User, bool) {
	u, ok := ctx.Value(userKey).(store.User)
	return u, ok
}

// grant is what a request may do in its organisation: through membership (role), or else through a link (level, and
// plan when the link is to one plan).
type grant struct {
	role  string
	level string
	plan  string
}

func grantFrom(ctx context.Context) grant {
	g, _ := ctx.Value(grantKey).(grant)
	return g
}

// scoped is a request about an organisation or a plan (the generated getters of `org` and `plan`).
type scoped interface{ GetOrg() string }
type planned interface{ GetPlan() string }

// ---- the decision ---------------------------------------------------------------------------------------------------

var (
	errSignedOut = problem(connect.CodeUnauthenticated, "signed_out", "", nil, "sign in first")
	errNotFound  = problem(connect.CodeNotFound, "not_found", "", nil, "not found")
	errReadOnly  = problem(connect.CodePermissionDenied, "forbidden", "read_only", nil, "this link only lets you look")
	errAdminOnly = problem(connect.CodePermissionDenied, "forbidden", "admin_only", nil, "only an admin of the organisation may do this")
)

// decide: may this caller make a request p about org (and plan)? It returns what they may do there.
// self is the id of the member the request is about, for leaving (0 otherwise).
func (s *server) decide(ctx context.Context, p proc, orgSlug, planSlug string, self int64) (grant, error) {
	u, signedIn := userFrom(ctx)
	if p.scope == scopeNone {
		// the organisation list is for anyone signed in or with a link; making one needs an account
		if !p.open && !signedIn && (p.write || len(s.links(ctx)) == 0) {
			return grant{}, errSignedOut
		}
		return grant{}, nil
	}
	g := grant{}
	if signedIn {
		role, err := s.st.Role(ctx, orgSlug, u.ID)
		if err != nil {
			return g, fail(s.log, err)
		}
		g.role = role
	}
	if g.role != store.RoleNone {
		if g.role != store.RoleAdmin && p.admin && !(p.self && self == u.ID) {
			return g, errAdminOnly
		}
		return g, nil
	}
	var reach, allow *store.Link
	for _, l := range s.links(ctx) {
		if l.Org != orgSlug {
			continue
		}
		if linkAllows(l, p, planSlug) && (allow == nil || wider(l, *allow)) {
			allow = &l
		}
		if linkReaches(l, p, planSlug) {
			reach = &l
		}
	}
	switch {
	case allow != nil:
		g.level, g.plan = allow.Level, allow.Plan
		return g, nil
	case reach != nil:
		return g, errReadOnly
	case !signedIn:
		return g, errSignedOut
	default:
		return g, errNotFound
	}
}

// linkReaches: may a link to one plan reach this procedure at all, reading or not.
func linkReaches(l store.Link, p proc, planSlug string) bool {
	if l.Plan == "" || !p.link {
		return false
	}
	if p.scope == scopePlan {
		return planSlug == l.Plan
	}
	return true
}

// linkAllows: may the link make this request. The organisation itself (renaming, deleting) is never changed by a link.
func linkAllows(l store.Link, p proc, planSlug string) bool {
	if !linkReaches(l, p, planSlug) {
		return false
	}
	if !p.write {
		return true
	}
	if l.Level == store.LevelAnswer {
		return p.answer
	}
	return l.Level == store.LevelEdit && !p.admin
}

// wider: edit before answer before view.
func wider(a, b store.Link) bool { return linkRank[a.Level] > linkRank[b.Level] }

var linkRank = map[string]int{store.LevelView: 1, store.LevelAnswer: 2, store.LevelEdit: 3}

// admin: is the asker an admin of the organisation (everyone is, without a login).
func (s *server) admin(ctx context.Context) bool {
	return s.auth == nil || grantFrom(ctx).role == store.RoleAdmin
}

// ---- the gate: access and the rate limit, before the request is decoded -------------------------------------------

// gate decides access and the rate limit for a call of the API before Connect decodes it, so someone who may not
// call learns nothing from how a request is refused (a stranger gets 404 or 401, never «invalid»). It reads only
// `org`, `plan` and `id` from the body, JSON or binary (the body is already capped by requireFormat); the grant goes into
// the context for the handlers.
func (s *server) gate(next http.Handler) http.Handler {
	writes, dear := s.writes, s.dear
	refuse := connect.NewErrorWriter()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, ok := procs[r.URL.Path]
		if !ok { // a procedure without a line is nobody's
			refuse.Write(w, r, errNotFound)
			return
		}
		if s.auth == nil { // without a login everyone may do everything, and nothing is limited
			next.ServeHTTP(w, r)
			return
		}
		raw, _ := io.ReadAll(r.Body) // read already by requireFormat
		r.Body = io.NopCloser(bytes.NewReader(raw))
		var peek struct {
			Org  any `json:"org"`
			Plan any `json:"plan"`
			ID   any `json:"id"`
		}
		if r.Header.Get("Content-Type") == binaryType {
			peek.Org, peek.Plan, peek.ID = peekBinary(r.URL.Path, raw)
		} else {
			_ = json.Unmarshal(raw, &peek) // a body that is not JSON names nothing; Connect refuses it if the gate lets it by
		}
		var orgSlug, planSlug string
		if p.scope != scopeNone {
			orgSlug, _ = peek.Org.(string)
		}
		if p.scope == scopePlan {
			planSlug, _ = peek.Plan.(string)
		}
		var self int64
		if id, ok := peek.ID.(float64); ok && p.self {
			self = int64(id)
		}
		ctx := r.Context()
		g, err := s.decide(ctx, p, orgSlug, planSlug, self)
		if err != nil {
			refuse.Write(w, r, err)
			return
		}
		ctx = context.WithValue(ctx, grantKey, g)
		if p.write {
			if err := s.limited(ctx, r, p, writes, dear); err != nil {
				refuse.Write(w, r, err)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// peekBinary reads `org`, `plan` and `id` from a binary request by their names in the procedure's request message, as
// the JSON peek does (nil for a field the message lacks or a body that does not decode: it then names nothing).
func peekBinary(path string, raw []byte) (org, plan, id any) {
	svc, method, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
	d, err := protoregistry.GlobalFiles.FindDescriptorByName(protoreflect.FullName(svc))
	sd, ok := d.(protoreflect.ServiceDescriptor)
	if err != nil || !ok || sd.Methods().ByName(protoreflect.Name(method)) == nil {
		return nil, nil, nil
	}
	in := sd.Methods().ByName(protoreflect.Name(method)).Input()
	m := dynamicpb.NewMessage(in)
	if proto.Unmarshal(raw, m) != nil {
		return nil, nil, nil
	}
	field := func(name string, kind protoreflect.Kind) (protoreflect.Value, bool) {
		f := in.Fields().ByName(protoreflect.Name(name))
		if f == nil || f.Kind() != kind || f.Cardinality() == protoreflect.Repeated {
			return protoreflect.Value{}, false
		}
		return m.Get(f), true
	}
	if v, ok := field("org", protoreflect.StringKind); ok {
		org = v.String()
	}
	if v, ok := field("plan", protoreflect.StringKind); ok {
		plan = v.String()
	}
	if v, ok := field("id", protoreflect.Int32Kind); ok {
		id = float64(v.Int()) // as JSON numbers are
	}
	return org, plan, id
}

// interceptor announces every write that went through on the organisation's stream (stream.go).
func (s *server) interceptor() connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			name := req.Spec().Procedure
			p := procs[name]
			res, err := next(ctx, req)
			if err != nil || !p.write || p.scope == scopeNone {
				return res, err
			}
			c := change{Origin: req.Header().Get(clientHeader), Proc: procName(name)}
			if m, ok := req.Any().(planned); ok && p.scope == scopePlan {
				c.Plan = m.GetPlan()
			}
			if m, ok := req.Any().(scoped); ok {
				s.hub.publish(m.GetOrg(), c)
			}
			return res, err
		}
	}
}

// limited: is this caller over their budget of changes? (ratelimit.go)
func (s *server) limited(ctx context.Context, r *http.Request, p proc, writes, dear *limiter) error {
	key, now := s.clientKey(ctx, r), time.Now()
	ok, wait := writes.allow(key, now)
	if ok && p.costly {
		ok, wait = dear.allow(key, now)
	}
	if !ok {
		e := problem(connect.CodeResourceExhausted, "rate_limited", "", nil, "too many changes; wait a moment")
		e.Meta().Set("Retry-After", strconv.Itoa(int(wait.Seconds())+1))
		return e
	}
	return nil
}

// clientKey: the person signed in, else the address (from the proxy's header when it is set).
func (s *server) clientKey(ctx context.Context, r *http.Request) string {
	if u, ok := userFrom(ctx); ok {
		return "user:" + strconv.FormatInt(u.ID, 10)
	}
	if h := s.auth.opt.ClientIPHeader; h != "" {
		if ip := strings.TrimSpace(r.Header.Get(h)); ip != "" {
			return "ip:" + ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	return "ip:" + host
}
