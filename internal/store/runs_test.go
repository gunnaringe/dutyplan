package store

import (
	"errors"
	"strings"
	"testing"
)

func TestPlanningDataIsCompleteAndScoped(t *testing.T) {
	a := newPrefEnv(t)
	a.s.SetPersonTask(ctx, a.org.Slug, a.person.ID, a.sound.ID, "good")
	a.s.SetAvailability(ctx, a.org.Slug, a.plan.Slug, a.person.ID, map[int64]string{a.ev[0].ID: "never"})
	a.s.SetCombination(ctx, a.org.Slug, a.sound.ID, a.camera.ID, "impractical")

	b := mustOrg(t, a.s, "Org B")
	planB, _ := a.s.CreatePlan(ctx, b.Slug, "Autumn 2026") // same slug as A's plan
	personB, _ := a.s.CreatePerson(ctx, b.Slug, "Eva Test")
	taskB, _ := a.s.CreateTask(ctx, b.Slug, "Coffee")
	a.s.SetPersonTask(ctx, b.Slug, personB.ID, taskB.ID, "neutral")

	d, err := a.s.GetPlanningData(ctx, a.org.Slug, a.plan.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Tasks) != 3 || len(d.Events) != 3 || len(d.People) != 1 || len(d.Combinations) != 1 {
		t.Fatalf("A's data: %d tasks %d events %d people %d combos", len(d.Tasks), len(d.Events), len(d.People), len(d.Combinations))
	}
	if d.PersonTasks[a.person.ID][a.sound.ID] != "good" || d.PersonEvents[a.person.ID][a.ev[0].ID] != "never" {
		t.Errorf("answers missing: %v %v", d.PersonTasks, d.PersonEvents)
	}
	db, err := a.s.GetPlanningData(ctx, b.Slug, planB.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(db.Tasks) != 1 || db.Tasks[0].ID != taskB.ID || len(db.Events) != 0 || len(db.People) != 1 || db.People[0].ID != personB.ID || len(db.Combinations) != 0 {
		t.Errorf("B must get only its own data: %+v", db)
	}
	if _, ok := db.PersonTasks[a.person.ID]; ok {
		t.Error("A's person answers leaked into B's data")
	}
	if _, err := a.s.GetPlanningData(ctx, a.org.Slug, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
}

func TestRunLifecycle(t *testing.T) {
	e := newPrefEnv(t)
	seed, _ := e.s.NextSeed(ctx, e.org.Slug, e.plan.Slug)
	if seed != 1 {
		t.Fatalf("first seed %d", seed)
	}
	id, err := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, seed, 20, "hash1", []byte(`{"in":1}`), []byte(`{"snap":1}`))
	if err != nil {
		t.Fatal(err)
	}
	r, err := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, id)
	if err != nil || r.Status != "running" || r.Seed != 1 || string(r.InputJSON) != `{"in":1}` || string(r.SnapshotJSON) != `{"snap":1}` || len(r.OutputJSON) != 0 {
		t.Fatalf("running: %+v %v", r, err)
	}
	if err := e.s.FinishRun(ctx, id, []byte(`{"status":"FEASIBLE"}`), ""); err != nil {
		t.Fatal(err)
	}
	r, _ = e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, id)
	if r.Status != "done" || string(r.OutputJSON) != `{"status":"FEASIBLE"}` || r.FinishedAt == "" || r.Error != "" {
		t.Fatalf("done: %+v", r)
	}
	if err := e.s.FinishRun(ctx, id, nil, "igjen"); !errors.Is(err, ErrConflict) {
		t.Errorf("a run finishes once: %v", err)
	}
	if err := e.s.FinishRun(ctx, 99999, nil, "x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown run: %v", err)
	}
	if next, _ := e.s.NextSeed(ctx, e.org.Slug, e.plan.Slug); next != 2 {
		t.Errorf("next seed %d", next)
	}

	id2, _ := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, 2, 10, "hash1", []byte(`{}`), []byte(`{}`))
	e.s.FinishRun(ctx, id2, nil, strings.Repeat("feil ", 200))
	r2, _ := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, id2)
	if r2.Status != "failed" || len([]rune(r2.Error)) > 501 || r2.OutputJSON != nil && len(r2.OutputJSON) != 0 {
		t.Errorf("failed: status %s, error %d characters", r2.Status, len([]rune(r2.Error)))
	}
	runs, _ := e.s.ListRuns(ctx, e.org.Slug, e.plan.Slug, 10)
	if len(runs) != 2 || runs[0].ID != id2 || runs[1].ID != id {
		t.Errorf("newest first: %+v", runs)
	}
}

