package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gunnaringe/tjenesteplan/internal/planner"
	"github.com/gunnaringe/tjenesteplan/internal/store"
)

type fakeSolver func(ctx context.Context, input []byte) ([]byte, error)

func (f fakeSolver) Solve(ctx context.Context, input []byte) ([]byte, error) { return f(ctx, input) }

var emptyAnswer = []byte(`{"status":"OPTIMAL","wall_time":0.1,"assignments":[],"unfilled":[],"load":{},"violations":[]}`)

// plannable: an organisation with one task, a day type with one place, a plan with one event and one willing person.
func (e *env) plannable(name string) (org, plan string) {
	e.t.Helper()
	ctx := context.Background()
	must := func(err error) {
		e.t.Helper()
		if err != nil {
			e.t.Fatal(err)
		}
	}
	o, err := e.st.CreateOrg(ctx, "T", name, "")
	must(err)
	task, err := e.st.CreateTask(ctx, o.Slug, "T", "Lyd", true)
	must(err)
	dt, err := e.st.CreateDayType(ctx, o.Slug, "T", "Gudstjeneste")
	must(err)
	_, err = e.st.SetDayTypeTasks(ctx, o.Slug, "T", dt.ID, []store.DayTypeTask{{TaskID: task.ID, Slots: 1}})
	must(err)
	p, err := e.st.CreatePlan(ctx, o.Slug, "T", "Høst 2026", "")
	must(err)
	_, err = e.st.AddEvent(ctx, o.Slug, p.Slug, "T", "2026-08-09", dt.ID, "")
	must(err)
	person, err := e.st.CreatePerson(ctx, o.Slug, "T", "Aslaug")
	must(err)
	must(e.st.SetPersonTask(ctx, o.Slug, "T", person.ID, task.ID, "noytral"))
	return o.Slug, p.Slug
}

func (e *env) wait() {
	e.t.Helper()
	done := make(chan struct{})
	go func() { e.pl.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		e.t.Fatal("the run did not finish")
	}
}

func TestPlanningFlowAndIsolation(t *testing.T) {
	e := newEnvWith(t, fakeSolver(func(context.Context, []byte) ([]byte, error) { return emptyAnswer, nil }))
	a, planA := e.plannable("Org A")
	b, planB := e.plannable("Org B")
	if planA != planB {
		t.Fatal("test needs the same plan slug in both organisations")
	}
	var h map[string]bool
	e.mustDo(200, "GET", "/api/health", nil).json(&h)
	if !h["planner"] {
		t.Errorf("health must say that planning is possible: %v", h)
	}
	if r := e.do("GET", "/api/orgs/"+a+"/plans/"+planA+"/runs/latest", "", nil); r.code != 404 {
		t.Errorf("no run yet: %d", r.code)
	}

	var started struct{ ID int64 }
	r := e.mustDo(202, "POST", "/api/orgs/"+a+"/plans/"+planA+"/runs", map[string]any{"time_limit": 5})
	r.json(&started)
	e.wait()
	var v planner.RunView
	e.mustDo(200, "GET", "/api/orgs/"+a+"/plans/"+planA+"/runs/"+itoa(started.ID), nil).json(&v)
	if v.Run.Status != "done" || v.Run.Actor != "Test" || len(v.Events) != 1 || len(v.Tasks) != 1 {
		t.Fatalf("run: %+v", v)
	}
	var latest planner.RunView
	e.mustDo(200, "GET", "/api/orgs/"+a+"/plans/"+planA+"/runs/latest", nil).json(&latest)
	if latest.Run.ID != started.ID {
		t.Errorf("latest = %d, want %d", latest.Run.ID, started.ID)
	}
	var list []store.Run
	e.mustDo(200, "GET", "/api/orgs/"+a+"/plans/"+planA+"/runs", nil).json(&list)
	if len(list) != 1 || list[0].ID != started.ID {
		t.Errorf("list: %v", list)
	}

	// B has a plan with the very same slug, but A's run is invisible there, and B's list stays empty
	for _, path := range []string{
		"/api/orgs/" + b + "/plans/" + planB + "/runs/" + itoa(started.ID),
		"/api/orgs/" + b + "/plans/" + planB + "/runs/latest",
		"/api/orgs/ingen/plans/" + planA + "/runs/" + itoa(started.ID),
		"/api/orgs/" + a + "/plans/ingen/runs/" + itoa(started.ID),
		"/api/orgs/" + a + "/plans/" + planA + "/runs/abc",
		"/api/orgs/" + a + "/plans/" + planA + "/runs/99999",
	} {
		if r := e.do("GET", path, "", nil); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("GET %s = %d %s, want 404", path, r.code, r.body)
		}
	}
	var listB []store.Run
	e.mustDo(200, "GET", "/api/orgs/"+b+"/plans/"+planB+"/runs", nil).json(&listB)
	if len(listB) != 0 {
		t.Errorf("B must not see A's runs: %v", listB)
	}
	if r := e.do("POST", "/api/orgs/ingen/plans/"+planA+"/runs", "T", map[string]any{}); r.code != 404 {
		t.Errorf("planning in an unknown organisation: %d", r.code)
	}
}

