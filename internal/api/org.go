package api

import (
	"context"
	"slices"

	"connectrpc.com/connect"
	"google.golang.org/protobuf/proto"

	v1 "github.com/gunnaringe/dutyplan/gen/dutyplan/v1"
	"github.com/gunnaringe/dutyplan/internal/store"
)

// res wraps an answer.
func res[T any](m *T) (*connect.Response[T], error) { return connect.NewResponse(m), nil }

// done answers an empty message, or the store's error.
func done[T any](s *server, err error) (*connect.Response[T], error) {
	if err != nil {
		return nil, fail(s.log, err)
	}
	return connect.NewResponse(new(T)), nil
}

type protoMsg[T any] interface {
	*T
	proto.Message
}

// conv answers the store's value (through out()), or the store's error as Connect's.
func conv[T any, M protoMsg[T]](s *server, v any, err error) (*connect.Response[T], error) {
	if err != nil {
		return nil, fail(s.log, err)
	}
	m := M(new(T))
	if _, err := out(v, m); err != nil {
		return nil, fail(s.log, err)
	}
	return connect.NewResponse((*T)(m)), nil
}

// ---- the server and you -------------------------------------------------------------------------------------------

type accountSvc struct{ *server }

// Health: can this server plan at all? The page hides «Plan» when not.
func (s accountSvc) Health(ctx context.Context, _ *connect.Request[v1.HealthRequest]) (*connect.Response[v1.HealthResponse], error) {
	return res(&v1.HealthResponse{Ok: true, Planner: s.planner.Available()})
}

// Me says whether there is a login, who is signed in, and which links are open in this browser (without their tokens).
func (s accountSvc) Me(ctx context.Context, _ *connect.Request[v1.MeRequest]) (*connect.Response[v1.MeResponse], error) {
	if s.auth == nil {
		return res(&v1.MeResponse{})
	}
	m := &v1.MeResponse{}
	links := s.links(ctx)
	for i := range links {
		links[i].Token, links[i].CreatedBy, links[i].ID = "", "", 0
	}
	if _, err := out(map[string]any{"links": links}, m); err != nil {
		return nil, fail(s.log, err)
	}
	m.Login = true
	if u, ok := userFrom(ctx); ok {
		m.User = &v1.User{Name: u.Name, Email: u.Email}
	}
	m.Signup = s.auth.opt.SignupURL
	return res(m)
}

// ListOrgs: with a login, the organisations the person is a member of, and those of the links opened in this browser.
func (s accountSvc) ListOrgs(ctx context.Context, _ *connect.Request[v1.ListOrgsRequest]) (*connect.Response[v1.ListOrgsResponse], error) {
	if s.auth == nil {
		orgs, err := s.st.ListOrgs(ctx)
		return conv[v1.ListOrgsResponse](s.server, map[string]any{"orgs": orgs}, err)
	}
	orgs := []store.Org{}
	var err error
	if u, ok := userFrom(ctx); ok {
		orgs, err = s.st.ListOrgsOf(ctx, u.ID)
	}
	for _, l := range s.links(ctx) {
		if !slices.ContainsFunc(orgs, func(o store.Org) bool { return o.Slug == l.Org }) {
			orgs = append(orgs, store.Org{Slug: l.Org, Name: l.OrgName})
		}
	}
	return conv[v1.ListOrgsResponse](s.server, map[string]any{"orgs": orgs}, err)
}

// CreateOrg: the one who makes it owns it.
func (s accountSvc) CreateOrg(ctx context.Context, req *connect.Request[v1.CreateOrgRequest]) (*connect.Response[v1.CreateOrgResponse], error) {
	var o store.Org
	var err error
	if u, ok := userFrom(ctx); ok {
		o, err = s.st.CreateOrgOwnedBy(ctx, req.Msg.Name, u.ID)
	} else {
		o, err = s.st.CreateOrg(ctx, req.Msg.Name)
	}
	return conv[v1.CreateOrgResponse](s.server, map[string]any{"org": o}, err)
}

// ---- the organisation, members and links ---------------------------------------------------------------------------

type orgSvc struct{ *server }

// GetOrg also says what the asker may do there: role admin or member (without a login everyone is admin), or else
// the level of their link and, for a link to one plan, that plan.
func (s orgSvc) GetOrg(ctx context.Context, req *connect.Request[v1.GetOrgRequest]) (*connect.Response[v1.GetOrgResponse], error) {
	o, err := s.st.GetOrg(ctx, req.Msg.Org)
	if err != nil {
		return nil, fail(s.log, err)
	}
	g := grantFrom(ctx)
	if s.auth == nil {
		g.role = store.RoleAdmin
	}
	return res(&v1.GetOrgResponse{Slug: o.Slug, Name: o.Name, CreatedAt: o.CreatedAt, Role: g.role, Level: g.level, Plan: g.plan})
}

