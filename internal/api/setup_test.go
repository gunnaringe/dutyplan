package api

import (
	"fmt"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestSetupAndDatesFlow(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})

	var sound store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "Sound"}).json(&sound)
	if !sound.Planned {
		t.Error("tasks are planned by default")
	}
	var camera store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "Camera"}).json(&camera)
	if r := e.do("POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "Sound"}); r.code != 409 {
		t.Errorf("a second task with the same name: %d %s", r.code, r.body)
	}
	if r := e.do("POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "Coffee", "planned": false}); r.code != 400 {
		t.Errorf("a new task is always planned; the page switches it off afterwards: %d %s", r.code, r.body)
	}
	e.mustDo(200, "PATCH", fmt.Sprintf("/api/orgs/org-a/plans/"+term+"/tasks/%d", sound.ID), map[string]any{"planned": false})

	e.mustDo(204, "PUT", "/api/orgs/org-a/plans/"+term+"/combinations", map[string]any{"a": camera.ID, "b": sound.ID, "level": "cannot"})
	var combos []store.Combination
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/"+term+"/combinations", nil).json(&combos)
	if len(combos) != 1 || combos[0].A > combos[0].B {
		t.Errorf("combinations: %+v", combos)
	}

	// events: one with places, a second on the same date, then copies of the first every week
	events := "/api/orgs/org-a/plans/autumn-2026/events"
	var evs []store.Event
	e.mustDo(201, "POST", events, map[string]any{"date": "2026-08-09", "start_time": "11:00"}).json(&evs)
	d := evs[0]
	e.mustDo(204, "PUT", fmt.Sprintf("%s/%d/places/%d", events, d.ID, sound.ID), map[string]int{"slots": 1})
	e.mustDo(204, "PUT", fmt.Sprintf("%s/%d/places/%d", events, d.ID, camera.ID), map[string]int{"slots": 3})
	if r := e.do("POST", events, map[string]any{"date": "2026-08-09", "places": map[string]int{itoa(sound.ID): 1}}); r.code != 400 {
		t.Errorf("places are set per cell, not with the new event: %d %s", r.code, r.body)
	}
	e.mustDo(201, "POST", events, map[string]any{"date": "2026-08-09"}).json(&evs)
	if second := evs[0]; second.ID == d.ID || second.Date != d.Date {
		t.Errorf("two events on one date: %+v / %+v", d, second)
	}
	var field store.PlanField
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/autumn-2026/fields", map[string]any{"name": "Theme"}).json(&field)
	e.mustDo(204, "PUT", fmt.Sprintf("%s/%d/fields/%d", events, d.ID, field.ID), map[string]any{"value": "Harvest"})
	var copies []store.Event
	e.mustDo(201, "POST", fmt.Sprintf("%s/%d/copy", events, d.ID),
		map[string]any{"dates": []string{"2026-08-16"}, "until": "2026-08-30", "times": true, "places": true}).json(&copies)
	if len(copies) != 3 || copies[0].Date != "2026-08-16" || copies[2].Date != "2026-08-30" || copies[0].StartTime != "11:00" {
		t.Errorf("copies: %+v", copies)
	}
	placesOf := func(ev int64) string {
		t.Helper()
		var all []store.EventPlaces
		e.mustDo(200, "GET", "/api/orgs/org-a/plans/autumn-2026/places", nil).json(&all)
		out := ""
		for _, p := range all {
			if p.EventID == ev {
				out += fmt.Sprintf("[%d:%d]", p.TaskID, p.Slots)
			}
		}
		return out
	}
	if got, want := placesOf(copies[1].ID), fmt.Sprintf("[%d:1][%d:3]", sound.ID, camera.ID); got != want {
		t.Errorf("a copy takes the places: %s, want %s", got, want)
	}
	var fields store.PlanFields
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/autumn-2026/fields", nil).json(&fields)
	if len(fields.Values) != 1 || fields.Values[0].EventID != d.ID {
		t.Errorf("details were not asked for: %+v", fields.Values)
	}
	if r := e.do("POST", fmt.Sprintf("%s/%d/copy", events, d.ID), map[string]any{"dates": []string{}}); r.code != 400 || r.errCode() != "invalid" {
		t.Errorf("a copy needs a date: %d %s", r.code, r.body)
	}

	var dates []store.Event
	e.mustDo(200, "GET", events, nil).json(&dates)
	if len(dates) != 5 {
		t.Errorf("dates: %+v", dates)
	}
	e.mustDo(200, "PATCH", fmt.Sprintf("%s/%d", events, dates[1].ID), map[string]any{"start_time": "10:00"})
	e.mustDo(204, "DELETE", fmt.Sprintf("%s/%d", events, dates[1].ID), nil)
	e.mustDo(204, "DELETE", fmt.Sprintf("/api/orgs/org-a/plans/"+term+"/tasks/%d", camera.ID), nil) // its places and its combination go with it
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/"+term+"/combinations", nil).json(&combos)
	if len(combos) != 0 {
		t.Errorf("combination must go with its task: %+v", combos)
	}
	if got, want := placesOf(d.ID), fmt.Sprintf("[%d:1]", sound.ID); got != want {
		t.Errorf("the places of a deleted task go: %s, want %s", got, want)
	}
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/autumn-2026/cells", map[string]any{"event": d.ID, "task": sound.ID, "label": "Outside guest"})
	if r := e.do("DELETE", fmt.Sprintf("/api/orgs/org-a/plans/"+term+"/tasks/%d", sound.ID), nil); r.code != 409 {
		t.Errorf("task in use: %d %s", r.code, r.body)
	}
}

