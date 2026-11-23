package api

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/gunnaringe/dutyplan/internal/planner"
	"github.com/gunnaringe/dutyplan/internal/store"
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
	o, err := e.st.CreateOrg(ctx, name)
	must(err)
	p, err := e.st.CreatePlan(ctx, o.Slug, "Autumn 2026")
	must(err)
	task, err := e.st.CreateTask(ctx, o.Slug, p.Slug, "Sound")
	must(err)
	_, err = addWithPlaces(e.st, o.Slug, p.Slug, store.NewEvents{Date: "2026-08-09"}, map[int64]int{task.ID: 1})
	must(err)
	person, err := e.st.CreatePerson(ctx, o.Slug, p.Slug, "Aslaug")
	must(err)
	must(e.st.SetPersonTask(ctx, o.Slug, p.Slug, person.ID, task.ID, "good"))
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
	if r := e.do("GET", "/api/orgs/"+a+"/plans/"+planA+"/runs/latest", nil); r.code != 404 {
		t.Errorf("no run yet: %d", r.code)
	}

	var started struct{ ID int64 }
	r := e.mustDo(202, "POST", "/api/orgs/"+a+"/plans/"+planA+"/runs", map[string]any{"time_limit": 5})
	r.json(&started)
	e.wait()
	var v planner.RunView
	e.mustDo(200, "GET", "/api/orgs/"+a+"/plans/"+planA+"/runs/"+itoa(started.ID), nil).json(&v)
	if v.Run.Status != "done" || len(v.Events) != 1 || len(v.Tasks) != 1 {
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
		"/api/orgs/nope/plans/" + planA + "/runs/" + itoa(started.ID),
		"/api/orgs/" + a + "/plans/nope/runs/" + itoa(started.ID),
		"/api/orgs/" + a + "/plans/" + planA + "/runs/abc",
		"/api/orgs/" + a + "/plans/" + planA + "/runs/99999",
	} {
		if r := e.do("GET", path, nil); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("GET %s = %d %s, want 404", path, r.code, r.body)
		}
	}
	var listB []store.Run
	e.mustDo(200, "GET", "/api/orgs/"+b+"/plans/"+planB+"/runs", nil).json(&listB)
	if len(listB) != 0 {
		t.Errorf("B must not see A's runs: %v", listB)
	}
	if r := e.do("POST", "/api/orgs/nope/plans/"+planA+"/runs", map[string]any{}); r.code != 404 {
		t.Errorf("planning in an unknown organisation: %d", r.code)
	}
}

func TestPlanningRefusals(t *testing.T) {
	release := make(chan struct{})
	e := newEnvWith(t, fakeSolver(func(context.Context, []byte) ([]byte, error) { <-release; return emptyAnswer, nil }))
	org, plan := e.plannable("Org A")
	path := "/api/orgs/" + org + "/plans/" + plan + "/runs"

	if r := e.do("POST", path, map[string]any{"time_limit": 1}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("too short a time limit: %d %s", r.code, r.body)
	}
	if r := e.do("POST", path, map[string]any{"tid": 5}); r.code != 400 {
		t.Errorf("unknown field: %d %s", r.code, r.body)
	}
	empty, _ := e.st.CreatePlan(context.Background(), org, "Tom")
	if r := e.do("POST", "/api/orgs/"+org+"/plans/"+empty.Slug+"/runs", map[string]any{}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("a plan without events: %d %s", r.code, r.body)
	}
	if n := len(mustRuns(t, e, org, plan)); n != 0 {
		t.Errorf("refused requests must not leave runs behind: %d", n)
	}

	e.mustDo(202, "POST", path, map[string]any{})
	if r := e.do("POST", path, map[string]any{}); r.code != 409 || r.errCode() != "busy" {
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
	if r := e.do("POST", "/api/orgs/"+org+"/plans/"+plan+"/runs", map[string]any{}); r.code != 503 || r.errCode() != "unavailable" {
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
		"assignments": []map[string]any{{"date": in.Dates[0].ID, "task": in.Tasks[0].ID, "person": in.People[0].ID, "locked": false, "task_level": "neutral", "date_level": "neutral"}},
		"unfilled":    []any{}, "load": map[string]any{}, "violations": []string{},
	})
}

func TestPinningARunOverHTTP(t *testing.T) {
	e := newEnvWith(t, fakeSolver(oneAssignment))
	orgA, planA := e.plannable("Org A")
	orgB, planB := e.plannable("Org B")
	runPath := func(org, plan string) string { return "/api/orgs/" + org + "/plans/" + plan + "/runs" }
	var first, second struct{ ID int64 }
	e.mustDo(202, "POST", runPath(orgA, planA), map[string]any{"time_limit": 5}).json(&first)
	e.wait()
	e.mustDo(202, "POST", runPath(orgA, planA), map[string]any{"time_limit": 5}).json(&second)
	e.wait()
	pin := runPath(orgA, planA) + "/" + itoa(first.ID) + "/pin"

	e.mustDo(204, "PUT", pin, nil)
	if r := e.do("PUT", pin, nil); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("pinning twice: %d %s", r.code, r.body)
	}
	list := mustRuns(t, e, orgA, planA)
	if len(list) != 2 || list[0].ID != second.ID || list[0].Pinned || list[1].ID != first.ID || !list[1].Pinned {
		t.Errorf("the list says which run is pinned: %+v", list)
	}
	var v planner.RunView
	e.mustDo(200, "GET", runPath(orgA, planA)+"/"+itoa(first.ID), nil).json(&v)
	if !v.Run.Pinned {
		t.Errorf("a run's view says it is pinned: %+v", v.Run)
	}

	// another organisation's plan with the same slug cannot touch A's runs, and an unknown run is not found
	for _, c := range []struct{ method, path string }{
		{"PUT", runPath(orgB, planB) + "/" + itoa(second.ID) + "/pin"},
		{"DELETE", runPath(orgB, planB) + "/" + itoa(first.ID) + "/pin"},
		{"PUT", runPath(orgA, "nope") + "/" + itoa(second.ID) + "/pin"},
		{"PUT", runPath(orgA, planA) + "/99999/pin"},
		{"PUT", runPath(orgA, planA) + "/abc/pin"},
	} {
		if r := e.do(c.method, c.path, nil); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s", c.method, c.path, r.code, r.body)
		}
	}
	if got := mustRuns(t, e, orgA, planA); !got[1].Pinned || got[0].Pinned {
		t.Errorf("refused requests changed the pin: %+v", got)
	}
	e.mustDo(204, "PUT", runPath(orgA, planA)+"/"+itoa(second.ID)+"/pin", nil)
	if got := mustRuns(t, e, orgA, planA); !got[0].Pinned || got[1].Pinned {
		t.Errorf("pinning the other run moves the pin: %+v", got)
	}
	e.mustDo(204, "DELETE", runPath(orgA, planA)+"/"+itoa(second.ID)+"/pin", nil)
	if r := e.do("DELETE", runPath(orgA, planA)+"/"+itoa(second.ID)+"/pin", nil); r.code != 400 {
		t.Errorf("unpinning twice: %d %s", r.code, r.body)
	}
}
