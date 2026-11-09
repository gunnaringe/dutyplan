package api

import (
	"testing"

	"github.com/gunnaringe/tjenesteplan/internal/store"
)

func TestGroupsOverHTTP(t *testing.T) {
	e := newEnv(t)
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org A"})
	e.mustDo(201, "POST", "/api/orgs", map[string]string{"name": "Org B"})
	var anna, annaB store.Person
	e.mustDo(201, "POST", "/api/orgs/org-a/people", map[string]string{"name": "Aslaug"}).json(&anna)
	e.mustDo(201, "POST", "/api/orgs/org-b/people", map[string]string{"name": "Aslaug"}).json(&annaB)
	var lyd, lydB, sang store.Task
	e.mustDo(201, "POST", "/api/orgs/org-a/tasks", map[string]any{"name": "Lyd"}).json(&lyd)
	e.mustDo(201, "POST", "/api/orgs/org-b/tasks", map[string]any{"name": "Lyd"}).json(&lydB)
	e.mustDo(201, "POST", "/api/orgs/org-a/tasks", map[string]any{"name": "Lovsang", "planned": false}).json(&sang)

	var g store.Group
	e.mustDo(201, "POST", "/api/orgs/org-a/groups", map[string]string{"name": "Kirkekoret"}).json(&g)
	base := "/api/orgs/org-a/groups/" + itoa(g.ID)
	e.mustDo(200, "PUT", base+"/members", map[string]any{"people": []int64{anna.ID}}).json(&g)
	e.mustDo(200, "PUT", base+"/tasks", map[string]any{"tasks": []store.GroupTask{{TaskID: lyd.ID, Fills: 2, Level: "godt"}}}).json(&g)
	e.mustDo(200, "PATCH", base, map[string]any{"proposable": false}).json(&g)
	if g.Proposable || len(g.Members) != 1 || g.Tasks[0].Fills != 2 {
		t.Fatalf("group: %+v", g)
	}
	var list []store.Group
	e.mustDo(200, "GET", "/api/orgs/org-a/groups", nil).json(&list)
	if len(list) != 1 || list[0].ID != g.ID {
		t.Errorf("list: %+v", list)
	}

	for _, c := range []struct {
		code         int
		method, path string
		body         any
		what         string
	}{
		{404, "PATCH", "/api/orgs/org-b/groups/" + itoa(g.ID), map[string]any{"proposable": true}, "A's group through B"},
		{404, "PUT", "/api/orgs/org-b/groups/" + itoa(g.ID) + "/members", map[string]any{"people": []int64{}}, "A's members through B"},
		{404, "DELETE", "/api/orgs/org-b/groups/" + itoa(g.ID), nil, "delete through B"},
		{404, "PUT", base + "/members", map[string]any{"people": []int64{annaB.ID}}, "B's person in A's group"},
		{404, "PUT", base + "/tasks", map[string]any{"tasks": []store.GroupTask{{TaskID: lydB.ID, Fills: 1, Level: "godt"}}}, "B's task for A's group"},
		{400, "PUT", base + "/tasks", map[string]any{"tasks": []store.GroupTask{{TaskID: sang.ID, Fills: 1, Level: "godt"}}}, "a task the planner does not distribute"},
		{400, "PUT", base + "/tasks", map[string]any{"tasks": []store.GroupTask{{TaskID: lyd.ID, Fills: 0, Level: "godt"}}}, "fills 0"},
		{400, "POST", "/api/orgs/org-a/groups", map[string]any{"name": "Ny", "members": 3}, "unknown field"},
		{409, "POST", "/api/orgs/org-a/groups", map[string]any{"name": "Kirkekoret"}, "duplicate name"},
		{404, "PATCH", "/api/orgs/org-a/groups/abc", map[string]any{"proposable": true}, "bad id"},
	} {
		if r := e.do(c.method, c.path, "T", c.body); r.code != c.code {
			t.Errorf("%s: %s %s = %d %s, want %d", c.what, c.method, c.path, r.code, r.body, c.code)
		}
	}
	if r := e.do("POST", "/api/orgs/org-a/groups", "", map[string]string{"name": "Uten aktør"}); r.code != 400 || r.errCode() != "actor_required" {
		t.Errorf("no actor: %d %s", r.code, r.body)
	}
	e.mustDo(204, "DELETE", base, nil)
	e.mustDo(200, "GET", "/api/orgs/org-a/groups", nil).json(&list)
	if len(list) != 0 {
		t.Errorf("after delete: %+v", list)
	}
}
