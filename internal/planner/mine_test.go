package planner

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func (w world) runWith(t *testing.T, answer []byte) *Service {
	t.Helper()
	s := newService(w, func(context.Context, []byte) ([]byte, error) { return answer, nil })
	if _, err := s.Start(ctx, w.org.Slug, w.plan.Slug, "Kari", Params{TimeLimit: 5}); err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	return s
}

func describe(m Mine) string {
	out := ""
	for _, a := range m.Assignments {
		out += fmt.Sprintf("[%s %s via=%q locked=%v weak=%v]", a.Date, a.Task, a.Via, a.Locked, a.Weak)
	}
	return out
}

// groupAnswer: Koret (Aslaug, Birger) is on Lyd on event 0 (Birger said «dårlig»), set by hand on Lovsang on event 1;
// Cecilie is locked on Bilde on event 0; a guest has Lovsang on event 0.
func TestMineShowsOwnAndGroupAssignmentsInTheOrderOfTheEvents(t *testing.T) {
	w := newWorld(t, "Menighet")
	koret := w.group(t, "Kirkekoret", []string{"Aslaug", "Birger"}, store.GroupTask{TaskID: w.lyd.ID, Fills: 1, Level: "neutral"})
	s := w.runWith(t, groupAnswer(w, koret))
	mine := func(name string) Mine {
		m, err := s.Mine(ctx, w.org.Slug, w.plan.Slug, w.people[name].ID)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	for name, want := range map[string]string{
		"Aslaug":  `[2026-08-09 Lyd via="Kirkekoret" locked=false weak=false][2026-08-16 Lovsang via="Kirkekoret" locked=true weak=false]`,
		"Birger":  `[2026-08-09 Lyd via="Kirkekoret" locked=false weak=true][2026-08-16 Lovsang via="Kirkekoret" locked=true weak=false]`, // «dårlig» is his, not the group's
		"Cecilie": `[2026-08-09 Bilde via="" locked=true weak=false]`,
		"Frode":   ``,
	} {
		if got := describe(mine(name)); got != want {
			t.Errorf("%s:\n got %s\nwant %s", name, got, want)
		}
	}
	m := mine("Aslaug")
	if !m.HasPlan || m.Run == nil || m.Run.Actor != "Kari" || m.Run.Stale || m.Person.Name != "Aslaug" || m.Assignments[0].DayType != "Gudstjeneste" {
		t.Errorf("%+v", m)
	}
	// «a text» is nobody's assignment, and a person with nothing gets a list, not null
	if f := mine("Frode"); f.Assignments == nil || len(f.Assignments) != 0 || !f.HasPlan {
		t.Errorf("nothing for Frode: %+v", f)
	}
	// the plan is out of date once the data has changed
	w.st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, "T", w.people["Dina"].ID, map[int64]string{w.events[3].ID: "never"})
	if !mine("Aslaug").Run.Stale {
		t.Error("a change after the run makes it stale")
	}
}

func TestMineUsesTheNewestFinishedRunAndSaysWhenThereIsNoPlan(t *testing.T) {
	w := newWorld(t, "Menighet")
	koret := w.group(t, "Kirkekoret", []string{"Aslaug", "Birger"}, store.GroupTask{TaskID: w.lyd.ID, Fills: 1, Level: "neutral"})
	anna := w.people["Aslaug"].ID
	none := newService(w, nil)
	if m, err := none.Mine(ctx, w.org.Slug, w.plan.Slug, anna); err != nil || m.HasPlan || m.Run != nil || m.Assignments == nil || len(m.Assignments) != 0 {
		t.Errorf("no run yet: %+v %v", m, err)
	}
	failing := newService(w, func(context.Context, []byte) ([]byte, error) { return nil, errors.New("boom") })
	failing.Start(ctx, w.org.Slug, w.plan.Slug, "Kari", Params{TimeLimit: 5})
	waitDone(t, failing)
	if m, _ := failing.Mine(ctx, w.org.Slug, w.plan.Slug, anna); m.HasPlan {
		t.Errorf("a failed run is not a plan: %+v", m)
	}
	good := w.runWith(t, groupAnswer(w, koret))
	failing2 := newService(w, func(context.Context, []byte) ([]byte, error) { return nil, errors.New("boom") })
	failing2.Start(ctx, w.org.Slug, w.plan.Slug, "Ola", Params{TimeLimit: 5})
	waitDone(t, failing2)
	m, err := good.Mine(ctx, w.org.Slug, w.plan.Slug, anna)
	if err != nil || !m.HasPlan || len(m.Assignments) != 2 || m.Run.Actor != "Kari" {
		t.Errorf("the newest run failed, so the plan is the one before it: %+v %v", m, err)
	}
	// of two finished runs the newer is the plan (here one that gives Aslaug nothing)
	nothing := w.runWith(t, []byte(`{"status":"OPTIMAL","assignments":[],"unfilled":[],"load":{},"violations":[]}`))
	newer := newService(w, func(context.Context, []byte) ([]byte, error) {
		return []byte(`{"status":"OPTIMAL","assignments":[],"unfilled":[],"load":{},"violations":[]}`), nil
	})
	newer.Start(ctx, w.org.Slug, w.plan.Slug, "Eva", Params{TimeLimit: 5})
	waitDone(t, newer)
	_ = nothing
	if m, _ = newer.Mine(ctx, w.org.Slug, w.plan.Slug, anna); m.Run == nil || m.Run.Actor != "Eva" || len(m.Assignments) != 0 {
		t.Errorf("the newest finished run is the plan: %+v", m)
	}
}

