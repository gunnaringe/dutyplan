package store

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// an invented semester: no real names (the repo is public)
func sampleImport() Import {
	return Import{
		Plan:   "Spring 2031",
		Tasks:  []string{"sound", "Coffee", "Welcome"},
		Fields: []string{"Theme"},
		Events: []ImportEvent{
			{Date: "2031-01-12", StartTime: "11:00", Description: "Service", Fields: map[string]string{"Theme": "Light"}, Places: map[string]int{"Sound": 1, "coffee": 2}},
			{Date: "2031-01-15", Description: "Prayer evening", Places: map[string]int{"Welcome": 0}},
			{Date: "2031-01-19", StartTime: "11:00", EndTime: "12:30", Places: map[string]int{"Sound": 1, "Welcome": 1}},
		},
	}
}

func tasksOf(t *testing.T, s *Store, org string) string {
	ts, err := s.ListTasks(ctx, org)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, x := range ts {
		names = append(names, x.Name)
	}
	return strings.Join(names, ",")
}

func TestAnImportMakesAWholePlan(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Choir")
	if _, err := s.CreateTask(ctx, o.Slug, "Sound"); err != nil {
		t.Fatal(err)
	}
	res, err := s.Import(ctx, o.Slug, sampleImport(), false)
	if err != nil {
		t.Fatal(err)
	}
	if res.Plan.Slug != "spring-2031" || res.Events != 3 || res.Places != 4 || strings.Join(res.NewTasks, ",") != "Coffee,Welcome" {
		t.Fatalf("result: %+v", res)
	}
	if got := tasksOf(t, s, o.Slug); got != "Sound,Coffee,Welcome" {
		t.Fatalf("tasks: %s (an existing task is matched whatever the case)", got)
	}
	evs, err := s.ListEvents(ctx, o.Slug, "spring-2031")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range evs {
		got = append(got, e.Date+" "+e.StartTime+"-"+e.EndTime+" "+e.Description)
	}
	if want := "2031-01-12 11:00- Service|2031-01-15 - Prayer evening|2031-01-19 11:00-12:30 "; strings.Join(got, "|") != want {
		t.Fatalf("events:\n%s\nwant\n%s", strings.Join(got, "|"), want)
	}
	ps, err := s.ListEventPlaces(ctx, o.Slug, "spring-2031")
	if err != nil {
		t.Fatal(err)
	}
	var places []string
	for _, p := range ps {
		places = append(places, fmt.Sprintf("%d/%d=%d", p.EventID-evs[0].ID, p.TaskID, p.Slots))
	}
	sort.Strings(places)
	if len(places) != 4 {
		t.Fatalf("places: %v", places)
	}
	fs, err := s.ListFields(ctx, o.Slug, "spring-2031")
	if err != nil {
		t.Fatal(err)
	}
	if len(fs.Fields) != 1 || fs.Fields[0].Name != "Theme" || len(fs.Values) != 1 || fs.Values[0].Value != "Light" || fs.Values[0].EventID != evs[0].ID {
		t.Fatalf("fields: %+v", fs)
	}
}

func TestAnImportIsAllOrNothing(t *testing.T) {
	s := open(t)
	o := mustOrg(t, s, "Choir")
	for name, change := range map[string]func(*Import){
		"a bad date at the end":    func(in *Import) { in.Events[2].Date = "2031-02-30" },
		"a task that is not there": func(in *Import) { in.Events[2].Places["Bells"] = 1 },
		"a field that is not there": func(in *Import) {
			in.Events[1].Fields = map[string]string{"Colour": "Red"}
		},
		"too many places": func(in *Import) { in.Events[2].Places["Sound"] = 51 },
		"no events":       func(in *Import) { in.Events = nil },
		"no plan name":    func(in *Import) { in.Plan = " " },
	} {
		in := sampleImport()
		change(&in)
		if _, err := s.Import(ctx, o.Slug, in, false); err == nil {
			t.Errorf("%s: imported", name)
		}
		if plans, _ := s.ListPlans(ctx, o.Slug); len(plans) != 0 {
			t.Errorf("%s: a plan was left: %+v", name, plans)
		}
		if got := tasksOf(t, s, o.Slug); got != "" {
			t.Errorf("%s: tasks were left: %s", name, got)
		}
	}
	// a task or field that is not in the lists is named as such, not just «not found»
	for name, change := range map[string]func(*Import){
		"task":  func(in *Import) { in.Events[2].Places["Bells"] = 1 },
		"field": func(in *Import) { in.Events[1].Fields = map[string]string{"Colour": "Red"} },
	} {
		in := sampleImport()
		change(&in)
		var c *Coded
		if _, err := s.Import(ctx, o.Slug, in, false); !errors.As(err, &c) || c.Code != "import_unknown" {
			t.Errorf("an unknown %s: %v, want import_unknown", name, err)
		}
	}
	// a dry run checks it all and stores nothing
	res, err := s.Import(ctx, o.Slug, sampleImport(), true)
	if err != nil || res.Events != 3 {
		t.Fatalf("dry run: %+v %v", res, err)
	}
	if plans, _ := s.ListPlans(ctx, o.Slug); len(plans) != 0 || tasksOf(t, s, o.Slug) != "" {
		t.Fatal("a dry run stored something")
	}
	// the plan must be new, and only in an organisation that exists
	if _, err := s.Import(ctx, o.Slug, sampleImport(), false); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Import(ctx, o.Slug, sampleImport(), false); err == nil {
		t.Fatal("the same plan imported twice")
	}
	if _, err := s.Import(ctx, "nobody", sampleImport(), false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an organisation that is not there: %v", err)
	}
	// and ordinary writes after an import start their own transactions again
	if _, err := s.CreateTask(ctx, o.Slug, "Bells"); err != nil {
		t.Fatal(err)
	}
}
