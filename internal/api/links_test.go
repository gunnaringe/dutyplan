package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// open follows a link (/s/<token>) the way a browser does, and returns where it went and the cookie it was given.
func (e *loginEnv) open(token string, cookies ...*http.Cookie) (string, *http.Cookie) {
	e.t.Helper()
	req := httptest.NewRequest("GET", "/s/"+token, nil)
	for _, c := range cookies {
		if c != nil {
			req.AddCookie(c)
		}
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	if w.Code != 303 {
		e.t.Fatalf("open link: %d", w.Code)
	}
	for _, c := range cookiesOf(w.Header()) {
		if c.Name == "__Host-dutyplan-links" {
			return w.Header().Get("Location"), c
		}
	}
	return w.Header().Get("Location"), nil
}

// asMany sends a request with several cookies.
func (e *loginEnv) asMany(cs []*http.Cookie, method, path string, body any) reply {
	req := httptest.NewRequest(method, path, nil)
	if body != nil {
		req = httptest.NewRequest(method, path, strings.NewReader(mustJSON(body)))
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cs {
		if c != nil {
			req.AddCookie(c)
		}
	}
	w := httptest.NewRecorder()
	e.h.ServeHTTP(w, req)
	return reply{w.Code, w.Body.Bytes(), w.Header()}
}

// sharedOrg: kari owns «Choir» with plans «Autumn» and «Spring».
func sharedOrg(t *testing.T) (*loginEnv, *http.Cookie) {
	e := newLoginEnv(t)
	kari := e.signIn("kari", "/")
	for _, req := range []struct{ path, name string }{{"/api/orgs", "Choir"}, {"/api/orgs/choir/plans", "Autumn"}, {"/api/orgs/choir/plans", "Spring"}} {
		if r := e.as(kari, "POST", req.path, map[string]string{"name": req.name}); r.code != 201 {
			t.Fatalf("POST %s: %d %s", req.path, r.code, r.body)
		}
	}
	return e, kari
}

func (e *loginEnv) link(owner *http.Cookie, plan, level string) store.Link {
	e.t.Helper()
	var l store.Link
	r := e.as(owner, "POST", "/api/orgs/choir/links", map[string]any{"plan": plan, "level": level, "label": "test"})
	if r.code != 201 {
		e.t.Fatalf("create link: %d %s", r.code, r.body)
	}
	r.json(&l)
	return l
}

func TestAViewLinkToAPlanReadsButNeverWrites(t *testing.T) {
	e, kari := sharedOrg(t)
	l := e.link(kari, "autumn", "view")
	to, c := e.open(l.Token)
	if to != "/o/choir/p/autumn" || c == nil || !c.Secure || !c.HttpOnly {
		t.Fatalf("opening the link: to %q, cookie %+v", to, c)
	}
	for _, path := range []string{"/api/orgs", "/api/orgs/choir", "/api/orgs/choir/plans/autumn", "/api/orgs/choir/plans/autumn/events", "/api/orgs/choir/people"} {
		if r := e.as(c, "GET", path, nil); r.code != 200 {
			t.Errorf("GET %s with a view link: %d", path, r.code)
		}
	}
	for _, req := range [][2]string{{"POST", "/api/orgs/choir/people"}, {"PATCH", "/api/orgs/choir/plans/autumn"}, {"PATCH", "/api/orgs/choir"},
		{"PUT", "/api/orgs/choir/plans/autumn/answers"}, {"PUT", "/api/orgs/choir/plans/autumn/places"}, {"PUT", "/api/orgs/choir/plans/autumn/events/1/notes/1"}} {
		if r := e.as(c, req[0], req[1], map[string]string{"name": "X"}); r.code != 403 {
			t.Errorf("%s %s with a view link: %d, want 403", req[0], req[1], r.code)
		}
	}
	// not the members or the links, and not another organisation
	for _, path := range []string{"/api/orgs/choir/members", "/api/orgs/choir/links", "/api/orgs/other/people"} {
		if r := e.as(c, "GET", path, nil); r.code != 401 && r.code != 404 {
			t.Errorf("GET %s with a view link: %d, want 401/404", path, r.code)
		}
	}
	var me struct{ Links []store.Link }
	e.as(c, "GET", "/api/me", nil).json(&me)
	if len(me.Links) != 1 || me.Links[0].Org != "choir" || me.Links[0].Level != "view" || me.Links[0].Token != "" {
		t.Fatalf("me with a link: %+v", me.Links)
	}
}

func TestAViewOrEditLinkIsToOnePlan(t *testing.T) {
	e, kari := sharedOrg(t)
	for _, level := range []string{"view", "edit"} {
		if r := e.as(kari, "POST", "/api/orgs/choir/links", map[string]any{"level": level}); r.code != 400 || r.errCode() != "invalid" {
			t.Errorf("a %s link to the whole organisation: %d %s", level, r.code, r.body)
		}
	}
	if r := e.as(kari, "POST", "/api/orgs/choir/links", map[string]any{"level": "join", "plan": "autumn"}); r.code != 400 {
		t.Errorf("an invitation to a plan: %d", r.code)
	}
}

func TestAnEditLinkToAPlanReachesThatPlanOnly(t *testing.T) {
	e, kari := sharedOrg(t)
	l := e.link(kari, "autumn", "edit")
	to, c := e.open(l.Token)
	if to != "/o/choir/p/autumn" {
		t.Fatalf("to %q", to)
	}
	ok := [][2]string{{"GET", "/api/orgs/choir"}, {"GET", "/api/orgs/choir/plans/autumn"}, {"GET", "/api/orgs/choir/people"}, {"GET", "/api/orgs/choir/tasks"}}
	for _, req := range ok {
		if r := e.as(c, req[0], req[1], nil); r.code != 200 {
			t.Errorf("%s %s: %d", req[0], req[1], r.code)
		}
	}
	if r := e.as(c, "POST", "/api/orgs/choir/people", map[string]string{"name": "Aslaug"}); r.code != 201 {
		t.Errorf("adding a person from the plan's table: %d %s", r.code, r.body)
	}
	if r := e.as(c, "PATCH", "/api/orgs/choir/plans/autumn", map[string]string{"name": "Autumn 2026"}); r.code != 200 {
		t.Errorf("renaming its plan: %d", r.code)
	}
	// what web/llms.txt tells an agent with an edit link to do: tasks, details rows, events, places
	if r := e.as(c, "POST", "/api/orgs/choir/tasks", map[string]string{"name": "Coffee"}); r.code != 201 {
		t.Errorf("adding a task: %d %s", r.code, r.body)
	}
	if r := e.as(c, "POST", "/api/orgs/choir/plans/autumn/fields", map[string]string{"name": "Theme"}); r.code != 201 {
		t.Errorf("adding a details row: %d %s", r.code, r.body)
	}
	if r := e.as(c, "POST", "/api/orgs/choir/plans/autumn/events", map[string]string{"date": "2031-01-12", "start_time": "11:00"}); r.code != 201 || !strings.HasPrefix(string(r.body), "[") {
		t.Errorf("adding an event: %d %s (the guide says it answers with a list)", r.code, r.body)
	}
	for _, req := range [][2]string{{"GET", "/api/orgs/choir/plans"}, {"POST", "/api/orgs/choir/plans"}, {"GET", "/api/orgs/choir/plans/spring"},
		{"PATCH", "/api/orgs/choir/plans/spring"}, {"PUT", "/api/orgs/choir/plans/spring/answers"}, {"PUT", "/api/orgs/choir/plans/spring/places"}, {"GET", "/api/orgs/choir/links"}, {"POST", "/api/orgs/choir/links"}, {"PATCH", "/api/orgs/choir"}, {"DELETE", "/api/orgs/choir?confirm=choir"}} {
		if r := e.as(c, req[0], req[1], map[string]string{"name": "X"}); r.code < 400 {
			t.Errorf("%s %s with a link to another plan: %d", req[0], req[1], r.code)
		}
	}
	// the stream hears nothing of the other plan
	g := grant{level: "edit", plan: "autumn"}
	if g.sees("choir", change{Path: "/orgs/choir/plans/spring/events"}) || !g.sees("choir", change{Path: "/orgs/choir/plans/autumn/events"}) ||
		!g.sees("choir", change{Path: "/orgs/choir/people"}) || !g.sees("choir", change{}) {
		t.Fatal("a plan link's stream hears the wrong changes")
	}
}

func TestADeletedOrRunOutLinkStopsWorking(t *testing.T) {
	e, kari := sharedOrg(t)
	l := e.link(kari, "autumn", "edit")
	_, c := e.open(l.Token)
	if r := e.as(kari, "DELETE", "/api/orgs/choir/links/"+itoa(l.ID), nil); r.code != 204 {
		t.Fatalf("delete: %d", r.code)
	}
	if r := e.as(c, "GET", "/api/orgs/choir", nil); r.code != 401 {
		t.Fatalf("a deleted link still works: %d", r.code)
	}
	l = e.link(kari, "autumn", "edit")
	_, c = e.open(l.Token)
	e.st.DB().Exec(`UPDATE share_link SET expires_at = '2000-01-01T00:00:00Z'`)
	if r := e.as(c, "GET", "/api/orgs/choir", nil); r.code != 401 {
		t.Fatalf("a link that ran out still works: %d", r.code)
	}
	if to, _ := e.open("not-a-token"); to != "/?link=invalid" {
		t.Fatalf("an unknown link went to %q", to)
	}
}

func TestAdminsChangeMembersAndMembersSharePlans(t *testing.T) {
	e, kari := sharedOrg(t)
	invite := e.link(kari, "", "join")
	// opening an invitation without signing in goes to a page that names the organisation and signs in back to the invitation
	if to, _ := e.open(invite.Token); to != "/?invite="+invite.Token+"&org=Choir" {
		t.Fatalf("an invitation without a session: to %q", to)
	}
	ola := e.signIn("ola", "/")
	if to, _ := e.open(invite.Token, ola); to != "/o/choir?joined" {
		t.Fatalf("joining: to %q", to)
	}
	var members []struct {
		store.Member
		You bool
	}
	e.as(ola, "GET", "/api/orgs/choir/members", nil).json(&members)
	if len(members) != 2 || members[0].Role != "admin" || members[1].Role != "member" || !members[1].You {
		t.Fatalf("members: %+v", members)
	}
	kariID, olaID := itoa(members[0].ID), itoa(members[1].ID)
	// a member shares plans: makes, lists and deletes plan links
	pl := e.link(ola, "autumn", "edit")
	if r := e.as(ola, "GET", "/api/orgs/choir/links", nil); r.code != 200 {
		t.Errorf("a member lists the links: %d", r.code)
	}
	if r := e.as(ola, "DELETE", "/api/orgs/choir/links/"+itoa(pl.ID), nil); r.code != 204 {
		t.Errorf("a member deletes a plan link: %d", r.code)
	}
	// but invitations and members are the admin's
	if r := e.as(ola, "POST", "/api/orgs/choir/links", map[string]string{"level": "join"}); r.code != 403 {
		t.Errorf("a member invites: %d, want 403", r.code)
	}
	if r := e.as(ola, "DELETE", "/api/orgs/choir/links/"+itoa(invite.ID), nil); r.code != 403 {
		t.Errorf("a member deletes an invitation: %d, want 403", r.code)
	}
	for _, req := range [][2]string{{"PATCH", "/api/orgs/choir/members/" + kariID}, {"DELETE", "/api/orgs/choir/members/" + kariID}} {
		if r := e.as(ola, req[0], req[1], map[string]string{"role": "member"}); r.code != 403 {
			t.Errorf("%s %s by a member: %d, want 403", req[0], req[1], r.code)
		}
	}
	// the last admin stays
	if r := e.as(kari, "PATCH", "/api/orgs/choir/members/"+kariID, map[string]string{"role": "member"}); r.code != 409 {
		t.Fatalf("the last admin made member: %d", r.code)
	}
	if r := e.as(kari, "PATCH", "/api/orgs/choir/members/"+olaID, map[string]string{"role": "admin"}); r.code != 204 {
		t.Fatalf("make ola admin: %d %s", r.code, r.body)
	}
	if r := e.as(kari, "DELETE", "/api/orgs/choir/members/"+kariID, nil); r.code != 204 {
		t.Fatalf("kari leaves: %d", r.code)
	}
	if r := e.as(kari, "GET", "/api/orgs/choir", nil); r.code != 404 {
		t.Fatalf("after leaving: %d", r.code)
	}
	// a member may leave too
	l := e.link(ola, "", "join")
	e.open(l.Token, kari)
	if r := e.as(kari, "DELETE", "/api/orgs/choir/members/"+kariID, nil); r.code != 204 {
		t.Fatalf("a member leaves: %d %s", r.code, r.body)
	}
}

func TestAMemberKeepsTheirRoleWhenOpeningAnInvitation(t *testing.T) {
	e, kari := sharedOrg(t)
	invite := e.link(kari, "", "join")
	e.open(invite.Token, kari)
	if r := e.as(kari, "DELETE", "/api/orgs/choir/plans/spring", nil); r.code != 204 {
		t.Fatalf("the admin is still admin: %d", r.code)
	}
	var role string
	e.st.DB().QueryRow(`SELECT role FROM membership`).Scan(&role)
	if role != "admin" {
		t.Fatalf("role %q", role)
	}
}

func TestLinksPileUpInTheBrowserAndAllWork(t *testing.T) {
	e, kari := sharedOrg(t)
	a, b := e.link(kari, "autumn", "view"), e.link(kari, "spring", "edit")
	_, c := e.open(a.Token)
	_, c = e.open(b.Token, c)
	if !strings.Contains(c.Value, a.Token) || !strings.Contains(c.Value, b.Token) {
		t.Fatalf("the cookie forgot a link: %q", c.Value)
	}
	if r := e.asMany([]*http.Cookie{c}, "GET", "/api/orgs/choir/plans/autumn", nil); r.code != 200 {
		t.Errorf("the first link: %d", r.code)
	}
	if r := e.asMany([]*http.Cookie{c}, "PATCH", "/api/orgs/choir/plans/spring", map[string]string{"name": "Spring 2027"}); r.code != 200 {
		t.Errorf("the second link (edit): %d", r.code)
	}
	if r := e.asMany([]*http.Cookie{c}, "PATCH", "/api/orgs/choir/plans/autumn", map[string]string{"name": "X"}); r.code != 403 {
		t.Errorf("the view link writes: %d", r.code)
	}
	var orgs []store.Org
	e.asMany([]*http.Cookie{c}, "GET", "/api/orgs", nil).json(&orgs)
	if len(orgs) != 1 || orgs[0].Slug != "choir" {
		t.Fatalf("orgs with links: %+v", orgs)
	}
}

func mustJSON(v any) string { b, _ := json.Marshal(v); return string(b) }

func TestALinkOpensItsOwnOrganisationOnly(t *testing.T) {
	e, kari := sharedOrg(t)
	ola := e.signIn("ola", "/")
	if r := e.as(ola, "POST", "/api/orgs", map[string]string{"name": "Band"}); r.code != 201 {
		t.Fatalf("band: %d", r.code)
	}
	_, c := e.open(e.link(kari, "autumn", "edit").Token)
	for _, req := range [][2]string{{"GET", "/api/orgs/band"}, {"GET", "/api/orgs/band/people"}, {"POST", "/api/orgs/band/people"}} {
		if r := e.as(c, req[0], req[1], map[string]string{"name": "X"}); r.code != 401 {
			t.Errorf("%s %s with a link to another organisation: %d, want 401", req[0], req[1], r.code)
		}
	}
}

func TestDeletingAPlanEndsItsLinks(t *testing.T) {
	e, kari := sharedOrg(t)
	l := e.link(kari, "spring", "view")
	_, c := e.open(l.Token)
	if r := e.as(kari, "DELETE", "/api/orgs/choir/plans/spring", nil); r.code != 204 {
		t.Fatalf("delete a plan with a link: %d %s", r.code, r.body)
	}
	if r := e.as(c, "GET", "/api/orgs/choir", nil); r.code != 401 {
		t.Fatalf("the link to the deleted plan: %d", r.code)
	}
}

// An invitation for one person ends when someone joins with it; a member opening it uses nothing up. One for anyone stays.
func TestAnInvitationForOnePersonEndsWhenSomeoneJoins(t *testing.T) {
	e, kari := sharedOrg(t)
	var once, many store.Link
	if r := e.as(kari, "POST", "/api/orgs/choir/links", map[string]any{"level": "join", "label": "Ola", "single_use": true}); r.code != 201 {
		t.Fatalf("create: %d %s", r.code, r.body)
	} else {
		r.json(&once)
	}
	if !once.SingleUse {
		t.Fatalf("the invitation does not say it is for one person: %+v", once)
	}
	e.as(kari, "POST", "/api/orgs/choir/links", map[string]any{"level": "join", "label": "anyone"}).json(&many)
	if r := e.as(kari, "POST", "/api/orgs/choir/links", map[string]any{"level": "view", "plan": "autumn", "single_use": true}); r.code != 400 || !strings.Contains(string(r.body), "link_single_use") {
		t.Fatalf("a view link for one person = %d %s, want 400 link_single_use", r.code, r.body)
	}
	e.open(once.Token, kari) // the admin is a member already: nothing is used up
	ola := e.signIn("ola", "/")
	if to, _ := e.open(once.Token, ola); to != "/o/choir?joined" {
		t.Fatalf("Ola joins: to %q", to)
	}
	per := e.signIn("per", "/")
	if to, _ := e.open(once.Token, per); to != "/?link=invalid" {
		t.Fatalf("a second person with the same invitation: to %q, want it ended", to)
	}
	for _, who := range []*http.Cookie{per, ola} {
		if to, _ := e.open(many.Token, who); to != "/o/choir?joined" {
			t.Fatalf("an invitation for anyone works again: to %q", to)
		}
	}
	var left []store.Link
	e.as(kari, "GET", "/api/orgs/choir/links", nil).json(&left)
	if len(left) != 1 || left[0].ID != many.ID {
		t.Fatalf("links left: %+v", left)
	}
}
