package store

import (
	"errors"
	"testing"
)

func cellsOf(t *testing.T, s *Store, org, plan string) map[[3]int64]bool {
	t.Helper()
	cells, err := s.ListCells(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	out := map[[3]int64]bool{} // event, task, person -> locked
	for _, c := range cells {
		if c.PersonID != nil {
			out[[3]int64{c.EventID, c.TaskID, *c.PersonID}] = c.Locked
		}
	}
	return out
}

func TestLockingManyNamesAtOnceKeepsWhatIsSetByHand(t *testing.T) {
	s := open(t)
	a := makeRich(t, s, "Club A", "Autumn 2026")
	b := makeRich(t, s, "Club B", "Autumn 2026")
	org, plan := a.org.Slug, a.plan.Slug
	evs, err := s.ListEvents(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	others, _ := s.ListEvents(ctx, org, a.other.Slug)
	kari, ola := a.people[0].ID, a.people[1].ID
	sound := a.sound.ID
	lock := func(on bool, l ...Lock) error { return s.SetLocks(ctx, org, plan, l, on) }

	// Ola is set by hand for Sound on the first event (makeRich): locking him there leaves the cell as it is
	if err := lock(true, Lock{evs[0].ID, sound, kari}, Lock{evs[1].ID, sound, kari}, Lock{evs[0].ID, sound, ola}); err != nil {
		t.Fatal(err)
	}
	got := cellsOf(t, s, org, plan)
	if locked, ok := got[[3]int64{evs[0].ID, sound, kari}]; !ok || !locked {
		t.Errorf("Kari on the first event is not locked: %v", got)
	}
	if locked, ok := got[[3]int64{evs[1].ID, sound, kari}]; !ok || !locked {
		t.Errorf("Kari on the second event is not locked: %v", got)
	}
	if locked, ok := got[[3]int64{evs[0].ID, sound, ola}]; !ok || locked {
		t.Errorf("Ola, set by hand, became locked or went: %v", got)
	}
	if err := lock(true, Lock{evs[0].ID, sound, kari}); !codeIs(err, "nothing_to_change") {
		t.Errorf("locking a locked name again: %v", err)
	}

	// unlocking takes the locked names away and never one set by hand
	if err := lock(false, Lock{evs[0].ID, sound, ola}); !codeIs(err, "nothing_to_change") {
		t.Errorf("unlocking a name set by hand: %v", err)
	}
	if err := lock(false, Lock{evs[0].ID, sound, kari}, Lock{evs[0].ID, sound, ola}); err != nil {
		t.Fatal(err)
	}
	got = cellsOf(t, s, org, plan)
	if _, ok := got[[3]int64{evs[0].ID, sound, kari}]; ok {
		t.Errorf("Kari is still locked on the first event: %v", got)
	}
	if _, ok := got[[3]int64{evs[1].ID, sound, kari}]; !ok {
		t.Errorf("Kari on the second event was unlocked too: %v", got)
	}
	if _, ok := got[[3]int64{evs[0].ID, sound, ola}]; !ok {
		t.Errorf("Ola, set by hand, was removed: %v", got)
	}

	// all or nothing: one name of another organisation, or an event of another plan, and nothing is written
	before := len(cellsOf(t, s, org, plan))
	for name, bad := range map[string]Lock{
		"another organisation's person": {evs[0].ID, sound, b.people[0].ID},
		"another organisation's task":   {evs[0].ID, b.sound.ID, kari},
		"another plan's event":          {others[0].ID, sound, kari},
	} {
		if err := lock(true, Lock{evs[0].ID, sound, kari}, bad); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v, want not found", name, err)
		}
	}
	if n := len(cellsOf(t, s, org, plan)); n != before {
		t.Errorf("a refused call wrote cells: %d, want %d", n, before)
	}
	if err := lock(true); !codeIs(err, "cells_count") {
		t.Errorf("no names: %v", err)
	}
}

func TestSettingTheDraftAsideHidesItUntilANewRunOrAPin(t *testing.T) {
	s := open(t)
	a := makeRich(t, s, "Club A", "Autumn 2026")
	org, plan := a.org.Slug, a.plan.Slug
	aside := func() bool {
		t.Helper()
		p, err := s.GetPlan(ctx, org, plan)
		if err != nil {
			t.Fatal(err)
		}
		return p.DraftAside
	}
	if aside() {
		t.Fatal("a new plan has its drafts set aside")
	}
	if err := s.SetDraftAside(ctx, org, plan); err != nil {
		t.Fatal(err)
	}
	if !aside() {
		t.Error("the drafts are not set aside")
	}
	if other, _ := s.GetPlan(ctx, org, a.other.Slug); other.DraftAside {
		t.Error("the other plan's drafts were set aside too")
	}
	runs, _ := s.ListRuns(ctx, org, plan, 0)
	for _, r := range runs {
		if r.Pinned {
			t.Errorf("run %d is still pinned", r.ID)
		}
	}
	if cells := cellsOf(t, s, org, plan); len(cells) == 0 {
		t.Error("what is set by hand went with the draft")
	}
	if err := s.SetDraftAside(ctx, org, plan); !codeIs(err, "nothing_to_change") {
		t.Errorf("setting aside again: %v", err)
	}
	if err := s.PinRun(ctx, org, plan, a.pinned, true); err != nil {
		t.Fatal(err)
	}
	if aside() {
		t.Error("pinning a draft does not show it again")
	}
	if err := s.SetDraftAside(ctx, org, plan); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateRun(ctx, org, plan, 9, 5, "h", []byte("{}"), []byte("{}")); err != nil {
		t.Fatal(err)
	}
	if aside() {
		t.Error("a new run does not show the drafts again")
	}
	if err := s.SetDraftAside(ctx, "nope", plan); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unknown organisation: %v", err)
	}
}
