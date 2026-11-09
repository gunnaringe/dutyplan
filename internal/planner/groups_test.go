package planner

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/i18n"
	"github.com/gunnaringe/dutyplan/internal/store"
)

func (w world) group(t *testing.T, name string, members []string, tasks ...store.GroupTask) store.Group {
	t.Helper()
	g, err := w.st.CreateGroup(ctx, w.org.Slug, "T", name)
	if err != nil {
		t.Fatal(err)
	}
	var ids []int64
	for _, m := range members {
		ids = append(ids, w.people[m].ID)
	}
	if len(ids) > 0 {
		if g, err = w.st.SetGroupMembers(ctx, w.org.Slug, "T", g.ID, ids); err != nil {
			t.Fatal(err)
		}
	}
	if len(tasks) > 0 {
		if g, err = w.st.SetGroupTasks(ctx, w.org.Slug, "T", g.ID, tasks); err != nil {
			t.Fatal(err)
		}
	}
	return g
}

type inputView struct {
	Groups []struct {
		ID         string   `json:"id"`
		Members    []string `json:"members"`
		Proposable bool     `json:"proposable"`
	} `json:"groups"`
	GroupTasks []struct {
		Group string `json:"group"`
		Task  string `json:"task"`
		Fills int    `json:"fills"`
		Level string `json:"level"`
	} `json:"group_tasks"`
}

func (w world) input(t *testing.T) (inputView, Snapshot, string) {
	t.Helper()
	raw, hash, snap, err := BuildInput(w.data(t), Params{})
	if err != nil {
		t.Fatal(err)
	}
	var in inputView
	if err := json.Unmarshal(raw, &in); err != nil {
		t.Fatal(err)
	}
	return in, snap, hash
}

func TestBuildInputSendsGroupsWithTheirActiveMembersOnly(t *testing.T) {
	w := newWorld(t, "Club")
	choir := w.group(t, "Choir", []string{"Aslaug", "Birger", "Dina"}, store.GroupTask{TaskID: w.sound.ID, Fills: 2, Level: "good"})
	w.group(t, "Empty", nil, store.GroupTask{TaskID: w.camera.ID, Fills: 1, Level: "neutral"}) // meant to be used, has nobody
	w.group(t, "Rolig", nil)                                                                   // an empty group nobody asked for
	inactive := false
	w.st.UpdatePerson(ctx, w.org.Slug, "T", w.people["Dina"].ID, store.PersonPatch{Active: &inactive})

	in, snap, hash := w.input(t)
	if len(in.Groups) != 1 || in.Groups[0].ID != gid(choir.ID) || !in.Groups[0].Proposable {
		t.Fatalf("only the group with active members is sent: %+v", in.Groups)
	}
	if got := strings.Join(in.Groups[0].Members, ","); got != pid(w.people["Aslaug"].ID)+","+pid(w.people["Birger"].ID) {
		t.Errorf("an inactive member is left out: %s", got)
	}
	if len(in.GroupTasks) != 1 || in.GroupTasks[0] != (struct {
		Group string `json:"group"`
		Task  string `json:"task"`
		Fills int    `json:"fills"`
		Level string `json:"level"`
	}{gid(choir.ID), tid(w.sound.ID), 2, "good"}) {
		t.Errorf("group tasks: %+v", in.GroupTasks)
	}
	if len(snap.Groups) != 1 || snap.Groups[0].Name != "Choir" || len(snap.Groups[0].Members) != 2 {
		t.Errorf("snapshot groups: %+v", snap.Groups)
	}
	if len(snap.rendered(i18n.EN)) != 1 || !strings.Contains(snap.rendered(i18n.EN)[0], "Empty") || strings.Contains(strings.Join(snap.rendered(i18n.EN), ""), "Rolig") {
		t.Errorf("a group that was meant to take part but has nobody is worth a warning, an unused empty one is not: %v", snap.rendered(i18n.EN))
	}

	// «can be proposed» is passed on, and changing a group makes the stored run out of date
	off := false
	w.st.UpdateGroup(ctx, w.org.Slug, "T", choir.ID, store.GroupPatch{Proposable: &off})
	in2, _, hash2 := w.input(t)
	if in2.Groups[0].Proposable || hash2 == hash {
		t.Errorf("proposable=false must reach the planner and change the hash: %+v", in2.Groups)
	}
	// a task taken out of the planner's hands later: the solver refuses group tasks for it, so it is not sent
	unplanned := false
	w.st.UpdateTask(ctx, w.org.Slug, "T", w.sound.ID, store.TaskPatch{Planned: &unplanned})
	if in3, _, _ := w.input(t); len(in3.GroupTasks) != 0 {
		t.Errorf("a task the planner no longer distributes: %+v", in3.GroupTasks)
	}
}

