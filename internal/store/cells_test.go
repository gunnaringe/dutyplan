package store

import (
	"errors"
	"strings"
	"testing"
)

// cellEnv: Sound and Camera are planned, Singing is manual-only; the people Aslaug, Birger and Cecilie; two plans.
type cellEnv struct {
	s                   *Store
	org                 Org
	sound, camera, sang Task
	people              map[string]Person
	plan1, plan2        Plan
	ev                  []Event // ev[0], ev[1] in plan1; ev[2] in plan2
}

func cellWorld(t *testing.T, s *Store, orgName string) cellEnv {
	t.Helper()
	e := cellEnv{s: s, org: mustOrg(t, s, orgName), people: map[string]Person{}}
	var err error
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	e.plan1, err = s.CreatePlan(ctx, e.org.Slug, "Autumn 2026")
	must(err)
	e.sound, err = s.CreateTask(ctx, e.org.Slug, e.plan1.Slug, "Sound")
	must(err)
	e.camera, err = s.CreateTask(ctx, e.org.Slug, e.plan1.Slug, "Camera")
	must(err)
	e.sang, err = unplannedTask(ctx, s, e.org.Slug, e.plan1.Slug, "Singing")
	must(err)
	for _, n := range []string{"Aslaug", "Birger", "Cecilie"} {
		p, err := s.CreatePerson(ctx, e.org.Slug, e.plan1.Slug, n)
		must(err)
		e.people[n] = p
	}
	e.plan2, err = s.CreatePlanFrom(ctx, e.org.Slug, NewPlan{Name: "Spring 2027", From: e.plan1.Slug}) // copies, ids of its own
	must(err)
	for _, c := range []struct {
		plan Plan
		date string
	}{{e.plan1, "2026-09-06"}, {e.plan1, "2026-09-13"}, {e.plan2, "2027-02-07"}} {
		ev, err := s.AddEvent(ctx, e.org.Slug, c.plan.Slug, c.date)
		must(err)
		e.ev = append(e.ev, ev)
	}
	return e
}

func TestManualCellsAreAddedOnAnEvent(t *testing.T) {
	s := open(t)
	e := cellWorld(t, s, "Club")
	anna := e.people["Aslaug"].ID
	c, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[0].ID, TaskID: e.sound.ID, PersonID: anna})
	if err != nil || *c.PersonID != anna || c.Fills != 1 || c.EventID != e.ev[0].ID || c.Locked {
		t.Fatalf("a person set by hand: %+v %v", c, err)
	}
	if _, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[0].ID, TaskID: e.sound.ID, PersonID: anna}); !errors.Is(err, ErrConflict) {
		t.Errorf("the same cell again: %v", err)
	}
	// a person may be set on several tasks of one event (the planner will warn if they cannot be combined)
	if _, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[0].ID, TaskID: e.camera.ID, PersonID: anna}); err != nil {
		t.Fatalf("another task, same event: %v", err)
	}
	// a task the planner does not distribute can have manual cells (that is what it is for); a text fills several places
	g, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[1].ID, TaskID: e.sang.ID, Label: "Choir", Fills: 3})
	if err != nil || g.Fills != 3 || g.Label != "Choir" {
		t.Fatalf("a text in a task the planner does not distribute: %+v %v", g, err)
	}
	txt, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[1].ID, TaskID: e.sound.ID, Label: "  Outside guest "})
	if err != nil || txt.Label != "Outside guest" || txt.Fills != 1 {
		t.Fatalf("a text: %+v %v", txt, err)
	}
	locked, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[1].ID, TaskID: e.camera.ID, PersonID: anna, Locked: true})
	if err != nil || !locked.Locked {
		t.Fatalf("a locked planner name: %+v %v", locked, err)
	}
	list, _ := s.ListCells(ctx, e.org.Slug, e.plan1.Slug)
	if len(list) != 5 || list[0].EventID != e.ev[0].ID || list[len(list)-1].EventID != e.ev[1].ID {
		t.Errorf("listed by event date: %+v", list)
	}
	if other, _ := s.ListCells(ctx, e.org.Slug, e.plan2.Slug); len(other) != 0 {
		t.Errorf("another plan lists none of them: %+v", other)
	}
}

