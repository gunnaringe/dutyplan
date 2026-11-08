package store

import (
	"errors"
	"strings"
	"testing"
)

func TestTasksCrud(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	a, err := s.CreateTask(ctx, o.Slug, "Ola", "Lyd", true)
	if err != nil || !a.Planned {
		t.Fatalf("create: %+v %v", a, err)
	}
	if _, err := s.CreateTask(ctx, o.Slug, "Ola", "Lyd", true); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate: %v", err)
	}
	if _, err := s.CreateTask(ctx, o.Slug, "Ola", "  ", true); !errors.Is(err, ErrInvalid) {
		t.Errorf("blank: %v", err)
	}
	off := false
	got, err := s.UpdateTask(ctx, o.Slug, "Ola", a.ID, TaskPatch{Planned: &off})
	if err != nil || got.Planned {
		t.Fatalf("update: %+v %v", got, err)
	}
	if _, err := s.UpdateTask(ctx, o.Slug, "Ola", a.ID, TaskPatch{Planned: &off}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: %v", err)
	}
	created, skipped, err := s.CreateTasks(ctx, o.Slug, "Ola", "Lyd\n\n Bilde \nKafé\nbilde\n", true)
	if err != nil || len(created) != 2 || len(skipped) != 1 || skipped[0] != "Lyd" {
		t.Errorf("bulk: created %v skipped %v err %v", created, skipped, err)
	}
	if _, _, err := s.CreateTasks(ctx, o.Slug, "Ola", " \n \n", true); !errors.Is(err, ErrInvalid) {
		t.Errorf("empty bulk: %v", err)
	}
	if err := s.DeleteTask(ctx, o.Slug, "Ola", created[0].ID); err != nil {
		t.Fatal(err)
	}
	if tasks, _ := s.ListTasks(ctx, o.Slug); len(tasks) != 2 {
		t.Errorf("after delete: %v", tasks)
	}
}

