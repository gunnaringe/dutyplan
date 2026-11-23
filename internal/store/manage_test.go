package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// rich is an organisation with one of everything, and two plans that both have events, answers, manual cells, a selection,
// weights and runs, so that deleting one of them can be checked against the other.
type rich struct {
	org           Org
	plan, other   Plan
	people        []Person
	sound, camera Task
	section       Section
	pinned, dying int64 // runs of plan: a finished pinned run, and one that failed
}

func makeRich(t *testing.T, s *Store, orgName, planName string) rich {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	r := rich{org: mustOrg(t, s, orgName)}
	slug := r.org.Slug
	for _, n := range []string{"Kari Test", "Ola Test"} {
		p, err := s.CreatePerson(ctx, slug, n)
		must(err)
		r.people = append(r.people, p)
	}
	var err error
	r.sound, err = s.CreateTask(ctx, slug, "Sound")
	must(err)
	r.camera, err = s.CreateTask(ctx, slug, "Camera")
	must(err)
	must(s.SetCombination(ctx, slug, r.sound.ID, r.camera.ID, "cannot"))
	r.section, err = s.CreateSection(ctx, slug, "Guests")
	must(err)
	must(s.MovePerson(ctx, slug, r.people[1].ID, r.section.ID, 0))
	ts, err := s.CreateSectionOf(ctx, Tasks, slug, "Tech")
	must(err)
	must(s.MoveTask(ctx, slug, r.camera.ID, &ts.ID, 0))
	must(s.SetPersonTask(ctx, slug, r.people[0].ID, r.sound.ID, "good"))

	r.plan, err = s.CreatePlan(ctx, slug, planName)
	must(err)
	r.other, err = s.CreatePlan(ctx, slug, "Other plan")
	must(err)
	for i, p := range []Plan{r.plan, r.other} {
		var evs []Event
		for _, d := range []string{"2026-08-09", "2026-08-16"} {
			ev, err := addWithPlaces(ctx, s, slug, p.Slug, NewEvents{Date: d}, map[int64]int{r.sound.ID: 1, r.camera.ID: 1})
			must(err)
			evs = append(evs, ev[0])
		}
		must(s.SetAvailability(ctx, slug, p.Slug, r.people[0].ID, map[int64]string{evs[0].ID: "never", evs[1].ID: "poor"}))
		_, err = s.AddCell(ctx, slug, p.Slug, NewCell{EventID: evs[0].ID, TaskID: r.sound.ID, PersonID: r.people[1].ID})
		must(err)
		_, err = s.AddCell(ctx, slug, p.Slug, NewCell{EventID: evs[1].ID, TaskID: r.camera.ID, Label: "Guest", Fills: 1})
		must(err)
		must(s.SetPool(ctx, slug, p.Slug, evs[0].ID, r.camera.ID, []int64{r.people[0].ID}))
		must(s.SetEventPlaces(ctx, slug, p.Slug, evs[0].ID, r.sound.ID, 2))
		field, err := s.AddField(ctx, slug, p.Slug, "Theme")
		must(err)
		must(s.SetFieldValue(ctx, slug, p.Slug, evs[1].ID, field.ID, "Light"))
		must(s.SetAnswerNote(ctx, slug, p.Slug, evs[1].ID, r.people[0].ID, "After the meeting"))
		_, err = s.SetWeights(ctx, slug, p.Slug, map[string]any{"w_fair": 12.0})
		must(err)
		done, err := s.CreateRun(ctx, slug, p.Slug, 1, 5, "h", []byte("{}"), []byte("{}"))
		must(err)
		must(s.FinishRun(ctx, done, []byte(`{"status":"OPTIMAL"}`), ""))
		must(s.PinRun(ctx, slug, p.Slug, done, true))
		bad, err := s.CreateRun(ctx, slug, p.Slug, 2, 5, "h", []byte("{}"), []byte("{}"))
		must(err)
		must(s.FinishRun(ctx, bad, nil, "boom"))
		if i == 0 {
			r.pinned, r.dying = done, bad
		}
	}
	return r
}

// tablesWith lists the tables that have a column of this name.
func tablesWith(t *testing.T, s *Store, column string) []string {
	t.Helper()
	rows, err := s.DB().Query(`SELECT m.name FROM sqlite_master m, pragma_table_info(m.name) c
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%' AND c.name = ? ORDER BY m.name`, column)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var n string
		if err := rows.Scan(&n); err != nil {
			t.Fatal(err)
		}
		out = append(out, n)
	}
	return out
}

