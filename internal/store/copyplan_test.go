package store

import (
	"fmt"
	"sort"
	"strings"
	"testing"
)

// planShape is a plan by names, without ids: its people and tasks with their sections and choices, and its table.
func planShape(t *testing.T, s *Store, org, plan string) (setup, table string) {
	t.Helper()
	people, err := s.ListPeople(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	tasks, _ := s.ListTasks(ctx, org, plan)
	ps, _ := s.ListSections(ctx, org, plan)
	ts, _ := s.ListSectionsOf(ctx, Tasks, org, plan)
	combos, _ := s.ListCombinations(ctx, org, plan)
	answers, _ := s.ListAnswers(ctx, org, plan)
	events, _ := s.ListEvents(ctx, org, plan)
	places, _ := s.ListEventPlaces(ctx, org, plan)
	fields, _ := s.ListFields(ctx, org, plan)
	cells, _ := s.ListCells(ctx, org, plan)
	pools, _ := s.ListPools(ctx, org, plan)
	weights, _ := s.GetWeights(ctx, org, plan)
	name := map[string]string{}
	for _, x := range ps {
		name[fmt.Sprint("s", x.ID)] = x.Name
	}
	for _, x := range ts {
		name[fmt.Sprint("ts", x.ID)] = x.Name
	}
	for _, x := range people {
		name[fmt.Sprint("p", x.ID)] = x.Name
	}
	for _, x := range tasks {
		name[fmt.Sprint("t", x.ID)] = x.Name
	}
	for _, x := range events {
		name[fmt.Sprint("e", x.ID)] = x.Date + " " + x.StartTime
	}
	for _, x := range fields.Fields {
		name[fmt.Sprint("f", x.ID)] = x.Name
	}
	var a, b []string
	for _, x := range people {
		a = append(a, fmt.Sprintf("person %s %v in %s", x.Name, x.Proposable, name[fmt.Sprint("s", x.Section)]))
	}
	for _, x := range tasks {
		a = append(a, fmt.Sprintf("task %s %v in %s", x.Name, x.Planned, name[fmt.Sprint("ts", x.Section)]))
	}
	for _, x := range combos {
		a = append(a, fmt.Sprintf("combination %s %s %s", name[fmt.Sprint("t", x.A)], name[fmt.Sprint("t", x.B)], x.Level))
	}
	for _, x := range answers.Tasks {
		a = append(a, fmt.Sprintf("takes %s %s %s", name[fmt.Sprint("p", x.PersonID)], name[fmt.Sprint("t", x.TaskID)], x.Level))
	}
	for _, x := range events {
		b = append(b, "event "+name[fmt.Sprint("e", x.ID)]+" "+x.EndTime)
	}
	for _, x := range places {
		b = append(b, fmt.Sprintf("places %s %s %d", name[fmt.Sprint("e", x.EventID)], name[fmt.Sprint("t", x.TaskID)], x.Slots))
	}
	for _, x := range fields.Fields {
		b = append(b, fmt.Sprintf("field %s %v", x.Name, x.Frozen))
	}
	for _, x := range fields.Values {
		b = append(b, fmt.Sprintf("value %s %s %s", name[fmt.Sprint("e", x.EventID)], name[fmt.Sprint("f", x.FieldID)], x.Value))
	}
	for _, x := range cells {
		who := x.Label
		if x.PersonID != nil {
			who = name[fmt.Sprint("p", *x.PersonID)]
		}
		b = append(b, fmt.Sprintf("cell %s %s %s %v", name[fmt.Sprint("e", x.EventID)], name[fmt.Sprint("t", x.TaskID)], who, x.Locked))
	}
	for _, x := range pools {
		for _, p := range x.People {
			b = append(b, fmt.Sprintf("pool %s %s %s", name[fmt.Sprint("e", x.EventID)], name[fmt.Sprint("t", x.TaskID)], name[fmt.Sprint("p", p)]))
		}
	}
	for _, x := range answers.Events {
		b = append(b, fmt.Sprintf("answer %s %s %s", name[fmt.Sprint("p", x.PersonID)], name[fmt.Sprint("e", x.EventID)], x.Level))
	}
	for _, x := range answers.Notes {
		b = append(b, fmt.Sprintf("note %s %s %s", name[fmt.Sprint("p", x.PersonID)], name[fmt.Sprint("e", x.EventID)], x.Note))
	}
	b = append(b, fmt.Sprint("weights ", weights))
	sort.Strings(a)
	sort.Strings(b)
	return strings.Join(a, "\n"), strings.Join(b, "\n")
}

// A new plan starts with another plan's people and tasks (sections, ⚡, combinations, the tasks people take), or empty; a
// duplicate takes the filled-in table too (2026-11-23, the user: "include the copy/start fresh. Make a duplicate button …
// with option to keep the filled table values or not"). Drafts and links are never copied, nor the planner's locked names.
func TestAPlanIsCopiedWithOrWithoutItsTable(t *testing.T) {
	s := open(t)
	r := makeRich(t, s, "Club", "Autumn 2026") // one of everything, in two plans
	slug := r.org.Slug
	setup, table := planShape(t, s, slug, r.plan.Slug)
	if !strings.Contains(table, "cell") || !strings.Contains(setup, "takes") || !strings.Contains(setup, "combination") {
		t.Fatalf("the plan must have something to copy:\n%s\n%s", setup, table)
	}
	// a locked name of the planner's, which belongs to the drafts
	evs, _ := s.ListEvents(ctx, slug, r.plan.Slug)
	if err := s.SetLocks(ctx, slug, r.plan.Slug, []Lock{{Event: evs[1].ID, Task: r.sound.ID, Person: r.people[0].ID}}, true); err != nil {
		t.Fatal(err)
	}

	setupOnly, err := s.CreatePlanFrom(ctx, slug, NewPlan{Name: "Spring 2027", From: r.plan.Slug})
	if err != nil {
		t.Fatal(err)
	}
	gotSetup, gotTable := planShape(t, s, slug, setupOnly.Slug)
	if gotSetup != setup || gotTable != "weights map[]" {
		t.Errorf("copied from autumn:\n%s\nwant\n%s\ntable:\n%s", gotSetup, setup, gotTable)
	}
	dup, err := s.CreatePlanFrom(ctx, slug, NewPlan{Name: "Autumn 2026 (copy)", From: r.plan.Slug, Table: true})
	if err != nil {
		t.Fatal(err)
	}
	gotSetup, gotTable = planShape(t, s, slug, dup.Slug)
	if gotSetup != setup || gotTable != table {
		t.Errorf("the duplicate:\n%s\n%s\nwant\n%s\n%s", gotSetup, gotTable, setup, table)
	}
	if runs, _ := s.ListRuns(ctx, slug, dup.Slug, 10); len(runs) != 0 {
		t.Errorf("no drafts are copied: %+v", runs)
	}
	if links, _ := s.ListLinks(ctx, slug); len(links) != 0 {
		t.Errorf("no links: %+v", links)
	}
	// the copies are the new plan's own: a change there leaves autumn alone
	people, _ := s.ListPeople(ctx, slug, dup.Slug)
	if people[0].ID == r.people[0].ID {
		t.Fatal("the duplicate has ids of its own")
	}
	if _, err := s.UpdatePerson(ctx, slug, dup.Slug, people[0].ID, PersonPatch{Name: ptr("Kari Renamed")}); err != nil {
		t.Fatal(err)
	}
	if again, _ := planShape(t, s, slug, r.plan.Slug); again != setup {
		t.Errorf("autumn changed with its copy:\n%s", again)
	}
	empty, _ := s.CreatePlan(ctx, slug, "Summer 2027")
	if gotSetup, _ := planShape(t, s, slug, empty.Slug); gotSetup != "" {
		t.Errorf("an empty plan: %q", gotSetup)
	}
	// only a plan of the organisation, and the usual name rules
	other := makeRich(t, s, "Other", "Elsewhere")
	if _, err := s.CreatePlanFrom(ctx, slug, NewPlan{Name: "Stolen", From: other.plan.Slug}); err == nil {
		t.Error("a plan of another organisation is not found")
	}
	if _, err := s.CreatePlanFrom(ctx, slug, NewPlan{Name: "Autumn 2026 (copy)", From: r.plan.Slug, Table: true}); !codeIs(err, "already_exists") {
		t.Errorf("the same name twice: %v", err)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM plan WHERE slug LIKE 'stolen%'`); n != 0 {
		t.Errorf("a refused copy leaves nothing: %d", n)
	}
	noBrokenReferences(t, s)
}