func TestDayTypeTasksAndDeletesAreProtected(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	lyd, _ := s.CreateTask(ctx, o.Slug, "Ola", "Lyd", true)
	kafe, _ := s.CreateTask(ctx, o.Slug, "Ola", "Kafé", true)
	dt, err := s.CreateDayType(ctx, o.Slug, "Ola", "Gudstjeneste")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateDayType(ctx, o.Slug, "Ola", "Gudstjeneste"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate day type: %v", err)
	}
	for name, tasks := range map[string][]DayTypeTask{
		"zero slots": {{lyd.ID, 0}}, "too many": {{lyd.ID, 51}}, "twice": {{lyd.ID, 1}, {lyd.ID, 2}},
	} {
		if _, err := s.SetDayTypeTasks(ctx, o.Slug, "Ola", dt.ID, tasks); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if _, err := s.SetDayTypeTasks(ctx, o.Slug, "Ola", dt.ID, []DayTypeTask{{999, 1}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown task: %v", err)
	}
	got, err := s.SetDayTypeTasks(ctx, o.Slug, "Ola", dt.ID, []DayTypeTask{{lyd.ID, 1}, {kafe.ID, 3}})
	if err != nil || len(got.Tasks) != 2 {
		t.Fatalf("set: %+v %v", got, err)
	}
	// replacing the list replaces, not appends
	if got, _ = s.SetDayTypeTasks(ctx, o.Slug, "Ola", dt.ID, []DayTypeTask{{kafe.ID, 2}}); len(got.Tasks) != 1 || got.Tasks[0].Slots != 2 {
		t.Errorf("replace: %+v", got)
	}

	before := len(mustLog(t, s, o.Slug, ""))
	if err := s.DeleteTask(ctx, o.Slug, "Ola", kafe.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("task used by a day type must not be deleted, got %v", err)
	}
	plan, _ := s.CreatePlan(ctx, o.Slug, "Ola", "Høst 2026", "")
	if _, err := s.AddEvent(ctx, o.Slug, plan.Slug, "Ola", "2026-08-09", dt.ID, ""); err != nil {
		t.Fatal(err)
	}
	before++
	if err := s.DeleteDayType(ctx, o.Slug, "Ola", dt.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("day type used by a date must not be deleted, got %v", err)
	}
	if after := len(mustLog(t, s, o.Slug, "")); after != before+1 { // +1 for the plan.create above
		t.Errorf("failed deletes must leave no log line: %d -> %d", before, after)
	}
	if tasks, _ := s.ListTasks(ctx, o.Slug); len(tasks) != 2 {
		t.Errorf("task still there: %v", tasks)
	}
	// once nothing uses them, both can go
	dates, _ := s.ListEvents(ctx, o.Slug, plan.Slug)
	s.DeleteEvent(ctx, o.Slug, plan.Slug, "Ola", dates[0].ID)
	if err := s.DeleteDayType(ctx, o.Slug, "Ola", dt.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTask(ctx, o.Slug, "Ola", kafe.ID); err != nil {
		t.Fatal(err)
	}
}

func TestCombinations(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Menighet")
	a, _ := s.CreateTask(ctx, o.Slug, "Ola", "Lyd", true)
	b, _ := s.CreateTask(ctx, o.Slug, "Ola", "Bilde", true)
	if err := s.SetCombination(ctx, o.Slug, "Ola", b.ID, a.ID, "kan_ikke"); err != nil { // either order
		t.Fatal(err)
	}
	list, _ := s.ListCombinations(ctx, o.Slug)
	if len(list) != 1 || list[0].A != a.ID || list[0].B != b.ID || list[0].Level != "kan_ikke" {
		t.Fatalf("stored with the smaller id first: %+v", list)
	}
	if err := s.SetCombination(ctx, o.Slug, "Ola", a.ID, b.ID, "bor"); err != nil { // overwrite, not a second row
		t.Fatal(err)
	}
	if list, _ = s.ListCombinations(ctx, o.Slug); len(list) != 1 || list[0].Level != "bor" {
		t.Errorf("overwrite: %+v", list)
	}
	if err := s.SetCombination(ctx, o.Slug, "Ola", a.ID, a.ID, "bor"); !errors.Is(err, ErrInvalid) {
		t.Errorf("a task with itself: %v", err)
	}
	if err := s.SetCombination(ctx, o.Slug, "Ola", a.ID, b.ID, "kanskje"); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad level: %v", err)
	}
	if err := s.SetCombination(ctx, o.Slug, "Ola", a.ID, b.ID, "standard"); err != nil {
		t.Fatal(err)
	}
	if list, _ = s.ListCombinations(ctx, o.Slug); len(list) != 0 {
		t.Errorf("standard removes the setting: %+v", list)
	}
	s.SetCombination(ctx, o.Slug, "Ola", a.ID, b.ID, "upraktisk")
	if err := s.DeleteTask(ctx, o.Slug, "Ola", a.ID); err != nil { // combinations go with their task
		t.Fatal(err)
	}
	if list, _ = s.ListCombinations(ctx, o.Slug); len(list) != 0 {
		t.Errorf("combination must go with the task: %+v", list)
	}
}

// Nothing may point across organisations, neither through the API-level checks nor (as a backstop) in the database.
func TestCrossOrganisationReferences(t *testing.T) {
	s := open(t)
	a, b := mustOrg(t, s, "Org A"), mustOrg(t, s, "Org B")
	taskA, _ := s.CreateTask(ctx, a.Slug, "Ola", "Lyd A", true)
	taskB, _ := s.CreateTask(ctx, b.Slug, "Eva", "Lyd B", true)
	dtA, _ := s.CreateDayType(ctx, a.Slug, "Ola", "Type A")
	dtB, _ := s.CreateDayType(ctx, b.Slug, "Eva", "Type B")
	planB, _ := s.CreatePlan(ctx, b.Slug, "Eva", "Høst 2026", "")

	if _, err := s.AddEvent(ctx, b.Slug, planB.Slug, "Eva", "2026-08-09", dtA.ID, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's day type on B's date: want ErrNotFound, got %v", err)
	}
	if _, _, err := s.AddEventsBulk(ctx, b.Slug, planB.Slug, "Eva", "2026-08-01", "2026-08-31", []int{7}, dtA.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's day type in a bulk add: %v", err)
	}
	if _, err := s.SetDayTypeTasks(ctx, b.Slug, "Eva", dtB.ID, []DayTypeTask{{taskA.ID, 1}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's task in B's day type: %v", err)
	}
	if _, err := s.SetDayTypeTasks(ctx, b.Slug, "Eva", dtA.ID, []DayTypeTask{{taskB.ID, 1}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B setting tasks on A's day type: %v", err)
	}
	if err := s.SetCombination(ctx, b.Slug, "Eva", taskA.ID, taskB.ID, "kan_ikke"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a combination spanning organisations: %v", err)
	}
	if _, err := s.UpdateTask(ctx, b.Slug, "Eva", taskA.ID, TaskPatch{Name: ptr("Kapret")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B renames A's task: %v", err)
	}
	if err := s.DeleteTask(ctx, b.Slug, "Eva", taskA.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B deletes A's task: %v", err)
	}
	if err := s.DeleteDayType(ctx, b.Slug, "Eva", dtA.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("B deletes A's day type: %v", err)
	}

	// Lists never show the other organisation's setup.
	if tasks, _ := s.ListTasks(ctx, b.Slug); len(tasks) != 1 || tasks[0].ID != taskB.ID {
		t.Errorf("B's tasks: %v", tasks)
	}
	if dts, _ := s.ListDayTypes(ctx, b.Slug); len(dts) != 1 || dts[0].ID != dtB.ID {
		t.Errorf("B's day types: %v", dts)
	}

	// Each organisation's day types carry only its own tasks, and its combinations are its own.
	s.SetDayTypeTasks(ctx, a.Slug, "Ola", dtA.ID, []DayTypeTask{{taskA.ID, 2}})
	s.SetDayTypeTasks(ctx, b.Slug, "Eva", dtB.ID, []DayTypeTask{{taskB.ID, 3}})
	otherA, _ := s.CreateTask(ctx, a.Slug, "Ola", "Bilde A", true)
	s.SetCombination(ctx, a.Slug, "Ola", taskA.ID, otherA.ID, "kan_ikke")
	if dts, _ := s.ListDayTypes(ctx, b.Slug); len(dts) != 1 || len(dts[0].Tasks) != 1 || dts[0].Tasks[0].TaskID != taskB.ID || dts[0].Tasks[0].Slots != 3 {
		t.Errorf("B's day type must hold only B's task: %+v", dts)
	}
	if dts, _ := s.ListDayTypes(ctx, a.Slug); len(dts) != 1 || len(dts[0].Tasks) != 1 || dts[0].Tasks[0].TaskID != taskA.ID {
		t.Errorf("A's day type must hold only A's task: %+v", dts)
	}
	if combos, _ := s.ListCombinations(ctx, b.Slug); len(combos) != 0 {
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
		{"date with A's day type", `INSERT INTO event (org_id, plan_id, date, day_type_id, created_at) VALUES (?, ?, '2026-08-09', ?, 'x')`, []any{orgB, planBID, dtA.ID}},
		{"B's day type holding A's task", `INSERT INTO day_type_task (org_id, day_type_id, task_id, slots) VALUES (?, ?, ?, 1)`, []any{orgB, dtB.ID, taskA.ID}},
		{"combination across", `INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (?, ?, ?, 'bor')`, []any{orgB, taskA.ID, taskB.ID}},
	} {
		if _, err := db.Exec(c.stmt, c.args...); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Errorf("%s must fail at the foreign key, got %v", c.name, err)
		}
	}
	_ = orgA
}

func ptr[T any](v T) *T { return &v }