// rowsWith counts, per table that has the column, the rows where the column is id.
func rowsWith(t *testing.T, s *Store, column string, id int64) map[string]int {
	t.Helper()
	out := map[string]int{}
	for _, table := range tablesWith(t, s, column) {
		out[table] = count(t, s, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE %s = ?`, table, column), id)
	}
	return out
}

func idOf(t *testing.T, s *Store, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := s.DB().QueryRow(query, args...).Scan(&id); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return id
}

func noBrokenReferences(t *testing.T, s *Store) {
	t.Helper()
	rows, err := s.DB().Query(`PRAGMA foreign_key_check`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		var table, parent string
		var rowid, fk any
		rows.Scan(&table, &rowid, &parent, &fk)
		t.Errorf("a row in %s refers to a missing row in %s", table, parent)
	}
}

func sameCounts(t *testing.T, what string, before, after map[string]int) {
	t.Helper()
	for table, n := range before {
		if after[table] != n {
			t.Errorf("%s: %s has %d rows, had %d", what, table, after[table], n)
		}
	}
}

// If a table is added that holds an organisation's or a plan's data, the deletions must learn about it: this fails until the
// list below (and DeletePlan / DeleteOrg) are updated.
func TestEveryTableWithOrgOrPlanDataIsKnownToTheDeletions(t *testing.T) {
	s := open(t)
	got := func(column string) string { return strings.Join(tablesWith(t, s, column), " ") }
	if want := "answer_note event event_field event_task manual_cell person_event plan_field plan_run pool_member share_link"; got("plan_id") != want {
		t.Errorf("tables with plan_id: %s\nwant: %s", got("plan_id"), want)
	}
	want := "answer_note event event_field event_task manual_cell membership person person_event person_section person_task plan plan_field plan_run pool_member share_link task task_combination task_section"
	if got("org_id") != want {
		t.Errorf("tables with org_id: %s\nwant: %s", got("org_id"), want)
	}
}

func TestRenamingChangesTheNameAndKeepsTheAddress(t *testing.T) {
	s := open(t)
	a := makeRich(t, s, "Club A", "Autumn 2026")
	b := makeRich(t, s, "Club B", "Autumn 2026") // same plan slug in the other organisation

	o, err := s.RenameOrg(ctx, a.org.Slug, "  Sports Club  ")
	if err != nil || o.Name != "Sports Club" || o.Slug != a.org.Slug {
		t.Fatalf("renamed organisation: %+v %v", o, err)
	}
	if got, err := s.GetOrg(ctx, a.org.Slug); err != nil || got.Name != "Sports Club" {
		t.Errorf("the old address still works: %+v %v", got, err)
	}
	p, err := s.RenamePlan(ctx, a.org.Slug, a.plan.Slug, "Fall")
	if err != nil || p.Name != "Fall" || p.Slug != a.plan.Slug {
		t.Fatalf("renamed plan: %+v %v", p, err)
	}
	if got, err := s.GetPlan(ctx, a.org.Slug, a.plan.Slug); err != nil || got.Name != "Fall" {
		t.Errorf("the old plan address still works: %+v %v", got, err)
	}
	if got, _ := s.GetPlan(ctx, b.org.Slug, b.plan.Slug); got.Name != "Autumn 2026" {
		t.Errorf("renaming A's plan changed B's: %+v", got)
	}
	if got, _ := s.GetOrg(ctx, b.org.Slug); got.Name != "Club B" {
		t.Errorf("renaming A changed B: %+v", got)
	}
	// the people stay the same
	if people, err := s.ListPeople(ctx, a.org.Slug); err != nil || len(people) == 0 || people[0].ID != a.people[0].ID {
		t.Errorf("the people after the rename: %+v %v", people, err)
	}

	for name, err := range map[string]error{
		"organisation to the same name": func() error { _, err := s.RenameOrg(ctx, a.org.Slug, "Sports Club"); return err }(),
		"plan to the same name":         func() error { _, err := s.RenamePlan(ctx, a.org.Slug, a.plan.Slug, "Fall"); return err }(),
	} {
		if !codeIs(err, "nothing_to_change") {
			t.Errorf("%s: %v", name, err)
		}
	}
	for name, err := range map[string]error{
		"empty organisation name": func() error { _, err := s.RenameOrg(ctx, a.org.Slug, "  "); return err }(),
		"long plan name": func() error {
			_, err := s.RenamePlan(ctx, a.org.Slug, a.plan.Slug, strings.Repeat("x", 81))
			return err
		}(),
	} {
		if !codeIs(err, "name_invalid") {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := s.RenameOrg(ctx, "nope", "X"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown organisation: %v", err)
	}
	for name, err := range map[string]error{
		"unknown plan":             func() error { _, err := s.RenamePlan(ctx, a.org.Slug, "nope", "X"); return err }(),
		"A's plan through nobody":  func() error { _, err := s.RenamePlan(ctx, "nope", a.plan.Slug, "X"); return err }(),
		"B's other plan through A": func() error { _, err := s.RenamePlan(ctx, a.org.Slug, b.other.Slug+"-x", "X"); return err }(),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestDeletingAPlanRemovesAllOfItAndNothingElse(t *testing.T) {
	s := open(t)
	a := makeRich(t, s, "Club A", "Autumn 2026")
	b := makeRich(t, s, "Club B", "Autumn 2026") // the same plan slug, the same everything
	planID := idOf(t, s, `SELECT p.id FROM plan p JOIN organization o ON o.id = p.org_id WHERE o.slug = ? AND p.slug = ?`, a.org.Slug, a.plan.Slug)
	otherID := idOf(t, s, `SELECT p.id FROM plan p JOIN organization o ON o.id = p.org_id WHERE o.slug = ? AND p.slug = ?`, a.org.Slug, a.other.Slug)
	planB := idOf(t, s, `SELECT p.id FROM plan p JOIN organization o ON o.id = p.org_id WHERE o.slug = ? AND p.slug = ?`, b.org.Slug, b.plan.Slug)
	orgA := idOf(t, s, `SELECT id FROM organization WHERE slug = ?`, a.org.Slug)
	orgB := idOf(t, s, `SELECT id FROM organization WHERE slug = ?`, b.org.Slug)

	before := rowsWith(t, s, "plan_id", planID)
	for _, table := range []string{"event", "manual_cell", "pool_member", "person_event", "plan_run", "event_task", "plan_field", "event_field"} {
		if before[table] == 0 {
			t.Fatalf("the plan must have rows in %s for this test to mean anything", table)
		}
	}
	otherBefore, planBBefore, orgBBefore := rowsWith(t, s, "plan_id", otherID), rowsWith(t, s, "plan_id", planB), rowsWith(t, s, "org_id", orgB)
	orgABefore := rowsWith(t, s, "org_id", orgA)

	for name, err := range map[string]error{
		"unknown plan":         s.DeletePlan(ctx, a.org.Slug, "nope"),
		"unknown organisation": s.DeletePlan(ctx, "nope", a.plan.Slug),
	} {
		if !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	sameCounts(t, "refused deletions", before, rowsWith(t, s, "plan_id", planID))

	if err := s.DeletePlan(ctx, a.org.Slug, a.plan.Slug); err != nil {
		t.Fatal(err)
	}
	for table, n := range rowsWith(t, s, "plan_id", planID) {
		if n != 0 {
			t.Errorf("%d rows of the deleted plan are left in %s", n, table)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM plan WHERE id = ?`, planID); n != 0 {
		t.Error("the plan row is still there")
	}
	sameCounts(t, "the same organisation's other plan", otherBefore, rowsWith(t, s, "plan_id", otherID))
	sameCounts(t, "the other organisation's plan with the same address", planBBefore, rowsWith(t, s, "plan_id", planB))
	sameCounts(t, "the other organisation", orgBBefore, rowsWith(t, s, "org_id", orgB))
	// the organisation keeps its people, tasks and sections
	after := rowsWith(t, s, "org_id", orgA)
	for _, table := range []string{"person", "task", "task_combination", "person_section", "person_task"} {
		if after[table] != orgABefore[table] {
			t.Errorf("deleting a plan changed the organisation's %s: %d -> %d", table, orgABefore[table], after[table])
		}
	}
	noBrokenReferences(t, s)

	if _, err := s.GetPlan(ctx, a.org.Slug, a.plan.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("the plan is gone: %v", err)
	}
	if _, err := s.GetRun(ctx, a.org.Slug, a.other.Slug, a.pinned); !errors.Is(err, ErrNotFound) {
		t.Errorf("the deleted plan's runs are not reachable through another plan: %v", err)
	}
	if got, err := s.GetPlan(ctx, b.org.Slug, b.plan.Slug); err != nil || got.Name != "Autumn 2026" {
		t.Errorf("the other organisation's plan with the same address: %+v %v", got, err)
	}
	// the address can be used again, and the new plan starts empty
	again, err := s.CreatePlan(ctx, a.org.Slug, "Autumn 2026")
	if err != nil || again.Slug != a.plan.Slug {
		t.Fatalf("a new plan on the same address: %+v %v", again, err)
	}
	if runs, _ := s.ListRuns(ctx, a.org.Slug, again.Slug, 10); len(runs) != 0 {
		t.Errorf("the new plan has the old runs: %+v", runs)
	}
	if evs, _ := s.ListEvents(ctx, a.org.Slug, again.Slug); len(evs) != 0 {
		t.Errorf("the new plan has the old events: %+v", evs)
	}
}

func TestAPlanWithARunInProgressCannotBeDeleted(t *testing.T) {
	s := open(t)
	a := makeRich(t, s, "Club A", "Autumn 2026")
	running, err := s.CreateRun(ctx, a.org.Slug, a.plan.Slug, 9, 5, "h", []byte("{}"), []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	err = s.DeletePlan(ctx, a.org.Slug, a.plan.Slug)
	if !codeIs(err, "plan_busy") || !errors.Is(err, ErrConflict) {
		t.Fatalf("a run in progress: %v", err)
	}
	if _, err := s.GetRun(ctx, a.org.Slug, a.plan.Slug, running); err != nil {
		t.Errorf("a refused deletion removed something: %v", err)
	}
	if evs, _ := s.ListEvents(ctx, a.org.Slug, a.plan.Slug); len(evs) != 2 {
		t.Errorf("a refused deletion removed events: %d", len(evs))
	}
	// another plan's run does not block this one
	if err := s.FinishRun(ctx, running, nil, "stopped"); err != nil {
		t.Fatal(err)
	}
	other, _ := s.CreateRun(ctx, a.org.Slug, a.other.Slug, 9, 5, "h", []byte("{}"), []byte("{}"))
	if err := s.DeletePlan(ctx, a.org.Slug, a.plan.Slug); err != nil {
		t.Errorf("a run in the other plan must not block: %v", err)
	}
	if _, err := s.GetRun(ctx, a.org.Slug, a.other.Slug, other); err != nil {
		t.Errorf("the other plan's run: %v", err)
	}
}

func TestDeletingAnOrganisationNeedsNoPlansAndTheAddressAndTakesAllOfIt(t *testing.T) {
	s := open(t)
	a := makeRich(t, s, "Club A", "Autumn 2026")
	b := makeRich(t, s, "Club B", "Autumn 2026")
	orgA := idOf(t, s, `SELECT id FROM organization WHERE slug = ?`, a.org.Slug)
	orgB := idOf(t, s, `SELECT id FROM organization WHERE slug = ?`, b.org.Slug)
	bBefore := rowsWith(t, s, "org_id", orgB)
	aBefore := rowsWith(t, s, "org_id", orgA)

	if _, err := s.DeleteOrg(ctx, "nope", "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown organisation: %v", err)
	}
	if _, err := s.DeleteOrg(ctx, a.org.Slug, "Club A"); !codeIs(err, "confirm_mismatch") || !errors.Is(err, ErrInvalid) {
		t.Errorf("the confirmation is the address name, not the name: %v", err)
	}
	if _, err := s.DeleteOrg(ctx, a.org.Slug, b.org.Slug); !codeIs(err, "confirm_mismatch") {
		t.Errorf("another organisation's address does not confirm this one: %v", err)
	}
	if _, err := s.DeleteOrg(ctx, a.org.Slug, a.org.Slug); !codeIs(err, "org_has_plans") || !errors.Is(err, ErrConflict) {
		t.Errorf("an organisation with plans: %v", err)
	}
	sameCounts(t, "refused deletions", aBefore, rowsWith(t, s, "org_id", orgA))

	for _, p := range []Plan{a.plan, a.other} {
		if err := s.DeletePlan(ctx, a.org.Slug, p.Slug); err != nil {
			t.Fatal(err)
		}
	}
	o, err := s.DeleteOrg(ctx, a.org.Slug, a.org.Slug)
	if err != nil || o.Name != "Club A" || o.Slug != a.org.Slug {
		t.Fatalf("deleted: %+v %v", o, err)
	}
	for table, n := range rowsWith(t, s, "org_id", orgA) {
		if n != 0 {
			t.Errorf("%d rows of the deleted organisation are left in %s", n, table)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM organization WHERE id = ?`, orgA); n != 0 {
		t.Error("the organisation row is still there")
	}
	if _, err := s.GetOrg(ctx, a.org.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("the organisation is gone: %v", err)
	}
	if _, err := s.GetPlan(ctx, a.org.Slug, a.plan.Slug); !errors.Is(err, ErrNotFound) {
		t.Errorf("its plans are gone: %v", err)
	}
	sameCounts(t, "the other organisation", bBefore, rowsWith(t, s, "org_id", orgB))
	noBrokenReferences(t, s)
	if orgs, _ := s.ListOrgs(ctx); len(orgs) != 1 || orgs[0].Slug != b.org.Slug {
		t.Errorf("organisations: %+v", orgs)
	}
	// the address is free again
	if again, err := s.CreateOrg(ctx, "Club A"); err != nil || again.Slug != a.org.Slug {
		t.Errorf("a new organisation on the same address: %+v %v", again, err)
	}
}
