package api

import (
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

func TestSetupAndDatesFlow(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Høst 2026"})

	var lyd store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/tasks", map[string]any{"name": "Lyd"}).json(&lyd)
	if !lyd.Planned {
		t.Error("tasks are planned by default")
	}
	var bulk struct {
		Created []store.Task `json:"created"`
		Skipped []string     `json:"skipped"`
	}
	e.mustDo(201, "POST", "/api/orgs/org-a/tasks/bulk", map[string]any{"names": "Lyd\nBilde\nKafé\n"}).json(&bulk)
	if len(bulk.Created) != 2 || len(bulk.Skipped) != 1 {
		t.Errorf("bulk tasks: %+v", bulk)
	}
	e.mustDo(200, "PATCH", fmt.Sprintf("/api/orgs/org-a/tasks/%d", lyd.ID), map[string]any{"planned": false})

	var dt store.DayType
	e.mustDo(201, "POST", "/api/orgs/org-a/day-types", map[string]string{"name": "Gudstjeneste"}).json(&dt)
	e.mustDo(200, "PUT", fmt.Sprintf("/api/orgs/org-a/day-types/%d/tasks", dt.ID),
		map[string]any{"tasks": []map[string]int64{{"task_id": lyd.ID, "slots": 1}, {"task_id": bulk.Created[1].ID, "slots": 3}}}).json(&dt)
	if len(dt.Tasks) != 2 {
		t.Errorf("day type tasks: %+v", dt)
	}
	e.mustDo(200, "PATCH", fmt.Sprintf("/api/orgs/org-a/day-types/%d", dt.ID), map[string]string{"name": "Gudstjeneste kl. 11"}).json(&dt)

	e.mustDo(204, "PUT", "/api/orgs/org-a/combinations", map[string]any{"a": bulk.Created[0].ID, "b": lyd.ID, "level": "kan_ikke"})
	var combos []store.Combination
	e.mustDo(200, "GET", "/api/orgs/org-a/combinations", nil).json(&combos)
	if len(combos) != 1 || combos[0].A > combos[0].B {
		t.Errorf("combinations: %+v", combos)
	}

	// dates: single, then bulk by weekday
	var d store.SlotDate
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/host-2026/dates", map[string]any{"date": "2026-08-09", "day_type_id": dt.ID, "note": "Alle sammen"}).json(&d)
	var bulkDates struct {
		Created []store.SlotDate `json:"created"`
		Skipped []string         `json:"skipped"`
	}
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/host-2026/dates/bulk",
		map[string]any{"from": "2026-08-09", "to": "2026-08-30", "weekdays": []int{7}, "day_type_id": dt.ID}).json(&bulkDates)
	if len(bulkDates.Created) != 3 || len(bulkDates.Skipped) != 1 || bulkDates.Skipped[0] != "2026-08-09" {
		t.Errorf("bulk: %+v", bulkDates)
	}

	var dates []store.SlotDate
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/host-2026/dates", nil).json(&dates)
	if len(dates) != 4 || dates[0].Note != "Alle sammen" {
		t.Errorf("dates: %+v", dates)
	}
	e.mustDo(200, "PATCH", fmt.Sprintf("/api/orgs/org-a/plans/host-2026/dates/%d", dates[1].ID), map[string]any{"note": "Høstferie"})
	e.mustDo(204, "DELETE", fmt.Sprintf("/api/orgs/org-a/plans/host-2026/dates/%d", dates[1].ID), nil)
	e.mustDo(204, "DELETE", fmt.Sprintf("/api/orgs/org-a/tasks/%d", bulk.Created[0].ID), nil) // unused: allowed, and its combination goes too
	e.mustDo(200, "GET", "/api/orgs/org-a/combinations", nil).json(&combos)
	if len(combos) != 0 {
		t.Errorf("combination must go with its task: %+v", combos)
	}
	if r := e.do("DELETE", fmt.Sprintf("/api/orgs/org-a/day-types/%d", dt.ID), "Test", nil); r.code != 409 || r.errCode() != "conflict" {
		t.Errorf("day type in use: %d %s", r.code, r.body)
	}
	if r := e.do("DELETE", fmt.Sprintf("/api/orgs/org-a/tasks/%d", lyd.ID), "Test", nil); r.code != 409 {
		t.Errorf("task in use: %d %s", r.code, r.body)
	}
}