func TestRunsArePrunedToTheNewestTwenty(t *testing.T) {
	e := newPrefEnv(t)
	var first, last int64
	for i := 0; i < maxRunsPerPlan+3; i++ {
		id, err := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, i+1, 5, "h", []byte("{}"), []byte("{}"))
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = id
		}
		last = id
	}
	if n := count(t, e.s, `SELECT COUNT(*) FROM plan_run`); n != maxRunsPerPlan {
		t.Errorf("%d runs kept, want %d", n, maxRunsPerPlan)
	}
	if _, err := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, first); !errors.Is(err, ErrNotFound) {
		t.Errorf("the oldest is gone: %v", err)
	}
	if _, err := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, last); err != nil {
		t.Errorf("the newest stays: %v", err)
	}
}

func TestRunsBelongToTheirPlanAndOrganisation(t *testing.T) {
	a := newPrefEnv(t)
	idA, _ := a.s.CreateRun(ctx, a.org.Slug, a.plan.Slug, 1, 5, "h", []byte("{}"), []byte("{}"))
	other, _ := a.s.CreatePlan(ctx, a.org.Slug, "Spring 2027")
	b := mustOrg(t, a.s, "Org B")
	planB, _ := a.s.CreatePlan(ctx, b.Slug, "Autumn 2026")

	if _, err := a.s.GetRun(ctx, a.org.Slug, other.Slug, idA); !errors.Is(err, ErrNotFound) {
		t.Errorf("a run through another plan: %v", err)
	}
	if _, err := a.s.GetRun(ctx, b.Slug, planB.Slug, idA); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's run through B: %v", err)
	}
	if l, _ := a.s.ListRuns(ctx, b.Slug, planB.Slug, 10); len(l) != 0 {
		t.Errorf("B lists A's runs: %v", l)
	}
	if l, _ := a.s.ListRuns(ctx, a.org.Slug, other.Slug, 10); len(l) != 0 {
		t.Errorf("another plan lists this plan's runs: %v", l)
	}
	if seed, _ := a.s.NextSeed(ctx, b.Slug, planB.Slug); seed != 1 {
		t.Errorf("seeds count per plan, not globally: %d", seed)
	}
	var orgA, orgB, planBID int64
	db := a.s.DB()
	db.QueryRow(`SELECT id FROM organization WHERE slug = ?`, a.org.Slug).Scan(&orgA)
	db.QueryRow(`SELECT id FROM organization WHERE slug = ?`, b.Slug).Scan(&orgB)
	db.QueryRow(`SELECT id FROM plan WHERE org_id = ?`, orgB).Scan(&planBID)
	_, err := db.Exec(`INSERT INTO plan_run (org_id, plan_id, status, seed, time_limit, input_hash, input_json, snapshot_json, started_at)
		VALUES (?, ?, 'running', 1, 5, 'h', '{}', '{}', 'nå')`, orgA, planBID)
	if err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
		t.Errorf("a run on another organisation's plan must fail at the foreign key: %v", err)
	}
}

func TestInterruptedRunsAreFailedAtStartup(t *testing.T) {
	e := newPrefEnv(t)
	done, _ := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, 1, 5, "h", []byte("{}"), []byte("{}"))
	e.s.FinishRun(ctx, done, []byte("{}"), "")
	running, _ := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, 2, 5, "h", []byte("{}"), []byte("{}"))
	n, err := e.s.FailRunningRuns(ctx)
	if err != nil || n != 1 {
		t.Fatalf("failed %d, %v", n, err)
	}
	r, _ := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, running)
	if r.Status != "failed" || !strings.Contains(r.Error, "was stopped") {
		t.Errorf("%+v", r)
	}
	if r, _ := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, done); r.Status != "done" {
		t.Errorf("a finished run is left alone: %+v", r)
	}
}
