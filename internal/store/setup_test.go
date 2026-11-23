package store

import (
	"errors"
	"strings"
	"testing"
)

func TestTasksCrud(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Club")
	pl1 := mustPlan(t, s, o.Slug)
	a, err := s.CreateTask(ctx, o.Slug, pl1.Slug, "Sound")
	if err != nil || !a.Planned {
		t.Fatalf("create: %+v %v", a, err)
	}
	if _, err := s.CreateTask(ctx, o.Slug, pl1.Slug, "Sound"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := s.CreateTask(ctx, o.Slug, pl1.Slug, "  "); !errors.Is(err, ErrInvalid) {
		t.Errorf("blank: %v", err)
	}
	off := false
	got, err := s.UpdateTask(ctx, o.Slug, pl1.Slug, a.ID, TaskPatch{Planned: &off})
	if err != nil || got.Planned {
		t.Fatalf("update: %+v %v", got, err)
	}
	if _, err := s.UpdateTask(ctx, o.Slug, pl1.Slug, a.ID, TaskPatch{Planned: &off}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: %v", err)
	}
	camera, err := s.CreateTask(ctx, o.Slug, pl1.Slug, "Camera")
	if err != nil || !camera.Planned {
		t.Fatalf("a new task is distributed by the planner: %+v %v", camera, err)
	}
	if err := s.DeleteTask(ctx, o.Slug, pl1.Slug, camera.ID); err != nil {
		t.Fatal(err)
	}
	if tasks, _ := s.ListTasks(ctx, o.Slug, pl1.Slug); len(tasks) != 1 {
		t.Errorf("after delete: %v", tasks)
	}
}

func TestCombinations(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Club")
	pl1 := mustPlan(t, s, o.Slug)
	a, _ := s.CreateTask(ctx, o.Slug, pl1.Slug, "Sound")
	b, _ := s.CreateTask(ctx, o.Slug, pl1.Slug, "Camera")
	if err := s.SetCombination(ctx, o.Slug, pl1.Slug, b.ID, a.ID, "cannot"); err != nil { // either order
		t.Fatal(err)
	}
	list, _ := s.ListCombinations(ctx, o.Slug, pl1.Slug)
	if len(list) != 1 || list[0].A != a.ID || list[0].B != b.ID || list[0].Level != "cannot" {
		t.Fatalf("stored with the smaller id first: %+v", list)
	}
	if err := s.SetCombination(ctx, o.Slug, pl1.Slug, a.ID, b.ID, "practical"); err != nil { // overwrite, not a second row
		t.Fatal(err)
	}
	if list, _ = s.ListCombinations(ctx, o.Slug, pl1.Slug); len(list) != 1 || list[0].Level != "practical" {
		t.Errorf("overwrite: %+v", list)
	}
	if err := s.SetCombination(ctx, o.Slug, pl1.Slug, a.ID, a.ID, "practical"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a task with itself: %v", err)
	}
	for _, level := range []string{"impractical", "preferred"} { // can't → bad → good: «bad» is no row
		if err := s.SetCombination(ctx, o.Slug, pl1.Slug, a.ID, b.ID, level); !errors.Is(err, ErrInvalid) {
			t.Errorf("level %s: %v", level, err)
		}
		if err := s.SetTaskCombinations(ctx, o.Slug, pl1.Slug, a.ID, level); !errors.Is(err, ErrInvalid) {
			t.Errorf("level %s for all: %v", level, err)
		}
	}
	if err := s.SetCombination(ctx, o.Slug, pl1.Slug, a.ID, b.ID, "kanskje"); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad level: %v", err)
	}
	if err := s.SetCombination(ctx, o.Slug, pl1.Slug, a.ID, b.ID, "standard"); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.ListCombinations(ctx, o.Slug, pl1.Slug); len(list) != 0 {
		t.Errorf("standard removes the setting: %+v", list)
	}
	s.SetCombination(ctx, o.Slug, pl1.Slug, a.ID, b.ID, "practical")
	if err := s.DeleteTask(ctx, o.Slug, pl1.Slug, a.ID); err != nil { // combinations go with their task
		t.Fatal(err)
	}
	if list, _ = s.ListCombinations(ctx, o.Slug, pl1.Slug); len(list) != 0 {
		t.Errorf("combination must go with the task: %+v", list)
	}
}