func TestMineIsScopedToThePersonsOrganisationAndPlan(t *testing.T) {
	a := newWorld(t, "Org A")
	b := populate(t, a.st, "Org B")
	koret := a.group(t, "Kirkekoret", []string{"Aslaug", "Birger"}, store.GroupTask{TaskID: a.lyd.ID, Fills: 1, Level: "neutral"})
	s := a.runWith(t, groupAnswer(a, koret))
	if _, err := s.Mine(ctx, a.org.Slug, a.plan.Slug, b.people["Aslaug"].ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("another organisation's person through A: %v", err)
	}
	if _, err := s.Mine(ctx, b.org.Slug, b.plan.Slug, a.people["Aslaug"].ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("A's person through B: %v", err)
	}
	if _, err := s.Mine(ctx, a.org.Slug, "ingen", a.people["Aslaug"].ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
	if _, err := s.Mine(ctx, a.org.Slug, a.plan.Slug, 99999); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("unknown person: %v", err)
	}
	// B has a plan with the same slug and a person of the same name, but A's run is not B's plan
	if m, err := s.Mine(ctx, b.org.Slug, b.plan.Slug, b.people["Aslaug"].ID); err != nil || m.HasPlan || len(m.Assignments) != 0 {
		t.Errorf("B has no plan yet: %+v %v", m, err)
	}
}

func TestMineUsesThePinnedRunInsteadOfTheNewestOne(t *testing.T) {
	w := newWorld(t, "Club")
	koret := w.group(t, "Choir", []string{"Aslaug", "Birger"}, store.GroupTask{TaskID: w.lyd.ID, Fills: 1, Level: "neutral"})
	anna := w.people["Aslaug"].ID
	w.runWith(t, groupAnswer(w, koret)) // Aslaug has two assignments here
	empty := []byte(`{"status":"OPTIMAL","assignments":[],"unfilled":[],"load":{},"violations":[]}`)
	newer := newService(w, func(context.Context, []byte) ([]byte, error) { return empty, nil })
	newer.Start(ctx, w.org.Slug, w.plan.Slug, "Eva", Params{TimeLimit: 5})
	waitDone(t, newer)
	mine := func() Mine {
		t.Helper()
		m, err := newer.Mine(ctx, w.org.Slug, w.plan.Slug, anna)
		if err != nil {
			t.Fatal(err)
		}
		return m
	}
	runs, _ := w.st.ListRuns(ctx, w.org.Slug, w.plan.Slug, 0)
	if len(runs) != 2 {
		t.Fatalf("runs: %+v", runs)
	}
	newestID, olderID := runs[0].ID, runs[1].ID
	if m := mine(); m.Run.ID != newestID || m.Run.Pinned || len(m.Assignments) != 0 {
		t.Errorf("nothing pinned: the newest finished run is the plan: %+v", m.Run)
	}
	if err := w.st.PinRun(ctx, w.org.Slug, w.plan.Slug, "Ola", olderID, true); err != nil {
		t.Fatal(err)
	}
	if m := mine(); m.Run.ID != olderID || !m.Run.Pinned || len(m.Assignments) != 2 {
		t.Errorf("the pinned run is the plan, although a newer one exists: %+v %d assignments", m.Run, len(m.Assignments))
	}
	// a newer run, finished or failed, does not move the pin
	failing := newService(w, func(context.Context, []byte) ([]byte, error) { return nil, errors.New("boom") })
	failing.Start(ctx, w.org.Slug, w.plan.Slug, "Ola", Params{TimeLimit: 5})
	waitDone(t, failing)
	third := newService(w, func(context.Context, []byte) ([]byte, error) { return empty, nil })
	third.Start(ctx, w.org.Slug, w.plan.Slug, "Eva", Params{TimeLimit: 5})
	waitDone(t, third)
	if m := mine(); m.Run.ID != olderID || len(m.Assignments) != 2 {
		t.Errorf("new runs leave the pin where it was: %+v", m.Run)
	}
	if err := w.st.PinRun(ctx, w.org.Slug, w.plan.Slug, "Ola", olderID, false); err != nil {
		t.Fatal(err)
	}
	if m := mine(); m.Run.Pinned || m.Run.ID == olderID || len(m.Assignments) != 0 {
		t.Errorf("unpinned: back to the newest finished run: %+v", m.Run)
	}
}