func TestPlanningRefusals(t *testing.T) {
	release := make(chan struct{})
	e := newEnvWith(t, fakeSolver(func(context.Context, []byte) ([]byte, error) { <-release; return emptyAnswer, nil }))
	org, plan := e.plannable("Org A")
	path := "/api/orgs/" + org + "/plans/" + plan + "/runs"

	if r := e.do("POST", path, "", map[string]any{}); r.code != 400 || r.errCode() != "actor_required" {
		t.Errorf("without actor: %d %s", r.code, r.body)
	}
	if r := e.do("POST", path, "T", map[string]any{"time_limit": 1}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("too short a time limit: %d %s", r.code, r.body)
	}
	if r := e.do("POST", path, "T", map[string]any{"tid": 5}); r.code != 400 {
		t.Errorf("unknown field: %d %s", r.code, r.body)
	}
	empty, _ := e.st.CreatePlan(context.Background(), org, "T", "Tom", "")
	if r := e.do("POST", "/api/orgs/"+org+"/plans/"+empty.Slug+"/runs", "T", map[string]any{}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("a plan without events: %d %s", r.code, r.body)
	}
	if n := len(mustRuns(t, e, org, plan)); n != 0 {
		t.Errorf("refused requests must not leave runs behind: %d", n)
	}

	e.mustDo(202, "POST", path, map[string]any{})
	if r := e.do("POST", path, "T", map[string]any{}); r.code != 409 || r.errCode() != "busy" {
		t.Errorf("a second run while one is going: %d %s", r.code, r.body)
	}
	var v planner.RunView
	e.mustDo(200, "GET", path+"/latest", nil).json(&v)
	if v.Run.Status != "running" || len(v.Events) != 0 {
		t.Errorf("while it runs: %+v", v.Run)
	}
	close(release)
	e.wait()
}

func mustRuns(t *testing.T, e *env, org, plan string) []store.Run {
	t.Helper()
	var l []store.Run
	e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+plan+"/runs", nil).json(&l)
	return l
}

func TestPlanningWithoutASolver(t *testing.T) {
	e := newEnv(t) // no solver configured
	org, plan := e.plannable("Org A")
	var h map[string]bool
	e.mustDo(200, "GET", "/api/health", nil).json(&h)
	if h["planner"] {
		t.Errorf("health must say planning is not possible: %v", h)
	}
	if r := e.do("POST", "/api/orgs/"+org+"/plans/"+plan+"/runs", "T", map[string]any{}); r.code != 503 || r.errCode() != "unavailable" {
		t.Errorf("no solver: %d %s", r.code, r.body)
	}
}

// oneAssignment: a solver that gives the first person the first task on the first event.
func oneAssignment(_ context.Context, input []byte) ([]byte, error) {
	var in struct {
		People []struct{ ID string } `json:"people"`
		Dates  []struct{ ID string } `json:"dates"`
		Tasks  []struct{ ID string } `json:"tasks"`
	}
	if err := json.Unmarshal(input, &in); err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"status": "OPTIMAL", "wall_time": 0.1,
		"assignments": []map[string]any{{"date": in.Dates[0].ID, "task": in.Tasks[0].ID, "person": in.People[0].ID, "locked": false, "task_level": "noytral", "date_level": "noytral"}},
		"unfilled":    []any{}, "load": map[string]any{}, "violations": []string{},
	})
}

func TestMyAssignmentsOverHTTP(t *testing.T) {
	e := newEnvWith(t, fakeSolver(oneAssignment))
	orgA, planA := e.plannable("Org A")
	orgB, planB := e.plannable("Org B")
	if planA != planB {
		t.Fatal("test needs the same plan slug in both organisations")
	}
	peopleA, _ := e.st.ListPeople(context.Background(), orgA)
	peopleB, _ := e.st.ListPeople(context.Background(), orgB)
	annaA, annaB := peopleA[0].ID, peopleB[0].ID
	mine := func(org string, person int64) planner.Mine {
		var m planner.Mine
		e.mustDo(200, "GET", "/api/orgs/"+org+"/plans/"+planA+"/people/"+itoa(person)+"/assignments", nil).json(&m)
		return m
	}
	if m := mine(orgA, annaA); m.HasPlan || len(m.Assignments) != 0 || m.Person.Name != "Aslaug" {
		t.Errorf("before any plan: %+v", m)
	}
	e.mustDo(202, "POST", "/api/orgs/"+orgA+"/plans/"+planA+"/runs", map[string]any{"time_limit": 5})
	e.wait()
	m := mine(orgA, annaA)
	if !m.HasPlan || len(m.Assignments) != 1 || m.Assignments[0].Task != "Lyd" || m.Assignments[0].Date != "2026-08-09" || m.Run == nil || m.Run.Actor != "Test" {
		t.Errorf("Aslaug's assignment: %+v", m)
	}
	// B has a plan with the same slug and a person with the same name, but A's run is not B's plan
	if mb := mine(orgB, annaB); mb.HasPlan || len(mb.Assignments) != 0 {
		t.Errorf("B: %+v", mb)
	}
	for _, c := range []struct{ path, what string }{
		{"/api/orgs/" + orgB + "/plans/" + planA + "/people/" + itoa(annaA) + "/assignments", "A's person through B"},
		{"/api/orgs/" + orgA + "/plans/" + planA + "/people/" + itoa(annaB) + "/assignments", "B's person through A"},
		{"/api/orgs/" + orgA + "/plans/ingen/people/" + itoa(annaA) + "/assignments", "unknown plan"},
		{"/api/orgs/" + orgA + "/plans/" + planA + "/people/abc/assignments", "bad id"},
		{"/api/orgs/" + orgA + "/plans/" + planA + "/people/99999/assignments", "unknown person"},
	} {
		if r := e.do("GET", c.path, "", nil); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s: %d %s", c.what, r.code, r.body)
		}
	}
}
