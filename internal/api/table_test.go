package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestTheTablesDataOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	base := "/api/orgs/" + a.org + "/plans/" + a.plan
	ev := a.events[0]

	// places per event (both events start with one place for Sound)
	other := a.events[1]
	soundOf := base + "/events/" + itoa(ev.ID) + "/places/" + itoa(a.sound.ID)
	e.mustDo(204, "PUT", soundOf, map[string]int{"slots": 2})
	var places []store.EventPlaces
	e.mustDo(200, "GET", base+"/places", nil).json(&places)
	if fmt.Sprint(places) != fmt.Sprintf("[{%d %d 2} {%d %d 1}]", ev.ID, a.sound.ID, other.ID, a.sound.ID) {
		t.Errorf("places: %+v", places)
	}
	if r := e.do("PUT", soundOf, map[string]int{"slots": 2}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("the same number again: %d %s", r.code, r.body)
	}
	for name, body := range map[string]any{"too many": map[string]int{"slots": 51}, "missing": map[string]any{}} {
		if r := e.do("PUT", base+"/events/"+itoa(ev.ID)+"/places/"+itoa(a.sound.ID), body); r.code != 400 {
			t.Errorf("places %s: %d %s", name, r.code, r.body)
		}
	}
	e.mustDo(204, "DELETE", soundOf, nil)
	e.mustDo(204, "PUT", base+"/events/"+itoa(other.ID)+"/places/"+itoa(a.sound.ID), map[string]int{"slots": 0})
	e.mustDo(200, "GET", base+"/places", nil).json(&places)
	if len(places) != 0 {
		t.Errorf("after DELETE and 0 places, Sound is needed on neither: %+v", places)
	}

	// info rows
	var f store.PlanField
	e.mustDo(201, "POST", base+"/fields", map[string]string{"name": "Theme"}).json(&f)
	e.mustDo(200, "PATCH", base+"/fields/"+itoa(f.ID), map[string]string{"name": "Motto"}).json(&f)
	if f.Name != "Motto" {
		t.Errorf("rename: %+v", f)
	}
	if r := e.do("POST", base+"/fields", map[string]string{"name": "Motto"}); r.code != 409 {
		t.Errorf("the same name twice: %d %s", r.code, r.body)
	}
	e.mustDo(204, "PUT", base+"/events/"+itoa(ev.ID)+"/fields/"+itoa(f.ID), map[string]string{"value": "Light"})
	var note store.PlanField
	e.mustDo(201, "POST", base+"/fields", map[string]string{"name": "Note"}).json(&note)
	var fields store.PlanFields
	e.mustDo(200, "GET", base+"/fields", nil).json(&fields)
	if fmt.Sprint(fields.Fields) != fmt.Sprintf("[{%d Motto 0 false} {%d Note 1 false}]", f.ID, note.ID) || fmt.Sprint(fields.Values) != fmt.Sprintf("[{%d %d Light}]", ev.ID, f.ID) {
		t.Errorf("fields: %+v", fields)
	}
	// a frozen row: kept, and kept through a rename; off again
	e.mustDo(204, "PUT", base+"/fields/"+itoa(note.ID)+"/frozen", map[string]bool{"frozen": true})
	e.mustDo(200, "PATCH", base+"/fields/"+itoa(note.ID), map[string]string{"name": "Notes"}).json(&note)
	if !note.Frozen || note.Name != "Notes" {
		t.Errorf("frozen and renamed: %+v", note)
	}
	e.mustDo(200, "GET", base+"/fields", nil).json(&fields)
	if !fields.Fields[1].Frozen || fields.Fields[0].Frozen {
		t.Errorf("only Notes is frozen: %+v", fields.Fields)
	}
	if r := e.do("PUT", base+"/fields/"+itoa(note.ID)+"/frozen", map[string]any{}); r.code != 400 || !strings.Contains(string(r.body), "frozen_missing") {
		t.Errorf("frozen left out: %d %s", r.code, r.body)
	}
	if r := e.do("PUT", base+"/fields/999999/frozen", map[string]bool{"frozen": true}); r.code != 404 {
		t.Errorf("an unknown row: %d %s", r.code, r.body)
	}
	e.mustDo(204, "PUT", base+"/fields/"+itoa(note.ID)+"/frozen", map[string]bool{"frozen": false})
	e.mustDo(200, "GET", base+"/fields", nil).json(&fields)
	if fields.Fields[1].Frozen {
		t.Errorf("frozen off: %+v", fields.Fields)
	}
	e.mustDo(200, "PATCH", base+"/fields/"+itoa(note.ID), map[string]string{"name": "Note"}).json(&note)
	// moving a row to the top
	e.mustDo(204, "PUT", base+"/fields/"+itoa(note.ID)+"/position", map[string]int{"position": 0})
	e.mustDo(200, "GET", base+"/fields", nil).json(&fields)
	if fmt.Sprint(fields.Fields) != fmt.Sprintf("[{%d Note 0 false} {%d Motto 1 false}]", note.ID, f.ID) {
		t.Errorf("moved: %+v", fields.Fields)
	}
	for name, body := range map[string]any{"negative": map[string]int{"position": -1}, "missing": map[string]any{}} {
		if r := e.do("PUT", base+"/fields/"+itoa(note.ID)+"/position", body); r.code != 400 {
			t.Errorf("position %s: %d %s", name, r.code, r.body)
		}
	}
	e.mustDo(204, "DELETE", base+"/fields/"+itoa(note.ID), nil)
	e.mustDo(204, "DELETE", base+"/fields/"+itoa(f.ID), nil)
	e.mustDo(200, "GET", base+"/fields", nil).json(&fields)
	if len(fields.Fields) != 0 || len(fields.Values) != 0 {
		t.Errorf("after the delete: %+v", fields)
	}

	// everyone's answers
	e.mustDo(204, "PUT", base+"/people/"+itoa(a.anna.ID)+"/availability", map[string]any{"levels": map[string]string{itoa(ev.ID): "poor"}})
	var answers store.Answers
	e.mustDo(200, "GET", base+"/answers", nil).json(&answers)
	if fmt.Sprint(answers.Events) != fmt.Sprintf("[{%d %d poor}]", a.anna.ID, ev.ID) || len(answers.Tasks) != 1 || answers.Tasks[0].TaskID != a.sound.ID {
		t.Errorf("answers: %+v", answers)
	}

	// one task against all the others
	e.mustDo(204, "PUT", "/api/orgs/"+a.org+"/plans/"+term+"/tasks/"+itoa(a.sound.ID)+"/combinations", map[string]string{"level": "cannot"})
	var pairs []store.Combination
	e.mustDo(200, "GET", "/api/orgs/"+a.org+"/plans/"+term+"/combinations", nil).json(&pairs)
	if len(pairs) != 1 || pairs[0].Level != "cannot" {
		t.Errorf("combinations: %+v", pairs)
	}
	if r := e.do("PUT", "/api/orgs/"+a.org+"/plans/"+term+"/tasks/"+itoa(a.sound.ID)+"/combinations", map[string]string{"level": "cannot"}); r.code != 400 {
		t.Errorf("nothing to change: %d %s", r.code, r.body)
	}
}