func TestManualCellRefusals(t *testing.T) {
	s := open(t)
	e := cellWorld(t, s, "Club")
	other := cellWorld(t, s, "Annen")
	ev := e.ev[0].ID
	for name, c := range map[string]struct {
		in   NewCell
		want error
	}{
		"nothing named":                 {NewCell{EventID: ev, TaskID: e.sound.ID}, ErrInvalid},
		"person and text":               {NewCell{EventID: ev, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID, Label: "x"}, ErrInvalid},
		"a person fills one":            {NewCell{EventID: ev, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID, Fills: 2}, ErrInvalid},
		"fills 51":                      {NewCell{EventID: ev, TaskID: e.sound.ID, Label: "x", Fills: 51}, ErrInvalid},
		"fills negative":                {NewCell{EventID: ev, TaskID: e.sound.ID, Label: "x", Fills: -1}, ErrInvalid},
		"text too long":                 {NewCell{EventID: ev, TaskID: e.sound.ID, Label: strings.Repeat("x", 81)}, ErrInvalid},
		"text with control chars":       {NewCell{EventID: ev, TaskID: e.sound.ID, Label: "a\nb"}, ErrInvalid},
		"a locked text":                 {NewCell{EventID: ev, TaskID: e.sound.ID, Label: "x", Locked: true}, ErrInvalid},
		"no event":                      {NewCell{TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID}, ErrNotFound},
		"unknown task":                  {NewCell{EventID: ev, TaskID: 99999, PersonID: e.people["Aslaug"].ID}, ErrNotFound},
		"another organisation's task":   {NewCell{EventID: ev, TaskID: other.sound.ID, PersonID: e.people["Aslaug"].ID}, ErrNotFound},
		"another organisation's person": {NewCell{EventID: ev, TaskID: e.sound.ID, PersonID: other.people["Aslaug"].ID}, ErrNotFound},
		"an event of another plan":      {NewCell{EventID: e.ev[2].ID, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID}, ErrNotFound},
		"another organisation's event":  {NewCell{EventID: other.ev[0].ID, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID}, ErrNotFound},
	} {
		if _, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, c.in); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", name, c.want, err)
		}
	}
	if _, err := s.AddCell(ctx, e.org.Slug, "nope", NewCell{EventID: ev, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID}); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM manual_cell`); n != 0 {
		t.Errorf("a refused request leaves nothing behind: %d rows", n)
	}
}

func TestManualCellsBelongToTheirPlanAndGoWithTheirEvent(t *testing.T) {
	s := open(t)
	e := cellWorld(t, s, "Club")
	other := cellWorld(t, s, "Annen")
	made, err := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[0].ID, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID})
	if err != nil {
		t.Fatal(err)
	}
	id := made.ID
	if err := s.DeleteCell(ctx, e.org.Slug, e.plan2.Slug, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete through another plan: %v", err)
	}
	if err := s.DeleteCell(ctx, other.org.Slug, other.plan1.Slug, id); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete through another organisation: %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM manual_cell`); n != 1 {
		t.Fatalf("the cell survived the refused deletes: %d", n)
	}
	// deleting the event takes its cells and selections with it
	if err := s.SetPool(ctx, e.org.Slug, e.plan1.Slug, e.ev[0].ID, e.sound.ID, []int64{e.people["Birger"].ID}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteEvent(ctx, e.org.Slug, e.plan1.Slug, e.ev[0].ID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM manual_cell`) + count(t, s, `SELECT COUNT(*) FROM pool_member`); n != 0 {
		t.Errorf("cells and selections go with the event: %d", n)
	}
	made, _ = s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[1].ID, TaskID: e.sound.ID, Label: "Guest"})
	if err := s.DeleteCell(ctx, e.org.Slug, e.plan1.Slug, made.ID); err != nil {
		t.Fatal(err)
	}
}

func TestPoolsAreReplacedPerTaskAndEvent(t *testing.T) {
	s := open(t)
	e := cellWorld(t, s, "Club")
	other := cellWorld(t, s, "Annen")
	a, b := e.people["Aslaug"].ID, e.people["Birger"].ID
	for _, ev := range e.ev[:2] {
		if err := s.SetPool(ctx, e.org.Slug, e.plan1.Slug, ev.ID, e.sound.ID, []int64{a, b, a}); err != nil {
			t.Fatal(err)
		}
	}
	pools, _ := s.ListPools(ctx, e.org.Slug, e.plan1.Slug)
	if len(pools) != 2 || len(pools[0].People) != 2 || pools[0].EventID != e.ev[0].ID || pools[1].EventID != e.ev[1].ID {
		t.Fatalf("pools: %+v", pools)
	}
	// replacing one event's selection leaves the other untouched; another task has its own
	if err := s.SetPool(ctx, e.org.Slug, e.plan1.Slug, e.ev[1].ID, e.sound.ID, []int64{b}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPool(ctx, e.org.Slug, e.plan1.Slug, e.ev[1].ID, e.camera.ID, []int64{a}); err != nil {
		t.Fatal(err)
	}
	pools, _ = s.ListPools(ctx, e.org.Slug, e.plan1.Slug)
	if len(pools) != 3 || len(pools[0].People) != 2 || len(pools[1].People) != 1 || pools[1].People[0] != b {
		t.Fatalf("after replacing: %+v", pools)
	}
	// no people removes the selection
	for _, ev := range e.ev[:2] {
		if err := s.SetPool(ctx, e.org.Slug, e.plan1.Slug, ev.ID, e.sound.ID, nil); err != nil {
			t.Fatal(err)
		}
	}
	if pools, _ = s.ListPools(ctx, e.org.Slug, e.plan1.Slug); len(pools) != 1 || pools[0].TaskID != e.camera.ID {
		t.Fatalf("after clearing Sound: %+v", pools)
	}
	if other, _ := s.ListPools(ctx, e.org.Slug, e.plan2.Slug); len(other) != 0 {
		t.Errorf("another plan lists none: %+v", other)
	}

	for name, c := range map[string]struct {
		ev     int64
		task   int64
		people []int64
		want   error
	}{
		"a task the planner does not distribute": {e.ev[0].ID, e.sang.ID, []int64{a}, ErrInvalid},
		"too many":                               {e.ev[0].ID, e.sound.ID, make([]int64, 401), ErrInvalid},
		"no event":                               {0, e.sound.ID, []int64{a}, ErrNotFound},
		"another organisation's person":          {e.ev[0].ID, e.sound.ID, []int64{other.people["Aslaug"].ID}, ErrNotFound},
		"another organisation's task":            {e.ev[0].ID, other.sound.ID, []int64{a}, ErrNotFound},
		"an event of another plan":               {e.ev[2].ID, e.sound.ID, []int64{a}, ErrNotFound},
		"another organisation's event":           {other.ev[0].ID, e.sound.ID, []int64{a}, ErrNotFound},
	} {
		if err := s.SetPool(ctx, e.org.Slug, e.plan1.Slug, c.ev, c.task, c.people); !errors.Is(err, c.want) {
			t.Errorf("%s: want %v, got %v", name, c.want, err)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM pool_member WHERE task_id = ?`, e.sound.ID); n != 0 {
		t.Errorf("a refused request changes nothing: %d rows for Sound", n)
	}
}

