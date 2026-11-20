package api

import (
	"strconv"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestEventTimesAndDescriptionsOverHTTP(t *testing.T) {
	e := newEnv(t)
	orgA, orgB := "org-a", "org-b"
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	e.mustDo(201, "POST", "/api/orgs/"+orgA+"/plans", map[string]string{"name": "Autumn 2026"})
	e.mustDo(201, "POST", "/api/orgs/"+orgB+"/plans", map[string]string{"name": "Autumn 2026"})
	events := "/api/orgs/" + orgA + "/plans/autumn-2026/events"

	add := func(body map[string]any) store.Event {
		t.Helper()
		var evs []store.Event
		e.mustDo(201, "POST", events, body).json(&evs)
		if len(evs) != 1 {
			t.Fatalf("one date gives one event: %+v", evs)
		}
		return evs[0]
	}
	ev := add(map[string]any{"date": "2026-08-09", "start_time": "11:00", "end_time": "12:30"})
	if ev.StartTime != "11:00" || ev.EndTime != "12:30" || ev.Description != "" {
		t.Errorf("created: %+v", ev)
	}
	plain := add(map[string]any{"date": "2026-08-09"})
	early := add(map[string]any{"date": "2026-08-09", "start_time": "08:00"})
	var list []store.Event
	e.mustDo(200, "GET", events, nil).json(&list)
	if len(list) != 3 || list[0].ID != early.ID || list[1].ID != ev.ID || list[2].ID != plain.ID {
		t.Errorf("the list is in time order, events without a time last: %+v", list)
	}

	for name, c := range map[string]struct {
		body map[string]any
		code string
	}{
		"bad start": {map[string]any{"start_time": "25:00"}, "start_time_invalid"},
		"bad end":   {map[string]any{"start_time": "10:00", "end_time": "noon"}, "end_time_invalid"},
		"no start":  {map[string]any{"end_time": "10:00"}, "end_without_start"},
		"end first": {map[string]any{"start_time": "10:00", "end_time": "09:00"}, "end_not_after_start"},
	} {
		body := map[string]any{"date": "2026-08-16"}
		for k, v := range c.body {
			body[k] = v
		}
		if got := e.refused(400, "POST", events, body); got.Error != "invalid" || got.Code != c.code {
			t.Errorf("%s: %+v", name, got)
		}
	}
	if got := e.refused(400, "POST", events, map[string]any{"date": "2026-08-16", "time": "11:00"}); got.Error != "invalid" {
		t.Errorf("an unknown field is still refused: %+v", got)
	}

	// changing
	one := events + "/" + itoa(plain.ID)
	e.mustDo(200, "PATCH", one, map[string]any{"start_time": "19:00", "end_time": "21:00", "description": "Evening session"}).json(&plain)
	if plain.StartTime != "19:00" || plain.EndTime != "21:00" || plain.Description != "Evening session" {
		t.Errorf("patched: %+v", plain)
	}
	for name, d := range map[string]string{"bad text": "a\x01b", "too long text": strings.Repeat("x", 2001)} {
		if got := e.refused(400, "PATCH", one, map[string]any{"description": d}); got.Code != "description_invalid" {
			t.Errorf("%s: %+v", name, got)
		}
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

	// every week until a date, with times, all or nothing
	var weekly []store.Event
	weeks := map[string]any{"date": "2026-09-06", "until": "2026-09-27", "start_time": "11:00", "end_time": "12:00"}
	e.mustDo(201, "POST", events, weeks).json(&weekly)
	if len(weekly) != 4 || weekly[0].Date != "2026-09-06" || weekly[3].Date != "2026-09-27" || weekly[3].StartTime != "11:00" || weekly[3].EndTime != "12:00" {
		t.Errorf("weekly: %+v", weekly)
	}
	weeks["end_time"] = "10:00"
	if got := e.refused(400, "POST", events, weeks); got.Code != "end_not_after_start" {
		t.Errorf("weekly with an end before the start: %+v", got)
	}
	weeks["end_time"], weeks["until"] = "12:00", "2026-09-01"
	if got := e.refused(400, "POST", events, weeks); got.Code != "range_invalid" {
		t.Errorf("until before the date: %+v", got)
	}
	e.mustDo(200, "GET", events, nil).json(&list)
	if len(list) != 7 {
		t.Errorf("refused additions add nothing: %d events", len(list))
	}

	// every few weeks: adding and copying
	var other []store.Event
	e.mustDo(201, "POST", events, map[string]any{"date": "2026-10-04", "until": "2026-11-01", "every": 2}).json(&other)
	if len(other) != 3 || other[0].Date != "2026-10-04" || other[1].Date != "2026-10-18" || other[2].Date != "2026-11-01" {
		t.Errorf("every other week: %+v", other)
	}
	e.mustDo(201, "POST", events+"/"+strconv.FormatInt(other[0].ID, 10)+"/copy",
		map[string]any{"dates": []string{"2026-11-08"}, "until": "2026-12-06", "every": 4, "times": true}).json(&other)
	if len(other) != 2 || other[0].Date != "2026-11-08" || other[1].Date != "2026-12-06" {
		t.Errorf("a copy every 4 weeks: %+v", other)
	}
	if got := e.refused(400, "POST", events, map[string]any{"date": "2026-10-04", "until": "2026-11-01", "every": 9}); got.Code != "every_invalid" {
		t.Errorf("every 9 weeks: %+v", got)
	}
}
