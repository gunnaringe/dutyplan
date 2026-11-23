package api

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestRenamingAndDeletingOverHTTP(t *testing.T) {
	e := newEnv(t)
	var a, b store.Org
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Club A"}).json(&a)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Club B"}).json(&b)
	for _, o := range []store.Org{a, b} {
		e.mustDo(201, "POST", "/api/orgs/"+o.Slug+"/plans", map[string]string{"name": "Autumn 2026"})
	}
	pa, pb := "/api/orgs/"+a.Slug+"/plans/autumn-2026", "/api/orgs/"+b.Slug+"/plans/autumn-2026"

	// rename: the name changes, the address does not
	var o store.Org
	e.mustDo(200, "PATCH", "/api/orgs/"+a.Slug, map[string]string{"name": "Sports Club"}).json(&o)
	if o.Name != "Sports Club" || o.Slug != a.Slug {
		t.Errorf("renamed: %+v", o)
	}
	var p store.Plan
	e.mustDo(200, "PATCH", pa, map[string]string{"name": "Fall"}).json(&p)
	if p.Name != "Fall" || p.Slug != "autumn-2026" {
		t.Errorf("renamed plan: %+v", p)
	}
	e.mustDo(200, "GET", pa, nil).json(&p)
	if p.Name != "Fall" {
		t.Errorf("the old address shows the new name: %+v", p)
	}
	e.mustDo(200, "GET", pb, nil).json(&p)
	if p.Name != "Autumn 2026" {
		t.Errorf("B's plan with the same address was renamed too: %+v", p)
	}
	if got := e.refused(400, "PATCH", pa, map[string]string{"name": "  "}); got.Code != "name_invalid" {
		t.Errorf("an empty name: %+v", got)
	}
	if got := e.refused(400, "PATCH", pa, map[string]string{"name": "Fall"}); got.Code != "nothing_to_change" {
		t.Errorf("the same name: %+v", got)
	}
	if r := e.do("PATCH", pa, map[string]any{"slug": "other"}); r.code != 400 {
		t.Errorf("the address cannot be changed: %d %s", r.code, r.body)
	}
	for _, path := range []string{"/api/orgs/nope", "/api/orgs/" + a.Slug + "/plans/nope", "/api/orgs/nope/plans/autumn-2026"} {
		if r := e.do("PATCH", path, map[string]string{"name": "X"}); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("PATCH %s = %d %s", path, r.code, r.body)
		}
		if r := e.do("DELETE", path, nil); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("DELETE %s = %d %s", path, r.code, r.body)
		}
	}

	// deleting a plan
	e.mustDo(204, "DELETE", pa, nil)
	if r := e.do("GET", pa, nil); r.code != 404 {
		t.Errorf("the deleted plan: %d", r.code)
	}
	e.mustDo(200, "GET", pb, nil)
	var plans []store.Plan
	e.mustDo(200, "GET", "/api/orgs/"+a.Slug+"/plans", nil).json(&plans)
	if len(plans) != 0 {
		t.Errorf("A's plans: %+v", plans)
	}

	// deleting an organisation: the address must be confirmed, and plans must be gone first
	if got := e.refused(400, "DELETE", "/api/orgs/"+b.Slug, nil); got.Code != "confirm_mismatch" {
		t.Errorf("no confirmation: %+v", got)
	}
	if got := e.refused(400, "DELETE", "/api/orgs/"+b.Slug+"?confirm="+a.Slug, nil); got.Code != "confirm_mismatch" {
		t.Errorf("another organisation's address: %+v", got)
	}
	if got := e.refused(409, "DELETE", "/api/orgs/"+b.Slug+"?confirm="+b.Slug, nil); got.Code != "org_has_plans" || got.Params["name"] != "Club B" {
		t.Errorf("an organisation with plans: %+v", got)
	}
	e.mustDo(204, "DELETE", "/api/orgs/"+a.Slug+"?confirm="+a.Slug, nil)
	if r := e.do("GET", "/api/orgs/"+a.Slug, nil); r.code != 404 {
		t.Errorf("the deleted organisation: %d", r.code)
	}
	var orgs []store.Org
	e.mustDo(200, "GET", "/api/orgs", nil).json(&orgs)
	if len(orgs) != 1 || orgs[0].Slug != b.Slug {
		t.Errorf("organisations: %+v", orgs)
	}
	e.mustDo(200, "GET", pb, nil) // B untouched
}

func TestDeletingAnOrganisationIsWrittenToTheServerLog(t *testing.T) {
	var logged bytes.Buffer
	st := newEnv(t).st // a store; the handler below has a log we can read
	h := legacy(New(st, planner.New(st, nil, 1, slog.Default()), nil, slog.New(slog.NewTextHandler(&logged, nil))))
	create := httptest.NewRequest("POST", "/api/orgs", bytes.NewReader([]byte(`{"name":"Gone Club"}`)))
	create.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(httptest.NewRecorder(), create)
	del := httptest.NewRequest("DELETE", "/api/orgs/gone-club?confirm=gone-club", nil)
	w := httptest.NewRecorder()
	h.ServeHTTP(w, del)
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	for _, want := range []string{"organisation deleted", "org=gone-club", `"Gone Club"`} {
		if !bytes.Contains(logged.Bytes(), []byte(want)) {
			t.Errorf("the server log lacks %s: %s", want, logged.String())
		}
	}
}

// A new plan from another one, and a duplicate with its table, over the API; members only (an edit link may not, see
// TestAnEditLinkToAPlanReachesThatPlanOnly).
func TestAPlanIsMadeFromAnotherOrDuplicated(t *testing.T) {
	e := newEnv(t)
	org, plan := e.plannable("Org A") // a task, a person who takes it, an event with a place
	var p store.Plan
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans", map[string]string{"name": "Spring 2027", "from": plan}).json(&p)
	var people []store.Person
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+p.Slug+"/people", nil).json(&people)
	var events []store.Event
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+p.Slug+"/events", nil).json(&events)
	if len(people) != 1 || people[0].Name != "Aslaug" || len(events) != 0 {
		t.Errorf("from autumn: %+v %+v", people, events)
	}
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans", map[string]string{"name": "Summer 2027"}).json(&p)
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+p.Slug+"/people", nil).json(&people)
	if len(people) != 0 {
		t.Errorf("an empty plan: %+v", people)
	}
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/"+plan+"/duplicate", map[string]any{"name": "Autumn 2026 (copy)", "table": true}).json(&p)
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+p.Slug+"/events", nil).json(&events)
	if p.Slug != "autumn-2026-copy" || len(events) != 1 {
		t.Errorf("the duplicate: %+v %+v", p, events)
	}
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/"+plan+"/duplicate", map[string]any{"name": "Autumn bare", "table": false}).json(&p)
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+p.Slug+"/events", nil).json(&events)
	if len(events) != 0 {
		t.Errorf("a duplicate without its table has dates: %+v", events)
	}
	if r := e.do("POST", "/api/orgs/"+org+"/plans", map[string]string{"name": "Winter", "from": "nope"}); r.code != 404 {
		t.Errorf("from a plan that is none: %d", r.code)
	}
}
