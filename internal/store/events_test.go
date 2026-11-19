package store

import (
	"errors"
	"fmt"
	"testing"
)

func setup(t *testing.T) (*Store, Org, Plan) {
	t.Helper()
	s := open(t)
	o := mustOrg(t, s, "Club")
	p, err := s.CreatePlan(ctx, o.Slug, "Autumn 2026", "")
	if err != nil {
		t.Fatal(err)
	}
	return s, o, p
}

func dates(evs []Event) string {
	var out []string
	for _, e := range evs {
		out = append(out, e.Date)
	}
	return fmt.Sprint(out)
}

func TestAddUpdateDeleteEventById(t *testing.T) {
	s, o, p := setup(t)
	for name, date := range map[string]string{"not a date": "2026-02-30", "wrong format": "09.08.2026", "empty": ""} {
		if _, err := s.AddEvent(ctx, o.Slug, p.Slug, date); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	d, err := s.AddEvent(ctx, o.Slug, p.Slug, "2026-08-09")
	if err != nil || d.Date != "2026-08-09" {
		t.Fatalf("add: %+v %v", d, err)
	}
	// Events are independent: a second event on the same date is just another event.
	evening, err := s.AddEvent(ctx, o.Slug, p.Slug, "2026-08-09")
	if err != nil || evening.ID == d.ID {
		t.Fatalf("a second event on the same date: %+v %v", evening, err)
	}
	if list, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(list) != 2 || list[0].ID != d.ID || list[1].ID != evening.ID {
		t.Fatalf("both listed, in creation order within the date: %v", list)
	}
	if err := s.DeleteEvent(ctx, o.Slug, p.Slug, evening.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(list) != 1 || list[0].ID != d.ID {
		t.Fatalf("deleting one event leaves the other: %v", list)
	}
	got, err := s.UpdateEvent(ctx, o.Slug, p.Slug, d.ID, EventPatch{Description: ptr("Coats in the side room")})
	if err != nil || got.Description != "Coats in the side room" || got.Date != d.Date {
		t.Fatalf("update: %+v %v", got, err)
	}
	if _, err := s.UpdateEvent(ctx, o.Slug, p.Slug, d.ID, EventPatch{Description: ptr("Coats in the side room")}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: %v", err)
	}

	// The event belongs to its plan: through another plan's slug it does not exist.
	p2, _ := s.CreatePlan(ctx, o.Slug, "Spring 2027", "")
	if _, err := s.UpdateEvent(ctx, o.Slug, p2.Slug, d.ID, EventPatch{Description: ptr("x")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("event through the wrong plan: %v", err)
	}
	if err := s.DeleteEvent(ctx, o.Slug, p2.Slug, d.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete through the wrong plan: %v", err)
	}
	if err := s.DeleteEvent(ctx, o.Slug, p.Slug, d.ID); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(list) != 0 {
		t.Errorf("after delete: %v", list)
	}
	// the same date may exist in two plans, and each plan lists only its own
	if _, err := s.AddEvent(ctx, o.Slug, p2.Slug, "2026-08-09"); err != nil {
		t.Errorf("same date in another plan: %v", err)
	}
	if l1, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(l1) != 0 {
		t.Errorf("plan 1 must not list plan 2's events: %v", l1)
	}
	if l2, _ := s.ListEvents(ctx, o.Slug, p2.Slug); len(l2) != 1 {
		t.Errorf("plan 2 lists its own: %v", l2)
	}
}

func TestAddingEveryWeekUntilADate(t *testing.T) {
	s, o, p := setup(t)
	sound, _ := s.CreateTask(ctx, o.Slug, "Sound", true)
	evs, err := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: "2026-08-09", Until: "2026-09-06", StartTime: "11:00", Places: map[int64]int{sound.ID: 2}})
	if err != nil {
		t.Fatal(err)
	}
	if got := dates(evs); got != "[2026-08-09 2026-08-16 2026-08-23 2026-08-30 2026-09-06]" {
		t.Errorf("Sundays 9.8 - 6.9: %s", got)
	}
	places, _ := s.ListEventPlaces(ctx, o.Slug, p.Slug)
	if len(places) != 5 || places[0].Slots != 2 || places[0].TaskID != sound.ID {
		t.Errorf("every new event gets the places: %+v", places)
	}
	for _, e := range evs {
		if e.StartTime != "11:00" {
			t.Errorf("every new event gets the time: %+v", e)
		}
	}
	if evs, _ := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: "2026-08-09", Until: "2026-08-15"}); dates(evs) != "[2026-08-09]" {
		t.Errorf("an until before the next week gives one event: %s", dates(evs))
	}
	other := mustOrg(t, s, "Other")
	foreign, _ := s.CreateTask(ctx, other.Slug, "Sound", true)
	for name, in := range map[string]NewEvents{
		"until before the date": {Date: "2026-09-01", Until: "2026-08-01"},
		"too long":              {Date: "2026-01-01", Until: "2027-06-01"},
		"bad until":             {Date: "2026-08-01", Until: "2026-13-01"},
		"places 0":              {Date: "2026-08-01", Places: map[int64]int{sound.ID: 0}},
		"places 51":             {Date: "2026-08-01", Places: map[int64]int{sound.ID: 51}},
		"end before start":      {Date: "2026-08-01", StartTime: "12:00", EndTime: "11:00"},
	} {
		if _, err := s.AddEvents(ctx, o.Slug, p.Slug, in); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	before, _ := s.ListEvents(ctx, o.Slug, p.Slug)
	if _, err := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: "2026-10-04", Until: "2026-10-25", Places: map[int64]int{foreign.ID: 1}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another organisation's task: %v", err)
	}
	if after, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(after) != len(before) {
		t.Errorf("a refused add stores nothing: %d -> %d", len(before), len(after))
	}
}

