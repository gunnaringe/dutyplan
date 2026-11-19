package api

import (
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestEventTimesAndDescriptionsOverHTTP(t *testing.T) {
	e := newEnv(t)
	orgA, orgB := "org-a", "org-b"
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	var dtA, dtB store.DayType
	e.mustDo(201, "POST", "/api/orgs/"+orgA+"/day-types", map[string]string{"name": "Weekend"}).json(&dtA)
	e.mustDo(201, "POST", "/api/orgs/"+orgB+"/day-types", map[string]string{"name": "Weekend"}).json(&dtB)
	e.mustDo(201, "POST", "/api/orgs/"+orgA+"/plans", map[string]string{"name": "Autumn 2026"})
	e.mustDo(201, "POST", "/api/orgs/"+orgB+"/plans", map[string]string{"name": "Autumn 2026"})
	events := "/api/orgs/" + orgA + "/plans/autumn-2026/events"

	var ev store.Event
	e.mustDo(201, "POST", events, map[string]any{"date": "2026-08-09", "day_type_id": dtA.ID, "start_time": "11:00", "end_time": "12:30", "description": "Bring the banner.\nDoors at 10:30."}).json(&ev)
	if ev.StartTime != "11:00" || ev.EndTime != "12:30" || ev.Description != "Bring the banner.\nDoors at 10:30." {
		t.Errorf("created: %+v", ev)
	}
	var plain store.Event
	e.mustDo(201, "POST", events, map[string]any{"date": "2026-08-09", "day_type_id": dtA.ID, "note": "evening"}).json(&plain)
	var early store.Event
	e.mustDo(201, "POST", events, map[string]any{"date": "2026-08-09", "day_type_id": dtA.ID, "start_time": "08:00"}).json(&early)
	var list []store.Event
	e.mustDo(200, "GET", events, nil).json(&list)
	if len(list) != 3 || list[0].ID != early.ID || list[1].ID != ev.ID || list[2].ID != plain.ID {
		t.Errorf("the list is in time order, events without a time last: %+v", list)
	}

	for name, c := range map[string]struct {
		body map[string]any
		code string
	}{
		"bad start":     {map[string]any{"start_time": "25:00"}, "start_time_invalid"},
		"bad end":       {map[string]any{"start_time": "10:00", "end_time": "noon"}, "end_time_invalid"},
		"no start":      {map[string]any{"end_time": "10:00"}, "end_without_start"},
		"end first":     {map[string]any{"start_time": "10:00", "end_time": "09:00"}, "end_not_after_start"},
		"bad text":      {map[string]any{"description": "a\x01b"}, "description_invalid"},
		"too long text": {map[string]any{"description": strings.Repeat("x", 2001)}, "description_invalid"},
	} {
		body := map[string]any{"date": "2026-08-16", "day_type_id": dtA.ID}
		for k, v := range c.body {
			body[k] = v
		}
		if got := e.refused(400, "POST", events, body); got.Error != "invalid" || got.Code != c.code {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := e.refused(400, "POST", events, map[string]any{"date": "2026-08-16", "day_type_id": dtA.ID, "time": "11:00"}); got.Error != "invalid" {
		t.Errorf("an unknown field is still refused: %+v", got)
	}

	// changing
	one := events + "/" + itoa(plain.ID)
	e.mustDo(200, "PATCH", one, map[string]any{"start_time": "19:00", "end_time": "21:00", "description": "Evening session"}).json(&plain)
	if plain.StartTime != "19:00" || plain.EndTime != "21:00" || plain.Description != "Evening session" || plain.Note != "evening" {
		t.Errorf("patched: %+v", plain)
	}
	if got := e.refused(400, "PATCH", one, map[string]any{"start_time": ""}); got.Code != "end_without_start" {
		t.Errorf("clearing only the start: %+v", got)
	}
	if got := e.refused(400, "PATCH", one, map[string]any{"start_time": "19:00"}); got.Code != "nothing_to_change" {
		t.Errorf("the same time: %+v", got)
	}
	e.mustDo(200, "PATCH", one, map[string]any{"start_time": "", "end_time": ""}).json(&plain)
	if plain.StartTime != "" || plain.EndTime != "" {
		t.Errorf("cleared: %+v", plain)
	}
	// B cannot change A's event, with or without times
	if r := e.do("PATCH", "/api/orgs/"+orgB+"/plans/autumn-2026/events/"+itoa(plain.ID), map[string]any{"start_time": "10:00"}); r.code != 404 {
		t.Errorf("A's event through B: %d %s", r.code, r.body)
	}

	// bulk, with times
	var res struct {
		Created []store.Event
		Skipped []string
	}
	bulk := map[string]any{"from": "2026-09-01", "to": "2026-09-30", "weekdays": []int{7}, "day_type_id": dtA.ID, "start_time": "11:00", "end_time": "12:00"}
	e.mustDo(201, "POST", events+"/bulk", bulk).json(&res)
	if len(res.Created) != 4 || res.Created[0].StartTime != "11:00" || res.Created[0].EndTime != "12:00" {
		t.Errorf("bulk: %+v", res)
	}
	e.mustDo(201, "POST", events+"/bulk", bulk).json(&res)
	if len(res.Created) != 0 || len(res.Skipped) != 4 {
		t.Errorf("bulk twice: %+v", res)
	}
	bulk["end_time"] = "10:00"
	if got := e.refused(400, "POST", events+"/bulk", bulk); got.Code != "end_not_after_start" {
		t.Errorf("bulk with an end before the start: %+v", got)
	}
}
