package store

import (
	"errors"
	"strings"
	"testing"
)

func addTimed(t *testing.T, s *Store, o Org, p Plan, date, start, end string) Event {
	t.Helper()
	evs, err := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: date, StartTime: start, EndTime: end})
	if err != nil {
		t.Fatalf("%s %s-%s: %v", date, start, end, err)
	}
	return evs[0]
}

func TestAnEventCanHaveATimeOfDayAndADescription(t *testing.T) {
	s, o, p := setup(t)
	evs, err := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: "2026-08-09", StartTime: " 11:00 ", EndTime: "12:30",
		Description: "  Bring the banner.\r\nDoors open at 10:30.\tThanks!  "})
	if err != nil {
		t.Fatal(err)
	}
	e := evs[0]
	if e.StartTime != "11:00" || e.EndTime != "12:30" || e.Description != "Bring the banner.\nDoors open at 10:30.\tThanks!" {
		t.Errorf("added: %+v", e)
	}
	// without times it is as before
	plain, err := s.AddEvent(ctx, o.Slug, p.Slug, "2026-08-16")
	if err != nil || plain.StartTime != "" || plain.EndTime != "" || plain.Description != "" {
		t.Errorf("plain: %+v %v", plain, err)
	}
	// a start time without an end time is fine; the times stay with the event
	got, _ := s.ListEvents(ctx, o.Slug, p.Slug)
	if len(got) != 2 || got[0].StartTime != "11:00" || got[0].EndTime != "12:30" || got[0].Description != e.Description {
		t.Errorf("listed: %+v", got)
	}
	startOnly := addTimed(t, s, o, p, "2026-08-23", "09:05", "")
	if startOnly.StartTime != "09:05" || startOnly.EndTime != "" {
		t.Errorf("start only: %+v", startOnly)
	}
	for _, tm := range []string{"00:00", "23:59"} {
		addTimed(t, s, o, p, "2026-08-30", tm, "")
	}
}

func TestRefusedTimesAndDescriptions(t *testing.T) {
	s, o, p := setup(t)
	add := func(start, end, desc string) error {
		_, err := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: "2026-08-09", StartTime: start, EndTime: end, Description: desc})
		return err
	}
	for name, c := range map[string]struct{ start, end, desc, code string }{
		"hour 24":               {"24:00", "", "", "start_time_invalid"},
		"minute 60":             {"10:60", "", "", "start_time_invalid"},
		"no colon":              {"1100", "", "", "start_time_invalid"},
		"one digit hour":        {"9:30", "", "", "start_time_invalid"},
		"seconds":               {"11:00:00", "", "", "start_time_invalid"},
		"text":                  {"noon", "", "", "start_time_invalid"},
		"bad end":               {"11:00", "25:00", "", "end_time_invalid"},
		"end without start":     {"", "12:00", "", "end_without_start"},
		"end before start":      {"12:00", "11:00", "", "end_not_after_start"},
		"end equal to start":    {"12:00", "12:00", "", "end_not_after_start"},
		"control character":     {"", "", "bell\x07", "description_invalid"},
		"too long":              {"", "", strings.Repeat("x", maxDescriptionRunes+1), "description_invalid"},
		"too long (multi-byte)": {"", "", strings.Repeat("å", maxDescriptionRunes+1), "description_invalid"},
	} {
		if err := add(c.start, c.end, c.desc); !codeIs(err, c.code) || !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v, want %s", name, err, c.code)
		}
	}
	if evs, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(evs) != 0 {
		t.Errorf("refused events were stored: %+v", evs)
	}
	if err := add("", "", strings.Repeat("å", maxDescriptionRunes)); err != nil {
		t.Errorf("exactly the limit: %v", err)
	}
	// the database says no too, whatever the code does
	for name, q := range map[string]string{
		"a bad start":             `UPDATE event SET start_time = '9:00'`,
		"an end without a start":  `UPDATE event SET end_time = '12:00', start_time = ''`,
		"an end before the start": `UPDATE event SET start_time = '12:00', end_time = '11:00'`,
	} {
		if _, err := s.DB().Exec(q); err == nil || !strings.Contains(err.Error(), "CHECK") {
			t.Errorf("%s must fail at the CHECK: %v", name, err)
		}
	}
}