// RenameOrg changes the name only; the address stays.
func (s orgSvc) RenameOrg(ctx context.Context, req *connect.Request[v1.RenameOrgRequest]) (*connect.Response[v1.RenameOrgResponse], error) {
	o, err := s.st.RenameOrg(ctx, req.Msg.Org, req.Msg.Name)
	return conv[v1.RenameOrgResponse](s.server, map[string]any{"org": o}, err)
}

// DeleteOrg removes an organisation without plans. The caller confirms with the organisation's slug, so a stray call
// cannot do it. The deletion is written to the server log.
func (s orgSvc) DeleteOrg(ctx context.Context, req *connect.Request[v1.DeleteOrgRequest]) (*connect.Response[v1.DeleteOrgResponse], error) {
	o, err := s.st.DeleteOrg(ctx, req.Msg.Org, req.Msg.Confirm)
	if err != nil {
		return nil, fail(s.log, err)
	}
	s.log.Warn("organisation deleted", "org", o.Slug, "name", o.Name)
	return res(&v1.DeleteOrgResponse{})
}

func (s orgSvc) ListMembers(ctx context.Context, req *connect.Request[v1.ListMembersRequest]) (*connect.Response[v1.ListMembersResponse], error) {
	v, err := s.st.Members(ctx, req.Msg.Org)
	if err != nil {
		return nil, fail(s.log, err)
	}
	type member struct {
		store.Member
		You bool `json:"you"`
	}
	u, _ := userFrom(ctx)
	ms := make([]member, len(v))
	for i, m := range v {
		ms[i] = member{m, m.ID == u.ID}
	}
	return conv[v1.ListMembersResponse](s.server, map[string]any{"members": ms}, nil)
}

func (s orgSvc) SetMemberRole(ctx context.Context, req *connect.Request[v1.SetMemberRoleRequest]) (*connect.Response[v1.SetMemberRoleResponse], error) {
	return done[v1.SetMemberRoleResponse](s.server, s.st.SetRole(ctx, req.Msg.Org, int64(req.Msg.Id), req.Msg.Role))
}

func (s orgSvc) RemoveMember(ctx context.Context, req *connect.Request[v1.RemoveMemberRequest]) (*connect.Response[v1.RemoveMemberResponse], error) {
	return done[v1.RemoveMemberResponse](s.server, s.st.RemoveMember(ctx, req.Msg.Org, int64(req.Msg.Id)))
}

func (s orgSvc) ListLinks(ctx context.Context, req *connect.Request[v1.ListLinksRequest]) (*connect.Response[v1.ListLinksResponse], error) {
	v, err := s.st.ListLinks(ctx, req.Msg.Org)
	return conv[v1.ListLinksResponse](s.server, map[string]any{"links": v}, err)
}

// CreateLink: an invitation (join) only by an admin.
func (s orgSvc) CreateLink(ctx context.Context, req *connect.Request[v1.CreateLinkRequest]) (*connect.Response[v1.CreateLinkResponse], error) {
	b := req.Msg
	if b.Level == store.LevelJoin && !s.admin(ctx) {
		return nil, errAdminOnly
	}
	var by int64
	if u, ok := userFrom(ctx); ok {
		by = u.ID
	}
	l, err := s.st.CreateLink(ctx, b.Org, b.Plan, b.Level, b.Label, int(b.ExpiresDays), b.SingleUse, by)
	return conv[v1.CreateLinkResponse](s.server, map[string]any{"link": l}, err)
}

// DeleteLink: an invitation only by an admin.
func (s orgSvc) DeleteLink(ctx context.Context, req *connect.Request[v1.DeleteLinkRequest]) (*connect.Response[v1.DeleteLinkResponse], error) {
	id := int64(req.Msg.Id)
	if !s.admin(ctx) {
		ls, err := s.st.ListLinks(ctx, req.Msg.Org)
		if err != nil {
			return nil, fail(s.log, err)
		}
		for _, l := range ls {
			if l.ID == id && l.Level == store.LevelJoin {
				return nil, errAdminOnly
			}
		}
	}
	return done[v1.DeleteLinkResponse](s.server, s.st.DeleteLink(ctx, req.Msg.Org, id))
}
