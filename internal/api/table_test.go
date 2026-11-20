package api

import (
	"fmt"
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
	if fmt.Sprint(fields.Fields) != fmt.Sprintf("[{%d Motto 0} {%d Note 1}]", f.ID, note.ID) || fmt.Sprint(fields.Values) != fmt.Sprintf("[{%d %d Light}]", ev.ID, f.ID) {
		t.Errorf("fields: %+v", fields)
	}
	// moving a row to the top
	e.mustDo(204, "PUT", base+"/fields/"+itoa(note.ID)+"/position", map[string]int{"position": 0})
	e.mustDo(200, "GET", base+"/fields", nil).json(&fields)
	if fmt.Sprint(fields.Fields) != fmt.Sprintf("[{%d Note 0} {%d Motto 1}]", note.ID, f.ID) {
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
	e.mustDo(204, "PUT", "/api/orgs/"+a.org+"/tasks/"+itoa(a.sound.ID)+"/combinations", map[string]string{"level": "cannot"})
	var pairs []store.Combination
	e.mustDo(200, "GET", "/api/orgs/"+a.org+"/combinations", nil).json(&pairs)
	if len(pairs) != 1 || pairs[0].Level != "cannot" {
		t.Errorf("combinations: %+v", pairs)
	}
	if r := e.do("PUT", "/api/orgs/"+a.org+"/tasks/"+itoa(a.sound.ID)+"/combinations", map[string]string{"level": "cannot"}); r.code != 400 {
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
		{"PUT", "/api/orgs/" + b.org + "/tasks/" + itoa(a.sound.ID) + "/combinations", map[string]string{"level": "cannot"}},
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

func TestFillingARowAndMovingATaskOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	base := "/api/orgs/" + a.org + "/plans/" + a.plan
	fill := base + "/tasks/" + itoa(a.sound.ID) + "/places"
	e.mustDo(204, "PUT", fill, map[string]int{"slots": 3})
	var places []store.EventPlaces
	e.mustDo(200, "GET", base+"/places", nil).json(&places)
	if len(places) != len(a.events) {
		t.Fatalf("one row per event: %+v", places)
	}
	for _, p := range places {
		if p.TaskID != a.sound.ID || p.Slots != 3 {
			t.Errorf("filled: %+v", places)
		}
	}
	for name, body := range map[string]any{"too many": map[string]int{"slots": 51}, "missing": map[string]any{}, "the same": map[string]int{"slots": 3}} {
		if r := e.do("PUT", fill, body); r.code != 400 {
			t.Errorf("fill %s: %d %s", name, r.code, r.body)
		}
	}
	if r := e.do("PUT", base+"/tasks/x/places", map[string]int{"slots": 1}); r.code != 404 {
		t.Errorf("a malformed task: %d", r.code)
	}
	e.mustDo(204, "PUT", fill, map[string]int{"slots": 0})
	e.mustDo(200, "GET", base+"/places", nil).json(&places)
	if len(places) != 0 {
		t.Errorf("0 clears the row: %+v", places)
	}

	tasks := "/api/orgs/" + a.org + "/tasks"
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
	if r := e.do("PUT", "/api/orgs/"+b.org+"/tasks/"+itoa(a.sang.ID)+"/position", map[string]int{"position": 0}); r.code != 404 {
		t.Errorf("A's task through B: %d", r.code)
	}
	if r := e.do("PUT", "/api/orgs/"+b.org+"/plans/"+b.plan+"/tasks/"+itoa(a.sound.ID)+"/places", map[string]int{"slots": 2}); r.code != 404 {
		t.Errorf("filling A's task through B: %d", r.code)
	}
}
