package api

import (
	"fmt"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

type prefIDs struct {
	person, lyd, lovsang int64
	events               []int64
}

func prefSetup(t *testing.T, e *env, org string) prefIDs {
	t.Helper()
	var p store.Person
	var lyd, lov store.Task
	var dt store.DayType
	var ev store.Event
	e.mustDo(201, "POST", "/api/orgs/"+org+"/people", map[string]string{"name": "Kari Test"}).json(&p)
	e.mustDo(201, "POST", "/api/orgs/"+org+"/tasks", map[string]any{"name": "Lyd"}).json(&lyd)
	e.mustDo(201, "POST", "/api/orgs/"+org+"/tasks", map[string]any{"name": "Lovsang", "planned": false}).json(&lov)
	e.mustDo(201, "POST", "/api/orgs/"+org+"/plans", map[string]string{"name": "Høst 2026"})
	e.mustDo(201, "POST", "/api/orgs/"+org+"/day-types", map[string]string{"name": "Gudstjeneste"}).json(&dt)
	ids := prefIDs{person: p.ID, lyd: lyd.ID, lovsang: lov.ID}
	for _, d := range []string{"2026-08-09", "2026-08-16"} {
		e.mustDo(201, "POST", "/api/orgs/"+org+"/plans/host-2026/events", map[string]any{"date": d, "day_type_id": dt.ID}).json(&ev)
		ids.events = append(ids.events, ev.ID)
	}
	return ids
}

func TestPersonPageFlow(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	ids := prefSetup(t, e, "org-a")
	me := fmt.Sprintf("/api/orgs/org-a/plans/host-2026/people/%d/me", ids.person)

	var m store.Me
	e.mustDo(200, "GET", me, nil).json(&m)
	if m.Person.Name != "Kari Test" || len(m.Tasks) != 1 || m.Tasks[0].Level != "" || len(m.Events) != 2 {
		t.Fatalf("me before: %+v", m)
	}
	e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/people/%d/tasks/%d", ids.person, ids.lyd), map[string]string{"level": "godt"})
	e.mustDo(204, "PUT", fmt.Sprintf("/api/orgs/org-a/plans/host-2026/people/%d/availability", ids.person),
		map[string]any{"levels": map[string]string{fmt.Sprint(ids.events[0]): "ikke", fmt.Sprint(ids.events[1]): "godt"}})
	e.mustDo(200, "GET", me, nil).json(&m)
	if m.Tasks[0].Level != "godt" || m.Events[0].Level != "ikke" || m.Events[1].Level != "godt" {
		t.Errorf("me after: %+v", m)
	}
	e.mustDo(204, "DELETE", fmt.Sprintf("/api/orgs/org-a/people/%d/tasks/%d", ids.person, ids.lyd), nil)
	e.mustDo(200, "GET", me, nil).json(&m)
	if m.Tasks[0].Level != "" {
		t.Errorf("task removed: %+v", m.Tasks)
	}
}

func TestPersonPageErrorsAndIsolation(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	a := prefSetup(t, e, "org-a")
	b := prefSetup(t, e, "org-b")
	task := func(org string, person, task int64) string {
		return fmt.Sprintf("/api/orgs/%s/people/%d/tasks/%d", org, person, task)
	}
	avail := func(org string, person int64) string {
		return fmt.Sprintf("/api/orgs/%s/plans/host-2026/people/%d/availability", org, person)
	}
	notFound := func(method, path string, body any) {
		t.Helper()
		if r := e.do(method, path, "T", body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404", method, path, r.code, r.body)
		}
	}
	notFound("GET", fmt.Sprintf("/api/orgs/org-b/plans/host-2026/people/%d/me", a.person), nil)
	notFound("PUT", task("org-b", a.person, b.lyd), map[string]string{"level": "godt"})
	notFound("PUT", task("org-b", b.person, a.lyd), map[string]string{"level": "godt"})
	notFound("PUT", task("org-b", b.person, b.lovsang), map[string]string{"level": "godt"}) // manual-only task
	notFound("DELETE", task("org-b", b.person, b.lyd), nil)                                 // not chosen
	notFound("PUT", avail("org-b", b.person), map[string]any{"levels": map[string]string{fmt.Sprint(a.events[0]): "ikke"}})
	notFound("PUT", avail("org-b", a.person), map[string]any{"levels": map[string]string{fmt.Sprint(b.events[0]): "ikke"}})
	notFound("PUT", task("org-a", a.person, 99999), map[string]string{"level": "godt"})
	notFound("PUT", "/api/orgs/org-a/people/abc/tasks/1", map[string]string{"level": "godt"})

	for name, c := range map[string]struct {
		path string
		body any
	}{
		"bad task level":  {task("org-a", a.person, a.lyd), map[string]string{"level": "kanskje"}},
		"bad event level": {avail("org-a", a.person), map[string]any{"levels": map[string]string{fmt.Sprint(a.events[0]): "noytral"}}},
		"bad event id":    {avail("org-a", a.person), map[string]any{"levels": map[string]string{"x": "ikke"}}},
		"no events":       {avail("org-a", a.person), map[string]any{"levels": map[string]string{}}},
	} {
		if r := e.do("PUT", c.path, "T", c.body); r.code != 400 || r.errCode() != "invalid" {
			t.Errorf("%s: %d %s", name, r.code, r.body)
		}
	}
	if r := e.do("PUT", task("org-a", a.person, a.lyd), "", map[string]string{"level": "godt"}); r.code != 400 || r.errCode() != "actor_required" {
		t.Errorf("without actor: %d %s", r.code, r.body)
	}
	// nothing was stored by any refused call
	var m store.Me
	e.mustDo(200, "GET", fmt.Sprintf("/api/orgs/org-a/plans/host-2026/people/%d/me", a.person), nil).json(&m)
	if m.Tasks[0].Level != "" || m.Events[0].Level != "" || m.Events[1].Level != "" {
		t.Errorf("refused writes must change nothing: %+v", m)
	}
}
