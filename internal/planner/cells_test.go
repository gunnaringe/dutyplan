package planner

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

type lockedIn struct {
	Date   string `json:"date"`
	Task   string `json:"task"`
	Person string `json:"person"`
	Group  string `json:"group"`
	Label  string `json:"label"`
	Fills  int    `json:"fills"`
}
type poolIn struct {
	Date   string   `json:"date"`
	Task   string   `json:"task"`
	People []string `json:"people"`
	Groups []string `json:"groups"`
}

func (w world) cellsAndPools(t *testing.T) (locked []lockedIn, pools []poolIn, raw string, snap Snapshot, hash string) {
	t.Helper()
	in, h, sn, err := BuildInput(w.data(t), Params{})
	if err != nil {
		t.Fatal(err)
	}
	var v struct {
		Locked []lockedIn `json:"locked"`
		Pools  []poolIn   `json:"pools"`
	}
	if err := json.Unmarshal(in, &v); err != nil {
		t.Fatal(err)
	}
	return v.Locked, v.Pools, string(in), sn, h
}

func (w world) cell(t *testing.T, ev store.Event, task store.Task, c store.NewCell) {
	t.Helper()
	c.EventIDs, c.TaskID = []int64{ev.ID}, task.ID
	if _, _, err := w.st.AddCells(ctx, w.org.Slug, w.plan.Slug, "T", c); err != nil {
		t.Fatal(err)
	}
}

func (w world) pool(t *testing.T, ev store.Event, task store.Task, people []string, groups ...store.Group) {
	t.Helper()
	var ids, gids []int64
	for _, n := range people {
		ids = append(ids, w.people[n].ID)
	}
	for _, g := range groups {
		gids = append(gids, g.ID)
	}
	if err := w.st.SetPool(ctx, w.org.Slug, w.plan.Slug, "T", []int64{ev.ID}, task.ID, ids, gids); err != nil {
		t.Fatal(err)
	}
}

func TestBuildInputSendsManualCellsAndSelections(t *testing.T) {
	w := newWorld(t, "Menighet")
	koret := w.group(t, "Koret", []string{"Dina", "Embla"})
	w.cell(t, w.events[0], w.lyd, store.NewCell{PersonID: w.people["Aslaug"].ID})
	w.cell(t, w.events[1], w.lovsang, store.NewCell{GroupID: koret.ID, Fills: 2})
	w.cell(t, w.events[0], w.lovsang, store.NewCell{Label: "Gjest utenfra", Fills: 3})
	w.pool(t, w.events[2], w.lyd, []string{"Aslaug", "Cecilie"}, koret)
	w.pool(t, w.events[3], w.bilde, nil, koret) // groups only: «people» must be [] and not null

	locked, pools, raw, snap, hash := w.cellsAndPools(t)
	e := func(i int) string { return eid(w.events[i].ID) }
	want := []lockedIn{ // by event, then by task id (Lovsang was made first)
		{e(0), tid(w.lovsang.ID), "", "", "Gjest utenfra", 3},
		{e(0), tid(w.lyd.ID), pid(w.people["Aslaug"].ID), "", "", 0},
		{e(1), tid(w.lovsang.ID), "", gid(koret.ID), "", 2},
	}
	if fmt.Sprint(locked) != fmt.Sprint(want) {
		t.Errorf("locked:\n got %+v\nwant %+v", locked, want)
	}
	if len(pools) != 2 || fmt.Sprint(pools[0].People) != fmt.Sprint([]string{pid(w.people["Aslaug"].ID), pid(w.people["Cecilie"].ID)}) || fmt.Sprint(pools[0].Groups) != fmt.Sprint([]string{gid(koret.ID)}) {
		t.Errorf("pools: %+v", pools)
	}
	if !strings.Contains(raw, `"people":[],"groups":["`+gid(koret.ID)+`"]`) {
		t.Errorf("an empty list is [] and never null (the solver would fail on null): %s", raw)
	}
	if len(snap.Warnings) != 0 {
		t.Errorf("nothing to warn about: %v", snap.Warnings)
	}
	// the stored run is out of date when a manual cell or a selection changes
	w.pool(t, w.events[2], w.lyd, []string{"Aslaug"})
	if _, _, _, _, h2 := w.cellsAndPools(t); h2 == hash {
		t.Error("a changed selection must change the hash")
	}
	w.cell(t, w.events[3], w.bilde, store.NewCell{Label: "Gjest"})
	if _, _, _, _, h3 := w.cellsAndPools(t); h3 == hash {
		t.Error("a new manual cell must change the hash")
	}
}

