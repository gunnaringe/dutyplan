package planner

import (
	"encoding/json"
	"fmt"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

type datesView struct {
	Dates []struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	} `json:"dates"`
	DayTypeTasks map[string]map[string]int `json:"day_type_tasks"`
}

func (w world) dates(t *testing.T) (datesView, Snapshot, string) {
	t.Helper()
	raw, hash, snap, err := BuildInput(w.data(t), Params{})
	if err != nil {
		t.Fatal(err)
	}
	var in datesView
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in, snap, hash
}

func (w world) places(t *testing.T, ev store.Event, task store.Task, n int) {
	t.Helper()
	if err := w.st.SetEventPlaces(ctx, w.org.Slug, w.plan.Slug, ev.ID, task.ID, n); err != nil {
		t.Fatal(err)
	}
}

func TestEveryEventIsATypeOfItsOwnWithItsPlacesInTheInput(t *testing.T) {
	w := newWorld(t, "Club")
	before, snap, hash := w.dates(t)
	all := map[string]int{tid(w.sound.ID): 1, tid(w.camera.ID): 1, tid(w.singing.ID): 1}
	for i, d := range before.Dates {
		if d.ID != eid(w.events[i].ID) || d.Type != eid(w.events[i].ID) || fmt.Sprint(before.DayTypeTasks[d.Type]) != fmt.Sprint(all) {
			t.Fatalf("event %d: %+v %v", i, d, before.DayTypeTasks[d.Type])
		}
	}
	if got := snap.SlotsFor(snap.Events[0]); fmt.Sprint(got) != fmt.Sprint(map[int64]int{w.sound.ID: 1, w.camera.ID: 1}) {
		t.Errorf("the snapshot keeps the places of the planned tasks: %v", got)
	}

	// 2 on Sound and 0 on Camera (Singing stays) on event 1; the others keep theirs
	w.places(t, w.events[1], w.sound, 2)
	w.places(t, w.events[1], w.camera, 0)
	in, snap, h := w.dates(t)
	if h == hash {
		t.Error("an event's places must change the hash")
	}
	own := eid(w.events[1].ID)
	if want := map[string]int{tid(w.sound.ID): 2, tid(w.singing.ID): 1}; fmt.Sprint(in.DayTypeTasks[own]) != fmt.Sprint(want) {
		t.Errorf("the event's places: %v, want %v", in.DayTypeTasks[own], want)
	}
	if fmt.Sprint(in.DayTypeTasks[eid(w.events[0].ID)]) != fmt.Sprint(all) {
		t.Errorf("another event must not change: %v", in.DayTypeTasks[eid(w.events[0].ID)])
	}
	if got := snap.SlotsFor(snap.Events[1]); fmt.Sprint(got) != fmt.Sprint(map[int64]int{w.sound.ID: 2}) {
		t.Errorf("SlotsFor the changed event: %v", got)
	}

	// back as it was: the input is as before
	w.places(t, w.events[1], w.sound, 1)
	w.places(t, w.events[1], w.camera, 1)
	if _, _, h := w.dates(t); h != hash {
		t.Error("the same places must give the same input")
	}
}

func TestATaskOnlyOneEventHasIsSentForThatEvent(t *testing.T) {
	w := newWorld(t, "Club")
	extra, err := w.st.CreateTask(ctx, w.org.Slug, "Coffee")
	if err != nil {
		t.Fatal(err)
	}
	_, _, hash := w.dates(t)
	w.places(t, w.events[2], extra, 3)
	in, snap, h := w.dates(t)
	if h == hash {
		t.Error("a new place must change the hash")
	}
	own := eid(w.events[2].ID)
	if in.DayTypeTasks[own][tid(extra.ID)] != 3 || len(in.DayTypeTasks[own]) != 4 || len(in.DayTypeTasks[eid(w.events[1].ID)]) != 3 {
		t.Errorf("event 2: %v, event 1: %v", in.DayTypeTasks[own], in.DayTypeTasks[eid(w.events[1].ID)])
	}
	if snap.Places[w.events[2].ID][extra.ID] != 3 {
		t.Errorf("snapshot: %v", snap.Places)
	}
}

func TestAPlanHasWorkOnlyWithAPlaceForAPlannedTask(t *testing.T) {
	w := newWorld(t, "Club")
	for _, e := range w.events {
		w.places(t, e, w.sound, 0)
		w.places(t, e, w.camera, 0)
	}
	if HasWork(w.data(t)) {
		t.Fatal("only a manual task on the events: nothing for the planner")
	}
	w.places(t, w.events[0], w.camera, 1)
	if !HasWork(w.data(t)) {
		t.Error("one place for a planned task is work")
	}
}

func TestThePlanShowsAnEventsOwnPlaces(t *testing.T) {
	w := newWorld(t, "Club")
	// the canned answer says nothing about events 2 and 3: their places show as missing
	w.places(t, w.events[2], w.sound, 2)
	w.places(t, w.events[3], w.camera, 0)
	s := w.runWith(t, cannedAnswer(w))
	runs, _ := w.st.ListRuns(ctx, w.org.Slug, w.plan.Slug, 0)
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, runs[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	cells := func(row EventRow) string {
		out := ""
		for _, c := range row.Cells {
			out += fmt.Sprintf("[%d slots=%d missing=%d]", c.TaskID, c.Slots, c.Missing)
		}
		return out
	}
	if got, want := cells(v.Events[2]), fmt.Sprintf("[%d slots=1 missing=1][%d slots=2 missing=2]", w.camera.ID, w.sound.ID); got != want {
		t.Errorf("event 2: %s, want %s", got, want)
	}
	if got, want := cells(v.Events[3]), fmt.Sprintf("[%d slots=1 missing=1]", w.sound.ID); got != want {
		t.Errorf("event 3 (no Camera on it): %s, want %s", got, want)
	}
}

func TestDetailsRowsAreNotPartOfTheInput(t *testing.T) {
	w := newWorld(t, "Club")
	raw, hash, _, err := BuildInput(w.data(t), Params{})
	if err != nil {
		t.Fatal(err)
	}
	f, err := w.st.AddField(ctx, w.org.Slug, w.plan.Slug, "Theme")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.st.SetFieldValue(ctx, w.org.Slug, w.plan.Slug, w.events[0].ID, f.ID, "Light"); err != nil {
		t.Fatal(err)
	}
	again, h, _, _ := BuildInput(w.data(t), Params{})
	if h != hash || string(again) != string(raw) {
		t.Error("a details row is for people, not the planner: it must not change the input")
	}
}

func TestTheRealSolverFillsAnEventsOwnPlaces(t *testing.T) {
	solver := realSolver(t)
	w := newWorld(t, "Club")
	w.places(t, w.events[2], w.sound, 2) // Aslaug, Birger and Cecilie take Sound
	w.places(t, w.events[3], w.camera, 0)
	s := New(w.st, solver, 1, nopLogger())
	id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5, Seed: 1})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	if err != nil {
		t.Fatal(err)
	}
	if v.Run.Status != "done" || !v.Run.Checked || len(v.Run.Violations) != 0 {
		t.Fatalf("run: %+v", v.Run)
	}
	for _, c := range v.Events[2].Cells {
		if c.TaskID == w.sound.ID && (c.Slots != 2 || len(c.People) != 2 || c.Missing != 0) {
			t.Errorf("Sound on event 2 has two places, both filled: %+v", c)
		}
	}
	for _, c := range v.Events[3].Cells {
		if c.TaskID == w.camera.ID {
			t.Errorf("no Camera on event 3: %+v", c)
		}
	}
}
