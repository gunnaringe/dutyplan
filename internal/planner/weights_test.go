package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestWeightsBecomeNestedParamsAndPartOfTheHash(t *testing.T) {
	w := newWorld(t, "Club")
	paramsOf := func(p Params) (map[string]any, string) {
		raw, hash, _, err := BuildInput(w.data(t), p)
		if err != nil {
			t.Fatal(err)
		}
		var in struct {
			Params map[string]any `json:"params"`
		}
		json.Unmarshal(raw, &in)
		return in.Params, hash
	}
	plain, h0 := paramsOf(Params{TimeLimit: 5, Seed: 1, Workers: 1})
	if len(plain) != 3 || plain["time_limit"] != 5.0 {
		t.Errorf("without weights the planner is told only time, seed and workers: %v", plain)
	}
	if _, err := w.st.SetWeights(ctx, w.org.Slug, w.plan.Slug, map[string]any{"w_fair": 12.0, "w_task.good": 3.0, "w_task.poor": -2.0, "w_combo.practical": 4.0}); err != nil {
		t.Fatal(err)
	}
	got, h1 := paramsOf(Params{TimeLimit: 5, Seed: 1, Workers: 1})
	wt, _ := got["w_task"].(map[string]any)
	if got["w_fair"] != 12.0 || wt["good"] != 3.0 || wt["poor"] != -2.0 || len(wt) != 2 || got["w_combo"].(map[string]any)["practical"] != 4.0 || got["time_limit"] != 5.0 {
		t.Errorf("params: %v", got)
	}
	if _, has := got["w_task.good"]; has {
		t.Errorf("a dotted key must be nested, not passed as it is: %v", got)
	}
	if h1 == h0 {
		t.Error("another weighting gives another plan, so it must change the hash (the stored run is out of date)")
	}
	if _, h2 := paramsOf(Params{TimeLimit: 60, Seed: 9, Workers: 4}); h2 != h1 {
		t.Error("time limit, seed and workers must not change the hash")
	}
	w.st.SetWeights(ctx, w.org.Slug, w.plan.Slug, map[string]any{})
	if back, h3 := paramsOf(Params{TimeLimit: 5, Seed: 1, Workers: 1}); h3 != h0 || len(back) != 3 {
		t.Errorf("back to the defaults is back to the old input and hash: %v", back)
	}
}

func TestOtherPlansAndOrganisationsWeightsAreNotSent(t *testing.T) {
	a := newWorld(t, "Org A")
	b := populate(t, a.st, "Org B")
	a.st.SetWeights(ctx, a.org.Slug, a.plan.Slug, map[string]any{"w_fair": 33.0})
	rawB, _, _, _ := BuildInput(b.data(t), Params{})
	if strings.Contains(string(rawB), "w_fair") {
		t.Errorf("B's plan got A's weights: %s", rawB)
	}
}

func solverArgv(t *testing.T) []string {
	t.Helper()
	argv := strings.Fields(realSolverCommand())
	if len(argv) == 0 {
		t.Skip("DUTYPLAN_SOLVER is not set (run via mise run test:app)")
	}
	return argv
}

// The defaults in WeightSpecs are copies of the planner's own; this keeps them from drifting apart.
func TestWeightDefaultsAreThePlannersOwn(t *testing.T) {
	argv := solverArgv(t)
	out, err := exec.Command(argv[0], append(argv[1:], "--defaults")...).Output()
	if err != nil {
		t.Fatal(err)
	}
	var defaults map[string]any
	if err := json.Unmarshal(out, &defaults); err != nil {
		t.Fatal(err)
	}
	for _, spec := range store.WeightSpecs {
		var got any
		if group, name, nested := strings.Cut(spec.Key, "."); nested {
			sub, _ := defaults[group].(map[string]any)
			got = sub[name]
		} else {
			got = defaults[spec.Key]
		}
		if got == nil {
			t.Errorf("%s is not a parameter of the planner", spec.Key)
		} else if fmt.Sprint(got) != fmt.Sprint(spec.Default) {
			t.Errorf("%s: the planner's default is %v, ours is %v", spec.Key, got, spec.Default)
		}
	}
}