func TestRepeatingWeeklyGivesEachDateOnceAndHasLimits(t *testing.T) {
	got, err := repeatWeekly([]string{"2026-08-16", "2026-08-09", "2026-08-16"}, "2026-08-30")
	if err != nil || fmt.Sprint(got) != "[2026-08-09 2026-08-16 2026-08-23 2026-08-30]" {
		t.Errorf("two start dates a week apart, one of them twice: %v %v", got, err)
	}
	if got, err = repeatWeekly([]string{"2026-08-09", "2026-08-09"}, ""); err != nil || fmt.Sprint(got) != "[2026-08-09]" {
		t.Errorf("the same date twice without until: %v %v", got, err)
	}
	// four weekdays for a year is more than 200 events
	if _, err = repeatWeekly([]string{"2026-01-05", "2026-01-06", "2026-01-07", "2026-01-08"}, "2026-12-31"); !codeIs(err, "too_many_dates") {
		t.Errorf("too many dates: %v", err)
	}
	if got, err = repeatWeekly([]string{"2026-01-05", "2026-01-06", "2026-01-07"}, "2026-12-31"); err != nil || len(got) != 156 {
		t.Errorf("three weekdays for a year: %d %v", len(got), err)
	}
	if _, err = repeatWeekly(nil, ""); !codeIs(err, "no_dates") {
		t.Errorf("no dates: %v", err)
	}
}