func TestSetupIsolationBetweenOrganisations(t *testing.T) {
	e := newEnv(t)
	for _, n := range []string{"Org A", "Org B"} {
		e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": n})
	}
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})
	e.mustDo(201, "POST", "/api/orgs/org-b/plans", map[string]string{"name": "Autumn 2026"})
	var taskA, taskB store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "Sound"}).json(&taskA)
	e.mustDo(201, "POST", "/api/orgs/org-b/plans/"+term+"/tasks", map[string]any{"name": "Sound"}).json(&taskB) // same name in both is fine
	var evsA []store.Event
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/autumn-2026/events", map[string]any{"date": "2026-08-09"}).json(&evsA)
	dateA := evsA[0]

	notFound := func(method, path string, body any) {
		t.Helper()
		if r := e.do(method, path, body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404", method, path, r.code, r.body)
		}
	}
	notFound("PUT", fmt.Sprintf("/api/orgs/org-b/plans/autumn-2026/events/%d/places/%d", dateA.ID, taskA.ID), map[string]int{"slots": 1})
	notFound("POST", fmt.Sprintf("/api/orgs/org-b/plans/autumn-2026/events/%d/copy", dateA.ID), map[string]any{"dates": []string{"2026-08-16"}})
	notFound("PATCH", fmt.Sprintf("/api/orgs/org-b/plans/"+term+"/tasks/%d", taskA.ID), map[string]any{"name": "Kapret"})
	notFound("DELETE", fmt.Sprintf("/api/orgs/org-b/plans/"+term+"/tasks/%d", taskA.ID), nil)
	notFound("PUT", "/api/orgs/org-b/plans/"+term+"/combinations", map[string]any{"a": taskA.ID, "b": taskB.ID, "level": "practical"})
	notFound("PATCH", fmt.Sprintf("/api/orgs/org-b/plans/autumn-2026/events/%d", dateA.ID), map[string]any{"start_time": "10:00"})
	notFound("DELETE", fmt.Sprintf("/api/orgs/org-b/plans/autumn-2026/events/%d", dateA.ID), nil)
	notFound("GET", "/api/orgs/org-a/plans/nope/events", nil)
	notFound("PATCH", "/api/orgs/org-a/plans/"+term+"/tasks/abc", map[string]any{"name": "x"})

	var datesB []store.Event
	e.mustDo(200, "GET", "/api/orgs/org-b/plans/autumn-2026/events", nil).json(&datesB)
	var tasksB []store.Task
	e.mustDo(200, "GET", "/api/orgs/org-b/plans/"+term+"/tasks", nil).json(&tasksB)
	if len(datesB) != 0 || len(tasksB) != 1 || tasksB[0].ID != taskB.ID {
		t.Errorf("B must see only its own: %v %v", datesB, tasksB)
	}
	var datesA []store.Event
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/autumn-2026/events", nil).json(&datesA)
	if len(datesA) != 1 || datesA[0].StartTime != "" {
		t.Errorf("A's date untouched: %+v", datesA)
	}
}

func TestSetupWritesNeedValidInput(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Autumn 2026"})
	var task store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "Sound"}).json(&task)
	if r := e.do("POST", "/api/orgs/org-a/plans/autumn-2026/events", map[string]any{"date": "morgen"}); r.code != 400 {
		t.Errorf("bad date: %d", r.code)
	}
	if r := e.do("POST", "/api/orgs/org-a/plans/"+term+"/tasks", map[string]any{"name": "  "}); r.code != 400 {
		t.Errorf("a blank task name: %d", r.code)
	}
	if r := e.do("POST", "/api/orgs/org-a/plans/autumn-2026/events", map[string]any{"date": "2026-08-09", "description": "x"}); r.code != 400 {
		t.Errorf("an event has no description (a details row holds it): %d %s", r.code, r.body)
	}
}
