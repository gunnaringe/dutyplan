package planner

import (
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func ptr(s string) *string { return &s }

func TestTheViewShowsTheCurrentTimeAndDescriptionOfAnEvent(t *testing.T) {
	w := newWorld(t, "Club")
	// event 0 gets a time before the run (so the snapshot has it), the others later
	if _, err := w.st.UpdateEvent(ctx, w.org.Slug, w.plan.Slug, w.events[0].ID, store.EventPatch{StartTime: ptr("10:00"), EndTime: ptr("11:00"), Description: ptr("Doors at 9:30")}); err != nil {
		t.Fatal(err)
	}
	s := w.runWith(t, cannedAnswer(w))
	runs, _ := w.st.ListRuns(ctx, w.org.Slug, w.plan.Slug, 0)
	id := runs[0].ID
	view := func() RunView {
		t.Helper()
		v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
	v := view()
	if v.Events[0].StartTime != "10:00" || v.Events[0].EndTime != "11:00" || v.Events[0].Description != "Doors at 9:30" {
		t.Errorf("as it was when the run was made: %+v", v.Events[0])
	}

	// changing a time or a description afterwards shows in the plan at once and does not make the run out of date
	if _, err := w.st.UpdateEvent(ctx, w.org.Slug, w.plan.Slug, w.events[0].ID, store.EventPatch{StartTime: ptr("18:00"), EndTime: ptr("19:00"), Description: ptr("Moved to the evening")}); err != nil {
		t.Fatal(err)
	}
	v = view()
	if v.Events[0].StartTime != "18:00" || v.Events[0].EndTime != "19:00" || v.Events[0].Description != "Moved to the evening" {
		t.Errorf("the current time and description: %+v", v.Events[0])
	}
	if v.Run.Stale {
		t.Error("a time or a description does not change the plan, so the run is not out of date")
	}

	// an event that is gone keeps the time it had in the run
	if err := w.st.DeleteEvent(ctx, w.org.Slug, w.plan.Slug, w.events[0].ID); err != nil {
		t.Fatal(err)
	}
	v = view()
	if v.Events[0].ID != w.events[0].ID || v.Events[0].StartTime != "10:00" || v.Events[0].Description != "Doors at 9:30" {
		t.Errorf("a deleted event shows as in the run: %+v", v.Events[0])
	}
}

func TestEventsOnTheSameDateAreShownInTimeOrderInThePlanAndInMine(t *testing.T) {
	w := newWorld(t, "Club")
	// a second event on the first date, made later but earlier in the day, and the first one gets a later time
	late := w.events[0]
	early, err := w.st.AddEvent(ctx, w.org.Slug, w.plan.Slug, late.Date, late.DayTypeID, "early bird")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.UpdateEvent(ctx, w.org.Slug, w.plan.Slug, late.ID, store.EventPatch{StartTime: ptr("19:00")}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.UpdateEvent(ctx, w.org.Slug, w.plan.Slug, early.ID, store.EventPatch{StartTime: ptr("08:30"), EndTime: ptr("09:00")}); err != nil {
		t.Fatal(err)
	}
	plain, err := w.st.AddEvent(ctx, w.org.Slug, w.plan.Slug, late.Date, late.DayTypeID, "no time yet") // made last, no time: shown last
	if err != nil {
		t.Fatal(err)
	}
	s := w.runWith(t, cannedAnswer(w))
	runs, _ := w.st.ListRuns(ctx, w.org.Slug, w.plan.Slug, 0)
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	// the run was made after the events existed, so both are in it; the early one first although it has the higher id
	first, second, third := -1, -1, -1
	for i, e := range v.Events {
		switch e.ID {
		case early.ID:
			first = i
		case late.ID:
			second = i
		case plain.ID:
			third = i
		}
	}
	if first < 0 || second < 0 || third < 0 || first >= second || second >= third || v.Events[first].StartTime != "08:30" || v.Events[first].EndTime != "09:00" {
		t.Errorf("order: early at %d, late at %d, untimed at %d: %+v", first, second, third, v.Events)
	}
	for i := 1; i < len(v.Events); i++ {
		a, b := v.Events[i-1], v.Events[i]
		if a.Date > b.Date {
			t.Errorf("dates out of order: %s then %s", a.Date, b.Date)
		}
	}
	// «My assignments» carries the time of the event
	m, err := s.Mine(ctx, w.org.Slug, w.plan.Slug, w.people["Aslaug"].ID)
	if err != nil {
		t.Fatal(err)
	}
	var seen bool
	for _, a := range m.Assignments {
		if a.EventID == late.ID {
			seen = true
			if a.StartTime != "19:00" {
				t.Errorf("the assignment carries the start time: %+v", a)
			}
		}
	}
	if !seen {
		t.Errorf("Aslaug has no assignment on the late event: %+v", m.Assignments)
	}
}