func TestWhatIsUsedByCellsAndPoolsCannotBeDeleted(t *testing.T) {
	s := open(t)
	e := cellWorld(t, s, "Club")
	made, _ := s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[0].ID, TaskID: e.sang.ID, Label: "Choir"})
	if err := s.DeleteTask(ctx, e.org.Slug, e.plan1.Slug, e.sang.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("a task in a manual cell: %v", err)
	}
	s.DeleteCell(ctx, e.org.Slug, e.plan1.Slug, made.ID)
	s.SetPool(ctx, e.org.Slug, e.plan1.Slug, e.ev[0].ID, e.sound.ID, []int64{e.people["Aslaug"].ID})
	if err := s.DeleteTask(ctx, e.org.Slug, e.plan1.Slug, e.sound.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("a task in a selection: %v", err)
	}
	s.SetPool(ctx, e.org.Slug, e.plan1.Slug, e.ev[0].ID, e.sound.ID, nil)
	if err := s.DeleteTask(ctx, e.org.Slug, e.plan1.Slug, e.sound.ID); err != nil {
		t.Errorf("once nothing names it: %v", err)
	}
}

func TestCellForeignKeysAreTheBackstop(t *testing.T) {
	s := open(t)
	a := cellWorld(t, s, "Org A")
	b := cellWorld(t, s, "Org B")
	var orgA, planA, planA2 int64
	s.DB().QueryRow(`SELECT id FROM organization WHERE slug = ?`, a.org.Slug).Scan(&orgA)
	s.DB().QueryRow(`SELECT id FROM plan WHERE org_id = ? AND slug = ?`, orgA, a.plan1.Slug).Scan(&planA)
	s.DB().QueryRow(`SELECT id FROM plan WHERE org_id = ? AND slug = ?`, orgA, a.plan2.Slug).Scan(&planA2)
	cell := `INSERT INTO manual_cell (org_id, plan_id, event_id, task_id, person_id, label, fills, created_at) VALUES (?, ?, ?, ?, ?, ?, 1, 'x')`
	pool := `INSERT INTO pool_member (org_id, plan_id, event_id, task_id, person_id) VALUES (?, ?, ?, ?, ?)`
	for name, c := range map[string]struct {
		stmt string
		args []any
	}{
		"cell on an event of another plan":             {cell, []any{orgA, planA, a.ev[2].ID, a.sound.ID, a.people["Aslaug"].ID, nil}},
		"cell on another organisation's event":         {cell, []any{orgA, planA, b.ev[0].ID, a.sound.ID, a.people["Aslaug"].ID, nil}},
		"cell with another organisation's person":      {cell, []any{orgA, planA, a.ev[0].ID, a.sound.ID, b.people["Aslaug"].ID, nil}},
		"cell with another organisation's task":        {cell, []any{orgA, planA, a.ev[0].ID, b.sound.ID, a.people["Aslaug"].ID, nil}},
		"cell that names nothing":                      {cell, []any{orgA, planA, a.ev[0].ID, a.sound.ID, nil, nil}},
		"cell that names two things":                   {cell, []any{orgA, planA, a.ev[0].ID, a.sound.ID, a.people["Aslaug"].ID, "x"}},
		"selection on an event of another plan":        {pool, []any{orgA, planA, a.ev[2].ID, a.sound.ID, a.people["Aslaug"].ID}},
		"selection with another organisation's person": {pool, []any{orgA, planA, a.ev[0].ID, a.sound.ID, b.people["Aslaug"].ID}},
	} {
		if _, err := s.DB().Exec(c.stmt, c.args...); err == nil {
			t.Errorf("%s: the database must refuse it", name)
		}
	}
	_ = planA2
}