// The user's own example, through the plan's configuration: a takes only Camera, b both and c only Sound. With total load
// as the goal each gets 4 of 12; with an equal share of each task (set as weights on the plan) it is 3, 6 and 3.
func TestThePlansWeightsChangeWhatTheRealSolverDoes(t *testing.T) {
	solver := realSolver(t)
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	org, err := st.CreateOrg(ctx, "Club")
	must(err)
	camera, err := st.CreateTask(ctx, org.Slug, "Camera")
	must(err)
	sound, err := st.CreateTask(ctx, org.Slug, "Sound")
	must(err)
	plan, err := st.CreatePlan(ctx, org.Slug, "Autumn 2026")
	must(err)
	for _, d := range []string{"2026-08-02", "2026-08-09", "2026-08-16", "2026-08-23", "2026-08-30", "2026-09-06"} {
		_, err = addWithPlaces(st, org.Slug, plan.Slug, store.NewEvents{Date: d}, map[int64]int{camera.ID: 1, sound.ID: 1})
		must(err)
	}
	for name, tasks := range map[string][]store.Task{"a": {camera}, "b": {camera, sound}, "c": {sound}} {
		p, err := st.CreatePerson(ctx, org.Slug, name)
		must(err)
		for _, tk := range tasks {
			must(st.SetPersonTask(ctx, org.Slug, p.ID, tk.ID, "good"))
		}
	}
	svc := New(st, solver, 1, nopLogger())
	totals := func() string {
		id, err := svc.Start(context.Background(), org.Slug, plan.Slug, Params{TimeLimit: 5, Seed: 1})
		must(err)
		waitDone(t, svc)
		v, err := svc.View(context.Background(), org.Slug, plan.Slug, id)
		must(err)
		if v.Run.Status != "done" || len(v.Run.Violations) != 0 {
			t.Fatalf("%+v", v.Run)
		}
		var parts []string
		for _, l := range v.Load {
			parts = append(parts, fmt.Sprintf("%s=%d", l.Name, l.Count))
		}
		return strings.Join(parts, " ")
	}
	// Doing both tasks the same day should not cost anything here: it would only get in the way of the fairness.
	_, err = st.SetWeights(ctx, org.Slug, plan.Slug, map[string]any{"w_combo.impractical": 0.0})
	must(err)
	if got := totals(); got != "a=4 b=4 c=4" {
		t.Errorf("total load is the goal by default: %s", got)
	}
	_, err = st.SetWeights(ctx, org.Slug, plan.Slug, map[string]any{"w_combo.impractical": 0.0, "w_fair": 0.0, "w_fair_range": 0.0, "w_task_fair": 10.0, "w_task_range": 10.0})
	must(err)
	if got := totals(); got != "a=3 b=6 c=3" {
		t.Errorf("an equal share of each task, chosen on the plan: %s", got)
	}
}

// The order of tasks and people, and the people's sections, are only where they stand in the table: the planner gets the same input.
func TestTheTableOrderIsNotPartOfTheInput(t *testing.T) {
	w := newWorld(t, "Club")
	raw0, h0, _, err := BuildInput(w.data(t), Params{})
	if err != nil {
		t.Fatal(err)
	}
	same := func(what string) {
		t.Helper()
		raw1, h1, _, err := BuildInput(w.data(t), Params{})
		if err != nil {
			t.Fatal(err)
		}
		if h1 != h0 || string(raw1) != string(raw0) {
			t.Errorf("%s changed the planner's input", what)
		}
	}
	if err := w.st.MoveTask(ctx, w.org.Slug, w.camera.ID, 0); err != nil {
		t.Fatal(err)
	}
	same("moving a task in the table")
	x, err := w.st.CreateSection(ctx, w.org.Slug, "Guests")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.st.MovePerson(ctx, w.org.Slug, w.people["Embla"].ID, x.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := w.st.MovePerson(ctx, w.org.Slug, w.people["Dina"].ID, 0, 0); err != nil {
		t.Fatal(err)
	}
	same("placing people in sections and moving them")
}