func TestUpdatingTimesAndDescription(t *testing.T) {
	s, o, p := setup(t)
	e := addTimed(t, s, o, p, "2026-08-09", "", "")
	str := func(v string) *string { return &v }
	up := func(patch EventPatch) (Event, error) { return s.UpdateEvent(ctx, o.Slug, p.Slug, e.ID, patch) }

	got, err := up(EventPatch{StartTime: str("11:00")})
	if err != nil || got.StartTime != "11:00" || got.EndTime != "" {
		t.Fatalf("set the start: %+v %v", got, err)
	}
	if got, err = up(EventPatch{EndTime: str("12:15")}); err != nil || got.StartTime != "11:00" || got.EndTime != "12:15" {
		t.Fatalf("set the end: %+v %v", got, err)
	}
	if got, err = up(EventPatch{StartTime: str("10:00"), EndTime: str("11:30")}); err != nil || got.StartTime != "10:00" || got.EndTime != "11:30" {
		t.Fatalf("both: %+v %v", got, err)
	}
	if _, err = up(EventPatch{StartTime: str("")}); !codeIs(err, "end_without_start") {
		t.Errorf("clearing the start but not the end: %v", err)
	}
	if _, err = up(EventPatch{StartTime: str("12:00")}); !codeIs(err, "end_not_after_start") {
		t.Errorf("a start after the end: %v", err)
	}
	if _, err = up(EventPatch{StartTime: str("10:00")}); !codeIs(err, "nothing_to_change") {
		t.Errorf("the same time: %v", err)
	}
	if got, err = up(EventPatch{StartTime: str(""), EndTime: str("")}); err != nil || got.StartTime != "" || got.EndTime != "" {
		t.Fatalf("clear both: %+v %v", got, err)
	}
	if got, err = up(EventPatch{Description: str("Line one\r\nline two ")}); err != nil || got.Description != "Line one\nline two" {
		t.Fatalf("description: %+v %v", got, err)
	}
	if _, err = up(EventPatch{Description: str("Line one\nline two")}); !codeIs(err, "nothing_to_change") {
		t.Errorf("the same description: %v", err)
	}
	if _, err = up(EventPatch{Description: str("x\x00y")}); !codeIs(err, "description_invalid") {
		t.Errorf("a control character: %v", err)
	}
	if got, err = up(EventPatch{Description: str("")}); err != nil || got.Description != "" {
		t.Fatalf("clear the description: %+v %v", got, err)
	}
	// a description change leaves the time alone
	up(EventPatch{StartTime: str("09:00")})
	if got, err = up(EventPatch{Description: str("hall")}); err != nil || got.StartTime != "09:00" || got.Description != "hall" {
		t.Errorf("the description keeps the time: %+v %v", got, err)
	}
}

func TestEventsOfADateComeInTimeOrderAndTheRestInCreationOrder(t *testing.T) {
	s, o, p := setup(t)
	untimed1 := addTimed(t, s, o, p, "2026-08-09", "", "")
	evening := addTimed(t, s, o, p, "2026-08-09", "19:00", "")
	untimed2 := addTimed(t, s, o, p, "2026-08-09", "", "")
	morning := addTimed(t, s, o, p, "2026-08-09", "09:30", "10:30")
	earlier := addTimed(t, s, o, p, "2026-08-02", "23:00", "")
	want := []int64{earlier.ID, morning.ID, evening.ID, untimed1.ID, untimed2.ID}
	ids := func(evs []Event) (out []int64) {
		for _, e := range evs {
			out = append(out, e.ID)
		}
		return
	}
	evs, _ := s.ListEvents(ctx, o.Slug, p.Slug)
	if got := ids(evs); !equalIDs(got, want) {
		t.Errorf("list: %v, want %v", got, want)
	}
	person, _ := s.CreatePerson(ctx, o.Slug, "Kari Test")
	me, err := s.GetMe(ctx, o.Slug, p.Slug, person.ID)
	if err != nil {
		t.Fatal(err)
	}
	var meIDs []int64
	for _, e := range me.Events {
		meIDs = append(meIDs, e.EventID)
	}
	if !equalIDs(meIDs, want) || me.Events[1].StartTime != "09:30" || me.Events[1].EndTime != "10:30" {
		t.Errorf("a person's page: %v %+v", meIDs, me.Events[1])
	}
	// the planner gets the events in the order of date and creation, so setting a time never changes the input of a run
	data, err := s.GetPlanningData(ctx, o.Slug, p.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if got, w := ids(data.Events), []int64{earlier.ID, untimed1.ID, evening.ID, untimed2.ID, morning.ID}; !equalIDs(got, w) {
		t.Errorf("planning data: %v, want %v", got, w)
	}
	if data.Events[4].StartTime != "09:30" {
		t.Errorf("planning data carries the time: %+v", data.Events[4])
	}
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
