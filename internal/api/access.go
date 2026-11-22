package api

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// Who may do what. A request about an organisation (/api/orgs/{org}/...) is let through for:
//   - a member: owner or editor. Only the owner may delete the organisation, share it (links) and change its members.
//   - someone with a link (a cookie holds the tokens of the links opened in this browser, signed in or not): view = read
//     only, edit = change, both within the link's scope; a link to one plan reaches that plan and what its page needs from
//     the organisation (people, tasks, sections), but not the other plans, the plan list, the members or the links.
// Anyone else gets 404, so nobody learns which organisations exist (401 «signed_out» when not signed in, so the page can
// offer to sign in). Without a login (--no-login) everything goes through.

type ctxKey int

const (
	userKey ctxKey = iota
	grantKey
)

// userFrom is the person signed in; ok is false without a login (--no-login) or when nobody is.
func userFrom(r *http.Request) (store.User, bool) {
	u, ok := r.Context().Value(userKey).(store.User)
	return u, ok
}

// grant is what a request may do in its organisation: through membership (role), or else through a link (level, and
// plan when the link is to one plan).
type grant struct {
	role  string
	level string
	plan  string
}

func grantFrom(r *http.Request) grant {
	g, _ := r.Context().Value(grantKey).(grant)
	return g
}

// orgPath splits an /api/orgs/{org}/... path, as sent, into the organisation and the segments after it (each unescaped,
// so they are the same values the router gives the handlers; an encoded «/» stays inside its segment).
func orgPath(r *http.Request) (org string, rest []string, ok bool) {
	tail, found := strings.CutPrefix(r.URL.EscapedPath(), "/api/orgs/")
	if !found {
		return "", nil, false
	}
	parts := strings.Split(strings.TrimSuffix(tail, "/"), "/")
	for i, p := range parts {
		u, err := url.PathUnescape(p)
		if err != nil {
			return "", nil, false
		}
		parts[i] = u
	}
	if parts[0] == "" {
		return "", nil, false
	}
	return parts[0], parts[1:], true
}

func reading(r *http.Request) bool { return r.Method == http.MethodGet || r.Method == http.MethodHead }

// linkReaches: may a link (to the whole organisation, or to one plan) reach this path at all, reading or not.
func linkReaches(l store.Link, rest []string) bool {
	if len(rest) > 0 && (rest[0] == "links" || rest[0] == "members") {
		return false
	}
	if l.Plan != "" && len(rest) > 0 && rest[0] == "plans" {
		return len(rest) >= 2 && rest[1] == l.Plan
	}
	return true
}

// linkAllows: may the link make this request. The organisation itself (renaming, deleting) is never changed by a link.
func linkAllows(l store.Link, r *http.Request, rest []string) bool {
	if !linkReaches(l, rest) {
		return false
	}
	if reading(r) {
		return true
	}
	return l.Level == store.LevelEdit && len(rest) > 0
}

// ownerOnly: requests only the owner may make. Leaving (removing yourself) is for everyone.
func ownerOnly(r *http.Request, rest []string, me int64) bool {
	switch {
	case len(rest) == 0:
		return r.Method == http.MethodDelete
	case rest[0] == "links":
		return true
	case rest[0] == "members" && len(rest) == 2 && !reading(r):
		return !(r.Method == http.MethodDelete && rest[1] == strconv.FormatInt(me, 10))
	}
	return false
}

const linksCookie = "dutyplan-links"

// linkTokens are the tokens of the links opened in this browser (newest first).
func (s *server) linkTokens(r *http.Request) []string {
	c, err := r.Cookie(s.auth.cookieName(linksCookie))
	if err != nil || c.Value == "" {
		return nil
	}
	var out []string
	for _, t := range strings.Split(c.Value, ".") {
		if t != "" && len(t) <= 64 && len(out) < maxLinks {
			out = append(out, t)
		}
	}
	return out
}

const maxLinks = 20

