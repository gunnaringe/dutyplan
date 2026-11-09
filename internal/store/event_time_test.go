package store

import (
	"errors"
	"strings"
	"testing"
)

func addTimed(t *testing.T, s *Store, o Org, p Plan, dt DayType, date, start, end, note string) Event {
	t.Helper()
	e, err := s.AddEventFull(ctx, o.Slug, p.Slug, "Ola", NewEvent{Date: date, DayTypeID: dt.ID, Note: note, StartTime: start, EndTime: end})
	if err != nil {
		t.Fatalf("%s %s-%s: %v", date, start, end, err)
	}
	return e
}

func TestAnEventCanHaveATimeOfDayAndADescription(t *testing.T) {
	s, o, p, dt := setup(t)
	e, err := s.AddEventFull(ctx, o.Slug, p.Slug, "Ola", NewEvent{Date: "2026-08-09", DayTypeID: dt.ID, Note: "main hall", StartTime: " 11:00 ", EndTime: "12:30",
		Description: "  Bring the banner.\r\nDoors open at 10:30.\tThanks!  "})
	if err != nil {
		t.Fatal(err)
	}
	if e.StartTime != "11:00" || e.EndTime != "12:30" || e.Description != "Bring the banner.\nDoors open at 10:30.\tThanks!" || e.Note != "main hall" {
		t.Errorf("added: %+v", e)
	}
	// without times it is as before
	plain, err := s.AddEvent(ctx, o.Slug, p.Slug, "Ola", "2026-08-16", dt.ID, "")
	if err != nil || plain.StartTime != "" || plain.EndTime != "" || plain.Description != "" {
		t.Errorf("plain: %+v %v", plain, err)
	}
	// a start time without an end time is fine; the times stay with the event
	got, _ := s.ListEvents(ctx, o.Slug, p.Slug)
	if len(got) != 2 || got[0].StartTime != "11:00" || got[0].EndTime != "12:30" || got[0].Description != e.Description {
		t.Errorf("listed: %+v", got)
	}
	startOnly := addTimed(t, s, o, p, dt, "2026-08-23", "09:05", "", "")
	if startOnly.StartTime != "09:05" || startOnly.EndTime != "" {
		t.Errorf("start only: %+v", startOnly)
	}
	for _, tm := range []string{"00:00", "23:59"} {
		addTimed(t, s, o, p, dt, "2026-08-30", tm, "", "")
	}
	var found bool
	for _, l := range mustLog(t, s, o.Slug, p.Slug) {
		if l.Action == "event.add" && l.Detail == "2026-08-09 11:00 Service" {
			found = true
		}
	}
	if !found {
		t.Errorf("the log says when the event is: %+v", mustLog(t, s, o.Slug, p.Slug))
	}
}

