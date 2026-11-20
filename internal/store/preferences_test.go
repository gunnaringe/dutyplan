package store

import (
	"database/sql"
	"errors"
	"strings"
	"testing"
)

// prefEnv: one organisation with a person, three tasks (one manual-only), a plan with three events.
type prefEnv struct {
	s             *Store
	org           Org
	person        Person
	sound, camera Task
	singing       Task // not planned
	plan          Plan
	ev            []Event
}

func newPrefEnv(t *testing.T) prefEnv {
	t.Helper()
	s := open(t)
	e := prefEnv{s: s, org: mustOrg(t, s, "Club")}
	e.person, _ = s.CreatePerson(ctx, e.org.Slug, "Kari Test")
	e.sound, _ = s.CreateTask(ctx, e.org.Slug, "Sound")
	e.camera, _ = s.CreateTask(ctx, e.org.Slug, "Camera")
	e.singing, _ = unplannedTask(ctx, s, e.org.Slug, "Singing")
	e.plan, _ = s.CreatePlan(ctx, e.org.Slug, "Autumn 2026")
	for _, d := range []string{"2026-08-09", "2026-08-16", "2026-08-23"} {
		ev, err := s.AddEvent(ctx, e.org.Slug, e.plan.Slug, d)
		if err != nil {
			t.Fatal(err)
		}
		e.ev = append(e.ev, ev)
	}
	return e
}

// personView is what the plan's table shows for one person: the planner's tasks with the person's level, and the plan's
// events with the person's answer ("" = not chosen, not answered). It is read the way the page reads it.
type personView struct {
	Tasks  []viewTask
	Events []viewEvent
}
type viewTask struct {
	TaskID      int64
	Name, Level string
}
type viewEvent struct {
	EventID int64
	Level   string
}

func viewOf(t *testing.T, s *Store, org, plan string, personID int64) personView {
	t.Helper()
	tasks, err := s.ListTasks(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	events, err := s.ListEvents(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	ans, err := s.ListAnswers(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	v := personView{}
	for _, tk := range tasks {
		if !tk.Planned {
			continue
		}
		x := viewTask{TaskID: tk.ID, Name: tk.Name}
		for _, a := range ans.Tasks {
			if a.PersonID == personID && a.TaskID == tk.ID {
				x.Level = a.Level
			}
		}
		v.Tasks = append(v.Tasks, x)
	}
	for _, e := range events {
		x := viewEvent{EventID: e.ID}
		for _, a := range ans.Events {
			if a.PersonID == personID && a.EventID == e.ID {
				x.Level = a.Level
			}
		}
		v.Events = append(v.Events, x)
	}
	return v
}

func (e prefEnv) me(t *testing.T) personView {
	t.Helper()
	return viewOf(t, e.s, e.org.Slug, e.plan.Slug, e.person.ID)
}

func count(t *testing.T, s *Store, query string, args ...any) int {
	t.Helper()
	var n int
	if err := s.DB().QueryRow(query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestPersonTasksAreChosenByThePerson(t *testing.T) {
	e := newPrefEnv(t)
	me := e.me(t)
	if len(me.Tasks) != 2 || me.Tasks[0].Name != "Camera" || me.Tasks[0].Level != "" || me.Tasks[1].Name != "Sound" {
		t.Fatalf("only the planner's tasks are offered, none chosen yet: %+v", me.Tasks)
	}
	if err := e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID, "good"); err != nil {
		t.Fatal(err)
	}
	if got := e.me(t).Tasks[1]; got.Level != "good" {
		t.Errorf("Sound = %+v", got)
	}
	if err := e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID, "good"); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op: %v", err)
	}
	if err := e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID, "poor"); err != nil { // changes the level
		t.Fatal(err)
	}
	if got := count(t, e.s, `SELECT COUNT(*) FROM person_task`); got != 1 {
		t.Errorf("an update must not add a second row: %d", got)
	}
	if err := e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID, "kanskje"); !errors.Is(err, ErrInvalid) {
		t.Errorf("bad level: %v", err)
	}
	if err := e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.singing.ID, "good"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a manual-only task cannot be chosen: %v", err)
	}
	if err := e.s.RemovePersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID); err != nil {
		t.Fatal(err)
	}
	if err := e.s.RemovePersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("removing what is not chosen: %v", err)
	}
	if got := e.me(t).Tasks[1].Level; got != "" {
		t.Errorf("removed: %q", got)
	}
}