func TestTheTablesDataIs404AcrossPlansAndOrganisations(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	b := e.cellsWorld("Org B")
	baseA := "/api/orgs/" + a.org + "/plans/" + a.plan
	baseB := "/api/orgs/" + b.org + "/plans/" + b.plan // the same plan slug in another organisation
	var fa store.PlanField
	e.mustDo(201, "POST", baseA+"/fields", map[string]string{"name": "Theme"}).json(&fa)
	evA := itoa(a.events[0].ID)

	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"PUT", baseB + "/events/" + evA + "/places/" + itoa(b.sound.ID), map[string]int{"slots": 2}},
		{"PUT", baseA + "/events/" + evA + "/places/" + itoa(b.sound.ID), map[string]int{"slots": 2}},
		{"PUT", baseA + "/events/" + itoa(a.otherPlanEvent.ID) + "/places/" + itoa(a.sound.ID), map[string]int{"slots": 2}},
		{"DELETE", baseB + "/events/" + evA + "/places/" + itoa(a.sound.ID), nil},
		{"PUT", baseA + "/events/" + evA + "/places/abc", map[string]int{"slots": 2}},
		{"PUT", baseA + "/events/abc/places/" + itoa(a.sound.ID), map[string]int{"slots": 2}},
		{"GET", "/api/orgs/" + a.org + "/plans/nope/places", nil},
		{"PATCH", baseB + "/fields/" + itoa(fa.ID), map[string]string{"name": "Mine"}},
		{"DELETE", baseB + "/fields/" + itoa(fa.ID), nil},
		{"PUT", baseB + "/events/" + itoa(b.events[0].ID) + "/fields/" + itoa(fa.ID), map[string]string{"value": "x"}},
		{"PUT", baseA + "/events/" + itoa(a.otherPlanEvent.ID) + "/fields/" + itoa(fa.ID), map[string]string{"value": "x"}},
		{"PUT", baseA + "/events/" + evA + "/fields/abc", map[string]string{"value": "x"}},
		{"GET", "/api/orgs/nope/plans/" + a.plan + "/fields", nil},
		{"POST", "/api/orgs/" + a.org + "/plans/nope/fields", map[string]string{"name": "Theme"}},
		{"GET", "/api/orgs/" + a.org + "/plans/nope/answers", nil},
		{"PUT", "/api/orgs/" + b.org + "/plans/" + term + "/tasks/" + itoa(a.sound.ID) + "/combinations", map[string]string{"level": "cannot"}},
	} {
		if r := e.do(c.method, c.path, c.body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404", c.method, c.path, r.code, r.body)
		}
	}
	var fields store.PlanFields
	e.mustDo(200, "GET", baseA+"/fields", nil).json(&fields)
	if len(fields.Fields) != 1 || fields.Fields[0].Name != "Theme" || len(fields.Values) != 0 {
		t.Errorf("A's row must be untouched: %+v", fields)
	}
	e.mustDo(200, "GET", baseB+"/fields", nil).json(&fields)
	if len(fields.Fields) != 0 {
		t.Errorf("B sees A's rows: %+v", fields)
	}
	var places []store.EventPlaces
	e.mustDo(200, "GET", baseA+"/places", nil).json(&places)
	if fmt.Sprint(places) != fmt.Sprintf("[{%d %d 1} {%d %d 1}]", a.events[0].ID, a.sound.ID, a.events[1].ID, a.sound.ID) {
		t.Errorf("places were written across plans or organisations: %+v", places)
	}
}

func TestMovingATaskOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	tasks := "/api/orgs/" + a.org + "/plans/" + term + "/tasks"
	e.mustDo(204, "PUT", tasks+"/"+itoa(a.sang.ID)+"/position", map[string]int{"position": 0})
	var list []store.Task
	e.mustDo(200, "GET", tasks, nil).json(&list)
	if len(list) < 2 || list[0].ID != a.sang.ID || list[1].ID != a.sound.ID {
		t.Errorf("moved to the top: %+v", list)
	}
	for name, body := range map[string]any{"negative": map[string]int{"position": -1}, "missing": map[string]any{}} {
		if r := e.do("PUT", tasks+"/"+itoa(a.sang.ID)+"/position", body); r.code != 400 {
			t.Errorf("position %s: %d %s", name, r.code, r.body)
		}
	}
	b := e.cellsWorld("Org B")
	if r := e.do("PUT", "/api/orgs/"+b.org+"/plans/"+term+"/tasks/"+itoa(a.sang.ID)+"/position", map[string]int{"position": 0}); r.code != 404 {
		t.Errorf("A's task through B: %d", r.code)
	}
	if r := e.do("PUT", "/api/orgs/"+b.org+"/plans/"+b.plan+"/tasks/"+itoa(a.sound.ID)+"/places", map[string]int{"slots": 2}); r.code != 404 {
		t.Errorf("filling A's task through B: %d", r.code)
	}
}

func TestSectionsAndMovingPeopleOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	o := "/api/orgs/" + a.org + "/plans/" + a.plan
	var x store.Section
	e.mustDo(201, "POST", o+"/sections", map[string]string{"name": "Guests"}).json(&x)
	var y store.Section
	e.mustDo(201, "POST", o+"/sections", map[string]string{"name": "Youth"}).json(&y)
	e.mustDo(200, "PATCH", o+"/sections/"+itoa(x.ID), map[string]string{"name": "Visitors"}).json(&x)
	if x.Name != "Visitors" {
		t.Errorf("rename: %+v", x)
	}
	if r := e.do("POST", o+"/sections", map[string]string{"name": "Youth"}); r.code != 409 {
		t.Errorf("the same name twice: %d", r.code)
	}
	e.mustDo(204, "PUT", o+"/sections/"+itoa(y.ID)+"/position", map[string]int{"position": 0})
	var list []store.Section
	e.mustDo(200, "GET", o+"/sections", nil).json(&list)
	if fmt.Sprint(list) != fmt.Sprintf("[{%d Youth} {%d Visitors}]", y.ID, x.ID) {
		t.Errorf("sections: %+v", list)
	}
	e.mustDo(204, "PUT", o+"/people/"+itoa(a.anna.ID)+"/position", map[string]any{"section": x.ID, "position": 0})
	var people []store.Person
	e.mustDo(200, "GET", o+"/people", nil).json(&people)
	last := people[len(people)-1]
	if last.ID != a.anna.ID || last.Section != x.ID {
		t.Errorf("Anna is in Visitors, the last section: %+v", people)
	}
	for name, body := range map[string]any{"negative": map[string]any{"position": -1}, "missing": map[string]any{"section": x.ID}} {
		if r := e.do("PUT", o+"/people/"+itoa(a.anna.ID)+"/position", body); r.code != 400 {
			t.Errorf("person position %s: %d %s", name, r.code, r.body)
		}
		if r := e.do("PUT", o+"/sections/"+itoa(x.ID)+"/position", body); r.code != 400 {
			t.Errorf("section position %s: %d %s", name, r.code, r.body)
		}
	}
	b := e.cellsWorld("Org B")
	for what, r := range map[string]reply{
		"A's section through B":     e.do("DELETE", "/api/orgs/"+b.org+"/plans/"+term+"/sections/"+itoa(x.ID), nil),
		"B's person into A's":       e.do("PUT", "/api/orgs/"+b.org+"/plans/"+term+"/people/"+itoa(b.anna.ID)+"/position", map[string]any{"section": x.ID, "position": 0}),
		"rename A's section from B": e.do("PATCH", "/api/orgs/"+b.org+"/plans/"+term+"/sections/"+itoa(x.ID), map[string]string{"name": "X"}),
	} {
		if r.code != 404 {
			t.Errorf("%s: %d", what, r.code)
		}
	}
	e.mustDo(204, "DELETE", o+"/sections/"+itoa(x.ID), nil)
	e.mustDo(200, "GET", o+"/people", nil).json(&people)
	for _, p := range people {
		if p.Section != 0 {
			t.Errorf("after the delete nobody is in a section: %+v", people)
		}
	}
}

// «Add a person» under a section puts the new person last in that section; another organisation's section is not found.
func TestAddingAPersonInASection(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	b := e.cellsWorld("Org B")
	var guests, away store.Section
	e.mustDo(201, "POST", "/api/orgs/"+a.org+"/plans/"+term+"/sections", map[string]string{"name": "Guests"}).json(&guests)
	e.mustDo(201, "POST", "/api/orgs/"+b.org+"/plans/"+term+"/sections", map[string]string{"name": "Away"}).json(&away)
	var p store.Person
	e.mustDo(201, "POST", "/api/orgs/"+a.org+"/plans/"+term+"/people", map[string]any{"name": "Pia Demo", "section": guests.ID}).json(&p)
	if p.Section != guests.ID {
		t.Fatalf("the new person is not in the section: %+v", p)
	}
	var people []store.Person
	e.mustDo(200, "GET", "/api/orgs/"+a.org+"/plans/"+term+"/people", nil).json(&people)
	if last := people[len(people)-1]; last.ID != p.ID || last.Section != guests.ID {
		t.Fatalf("not last, in the section: %+v", people)
	}
	e.mustDo(201, "POST", "/api/orgs/"+a.org+"/plans/"+term+"/people", map[string]any{"name": "Jon Demo"}).json(&p)
	if p.Section != 0 {
		t.Fatalf("without a section: %+v", p)
	}
	for _, sec := range []int64{away.ID, 99999} {
		if r := e.do("POST", "/api/orgs/"+a.org+"/plans/"+term+"/people", map[string]any{"name": "Ola Sample", "section": sec}); r.code != 404 {
			t.Errorf("section %d: %d %s, want 404", sec, r.code, r.body)
		}
	}
}