// links are the links opened in this browser that still work.
func (s *server) links(r *http.Request) []store.Link {
	ls, err := s.st.LinksByTokens(r.Context(), s.linkTokens(r))
	if err != nil {
		s.log.Error("links", "err", err)
		return nil
	}
	return slices.DeleteFunc(ls, func(l store.Link) bool { return l.Level == store.LevelJoin })
}

func (s *server) access(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.auth == nil {
			next.ServeHTTP(w, r)
			return
		}
		u, signedIn := s.auth.user(r)
		if signedIn {
			r = r.WithContext(context.WithValue(r.Context(), userKey, u))
		}
		p := r.URL.Path
		if !strings.HasPrefix(p, "/api/") || p == "/api/health" || p == "/api/me" {
			next.ServeHTTP(w, r)
			return
		}
		signedOut := func() { writeJSON(w, 401, apiError{Error: "signed_out", Message: "sign in first"}) }
		org, rest, ok := orgPath(r)
		if !ok {
			// the organisation list is for anyone signed in or with a link; making one needs an account
			if !signedIn && (r.Method != http.MethodGet || len(s.links(r)) == 0) {
				signedOut()
				return
			}
			next.ServeHTTP(w, r)
			return
		}
		g := grant{}
		if signedIn {
			role, err := s.st.Role(r.Context(), org, u.ID)
			if err != nil {
				writeErr(w, s.log, err)
				return
			}
			g.role = role
		}
		if g.role != store.RoleNone {
			if g.role != store.RoleOwner && ownerOnly(r, rest, u.ID) {
				writeJSON(w, 403, apiError{Error: "forbidden", Code: "owner_only", Message: "only the organisation's owner may do this"})
				return
			}
		} else {
			var reach *store.Link
			var allow *store.Link
			for _, l := range s.links(r) {
				if l.Org != org {
					continue
				}
				if linkAllows(l, r, rest) && (allow == nil || wider(l, *allow)) {
					allow = &l
				}
				if linkReaches(l, rest) {
					reach = &l
				}
			}
			switch {
			case allow != nil:
				g.level, g.plan = allow.Level, allow.Plan
			case reach != nil:
				writeJSON(w, 403, apiError{Error: "forbidden", Code: "read_only", Message: "this link only lets you look"})
				return
			case !signedIn:
				signedOut()
				return
			default:
				writeErr(w, s.log, store.ErrNotFound)
				return
			}
		}
		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), grantKey, g)))
	})
}

// wider: a link to the whole organisation before one to a plan, and edit before view.
func wider(a, b store.Link) bool {
	if (a.Plan == "") != (b.Plan == "") {
		return a.Plan == ""
	}
	return a.Level == store.LevelEdit && b.Level != store.LevelEdit
}

// sees: may a stream with this grant hear of a change (a link to one plan hears nothing of the other plans).
func (g grant) sees(org string, c change) bool {
	if g.plan == "" || c.Path == "" {
		return true
	}
	rest, ok := strings.CutPrefix(c.Path, "/orgs/"+org+"/plans/")
	if !ok {
		return true
	}
	plan, _, _ := strings.Cut(rest, "/")
	return plan == g.plan
}

// ---- what the page asks ----------------------------------------------------------------------------------------

// me says whether there is a login, who is signed in, and which links are open in this browser: {"login": false}
// without a login, else {"login": true, "user": {"name", "email"} or null, "links": [...], "signup": address if set}.
func (s *server) me(w http.ResponseWriter, r *http.Request) {
	if s.auth == nil {
		writeJSON(w, 200, map[string]any{"login": false})
		return
	}
	var user *store.User
	if u, ok := userFrom(r); ok {
		user = &u
	}
	links := s.links(r)
	for i := range links {
		links[i].Token, links[i].CreatedBy, links[i].ID = "", "", 0
	}
	out := map[string]any{"login": true, "user": user, "links": links}
	if s.auth.opt.SignupURL != "" {
		out["signup"] = s.auth.opt.SignupURL
	}
	writeJSON(w, 200, out)
}