func TestBuildInputNeverSendsAnotherOrganisationsGroups(t *testing.T) {
	a := newWorld(t, "Org A")
	b := populate(t, a.st, "Org B")
	ga := a.group(t, "Choir", []string{"Aslaug"}, store.GroupTask{TaskID: a.sound.ID, Fills: 1, Level: "neutral"})
	gb := b.group(t, "Choir", []string{"Aslaug"}, store.GroupTask{TaskID: b.sound.ID, Fills: 1, Level: "neutral"})
	inA, _, _ := a.input(t)
	inB, _, _ := b.input(t)
	if len(inA.Groups) != 1 || inA.Groups[0].ID != gid(ga.ID) || len(inB.Groups) != 1 || inB.Groups[0].ID != gid(gb.ID) {
		t.Errorf("each organisation sends its own group only: %+v / %+v", inA.Groups, inB.Groups)
	}
}

// groupAnswer: Choir (Aslaug, Birger) is proposed for Sound on event 0 (Birger said «poor»), and set by hand for
// Singing on event 1; a guest is written in for Singing on event 0; Cecilie is locked on Camera on event 0.
func groupAnswer(w world, choir store.Group) []byte {
	e0, e1 := eid(w.events[0].ID), eid(w.events[1].ID)
	g := gid(choir.ID)
	out := map[string]any{
		"status": "OPTIMAL", "wall_time": 0.2,
		"assignments": []map[string]any{
			{"date": e0, "task": tid(w.camera.ID), "person": pid(w.people["Cecilie"].ID), "locked": true, "task_level": "neutral", "date_level": "neutral"},
		},
		"group_assignments": []map[string]any{
			{"date": e0, "task": tid(w.sound.ID), "group": g, "members": []string{pid(w.people["Birger"].ID), pid(w.people["Aslaug"].ID)}, "fills": 1, "locked": false},
		},
		"manual": []map[string]any{
			{"date": e1, "task": tid(w.singing.ID), "group": g, "fills": 1},
			{"date": e0, "task": tid(w.singing.ID), "label": "Outside guest", "fills": 2},
		},
		"concessions": []map[string]any{
			{"date": e0, "task": tid(w.sound.ID), "person": pid(w.people["Birger"].ID), "group": g, "task_level": "neutral", "date_level": "poor"},
		},
		"unfilled": []map[string]any{
			{"date": e1, "task": tid(w.sound.ID), "missing": 1, "why": map[string]any{
				"candidates": 0, "groups": map[string]any{g: "event_never:" + pid(w.people["Aslaug"].ID)}}},
		},
		"warnings": []string{
			"locked: " + pid(w.people["Cecilie"].ID) + " on " + tid(w.camera.ID) + " " + e0 + " breaks «not_proposable» (manual choice kept)",
			"locked: " + pid(w.people["Frode"].ID) + " on " + tid(w.sound.ID) + " " + e1 + " breaks «not_in_task» (manual choice kept)",
		},
		"load": map[string]any{}, "violations": []string{},
	}
	b, _ := json.Marshal(out)
	return b
}