// Task sections (2026-11-23) over HTTP: their own routes (not the people's), a task made in one or moved into one (and
// moved within it when section is left out), 404 for another organisation's.
func TestTaskSectionsOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	o := "/api/orgs/" + a.org + "/plans/" + a.plan
	var tech store.Section
	e.mustDo(201, "POST", o+"/task-sections", map[string]string{"name": "Tech"}).json(&tech)
	e.mustDo(200, "PATCH", o+"/task-sections/"+itoa(tech.ID), map[string]string{"name": "Technique"}).json(&tech)
	var list []store.Section
	e.mustDo(200, "GET", o+"/sections", nil).json(&list)
	if len(list) != 0 {
		t.Errorf("a task section is not a people's section: %+v", list)
	}
	e.mustDo(200, "GET", o+"/task-sections", nil).json(&list)
	if len(list) != 1 || list[0].Name != "Technique" {
		t.Errorf("task sections: %+v", list)
	}
	var lights store.Task
	e.mustDo(201, "POST", o+"/tasks", map[string]any{"name": "Lights", "section": tech.ID}).json(&lights)
	e.mustDo(204, "PUT", o+"/tasks/"+itoa(a.sound.ID)+"/position", map[string]any{"section": tech.ID, "position": 0})
	var tasks []store.Task
	e.mustDo(200, "GET", o+"/tasks", nil).json(&tasks)
	got := ""
	for _, x := range tasks {
		got += fmt.Sprintf("%s/%v ", x.Name, x.Section == tech.ID)
	}
	if !strings.HasSuffix(got, "Sound/true Lights/true ") {
		t.Errorf("the section's tasks last, Sound first in it: %s", got)
	}
	e.mustDo(204, "PUT", o+"/tasks/"+itoa(a.sound.ID)+"/position", map[string]any{"position": 9})
	e.mustDo(200, "GET", o+"/tasks", nil).json(&tasks)
	if last := tasks[len(tasks)-1]; last.ID != a.sound.ID || last.Section != tech.ID {
		t.Errorf("section left out: moved within its section: %+v", tasks)
	}
	b := e.cellsWorld("Org B")
	for what, r := range map[string]reply{
		"A's task section through B": e.do("DELETE", "/api/orgs/"+b.org+"/plans/"+term+"/task-sections/"+itoa(tech.ID), nil),
		"B's task into A's section":  e.do("PUT", "/api/orgs/"+b.org+"/plans/"+term+"/tasks/"+itoa(b.sound.ID)+"/position", map[string]any{"section": tech.ID, "position": 0}),
		"a task made in A's section": e.do("POST", "/api/orgs/"+b.org+"/plans/"+term+"/tasks", map[string]any{"name": "Bar", "section": tech.ID}),
		"rename it from B":           e.do("PATCH", "/api/orgs/"+b.org+"/plans/"+term+"/task-sections/"+itoa(tech.ID), map[string]string{"name": "X"}),
		"move it from B":             e.do("PUT", "/api/orgs/"+b.org+"/plans/"+term+"/task-sections/"+itoa(tech.ID)+"/position", map[string]int{"position": 0}),
	} {
		if r.code != 404 {
			t.Errorf("%s: %d", what, r.code)
		}
	}
	e.mustDo(204, "DELETE", o+"/task-sections/"+itoa(tech.ID), nil)
	e.mustDo(200, "GET", o+"/tasks", nil).json(&tasks)
	for _, x := range tasks {
		if x.Section != 0 {
			t.Errorf("after the delete no task is in a section: %+v", tasks)
		}
	}
}
