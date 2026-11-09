package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// codeIs says whether err is a coded refusal with this code.
func codeIs(err error, code string) bool {
	var c *Coded
	return errors.As(err, &c) && c.Code == code
}

func finishedRun(t *testing.T, e prefEnv, seed int) int64 {
	t.Helper()
	id, err := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, "Kari", seed, 5, "h", []byte("{}"), []byte("{}"))
	if err != nil {
		t.Fatal(err)
	}
	if err := e.s.FinishRun(ctx, id, []byte(`{"status":"OPTIMAL"}`), ""); err != nil {
		t.Fatal(err)
	}
	return id
}

func pinnedIDs(t *testing.T, e prefEnv) []int64 {
	t.Helper()
	runs, err := e.s.ListRuns(ctx, e.org.Slug, e.plan.Slug, 0)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, r := range runs {
		if r.Pinned {
			ids = append(ids, r.ID)
		}
	}
	return ids
}

func TestPinningARunMovesTheOnePinAndOnlyFinishedRunsCanBePinned(t *testing.T) {
	e := newPrefEnv(t)
	r1 := finishedRun(t, e, 1)
	failed, _ := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, "Kari", 2, 5, "h", []byte("{}"), []byte("{}"))
	e.s.FinishRun(ctx, failed, nil, "boom")
	running, _ := e.s.CreateRun(ctx, e.org.Slug, e.plan.Slug, "Kari", 3, 5, "h", []byte("{}"), []byte("{}"))

	for _, id := range []int64{failed, running} {
		if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", id, true); !codeIs(err, "run_not_done") || !errors.Is(err, ErrInvalid) {
			t.Errorf("pinning run %d that is not finished: %v", id, err)
		}
	}
	if got := pinnedIDs(t, e); len(got) != 0 {
		t.Fatalf("nothing is pinned yet: %v", got)
	}
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", r1, true); err != nil {
		t.Fatal(err)
	}
	if got := pinnedIDs(t, e); len(got) != 1 || got[0] != r1 {
		t.Fatalf("pinned: %v", got)
	}
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", r1, true); !codeIs(err, "nothing_to_change") {
		t.Errorf("pinning twice: %v", err)
	}
	e.s.FinishRun(ctx, running, []byte(`{"status":"OPTIMAL"}`), "")
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", running, true); err != nil {
		t.Fatal(err)
	}
	if got := pinnedIDs(t, e); len(got) != 1 || got[0] != running {
		t.Errorf("pinning another run moves the pin: %v", got)
	}
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", r1, false); !codeIs(err, "nothing_to_change") {
		t.Errorf("unpinning a run that is not pinned: %v", err)
	}
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", running, false); err != nil {
		t.Fatal(err)
	}
	if got := pinnedIDs(t, e); len(got) != 0 {
		t.Errorf("unpinned: %v", got)
	}
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "", r1, true); !errors.Is(err, ErrActor) {
		t.Errorf("pinning needs an actor: %v", err)
	}
	var actions []string
	for _, l := range mustLog(t, e.s, e.org.Slug, e.plan.Slug) {
		if strings.HasPrefix(l.Action, "run.") {
			actions = append(actions, l.Action+" "+l.Detail+" by "+l.Actor)
		}
	}
	if got, want := strings.Join(actions, "; "), fmt.Sprintf("run.unpin run #%d by Ola; run.pin run #%d by Ola; run.pin run #%d by Ola", running, running, r1); got != want {
		t.Errorf("log (newest first):\n got  %s\n want %s", got, want)
	}
}

func TestOnlyOneRunPerPlanCanBePinnedAtTheDatabase(t *testing.T) {
	e := newPrefEnv(t)
	r1, r2 := finishedRun(t, e, 1), finishedRun(t, e, 2)
	db := e.s.DB()
	if _, err := db.Exec(`UPDATE plan_run SET pinned = 1`); err == nil || !strings.Contains(err.Error(), "UNIQUE") {
		t.Errorf("two pinned runs in one plan must fail at the unique index: %v (runs %d, %d)", err, r1, r2)
	}
	if _, err := db.Exec(`UPDATE plan_run SET pinned = 2 WHERE id = ?`, r1); err == nil {
		t.Error("pinned must be 0 or 1")
	}
}