func TestPlanningDataCarriesCellsAndPoolsOfItsPlanOnly(t *testing.T) {
	s := open(t)
	e := cellWorld(t, s, "Club")
	s.AddCell(ctx, e.org.Slug, e.plan1.Slug, NewCell{EventID: e.ev[0].ID, TaskID: e.sound.ID, PersonID: e.people["Aslaug"].ID})
	people, _ := s.ListPeople(ctx, e.org.Slug, e.plan2.Slug)
	tasks, _ := s.ListTasks(ctx, e.org.Slug, e.plan2.Slug)
	birger, sound := people[1].ID, tasks[0].ID // plan2's own
	if _, err := s.AddCell(ctx, e.org.Slug, e.plan2.Slug, NewCell{EventID: e.ev[2].ID, TaskID: e.sound.ID, PersonID: e.people["Birger"].ID}); !errors.Is(err, ErrNotFound) {
		t.Errorf("plan1's people and tasks are not plan2's: %v", err)
	}
	s.AddCell(ctx, e.org.Slug, e.plan2.Slug, NewCell{EventID: e.ev[2].ID, TaskID: sound, PersonID: birger})
	s.SetPool(ctx, e.org.Slug, e.plan2.Slug, e.ev[2].ID, sound, []int64{birger})
	d1, err := s.GetPlanningData(ctx, e.org.Slug, e.plan1.Slug)
	d2, _ := s.GetPlanningData(ctx, e.org.Slug, e.plan2.Slug)
	if err != nil || len(d1.Cells) != 1 || len(d1.Pools) != 0 || len(d2.Cells) != 1 || len(d2.Pools) != 1 || *d2.Cells[0].PersonID != birger || d2.People[1].ID != birger {
		t.Fatalf("%+v %+v %v", d1, d2, err)
	}
}