// openLink is the address of a link: GET /s/<token>. A view or edit link is remembered in this browser (a cookie) and
// opens its organisation or plan; an invitation makes the person signed in an editor (after signing in, when needed).
func (s *server) openLink(w http.ResponseWriter, r *http.Request) {
	token := r.PathValue("token")
	ls, err := s.st.LinksByTokens(r.Context(), []string{token})
	if err != nil || len(ls) == 0 {
		http.Redirect(w, r, "/?link=invalid", http.StatusSeeOther)
		return
	}
	l := ls[0]
	if l.Level == store.LevelJoin {
		u, ok := s.auth.user(r)
		if !ok {
			http.Redirect(w, r, "/auth/login?next="+url.QueryEscape("/s/"+token), http.StatusSeeOther)
			return
		}
		o, err := s.st.Join(r.Context(), token, u.ID)
		if err != nil {
			http.Redirect(w, r, "/?link=invalid", http.StatusSeeOther)
			return
		}
		s.hub.publish(o.Slug, change{Method: "POST", Path: "/orgs/" + o.Slug + "/members"})
		http.Redirect(w, r, "/o/"+url.PathEscape(o.Slug), http.StatusSeeOther)
		return
	}
	tokens := []string{token}
	for _, t := range s.linkTokens(r) {
		if t != token && len(tokens) < maxLinks {
			tokens = append(tokens, t)
		}
	}
	http.SetCookie(w, s.auth.cookie(linksCookie, strings.Join(tokens, "."), 400*24*time.Hour))
	to := "/o/" + url.PathEscape(l.Org)
	if l.Plan != "" {
		to += "/p/" + url.PathEscape(l.Plan)
	}
	http.Redirect(w, r, to, http.StatusSeeOther)
}

// ---- links and members (links.go in the store) -------------------------------------------------------------------

func (s *server) linkRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/orgs/{org}/links", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.st.ListLinks(r.Context(), r.PathValue("org"))
		if err != nil {
			writeErr(w, s.log, err)
			return
		}
		writeJSON(w, 200, v)
	})
	mux.HandleFunc("POST /api/orgs/{org}/links", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Plan        string `json:"plan"`
			Level       string `json:"level"`
			Label       string `json:"label"`
			ExpiresDays int    `json:"expires_days"`
		}
		if !decode(w, r, &b) {
			return
		}
		var by int64
		if u, ok := userFrom(r); ok {
			by = u.ID
		}
		l, err := s.st.CreateLink(r.Context(), r.PathValue("org"), b.Plan, b.Level, b.Label, b.ExpiresDays, by)
		if err != nil {
			writeErr(w, s.log, err)
			return
		}
		writeJSON(w, 201, l)
	})
	mux.HandleFunc("DELETE /api/orgs/{org}/links/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, s.log, store.ErrNotFound)
			return
		}
		noContent(w, s.st.DeleteLink(r.Context(), r.PathValue("org"), id), s)
	})

	mux.HandleFunc("GET /api/orgs/{org}/members", func(w http.ResponseWriter, r *http.Request) {
		v, err := s.st.Members(r.Context(), r.PathValue("org"))
		if err != nil {
			writeErr(w, s.log, err)
			return
		}
		type member struct {
			store.Member
			You bool `json:"you,omitempty"`
		}
		u, _ := userFrom(r)
		out := make([]member, len(v))
		for i, m := range v {
			out[i] = member{m, m.ID == u.ID}
		}
		writeJSON(w, 200, out)
	})
	mux.HandleFunc("PATCH /api/orgs/{org}/members/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, s.log, store.ErrNotFound)
			return
		}
		var b struct {
			Role string `json:"role"`
		}
		if !decode(w, r, &b) {
			return
		}
		noContent(w, s.st.SetRole(r.Context(), r.PathValue("org"), id, b.Role), s)
	})
	mux.HandleFunc("DELETE /api/orgs/{org}/members/{id}", func(w http.ResponseWriter, r *http.Request) {
		id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
		if err != nil {
			writeErr(w, s.log, store.ErrNotFound)
			return
		}
		noContent(w, s.st.RemoveMember(r.Context(), r.PathValue("org"), id), s)
	})
}