func TestCopyingAnEventTakesWhatIsAskedFor(t *testing.T) {
	s, o, p := setup(t)
	sound, _ := s.CreateTask(ctx, o.Slug, "Sound", true)
	doors, _ := s.CreateTask(ctx, o.Slug, "Doors", true)
	kari, _ := s.CreatePerson(ctx, o.Slug, "Kari Test")
	ola, _ := s.CreatePerson(ctx, o.Slug, "Ola Sample")
	srcs, err := s.AddEvents(ctx, o.Slug, p.Slug, NewEvents{Date: "2026-12-19", StartTime: "18:30", EndTime: "22:00", Description: "Doors open at 18:00",
		Places: map[int64]int{sound.ID: 1, doors.ID: 3}})
	if err != nil {
		t.Fatal(err)
	}
	src := srcs[0]
	note, _ := s.AddField(ctx, o.Slug, p.Slug, "Note")
	if err := s.SetFieldValue(ctx, o.Slug, p.Slug, src.ID, note.ID, "Christmas gala"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddCells(ctx, o.Slug, p.Slug, NewCell{EventIDs: []int64{src.ID}, TaskID: doors.ID, Label: "Outside guest"}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddCells(ctx, o.Slug, p.Slug, NewCell{EventIDs: []int64{src.ID}, TaskID: doors.ID, PersonID: kari.ID}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddCells(ctx, o.Slug, p.Slug, NewCell{EventIDs: []int64{src.ID}, TaskID: sound.ID, PersonID: ola.ID, Locked: true}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAvailability(ctx, o.Slug, p.Slug, kari.ID, map[int64]string{src.ID: "good"}); err != nil {
		t.Fatal(err)
	}

	everything := CopyOf{Dates: []string{"2026-12-26", "2027-01-09"}, Times: true, Details: true, Places: true, Hand: true}
	copies, err := s.CopyEvent(ctx, o.Slug, p.Slug, src.ID, everything)
	if err != nil || dates(copies) != "[2026-12-26 2027-01-09]" {
		t.Fatalf("copies: %v %v", copies, err)
	}
	c := copies[0]
	if c.StartTime != "18:30" || c.EndTime != "22:00" || c.Description != "Doors open at 18:00" {
		t.Errorf("times and description come along: %+v", c)
	}
	fields, _ := s.ListFields(ctx, o.Slug, p.Slug)
	values := map[int64]string{}
	for _, v := range fields.Values {
		values[v.EventID] = v.Value
	}
	if values[c.ID] != "Christmas gala" {
		t.Errorf("the details come along: %+v", fields.Values)
	}
	places := map[string]int{}
	ps, _ := s.ListEventPlaces(ctx, o.Slug, p.Slug)
	for _, x := range ps {
		places[fmt.Sprint(x.EventID, "/", x.TaskID)] = x.Slots
	}
	if places[fmt.Sprint(c.ID, "/", doors.ID)] != 3 || places[fmt.Sprint(c.ID, "/", sound.ID)] != 1 {
		t.Errorf("the places come along: %v", places)
	}
	cells, _ := s.ListCells(ctx, o.Slug, p.Slug)
	var copied []string
	for _, x := range cells {
		if x.EventID == c.ID {
			copied = append(copied, fmt.Sprintf("%s|%v|%v", x.Label, x.PersonID != nil, x.Locked))
		}
	}
	if fmt.Sprint(copied) != "[Outside guest|false|false |true|false]" {
		t.Errorf("the cells set by hand come along, never a locked planner name: %v", copied)
	}
	answers, _ := s.ListAnswers(ctx, o.Slug, p.Slug)
	for _, a := range answers.Events {
		if a.EventID == c.ID {
			t.Errorf("answers are never copied: %+v", a)
		}
	}

	bare, err := s.CopyEvent(ctx, o.Slug, p.Slug, src.ID, CopyOf{Dates: []string{"2027-02-06"}, Until: "2027-02-20", StartTime: "10:00"})
	if err != nil || dates(bare) != "[2027-02-06 2027-02-13 2027-02-20]" {
		t.Fatalf("every week until: %v %v", bare, err)
	}
	if bare[0].StartTime != "10:00" || bare[0].EndTime != "" || bare[0].Description != "" {
		t.Errorf("without times and details the copy gets the given time and no description: %+v", bare[0])
	}
	ps, _ = s.ListEventPlaces(ctx, o.Slug, p.Slug)
	cells, _ = s.ListCells(ctx, o.Slug, p.Slug)
	fields, _ = s.ListFields(ctx, o.Slug, p.Slug)
	for _, x := range ps {
		if x.EventID == bare[0].ID {
			t.Errorf("no places asked for: %+v", x)
		}
	}
	for _, x := range cells {
		if x.EventID == bare[0].ID {
			t.Errorf("no cells asked for: %+v", x)
		}
	}
	for _, x := range fields.Values {
		if x.EventID == bare[0].ID {
			t.Errorf("no details asked for: %+v", x)
		}
	}

	p2, _ := s.CreatePlan(ctx, o.Slug, "Spring 2027", "")
	if _, err := s.CopyEvent(ctx, o.Slug, p2.Slug, src.ID, everything); !errors.Is(err, ErrNotFound) {
		t.Errorf("an event of another plan: %v", err)
	}
	if _, err := s.CopyEvent(ctx, o.Slug, p.Slug, src.ID, CopyOf{}); !codeIs(err, "no_dates") {
		t.Errorf("no dates: %v", err)
	}
	before, _ := s.ListEvents(ctx, o.Slug, p.Slug)
	if _, err := s.CopyEvent(ctx, o.Slug, p.Slug, src.ID, CopyOf{Dates: []string{"2027-03-06", "2027-13-01"}}); !errors.Is(err, ErrInvalid) {
		t.Errorf("a bad date among good ones: %v", err)
	}
	if after, _ := s.ListEvents(ctx, o.Slug, p.Slug); len(after) != len(before) {
		t.Errorf("a refused copy stores nothing")
	}
}

// Both organisations have a plan with the same slug; every write must reach the right one.
func TestSamePlanSlugInTwoOrganisations(t *testing.T) {
	s := open(t)
	a, b := mustOrg(t, s, "Org A"), mustOrg(t, s, "Org B")
	pa, _ := s.CreatePlan(ctx, a.Slug, "Autumn 2026", "")
	pb, _ := s.CreatePlan(ctx, b.Slug, "Autumn 2026", "")
	if pa.Slug != pb.Slug {
		t.Fatal("test needs equal slugs")
	}
	if _, err := s.AddEvent(ctx, a.Slug, pa.Slug, "2026-08-09"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddEvent(ctx, b.Slug, pb.Slug, "2026-08-16"); err != nil {
		t.Fatalf("a write to B's own plan: %v", err)
	}
	if _, err := s.AddEvents(ctx, b.Slug, pb.Slug, NewEvents{Date: "2026-08-23", Until: "2026-08-30"}); err != nil {
		t.Fatalf("a weekly add to B's own plan: %v", err)
	}
	la, _ := s.ListEvents(ctx, a.Slug, pa.Slug)
	lb, _ := s.ListEvents(ctx, b.Slug, pb.Slug)
	if len(la) != 1 || la[0].Date != "2026-08-09" || len(lb) != 3 {
		t.Errorf("A: %v\nB: %v", la, lb)
	}
	evA := la[0]
	if _, err := s.CopyEvent(ctx, b.Slug, pb.Slug, evA.ID, CopyOf{Dates: []string{"2026-09-06"}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("B copies A's event: %v", err)
	}
}