// Nothing may point across organisations, neither through the API-level checks nor (as a backstop) in the database.
func TestCrossOrganisationReferences(t *testing.T) {
	s := open(t)
	a, b := mustOrg(t, s, "Org A"), mustOrg(t, s, "Org B")
	pl1 := mustPlan(t, s, a.Slug)
	taskA, _ := s.CreateTask(ctx, a.Slug, pl1.Slug, "Sound A")
	planB, _ := s.CreatePlan(ctx, b.Slug, "Autumn 2026")
	taskB, _ := s.CreateTask(ctx, b.Slug, planB.Slug, "Sound B")
	evB, _ := s.AddEvent(ctx, b.Slug, planB.Slug, "2026-08-02")

	if err := s.SetEventPlaces(ctx, b.Slug, planB.Slug, evB.ID, taskA.ID, 1); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's task on B's event: %v", err)
	}
	if err := s.SetCombination(ctx, b.Slug, planB.Slug, taskA.ID, taskB.ID, "cannot"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a combination spanning organisations: %v", err)
	}
	if _, err := s.UpdateTask(ctx, b.Slug, planB.Slug, taskA.ID, TaskPatch{Name: ptr("Kapret")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B renames A's task: %v", err)
	}
	if err := s.DeleteTask(ctx, b.Slug, planB.Slug, taskA.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B deletes A's task: %v", err)
	}

	// Lists never show the other organisation's setup.
	if tasks, _ := s.ListTasks(ctx, b.Slug, planB.Slug); len(tasks) != 1 || tasks[0].ID != taskB.ID {
		t.Errorf("B's tasks: %v", tasks)
	}
	if places, _ := s.ListEventPlaces(ctx, b.Slug, planB.Slug); len(places) != 0 {
		t.Errorf("B's event got A's task: %+v", places)
	}

	// Each organisation's combinations are its own.
	otherA, _ := s.CreateTask(ctx, a.Slug, pl1.Slug, "Camera A")
	s.SetCombination(ctx, a.Slug, pl1.Slug, taskA.ID, otherA.ID, "cannot")
	if combos, _ := s.ListCombinations(ctx, b.Slug, planB.Slug); len(combos) != 0 {
		t.Errorf("B sees A's combinations: %+v", combos)
	}

	// The database refuses the same references even when the code above is bypassed.
	db := s.DB()
	var orgA, orgB, planBID int64
	db.QueryRow(`SELECT id FROM organization WHERE slug = ?`, a.Slug).Scan(&orgA)
	db.QueryRow(`SELECT id FROM organization WHERE slug = ?`, b.Slug).Scan(&orgB)
	db.QueryRow(`SELECT id FROM plan WHERE org_id = ?`, orgB).Scan(&planBID)
	for _, c := range []struct {
		name, stmt string
		args       []any
	}{
		{"an event in A for B's plan", `INSERT INTO event (org_id, plan_id, date, created_at) VALUES (?, ?, '2026-08-09', 'x')`, []any{orgA, planBID}},
		{"B's event holding A's task", `INSERT INTO event_task (org_id, plan_id, event_id, task_id, slots) VALUES (?, ?, ?, ?, 1)`, []any{orgB, planBID, evB.ID, taskA.ID}},
		{"combination across", `INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (?, ?, ?, 'practical')`, []any{orgB, taskA.ID, taskB.ID}},
	} {
		if _, err := db.Exec(c.stmt, c.args...); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Errorf("%s must fail at the foreign key, got %v", c.name, err)
		}
	}
}

func ptr[T any](v T) *T { return &v }