func TestAvailabilityPerEvent(t *testing.T) {
	e := newPrefEnv(t)
	set := func(levels map[int64]string) error {
		return e.s.SetAvailability(ctx, e.org.Slug, e.plan.Slug, e.person.ID, levels)
	}
	if err := set(map[int64]string{e.ev[0].ID: "never", e.ev[2].ID: "good"}); err != nil {
		t.Fatal(err)
	}
	me := e.me(t)
	if me.Events[0].Level != "never" || me.Events[1].Level != "" || me.Events[2].Level != "good" {
		t.Fatalf("events: %+v", me.Events)
	}
	if err := set(map[int64]string{e.ev[0].ID: "never"}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no change: %v", err)
	}
	if err := set(map[int64]string{e.ev[0].ID: ""}); err != nil { // neutral removes the answer
		t.Fatal(err)
	}
	if got := count(t, e.s, `SELECT COUNT(*) FROM person_event`); got != 1 {
		t.Errorf("neutral must remove the row, %d rows left", got)
	}
	for name, levels := range map[string]map[int64]string{
		"bad level": {e.ev[0].ID: "kanskje"}, "empty": {}, "task level on an event": {e.ev[0].ID: "neutral"},
	} {
		if err := set(levels); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	// all or nothing: one unknown event means none of the others are stored either
	if err := set(map[int64]string{e.ev[1].ID: "good", 99999: "never"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown event: %v", err)
	}
	if got := e.me(t).Events[1].Level; got != "" {
		t.Errorf("the valid half must not be stored: %q", got)
	}
}

func TestPreferencesNeverCrossOrganisationsOrPlans(t *testing.T) {
	a := newPrefEnv(t)
	b := mustOrg(t, a.s, "Org B")
	personB, _ := a.s.CreatePerson(ctx, b.Slug, "Eva Test")
	taskB, _ := a.s.CreateTask(ctx, b.Slug, "Sound B")
	planB, _ := a.s.CreatePlan(ctx, b.Slug, "Autumn 2026")
	evB, _ := a.s.AddEvent(ctx, b.Slug, planB.Slug, "2026-08-09")

	// A's person, task and event through B's path, and the other way round
	if err := a.s.SetPersonTask(ctx, b.Slug, a.person.ID, taskB.ID, "good"); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's person via B: %v", err)
	}
	if err := a.s.SetPersonTask(ctx, b.Slug, personB.ID, a.sound.ID, "good"); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's task for B's person: %v", err)
	}
	if err := a.s.SetPersonTask(ctx, a.org.Slug, personB.ID, a.sound.ID, "good"); !errors.Is(err, ErrNotFound) {
		t.Errorf("B's person through A: %v", err)
	}
	if err := a.s.RemovePersonTask(ctx, b.Slug, a.person.ID, a.sound.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("remove across: %v", err)
	}
	if err := a.s.SetAvailability(ctx, b.Slug, planB.Slug, personB.ID, map[int64]string{a.ev[0].ID: "never"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's event for B's person: %v", err)
	}
	if err := a.s.SetAvailability(ctx, b.Slug, planB.Slug, a.person.ID, map[int64]string{evB.ID: "never"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("A's person via B: %v", err)
	}
	// an event of another plan in the same organisation does not count for this plan
	other, _ := a.s.CreatePlan(ctx, a.org.Slug, "Spring 2027")
	if err := a.s.SetAvailability(ctx, a.org.Slug, other.Slug, a.person.ID, map[int64]string{a.ev[0].ID: "never"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another plan's event: %v", err)
	}
	if got := count(t, a.s, `SELECT COUNT(*) FROM person_task`) + count(t, a.s, `SELECT COUNT(*) FROM person_event`); got != 0 {
		t.Fatalf("every refused write must leave nothing behind, %d rows", got)
	}

	// B's table shows only B's things
	if me := viewOf(t, a.s, b.Slug, planB.Slug, personB.ID); len(me.Tasks) != 1 || me.Tasks[0].TaskID != taskB.ID || len(me.Events) != 1 || me.Events[0].EventID != evB.ID {
		t.Errorf("B's table: %+v", me)
	}
	// the other plan of A lists none of the first plan's events
	if me := viewOf(t, a.s, a.org.Slug, other.Slug, a.person.ID); len(me.Events) != 0 {
		t.Errorf("a plan lists only its own events: %+v", me.Events)
	}
}

// The database refuses the same references even when the code is bypassed.
func TestPreferencesForeignKeysAreTheBackstop(t *testing.T) {
	a := newPrefEnv(t)
	b := mustOrg(t, a.s, "Org B")
	personB, _ := a.s.CreatePerson(ctx, b.Slug, "Eva Test")
	taskB, _ := a.s.CreateTask(ctx, b.Slug, "Sound B")
	other, _ := a.s.CreatePlan(ctx, a.org.Slug, "Spring 2027")
	var orgA, orgB, planOther int64
	db := a.s.DB()
	db.QueryRow(`SELECT id FROM organization WHERE slug = ?`, a.org.Slug).Scan(&orgA)
	db.QueryRow(`SELECT id FROM organization WHERE slug = ?`, b.Slug).Scan(&orgB)
	db.QueryRow(`SELECT id FROM plan WHERE org_id = ? AND slug = ?`, orgA, other.Slug).Scan(&planOther)
	for _, c := range []struct {
		name, stmt string
		args       []any
	}{
		{"person_task with another organisation's task", `INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (?, ?, ?, 'good')`, []any{orgA, a.person.ID, taskB.ID}},
		{"person_task for another organisation's person", `INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (?, ?, ?, 'good')`, []any{orgA, personB.ID, a.sound.ID}},
		{"person_event with an event of another plan", `INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES (?, ?, ?, ?, 'never')`, []any{orgA, planOther, a.person.ID, a.ev[0].ID}},
		{"person_event for another organisation's person", `INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES (?, ?, ?, ?, 'never')`, []any{orgA, planOf(db, a.ev[0].ID), personB.ID, a.ev[0].ID}},
	} {
		if _, err := db.Exec(c.stmt, c.args...); err == nil || !strings.Contains(err.Error(), "FOREIGN KEY") {
			t.Errorf("%s must fail at the foreign key, got %v", c.name, err)
		}
	}
	_ = orgB
}

func TestPreferencesGoWhenTheirTaskOrEventGoes(t *testing.T) {
	e := newPrefEnv(t)
	e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID, "good")
	e.s.SetAvailability(ctx, e.org.Slug, e.plan.Slug, e.person.ID, map[int64]string{e.ev[0].ID: "never", e.ev[1].ID: "good"})
	if err := e.s.DeleteEvent(ctx, e.org.Slug, e.plan.Slug, e.ev[0].ID); err != nil {
		t.Fatal(err)
	}
	if got := count(t, e.s, `SELECT COUNT(*) FROM person_event`); got != 1 {
		t.Errorf("an event's answers go with it: %d", got)
	}
	if err := e.s.DeleteTask(ctx, e.org.Slug, e.sound.ID); err != nil {
		t.Fatal(err)
	}
	if got := count(t, e.s, `SELECT COUNT(*) FROM person_task`); got != 0 {
		t.Errorf("a task's choices go with it: %d", got)
	}
}

func planOf(db interface {
	QueryRow(string, ...any) *sql.Row
}, event int64) int64 {
	var id int64
	db.QueryRow(`SELECT plan_id FROM event WHERE id = ?`, event).Scan(&id)
	return id
}

func TestEachPersonSeesOnlyTheirOwnAnswers(t *testing.T) {
	e := newPrefEnv(t)
	other, _ := e.s.CreatePerson(ctx, e.org.Slug, "Ola Prøve")
	e.s.SetPersonTask(ctx, e.org.Slug, e.person.ID, e.sound.ID, "good")
	e.s.SetPersonTask(ctx, e.org.Slug, other.ID, e.sound.ID, "never")
	e.s.SetAvailability(ctx, e.org.Slug, e.plan.Slug, e.person.ID, map[int64]string{e.ev[0].ID: "good"})
	e.s.SetAvailability(ctx, e.org.Slug, e.plan.Slug, other.ID, map[int64]string{e.ev[0].ID: "never", e.ev[1].ID: "poor"})
	mine := e.me(t)
	theirs := viewOf(t, e.s, e.org.Slug, e.plan.Slug, other.ID)
	if mine.Tasks[1].Level != "good" || theirs.Tasks[1].Level != "never" {
		t.Errorf("task answers: %+v / %+v", mine.Tasks, theirs.Tasks)
	}
	if mine.Events[0].Level != "good" || mine.Events[1].Level != "" || theirs.Events[0].Level != "never" || theirs.Events[1].Level != "poor" {
		t.Errorf("event answers: %+v / %+v", mine.Events, theirs.Events)
	}
}