func TestRefusedTimesAndDescriptions(t *testing.T) {
	s, o, p, dt := setup(t)
	add := func(start, end, desc string) error {
		_, err := s.AddEventFull(ctx, o.Slug, p.Slug, "Ola", NewEvent{Date: "2026-08-09", DayTypeID: dt.ID, StartTime: start, EndTime: end, Description: desc})
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
	s, o, p, dt := setup(t)
	e := addTimed(t, s, o, p, dt, "2026-08-09", "", "", "")
	str := func(v string) *string { return &v }
	up := func(patch EventPatch) (Event, error) { return s.UpdateEvent(ctx, o.Slug, p.Slug, "Eva", e.ID, patch) }

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
	// a note change leaves the time alone and the other way round
	up(EventPatch{StartTime: str("09:00")})
	if got, err = up(EventPatch{Note: str("hall")}); err != nil || got.StartTime != "09:00" || got.Note != "hall" {
		t.Errorf("note keeps the time: %+v %v", got, err)
	}
	var details []string
	for _, l := range mustLog(t, s, o.Slug, p.Slug) {
		if l.Action == "event.update" {
			details = append(details, l.Detail)
		}
	}
	if got := strings.Join(details, "|"); got != "2026-08-09: note|2026-08-09: time|2026-08-09: description|2026-08-09: description|2026-08-09: time|2026-08-09: time|2026-08-09: time|2026-08-09: time" {
		t.Errorf("the log says what changed: %s", got)
	}
}

func TestEventsOfADateComeInTimeOrderAndTheRestInCreationOrder(t *testing.T) {
	s, o, p, dt := setup(t)
	untimed1 := addTimed(t, s, o, p, dt, "2026-08-09", "", "", "first without time")
	evening := addTimed(t, s, o, p, dt, "2026-08-09", "19:00", "", "evening")
	untimed2 := addTimed(t, s, o, p, dt, "2026-08-09", "", "", "second without time")
	morning := addTimed(t, s, o, p, dt, "2026-08-09", "09:30", "10:30", "morning")
	earlier := addTimed(t, s, o, p, dt, "2026-08-02", "23:00", "", "the week before")
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
	person, _ := s.CreatePerson(ctx, o.Slug, "Ola", "Kari Test")
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

func TestBulkAddWithATimeOfDay(t *testing.T) {
	s, o, p, dt := setup(t)
	created, skipped, err := s.AddEventsBulkTimed(ctx, o.Slug, p.Slug, "Ola", "2026-08-01", "2026-08-31", []int{7}, dt.ID, "11:00", "12:30")
	if err != nil || len(created) != 5 || len(skipped) != 0 {
		t.Fatalf("bulk: %d created, %v skipped, %v", len(created), skipped, err)
	}
	for _, e := range created {
		if e.StartTime != "11:00" || e.EndTime != "12:30" {
			t.Errorf("every event gets the times: %+v", e)
		}
	}
	// the same again adds nothing; another start time on the same days is another event; none at all is another too
	if c, sk, _ := s.AddEventsBulkTimed(ctx, o.Slug, p.Slug, "Ola", "2026-08-01", "2026-08-31", []int{7}, dt.ID, "11:00", "12:30"); len(c) != 0 || len(sk) != 5 {
		t.Errorf("repeating: %d created, %d skipped", len(c), len(sk))
	}
	if c, _, _ := s.AddEventsBulkTimed(ctx, o.Slug, p.Slug, "Ola", "2026-08-01", "2026-08-31", []int{7}, dt.ID, "18:00", ""); len(c) != 5 {
		t.Errorf("an evening on the same days: %d created", len(c))
	}
	if c, _, _ := s.AddEventsBulk(ctx, o.Slug, p.Slug, "Ola", "2026-08-01", "2026-08-31", []int{7}, dt.ID); len(c) != 5 {
		t.Errorf("without a time it is yet another event on those days: %d created", len(c))
	}
	for name, c := range map[string]struct{ start, end, code string }{
		"bad start": {"25:00", "", "start_time_invalid"}, "end without start": {"", "12:00", "end_without_start"}, "end first": {"12:00", "10:00", "end_not_after_start"},
	} {
		before, _ := s.ListEvents(ctx, o.Slug, p.Slug)
		if _, _, err := s.AddEventsBulkTimed(ctx, o.Slug, p.Slug, "Ola", "2026-09-01", "2026-09-30", []int{7}, dt.ID, c.start, c.end); !codeIs(err, c.code) {
			t.Errorf("%s: %v", name, err)
		}
		if after, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(after) != len(before) {
			t.Errorf("%s: a refused bulk add stored events", name)
		}
	}
}

func TestMigrationGivesOldEventsNoTimeAndNoDescription(t *testing.T) {
	path := t.TempDir() + "/old.db"
	databaseAtVersion8(t, path)
	s, err := Open(path) // applies the migrations after 8 to the old database
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	evs, err := s.ListEvents(ctx, "org", "a")
	if err != nil || len(evs) != 2 {
		t.Fatalf("old events: %+v %v", evs, err)
	}
	for _, e := range evs {
		if e.StartTime != "" || e.EndTime != "" || e.Description != "" {
			t.Errorf("an old event has no time or description: %+v", e)
		}
	}
	if n := count(t, s, `SELECT COUNT(*) FROM event WHERE start_time = '' AND end_time = '' AND description = ''`); n != 2 {
		t.Errorf("rows: %d", n)
	}
}
