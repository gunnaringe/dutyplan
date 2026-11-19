package api

import (
	"context"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

type cellsWorld struct {
	org, plan      string
	events         []store.Event // both in the plan
	otherPlanEvent store.Event   // in another plan of the same organisation
	sound, sang    store.Task    // sang is not distributed by the planner
	anna           store.Person
	choir          store.Group
}

// cellsWorld: an organisation (see plannable) with a second event, a task the planner does not distribute, a group,
// and a second plan with an event of its own. Both organisations get a plan with the same slug.
func (e *env) cellsWorld(name string) cellsWorld {
	e.t.Helper()
	ctx := context.Background()
	org, plan := e.plannable(name)
	w := cellsWorld{org: org, plan: plan}
	must := func(err error) {
		e.t.Helper()
		if err != nil {
			e.t.Fatal(err)
		}
	}
	tasks, err := e.st.ListTasks(ctx, org)
	must(err)
	w.sound = tasks[0]
	w.sang, err = e.st.CreateTask(ctx, org, "Singing", false)
	must(err)
	places := map[int64]int{w.sound.ID: 1}
	_, err = e.st.AddEvents(ctx, org, plan, store.NewEvents{Date: "2026-08-16", Places: places})
	must(err)
	w.events, err = e.st.ListEvents(ctx, org, plan)
	must(err)
	other, err := e.st.CreatePlan(ctx, org, "Spring 2027", "")
	must(err)
	others, err := e.st.AddEvents(ctx, org, other.Slug, store.NewEvents{Date: "2027-02-07", Places: places})
	must(err)
	w.otherPlanEvent = others[0]
	people, err := e.st.ListPeople(ctx, org)
	must(err)
	w.anna = people[0]
	w.choir, err = e.st.CreateGroup(ctx, org, "Choir")
	must(err)
	return w
}

func TestManualCellsAndSelectionsOverHTTP(t *testing.T) {
	e := newEnv(t)
	a := e.cellsWorld("Org A")
	base := "/api/orgs/" + a.org + "/plans/" + a.plan
	ids := []int64{a.events[0].ID, a.events[1].ID}

	var made struct {
		Created []store.ManualCell
		Skipped int
	}
	e.mustDo(201, "POST", base+"/cells", map[string]any{"events": ids, "task": a.sang.ID, "group": a.choir.ID, "fills": 2}).json(&made)
	if len(made.Created) != 2 || made.Skipped != 0 || made.Created[0].Fills != 2 || *made.Created[0].GroupID != a.choir.ID {
		t.Fatalf("a group on two events: %+v", made)
	}
	// what a manual cell names cannot be deleted from under it
	if r := e.do("DELETE", "/api/orgs/"+a.org+"/groups/"+itoa(a.choir.ID), nil); r.code != 409 {
		t.Errorf("deleting a group a manual cell names: %d %s", r.code, r.body)
	}
	if r := e.do("DELETE", "/api/orgs/"+a.org+"/tasks/"+itoa(a.sang.ID), nil); r.code != 409 {
		t.Errorf("deleting a task a manual cell names: %d %s", r.code, r.body)
	}
	e.mustDo(201, "POST", base+"/cells", map[string]any{"events": ids[:1], "task": a.sound.ID, "person": a.anna.ID})
	e.mustDo(201, "POST", base+"/cells", map[string]any{"events": ids, "task": a.sound.ID, "label": "Outside guest"})
	// one of the two is new: created 1, skipped 1
	e.mustDo(201, "POST", base+"/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID}).json(&made)
	if len(made.Created) != 1 || made.Skipped != 1 {
		t.Errorf("partly new: %+v", made)
	}
	// a planner's name that was locked is a cell too, marked as locked
	e.mustDo(201, "POST", base+"/cells", map[string]any{"events": ids[1:], "task": a.sang.ID, "person": a.anna.ID, "locked": true}).json(&made)
	if len(made.Created) != 1 || !made.Created[0].Locked {
		t.Errorf("locked: %+v", made)
	}
	var list []store.ManualCell
	e.mustDo(200, "GET", base+"/cells", nil).json(&list)
	if len(list) != 7 || !list[6].Locked || list[0].Locked {
		t.Fatalf("cells: %+v", list)
	}
	e.mustDo(204, "DELETE", base+"/cells/"+itoa(list[0].ID), nil)
	e.mustDo(200, "GET", base+"/cells", nil).json(&list)
	if len(list) != 6 {
		t.Errorf("after delete: %d", len(list))
	}

	e.mustDo(204, "PUT", base+"/pools", map[string]any{"events": ids, "task": a.sound.ID, "people": []int64{a.anna.ID}, "groups": []int64{a.choir.ID}})
	var pools []store.Pool
	e.mustDo(200, "GET", base+"/pools", nil).json(&pools)
	if len(pools) != 2 || len(pools[0].People) != 1 || len(pools[0].Groups) != 1 {
		t.Fatalf("pools: %+v", pools)
	}
	e.mustDo(204, "PUT", base+"/pools", map[string]any{"events": ids, "task": a.sound.ID, "people": []int64{}, "groups": []int64{}})
	e.mustDo(200, "GET", base+"/pools", nil).json(&pools)
	if len(pools) != 0 {
		t.Errorf("an empty selection removes it: %+v", pools)
	}
}

func TestCellAndSelectionRefusals(t *testing.T) {
	e := newEnv(t)
	a, b := e.cellsWorld("Org A"), e.cellsWorld("Org B")
	if a.plan != b.plan {
		t.Fatal("test needs the same plan slug in both organisations")
	}
	base := "/api/orgs/" + a.org + "/plans/" + a.plan
	ids := []int64{a.events[0].ID}
	inactive := false
	gone, _ := e.st.CreatePerson(context.Background(), a.org, "Gisle")
	e.st.UpdatePerson(context.Background(), a.org, gone.ID, store.PersonPatch{Active: &inactive})

	for _, c := range []struct {
		code         int
		method, path string
		body         any
		what         string
	}{
		{400, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID}, "names nothing"},
		{400, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID, "group": a.choir.ID}, "names two things"},
		{400, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID, "fills": 2}, "a person fills one"},
		{400, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "label": "Guest", "locked": true}, "a text cannot be locked"},
		{400, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": gone.ID}, "inactive person"},
		{400, "POST", base + "/cells", map[string]any{"events": []int64{}, "task": a.sound.ID, "person": a.anna.ID}, "no events"},
		{400, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID, "plass": 1}, "unknown field"},
		{404, "POST", base + "/cells", map[string]any{"events": []int64{a.otherPlanEvent.ID}, "task": a.sound.ID, "person": a.anna.ID}, "an event of another plan"},
		{404, "POST", base + "/cells", map[string]any{"events": []int64{b.events[0].ID}, "task": a.sound.ID, "person": a.anna.ID}, "another organisation's event"},
		{404, "POST", base + "/cells", map[string]any{"events": ids, "task": b.sound.ID, "person": a.anna.ID}, "another organisation's task"},
		{404, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": b.anna.ID}, "another organisation's person"},
		{404, "POST", base + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "group": b.choir.ID}, "another organisation's group"},
		{404, "POST", base + "/cells", map[string]any{"events": []int64{a.events[0].ID, a.otherPlanEvent.ID}, "task": a.sound.ID, "person": a.anna.ID}, "one of two events is not in the plan"},
		{404, "POST", "/api/orgs/nope/plans/" + a.plan + "/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID}, "unknown organisation"},
		{404, "POST", "/api/orgs/" + a.org + "/plans/nope/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID}, "unknown plan"},
		{400, "PUT", base + "/pools", map[string]any{"events": ids, "task": a.sang.ID, "people": []int64{a.anna.ID}}, "a selection for a task the planner does not distribute"},
		{400, "PUT", base + "/pools", map[string]any{"events": ids, "task": a.sound.ID, "people": []int64{gone.ID}}, "an inactive person in a selection"},
		{404, "PUT", base + "/pools", map[string]any{"events": ids, "task": a.sound.ID, "people": []int64{b.anna.ID}}, "another organisation's person in a selection"},
		{404, "PUT", base + "/pools", map[string]any{"events": []int64{a.otherPlanEvent.ID}, "task": a.sound.ID, "people": []int64{a.anna.ID}}, "a selection on another plan's event"},
		{404, "DELETE", base + "/cells/abc", nil, "bad id"},
		{404, "DELETE", base + "/cells/99999", nil, "no such cell"},
	} {
		if r := e.do(c.method, c.path, c.body); r.code != c.code {
			t.Errorf("%s: %s %s = %d %s, want %d", c.what, c.method, c.path, r.code, r.body, c.code)
		}
	}
	var list []store.ManualCell
	e.mustDo(200, "GET", base+"/cells", nil).json(&list)
	if len(list) != 0 {
		t.Errorf("a refused request leaves nothing behind: %+v", list)
	}

	// a conflict is 409, and one organisation's cell is invisible and undeletable for the other (same plan slug)
	e.mustDo(201, "POST", base+"/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID})
	if r := e.do("POST", base+"/cells", map[string]any{"events": ids, "task": a.sound.ID, "person": a.anna.ID}); r.code != 409 {
		t.Errorf("the same cell again: %d %s", r.code, r.body)
	}
	e.mustDo(200, "GET", base+"/cells", nil).json(&list)
	bBase := "/api/orgs/" + b.org + "/plans/" + b.plan
	var listB []store.ManualCell
	e.mustDo(200, "GET", bBase+"/cells", nil).json(&listB)
	if len(listB) != 0 || len(list) != 1 {
		t.Errorf("B must not see A's cell: %+v / %+v", listB, list)
	}
	if r := e.do("DELETE", bBase+"/cells/"+itoa(list[0].ID), nil); r.code != 404 {
		t.Errorf("B deleting A's cell: %d", r.code)
	}
	otherPlan := "/api/orgs/" + a.org + "/plans/spring-2027"
	if r := e.do("DELETE", otherPlan+"/cells/"+itoa(list[0].ID), nil); r.code != 404 {
		t.Errorf("deleting through another plan: %d", r.code)
	}
	e.mustDo(200, "GET", base+"/cells", nil).json(&list)
	if len(list) != 1 {
		t.Errorf("A's cell survived: %+v", list)
	}
}