func TestSetupIsolationBetweenOrganisations(t *testing.T) {
	e := newEnv(t)
	for _, n := range []string{"Org A", "Org B"} {
		e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": n})
	}
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Høst 2026"})
	e.mustDo(201, "POST", "/api/orgs/org-b/plans", map[string]string{"name": "Høst 2026"})
	var taskA, taskB store.Task
	var dtA store.DayType
	e.mustDo(201, "POST", "/api/orgs/org-a/tasks", map[string]any{"name": "Lyd"}).json(&taskA)
	e.mustDo(201, "POST", "/api/orgs/org-b/tasks", map[string]any{"name": "Lyd"}).json(&taskB) // same name in both is fine
	e.mustDo(201, "POST", "/api/orgs/org-a/day-types", map[string]string{"name": "Helg"}).json(&dtA)
	var dateA store.SlotDate
	e.mustDo(201, "POST", "/api/orgs/org-a/plans/host-2026/dates", map[string]any{"date": "2026-08-09", "day_type_id": dtA.ID}).json(&dateA)

	notFound := func(method, path string, body any) {
		t.Helper()
		if r := e.do(method, path, "Test", body); r.code != 404 || r.errCode() != "not_found" {
			t.Errorf("%s %s = %d %s, want 404", method, path, r.code, r.body)
		}
	}
	notFound("POST", "/api/orgs/org-b/plans/host-2026/dates", map[string]any{"date": "2026-08-09", "day_type_id": dtA.ID})
	notFound("POST", "/api/orgs/org-b/plans/host-2026/dates/bulk", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "weekdays": []int{7}, "day_type_id": dtA.ID})
	notFound("PUT", fmt.Sprintf("/api/orgs/org-b/day-types/%d/tasks", dtA.ID), map[string]any{"tasks": []map[string]int64{{"task_id": taskB.ID, "slots": 1}}})
	notFound("PATCH", fmt.Sprintf("/api/orgs/org-b/tasks/%d", taskA.ID), map[string]any{"name": "Kapret"})
	notFound("DELETE", fmt.Sprintf("/api/orgs/org-b/tasks/%d", taskA.ID), nil)
	notFound("DELETE", fmt.Sprintf("/api/orgs/org-b/day-types/%d", dtA.ID), nil)
	notFound("PUT", "/api/orgs/org-b/combinations", map[string]any{"a": taskA.ID, "b": taskB.ID, "level": "bor"})
	notFound("PATCH", fmt.Sprintf("/api/orgs/org-b/plans/host-2026/dates/%d", dateA.ID), map[string]any{"note": "x"})
	notFound("DELETE", fmt.Sprintf("/api/orgs/org-b/plans/host-2026/dates/%d", dateA.ID), nil)
	notFound("GET", "/api/orgs/org-a/plans/ingen/dates", nil)
	notFound("PATCH", "/api/orgs/org-a/tasks/abc", map[string]any{"name": "x"})

	var datesB []store.SlotDate
	e.mustDo(200, "GET", "/api/orgs/org-b/plans/host-2026/dates", nil).json(&datesB)
	var tasksB []store.Task
	e.mustDo(200, "GET", "/api/orgs/org-b/tasks", nil).json(&tasksB)
	if len(datesB) != 0 || len(tasksB) != 1 || tasksB[0].ID != taskB.ID {
		t.Errorf("B must see only its own: %v %v", datesB, tasksB)
	}
	var datesA []store.SlotDate
	e.mustDo(200, "GET", "/api/orgs/org-a/plans/host-2026/dates", nil).json(&datesA)
	if len(datesA) != 1 || datesA[0].Note != "" {
		t.Errorf("A's date untouched: %+v", datesA)
	}
}

func TestSetupWritesNeedAnActorAndValidInput(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs/org-a/plans", map[string]string{"name": "Høst 2026"})
	var task store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/tasks", map[string]any{"name": "Lyd"}).json(&task)
	for _, c := range []struct {
		method, path string
		body         any
	}{
		{"DELETE", fmt.Sprintf("/api/orgs/org-a/tasks/%d", task.ID), nil},
		{"POST", "/api/orgs/org-a/tasks", map[string]any{"name": "Kafé"}},
		{"PUT", "/api/orgs/org-a/combinations", map[string]any{"a": 1, "b": 2, "level": "bor"}},
		{"POST", "/api/orgs/org-a/plans/host-2026/dates/bulk", map[string]any{"from": "2026-08-01", "to": "2026-08-31", "weekdays": []int{7}, "day_type_id": 1}},
	} {
		if r := e.do(c.method, c.path, "", c.body); r.code != 400 || r.errCode() != "actor_required" {
			t.Errorf("%s %s without actor: %d %s", c.method, c.path, r.code, r.body)
		}
	}
	if r := e.do("POST", "/api/orgs/org-a/plans/host-2026/dates", "T", map[string]any{"date": "morgen", "day_type_id": 1}); r.code != 400 {
		t.Errorf("bad date: %d", r.code)
	}
	if r := e.do("POST", "/api/orgs/org-a/tasks/bulk", "T", map[string]any{"names": strings.Repeat("a\n", 3) + " "}); r.code != 201 {
		t.Errorf("duplicates in a paste are folded, not an error: %d %s", r.code, r.body)
	}
	if r := e.do("POST", "/api/orgs/org-a/tasks/bulk", "T", map[string]any{"names": "  \n"}); r.code != 400 {
		t.Errorf("empty paste: %d", r.code)
	}
}