func TestBuildInputLeavesOutWhatNamesSomeoneWhoIsNotActive(t *testing.T) {
	w := newWorld(t, "Menighet")
	koret := w.group(t, "Koret", []string{"Dina", "Embla"})
	w.cell(t, w.events[0], w.lyd, store.NewCell{PersonID: w.people["Aslaug"].ID})
	w.cell(t, w.events[1], w.lovsang, store.NewCell{GroupID: koret.ID})
	w.cell(t, w.events[2], w.lovsang, store.NewCell{Label: "Gjest"})
	w.pool(t, w.events[0], w.lyd, []string{"Aslaug", "Birger"}, koret)
	w.pool(t, w.events[1], w.lyd, []string{"Aslaug"})
	off := false
	deactivate := func(names ...string) {
		for _, n := range names {
			w.st.UpdatePerson(ctx, w.org.Slug, "T", w.people[n].ID, store.PersonPatch{Active: &off})
		}
	}

	deactivate("Aslaug")
	locked, pools, _, snap, _ := w.cellsAndPools(t)
	for _, l := range locked {
		if l.Person != "" {
			t.Errorf("a manual cell for an inactive person must not be sent (the solver would count the place without knowing who): %+v", l)
		}
	}
	if len(locked) != 2 {
		t.Errorf("the group and the text stay: %+v", locked)
	}
	if len(snap.Warnings) != 2 || !strings.Contains(snap.Warnings[0], "Ruten for Aslaug på Lyd 2026-08-09") {
		t.Errorf("the run says it left her out (and that her selection on 16 Aug is now empty): %v", snap.Warnings)
	}
	if fmt.Sprint(pools[0].People) != fmt.Sprint([]string{pid(w.people["Birger"].ID)}) {
		t.Errorf("an inactive person leaves the selection: %+v", pools[0])
	}
	// a selection left with nobody still restricts (nobody can be chosen) and says so
	if len(pools) != 2 || len(pools[1].People) != 0 || len(pools[1].Groups) != 0 {
		t.Errorf("the restriction stays when everyone in it is gone: %+v", pools)
	}
	if !strings.Contains(strings.Join(snap.Warnings, "|"), "Utvalget for Lyd 2026-08-16 har ingen aktive") {
		t.Errorf("warnings: %v", snap.Warnings)
	}

	// a group without active members: its manual cell and its place in a selection go too
	deactivate("Dina", "Embla")
	locked, pools, _, snap, _ = w.cellsAndPools(t)
	for _, l := range locked {
		if l.Group != "" {
			t.Errorf("a group with nobody active must not be sent: %+v", l)
		}
	}
	if len(pools[0].Groups) != 0 {
		t.Errorf("the group leaves the selection: %+v", pools[0])
	}
	if !strings.Contains(strings.Join(snap.Warnings, "|"), "gruppen «Koret» på Lovsang 2026-08-16") {
		t.Errorf("warnings: %v", snap.Warnings)
	}
}

func TestBuildInputNeverSendsAnotherPlansOrOrganisationsCells(t *testing.T) {
	a := newWorld(t, "Org A")
	b := populate(t, a.st, "Org B")
	other, _ := a.st.CreatePlan(ctx, a.org.Slug, "T", "Vår 2027", "")
	otherEv, _ := a.st.AddEvent(ctx, a.org.Slug, other.Slug, "T", "2027-02-07", a.dt.ID, "")
	a.cell(t, a.events[0], a.lyd, store.NewCell{PersonID: a.people["Aslaug"].ID})
	b.cell(t, b.events[0], b.lyd, store.NewCell{PersonID: b.people["Aslaug"].ID})
	b.pool(t, b.events[0], b.lyd, []string{"Birger"})
	if _, err := a.st.GetPlan(ctx, a.org.Slug, other.Slug); err != nil {
		t.Fatal(err)
	}
	a.st.AddCells(ctx, a.org.Slug, other.Slug, "T", store.NewCell{EventIDs: []int64{otherEv.ID}, TaskID: a.lyd.ID, PersonID: a.people["Birger"].ID})

	locked, pools, raw, _, _ := a.cellsAndPools(t)
	if len(locked) != 1 || locked[0].Person != pid(a.people["Aslaug"].ID) || len(pools) != 0 {
		t.Errorf("A's plan sends its own cell only: %+v %+v", locked, pools)
	}
	if strings.Contains(raw, eid(b.events[0].ID)) || strings.Contains(raw, eid(otherEv.ID)) {
		t.Errorf("an event of another plan or organisation is in the input: %s", raw)
	}
	lockedB, poolsB, _, _, _ := b.cellsAndPools(t)
	if len(lockedB) != 1 || lockedB[0].Person != pid(b.people["Aslaug"].ID) || len(poolsB) != 1 {
		t.Errorf("B: %+v %+v", lockedB, poolsB)
	}
}

