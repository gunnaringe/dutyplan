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
	Label  string `json:"label"`
	Fills  int    `json:"fills"`
}
type poolIn struct {
	Date   string   `json:"date"`
	Task   string   `json:"task"`
	People []string `json:"people"`
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
	c.EventID, c.TaskID = ev.ID, task.ID
	if _, err := w.st.AddCell(ctx, w.org.Slug, w.plan.Slug, c); err != nil {
		t.Fatal(err)
	}
}

func (w world) pool(t *testing.T, ev store.Event, task store.Task, people []string) {
	t.Helper()
	var ids []int64
	for _, n := range people {
		ids = append(ids, w.people[n].ID)
	}
	if err := w.st.SetPool(ctx, w.org.Slug, w.plan.Slug, ev.ID, task.ID, ids); err != nil {
		t.Fatal(err)
	}
}

func TestBuildInputSendsManualCellsAndSelections(t *testing.T) {
	w := newWorld(t, "Club")
	w.cell(t, w.events[0], w.sound, store.NewCell{PersonID: w.people["Aslaug"].ID})
	w.cell(t, w.events[1], w.singing, store.NewCell{PersonID: w.people["Dina"].ID})
	w.cell(t, w.events[0], w.singing, store.NewCell{Label: "Outside guest", Fills: 3})
	w.pool(t, w.events[2], w.sound, []string{"Aslaug", "Cecilie"})
	w.pool(t, w.events[3], w.camera, []string{"Birger"})

	locked, pools, raw, _, hash := w.cellsAndPools(t)
	e := func(i int) string { return eid(w.events[i].ID) }
	want := []lockedIn{ // by event, then by task id (Singing was made first)
		{e(0), tid(w.singing.ID), "", "Outside guest", 3},
		{e(0), tid(w.sound.ID), pid(w.people["Aslaug"].ID), "", 0},
		{e(1), tid(w.singing.ID), pid(w.people["Dina"].ID), "", 0},
	}
	if fmt.Sprint(locked) != fmt.Sprint(want) {
		t.Errorf("locked:\n got %+v\nwant %+v", locked, want)
	}
	if len(pools) != 2 || fmt.Sprint(pools[0].People) != fmt.Sprint([]string{pid(w.people["Aslaug"].ID), pid(w.people["Cecilie"].ID)}) || fmt.Sprint(pools[1].People) != fmt.Sprint([]string{pid(w.people["Birger"].ID)}) {
		t.Errorf("pools: %+v", pools)
	}
	if strings.Contains(raw, `"groups"`) || strings.Contains(raw, `"group"`) {
		t.Errorf("the planner is told nothing about groups: %s", raw)
	}
	// the stored run is out of date when a manual cell or a selection changes
	w.pool(t, w.events[2], w.sound, []string{"Aslaug"})
	if _, _, _, _, h2 := w.cellsAndPools(t); h2 == hash {
		t.Error("a changed selection must change the hash")
	}
	w.cell(t, w.events[3], w.camera, store.NewCell{Label: "Guest"})
	if _, _, _, _, h3 := w.cellsAndPools(t); h3 == hash {
		t.Error("a new manual cell must change the hash")
	}
}