func TestTheViewShowsGroupsManualCellsAndLockedPeopleWithNames(t *testing.T) {
	w := newWorld(t, "Club")
	choir := w.group(t, "Choir", []string{"Aslaug", "Birger"}, store.GroupTask{TaskID: w.sound.ID, Fills: 1, Level: "neutral"})
	s := newService(w, func(context.Context, []byte) ([]byte, error) { return groupAnswer(w, choir), nil })
	id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, "Kari", Params{TimeLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	if err != nil {
		t.Fatal(err)
	}
	cell := func(event int, task store.Task) *Cell {
		for i := range v.Events[event].Cells {
			if v.Events[event].Cells[i].TaskID == task.ID {
				return &v.Events[event].Cells[i]
			}
		}
		return nil
	}
	names := []string{}
	for _, tk := range v.Tasks {
		names = append(names, tk.Name)
	}
	if fmt.Sprint(names) != "[Camera Singing Sound]" {
		t.Errorf("Singing is not planned but has manual cells, so it is a column; columns: %v", names)
	}
	// proposed group: its members by name, and the weak mark from a member's «poor»
	lyd0 := cell(0, w.sound)
	if lyd0 == nil || len(lyd0.Groups) != 1 || lyd0.Groups[0].Name != "Choir" || fmt.Sprint(lyd0.Groups[0].Members) != "[Aslaug Birger]" || lyd0.Groups[0].Locked || !lyd0.Groups[0].Weak || lyd0.Slots != 1 || lyd0.Missing != 0 {
		t.Errorf("proposed group: %+v", lyd0)
	}
	// manual text with two places, and the locked person
	sang0 := cell(0, w.singing)
	if sang0 == nil || len(sang0.Texts) != 1 || sang0.Texts[0].Text != "Outside guest" || sang0.Texts[0].Fills != 2 || sang0.Slots != 2 || sang0.Missing != 0 {
		t.Errorf("manual text: %+v", sang0)
	}
	bilde0 := cell(0, w.camera)
	if bilde0 == nil || len(bilde0.People) != 1 || !bilde0.People[0].Locked || bilde0.People[0].Name != "Cecilie" {
		t.Errorf("locked person: %+v", bilde0)
	}
	// manual group: locked, members from the snapshot, no «poor»
	sang1 := cell(1, w.singing)
	if sang1 == nil || len(sang1.Groups) != 1 || !sang1.Groups[0].Locked || fmt.Sprint(sang1.Groups[0].Members) != "[Aslaug Birger]" || sang1.Groups[0].Weak {
		t.Errorf("manual group: %+v", sang1)
	}
	// a day type's Singing place that nothing fills is not a place at all (the planner does not distribute it)
	if c := cell(2, w.singing); c != nil {
		t.Errorf("no phantom place for a task the planner does not distribute: %+v", c)
	}
	// the explanation speaks about the group, by name
	if c := cell(1, w.sound); c == nil || !strings.Contains(c.Explanation, "Choir: Aslaug cannot attend") {
		t.Errorf("explanation: %+v", c)
	}
	if len(v.Weak) != 1 || v.Weak[0] != "Birger (as a member of Choir) got Sound 2026-08-09, even though it is «poor» for the event." {
		t.Errorf("compromise for a group member: %v", v.Weak)
	}
	// warnings: names, and silent for the designed use of «cannot be proposed»
	if len(v.Warnings) != 1 || v.Warnings[0] != "Frode was set on Sound 2026-08-16 by hand, even though they have not chosen the task." {
		t.Errorf("warnings: %v", v.Warnings)
	}
}

func TestTheRealSolverProposesAGroupAndKeepsItsMembersOccupied(t *testing.T) {
	solver := realSolver(t)
	w := newWorld(t, "Club")
	// Sound: only the group can take it. Dina and Embla never chose Sound themselves; the group is set up for it.
	for _, n := range []string{"Aslaug", "Birger", "Cecilie"} {
		w.st.RemovePersonTask(ctx, w.org.Slug, "T", w.people[n].ID, w.sound.ID)
	}
	choir := w.group(t, "Choir", []string{"Dina", "Embla"}, store.GroupTask{TaskID: w.sound.ID, Fills: 1, Level: "good"})
	s := New(w.st, solver, 1, nopLogger())
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
	cellOf := func(ev int, task store.Task) Cell {
		for _, c := range v.Events[ev].Cells {
			if c.TaskID == task.ID {
				return c
			}
		}
		t.Fatalf("no %s cell on event %d", task.Name, ev)
		return Cell{}
	}
	for ev := range v.Events {
		sound, camera := cellOf(ev, w.sound), cellOf(ev, w.camera)
		if ev == 1 { // Dina has «never» on this event, which rules the whole group out
			if sound.Missing != 1 || len(sound.Groups) != 0 || !strings.Contains(sound.Explanation, "Choir: Dina cannot attend") {
				t.Errorf("event 1 Sound: %+v", sound)
			}
			continue
		}
		if len(sound.Groups) != 1 || sound.Groups[0].ID != choir.ID || sound.Missing != 0 {
			t.Fatalf("event %d: the group takes Sound: %+v", ev, sound)
		}
		for _, p := range camera.People { // Sound and Camera cannot be combined: the group's members are busy
			if p.Name == "Dina" || p.Name == "Embla" {
				t.Errorf("event %d: %s is in the group on Sound and cannot also take Camera", ev, p.Name)
			}
		}
	}
	// «mine oppdrag» agrees with the planner's own count for everyone: places of their own plus the group's, one each
	loads := map[string]int{}
	for _, l := range v.Load {
		loads[l.Name] = l.Count
	}
	for name, person := range w.people {
		m, err := s.Mine(ctx, w.org.Slug, w.plan.Slug, person.ID)
		if err != nil || len(m.Assignments) != loads[name] {
			t.Errorf("%s: %d assignments in «mine oppdrag», %d in the load (%v)", name, len(m.Assignments), loads[name], err)
		}
	}
	// group work counts as one assignment per member
	counts := map[string]int{}
	for _, l := range v.Load {
		counts[l.Name] = l.Count
	}
	// the group works on three events; Dina has nothing else (not available when the group is out), Embla may also take Camera then
	if counts["Dina"] != 3 || counts["Embla"] < 3 {
		t.Errorf("each member gets one assignment for each time the group works: %v", counts)
	}
}