// The user's own case: the choir (a group) sings, set by hand on Lovsang, a task the planner does not distribute.
// Lovsang cannot be combined with Lyd or Bilde, so the choir's members are out of both, and Lyd is limited to two people.
func TestTheRealSolverKeepsTheChoirOutOfLydAndBildeAndHonoursTheSelection(t *testing.T) {
	solver := realSolver(t)
	w := newWorld(t, "Menighet")
	for _, n := range []string{"Dina", "Embla"} { // without the rule, Dina and Embla could take Lyd too
		w.st.SetPersonTask(ctx, w.org.Slug, "T", w.people[n].ID, w.lyd.ID, "godt")
	}
	w.st.SetCombination(ctx, w.org.Slug, "T", w.lovsang.ID, w.lyd.ID, "kan_ikke")
	w.st.SetCombination(ctx, w.org.Slug, "T", w.lovsang.ID, w.bilde.ID, "kan_ikke")
	koret := w.group(t, "Kirkekoret", []string{"Dina", "Embla"})
	ev := w.events[3]
	w.cell(t, ev, w.lovsang, store.NewCell{GroupID: koret.ID})
	w.pool(t, ev, w.lyd, []string{"Aslaug", "Cecilie"}) // «egen lydtekniker»

	s := New(w.st, solver, 1, nopLogger())
	run := func() RunView {
		id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, "Kari", Params{TimeLimit: 5, Seed: 1})
		if err != nil {
			t.Fatal(err)
		}
		waitDone(t, s)
		v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
		if err != nil || v.Run.Status != "done" {
			t.Fatalf("%+v %v", v.Run, err)
		}
		if !v.Run.Checked || len(v.Run.Violations) != 0 {
			t.Fatalf("the independent checker: %v", v.Run.Violations)
		}
		return v
	}
	cellOf := func(v RunView, ev store.Event, task store.Task) *Cell {
		for _, row := range v.Events {
			if row.ID != ev.ID {
				continue
			}
			for i := range row.Cells {
				if row.Cells[i].TaskID == task.ID {
					return &row.Cells[i]
				}
			}
		}
		return nil
	}
	v := run()
	var cols []string
	for _, tk := range v.Tasks {
		cols = append(cols, tk.Name)
	}
	if fmt.Sprint(cols) != "[Bilde Lovsang Lyd]" {
		t.Errorf("Lovsang is a column because it has a manual cell: %v", cols)
	}
	sang := cellOf(v, ev, w.lovsang)
	if sang == nil || len(sang.Groups) != 1 || !sang.Groups[0].Locked || sang.Groups[0].Name != "Kirkekoret" || sang.Slots != 1 || sang.Missing != 0 {
		t.Fatalf("the choir on Lovsang: %+v", sang)
	}
	for _, row := range v.Events { // the day type has a Lovsang place everywhere, but the planner does not distribute it
		if row.ID != ev.ID && cellOf(v, store.Event{ID: row.ID}, w.lovsang) != nil {
			t.Errorf("no empty Lovsang place on %s: %+v", row.Date, cellOf(v, store.Event{ID: row.ID}, w.lovsang))
		}
	}
	lyd, bilde := cellOf(v, ev, w.lyd), cellOf(v, ev, w.bilde)
	for _, p := range lyd.People {
		if p.Name != "Aslaug" && p.Name != "Cecilie" {
			t.Errorf("only the two in the selection can take Lyd, got %s", p.Name)
		}
	}
	if len(lyd.People) != 1 || lyd.Missing != 0 {
		t.Errorf("Lyd is filled from the selection: %+v", lyd)
	}
	for _, p := range append(append([]CellPerson{}, lyd.People...), bilde.People...) {
		if p.Name == "Dina" || p.Name == "Embla" {
			t.Errorf("the choir's members are busy with Lovsang and cannot take Lyd or Bilde: %+v", p)
		}
	}

	// nobody in the selection can: the place stays empty and the explanation names the selection and the reason
	for _, n := range []string{"Aslaug", "Cecilie"} {
		w.st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, "T", w.people[n].ID, map[int64]string{ev.ID: "ikke"})
	}
	v = run()
	lyd = cellOf(v, ev, w.lyd)
	if lyd.Missing != 1 || !strings.Contains(lyd.Explanation, "utvalget er Aslaug, Cecilie") || !strings.Contains(lyd.Explanation, "passer ikke: Aslaug, Cecilie") {
		t.Errorf("the explanation: %+v", lyd)
	}
}