func TestPinningStaysInsideThePlanAndOrganisation(t *testing.T) {
	a := newPrefEnv(t)
	idA := finishedRun(t, a, 1)
	other, _ := a.s.CreatePlan(ctx, a.org.Slug, "Ola", "Spring 2027", "")
	b := mustOrg(t, a.s, "Org B")
	planB, _ := a.s.CreatePlan(ctx, b.Slug, "Eva", "Autumn 2026", "")

	for name, call := range map[string]func() error{
		"through another plan": func() error { return a.s.PinRun(ctx, a.org.Slug, other.Slug, "Ola", idA, true) },
		"through another org":  func() error { return a.s.PinRun(ctx, b.Slug, planB.Slug, "Eva", idA, true) },
		"an unknown run":       func() error { return a.s.PinRun(ctx, a.org.Slug, a.plan.Slug, "Ola", 99999, true) },
		"an unknown plan":      func() error { return a.s.PinRun(ctx, a.org.Slug, "nope", "Ola", idA, true) },
		"an unknown org":       func() error { return a.s.PinRun(ctx, "nope", a.plan.Slug, "Ola", idA, true) },
		"unpinning through B":  func() error { return a.s.PinRun(ctx, b.Slug, planB.Slug, "Eva", idA, false) },
	} {
		if err := call(); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if got := pinnedIDs(t, a); len(got) != 0 {
		t.Errorf("a refused pin left a pin behind: %v", got)
	}
	// each plan has its own pin
	idOther, _ := a.s.CreateRun(ctx, a.org.Slug, other.Slug, "Kari", 1, 5, "h", []byte("{}"), []byte("{}"))
	a.s.FinishRun(ctx, idOther, []byte("{}"), "")
	if err := a.s.PinRun(ctx, a.org.Slug, a.plan.Slug, "Ola", idA, true); err != nil {
		t.Fatal(err)
	}
	if err := a.s.PinRun(ctx, a.org.Slug, other.Slug, "Ola", idOther, true); err != nil {
		t.Fatalf("a pin in another plan must not move this plan's pin: %v", err)
	}
	if got := pinnedIDs(t, a); len(got) != 1 || got[0] != idA {
		t.Errorf("the first plan keeps its pin: %v", got)
	}
	if l, _ := a.s.ListRuns(ctx, b.Slug, planB.Slug, 10); len(l) != 0 {
		t.Errorf("B lists A's pinned run: %+v", l)
	}
}

func TestAPinnedRunSurvivesPruningAndStaysInTheList(t *testing.T) {
	e := newPrefEnv(t)
	first := finishedRun(t, e, 1)
	if err := e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", first, true); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxRunsPerPlan+3; i++ {
		finishedRun(t, e, i+2)
	}
	if n := count(t, e.s, `SELECT COUNT(*) FROM plan_run`); n != maxRunsPerPlan+1 {
		t.Errorf("%d runs kept, want the newest %d and the pinned one", n, maxRunsPerPlan)
	}
	if _, err := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, first); err != nil {
		t.Errorf("the pinned run was pruned: %v", err)
	}
	runs, _ := e.s.ListRuns(ctx, e.org.Slug, e.plan.Slug, 5)
	if len(runs) != 6 || runs[5].ID != first || !runs[5].Pinned {
		t.Errorf("the list is the newest five and the pinned run: %+v", runs)
	}
	// unpinned, it is an ordinary old run again and goes with the next prune
	e.s.PinRun(ctx, e.org.Slug, e.plan.Slug, "Ola", first, false)
	finishedRun(t, e, 99)
	if _, err := e.s.GetRun(ctx, e.org.Slug, e.plan.Slug, first); !errors.Is(err, ErrNotFound) {
		t.Errorf("an unpinned old run is pruned: %v", err)
	}
}