func TestBuildInputNeverSendsAnotherPlansOrOrganisationsCells(t *testing.T) {
	a := newWorld(t, "Org A")
	b := populate(t, a.st, "Org B")
	other, _ := a.st.CreatePlan(ctx, a.org.Slug, "Spring 2027")
	otherEvs, _ := addWithPlaces(a.st, a.org.Slug, other.Slug, store.NewEvents{Date: "2027-02-07"}, map[int64]int{a.sound.ID: 1})
	otherEv := otherEvs[0]
	a.cell(t, a.events[0], a.sound, store.NewCell{PersonID: a.people["Aslaug"].ID})
	b.cell(t, b.events[0], b.sound, store.NewCell{PersonID: b.people["Aslaug"].ID})
	b.pool(t, b.events[0], b.sound, []string{"Birger"})
	if _, err := a.st.GetPlan(ctx, a.org.Slug, other.Slug); err != nil {
		t.Fatal(err)
	}
	a.st.AddCell(ctx, a.org.Slug, other.Slug, store.NewCell{EventID: otherEv.ID, TaskID: a.sound.ID, PersonID: a.people["Birger"].ID})

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

// The user's own case: Dina and Embla sing, set by hand on Singing, a task the planner does not distribute. Singing cannot
// be combined with Sound or Camera, so the singers are out of both, and Sound is limited to two people.
func TestTheRealSolverKeepsTheSingersOutOfSoundAndCameraAndHonoursTheSelection(t *testing.T) {
	solver := realSolver(t)
	w := newWorld(t, "Club")
	for _, n := range []string{"Dina", "Embla"} { // without the rule, Dina and Embla could take Sound too
		w.st.SetPersonTask(ctx, w.org.Slug, w.people[n].ID, w.sound.ID, "good")
	}
	w.st.SetCombination(ctx, w.org.Slug, w.singing.ID, w.sound.ID, "cannot")
	w.st.SetCombination(ctx, w.org.Slug, w.singing.ID, w.camera.ID, "cannot")
	ev := w.events[3]
	w.cell(t, ev, w.singing, store.NewCell{PersonID: w.people["Dina"].ID})
	w.cell(t, ev, w.singing, store.NewCell{PersonID: w.people["Embla"].ID})
	w.pool(t, ev, w.sound, []string{"Aslaug", "Cecilie"}) // «egen lydtekniker»

	s := New(w.st, solver, 1, nopLogger())
	run := func() RunView {
		id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5, Seed: 1})
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
	if fmt.Sprint(cols) != "[Camera Singing Sound]" {
		t.Errorf("Singing is a column because it has a manual cell: %v", cols)
	}
	sang := cellOf(v, ev, w.singing)
	if sang == nil || len(sang.People) != 2 || !sang.People[0].Locked || sang.People[0].Name != "Dina" || sang.Slots != 2 || sang.Missing != 0 {
		t.Fatalf("the singers on Singing: %+v", sang)
	}
	for _, row := range v.Events { // the day type has a Singing place everywhere, but the planner does not distribute it
		if row.ID != ev.ID && cellOf(v, store.Event{ID: row.ID}, w.singing) != nil {
			t.Errorf("no empty Singing place on %s: %+v", row.Date, cellOf(v, store.Event{ID: row.ID}, w.singing))
		}
	}
	sound, camera := cellOf(v, ev, w.sound), cellOf(v, ev, w.camera)
	for _, p := range sound.People {
		if p.Name != "Aslaug" && p.Name != "Cecilie" {
			t.Errorf("only the two in the selection can take Sound, got %s", p.Name)
		}
	}
	if len(sound.People) != 1 || sound.Missing != 0 {
		t.Errorf("Sound is filled from the selection: %+v", sound)
	}
	for _, p := range append(append([]CellPerson{}, sound.People...), camera.People...) {
		if p.Name == "Dina" || p.Name == "Embla" {
			t.Errorf("the singers are busy with Singing and cannot take Sound or Camera: %+v", p)
		}
	}

	// nobody in the selection can: the place stays empty and the explanation names the selection and the reason
	for _, n := range []string{"Aslaug", "Cecilie"} {
		w.st.SetAvailability(ctx, w.org.Slug, w.plan.Slug, w.people[n].ID, map[int64]string{ev.ID: "never"})
	}
	v = run()
	sound = cellOf(v, ev, w.sound)
	if sound.Missing != 1 || !strings.Contains(sound.Explanation, "the selection is Aslaug, Cecilie") || !strings.Contains(sound.Explanation, "cannot attend: Aslaug, Cecilie") {
		t.Errorf("the explanation: %+v", sound)
	}
}
