package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

type groupEnv struct {
	s                *Store
	org              Org
	lyd, bilde, sang Task
	people           map[string]Person
}

// groupWorld: Lyd and Bilde are planned; Lovsang is manual-only.
func groupWorld(t *testing.T, s *Store, orgName string) groupEnv {
	t.Helper()
	e := groupEnv{s: s, org: mustOrg(t, s, orgName), people: map[string]Person{}}
	var err error
	if e.lyd, err = s.CreateTask(ctx, e.org.Slug, "Ola", "Lyd", true); err != nil {
		t.Fatal(err)
	}
	if e.bilde, err = s.CreateTask(ctx, e.org.Slug, "Ola", "Bilde", true); err != nil {
		t.Fatal(err)
	}
	if e.sang, err = s.CreateTask(ctx, e.org.Slug, "Ola", "Lovsang", false); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"Aslaug", "Birger", "Cecilie"} {
		p, err := s.CreatePerson(ctx, e.org.Slug, "Ola", n)
		if err != nil {
			t.Fatal(err)
		}
		e.people[n] = p
	}
	return e
}

func TestGroupLifecycle(t *testing.T) {
	e := groupWorld(t, open(t), "Menighet")
	g, err := e.s.CreateGroup(ctx, e.org.Slug, "Ola", "  Kirkekoret ")
	if err != nil || g.Name != "Kirkekoret" || !g.Proposable || len(g.Members) != 0 || len(g.Tasks) != 0 {
		t.Fatalf("create: %+v %v", g, err)
	}
	if _, err := e.s.CreateGroup(ctx, e.org.Slug, "Ola", "Kirkekoret"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate name: %v", err)
	}
	for name, n := range map[string]string{"blank": "  ", "too long": strings.Repeat("x", 81), "control": "a\nb"} {
		if _, err := e.s.CreateGroup(ctx, e.org.Slug, "Ola", n); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := e.s.CreateGroup(ctx, e.org.Slug, "", "Uten aktør"); !errors.Is(err, ErrActor) {
		t.Errorf("no actor: %v", err)
	}

	off := false
	if g, err = e.s.UpdateGroup(ctx, e.org.Slug, "Ola", g.ID, GroupPatch{Proposable: &off}); err != nil || g.Proposable {
		t.Fatalf("proposable off: %+v %v", g, err)
	}
	if _, err = e.s.UpdateGroup(ctx, e.org.Slug, "Ola", g.ID, GroupPatch{Proposable: &off}); !errors.Is(err, ErrInvalid) {
		t.Errorf("no-op update: %v", err)
	}
	if g, err = e.s.UpdateGroup(ctx, e.org.Slug, "Ola", g.ID, GroupPatch{Name: ptr("Koret")}); err != nil || g.Name != "Koret" || g.Proposable {
		t.Fatalf("rename keeps the rest: %+v %v", g, err)
	}
	other, _ := e.s.CreateGroup(ctx, e.org.Slug, "Ola", "Band")
	if _, err = e.s.UpdateGroup(ctx, e.org.Slug, "Ola", other.ID, GroupPatch{Name: ptr("Koret")}); !errors.Is(err, ErrConflict) {
		t.Errorf("rename to an existing name: %v", err)
	}

	a, b := e.people["Aslaug"].ID, e.people["Birger"].ID
	if g, err = e.s.SetGroupMembers(ctx, e.org.Slug, "Ola", g.ID, []int64{b, a, a}); err != nil || len(g.Members) != 2 || g.Members[0] != a || g.Members[1] != b {
		t.Fatalf("members (deduplicated, in id order): %+v %v", g, err)
	}
	if _, err = e.s.SetGroupMembers(ctx, e.org.Slug, "Ola", g.ID, []int64{a, b}); !errors.Is(err, ErrInvalid) {
		t.Errorf("the same members again: %v", err)
	}
	if g, err = e.s.SetGroupMembers(ctx, e.org.Slug, "Ola", g.ID, []int64{b}); err != nil || len(g.Members) != 1 {
		t.Fatalf("remove one: %+v %v", g, err)
	}
	if g, err = e.s.SetGroupMembers(ctx, e.org.Slug, "Ola", g.ID, nil); err != nil || len(g.Members) != 0 {
		t.Fatalf("an empty group is allowed: %+v %v", g, err)
	}
	if g, err = e.s.SetGroupMembers(ctx, e.org.Slug, "Ola", g.ID, []int64{a, b}); err != nil {
		t.Fatal(err)
	}
	log, _ := e.s.ListLog(ctx, e.org.Slug, "", 3)
	if len(log) == 0 || log[0].Action != "group.members" || log[0].Detail != "Koret: +Aslaug, +Birger" {
		t.Errorf("the log names who joined: %+v", log)
	}

	if g, err = e.s.SetGroupTasks(ctx, e.org.Slug, "Ola", g.ID, []GroupTask{{e.lyd.ID, 2, "good"}, {e.bilde.ID, 1, "neutral"}}); err != nil || len(g.Tasks) != 2 {
		t.Fatalf("tasks: %+v %v", g, err)
	}
	for name, ts := range map[string][]GroupTask{
		"fills 0":                                {{e.lyd.ID, 0, "good"}},
		"fills 51":                               {{e.lyd.ID, 51, "good"}},
		"level ikke":                             {{e.lyd.ID, 1, "never"}},
		"level unknown":                          {{e.lyd.ID, 1, "kjempegodt"}},
		"the same task twice":                    {{e.lyd.ID, 1, "good"}, {e.lyd.ID, 2, "good"}},
		"a task the planner does not distribute": {{e.sang.ID, 1, "good"}},
	} {
		if _, err := e.s.SetGroupTasks(ctx, e.org.Slug, "Ola", g.ID, ts); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: want ErrInvalid, got %v", name, err)
		}
	}
	if got, _ := e.s.ListGroups(ctx, e.org.Slug); len(got[1].Tasks) != 2 {
		t.Errorf("a refused change leaves the tasks as they were: %+v", got)
	}
	if g, err = e.s.SetGroupTasks(ctx, e.org.Slug, "Ola", g.ID, nil); err != nil || len(g.Tasks) != 0 {
		t.Fatalf("clearing the tasks: %+v %v", g, err)
	}

	// the list is ordered by name and carries members and tasks
	list, _ := e.s.ListGroups(ctx, e.org.Slug)
	if len(list) != 2 || list[0].Name != "Band" || list[1].Name != "Koret" || len(list[1].Members) != 2 {
		t.Errorf("list: %+v", list)
	}
	if err := e.s.DeleteGroup(ctx, e.org.Slug, "Ola", g.ID); err != nil {
		t.Fatal(err)
	}
	if n := count(t, e.s, `SELECT COUNT(*) FROM grp_member WHERE group_id = ?`, g.ID); n != 0 {
		t.Errorf("members go with the group: %d", n)
	}
}

func TestGroupsNeverCrossOrganisations(t *testing.T) {
	s := open(t)
	a, b := groupWorld(t, s, "Org A"), groupWorld(t, s, "Org B")
	ga, _ := s.CreateGroup(ctx, a.org.Slug, "Ola", "Koret")
	gb, err := s.CreateGroup(ctx, b.org.Slug, "Eva", "Koret")
	if err != nil {
		t.Fatalf("the same group name in two organisations: %v", err)
	}
	// B reaching for A's group: not found, and nothing changes
	if _, err := s.UpdateGroup(ctx, b.org.Slug, "Eva", ga.ID, GroupPatch{Name: ptr("Kapret")}); !errors.Is(err, ErrNotFound) {
		t.Errorf("update through B: %v", err)
	}
	if _, err := s.SetGroupMembers(ctx, b.org.Slug, "Eva", ga.ID, []int64{b.people["Aslaug"].ID}); !errors.Is(err, ErrNotFound) {
		t.Errorf("members through B: %v", err)
	}
	if _, err := s.SetGroupTasks(ctx, b.org.Slug, "Eva", ga.ID, nil); !errors.Is(err, ErrNotFound) {
		t.Errorf("tasks through B: %v", err)
	}
	if err := s.DeleteGroup(ctx, b.org.Slug, "Eva", ga.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete through B: %v", err)
	}
	// a person or a task of the other organisation cannot be named
	if _, err := s.SetGroupMembers(ctx, a.org.Slug, "Ola", ga.ID, []int64{b.people["Aslaug"].ID}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another organisation's person as a member: %v", err)
	}
	if _, err := s.SetGroupTasks(ctx, a.org.Slug, "Ola", ga.ID, []GroupTask{{b.lyd.ID, 1, "good"}}); !errors.Is(err, ErrNotFound) {
		t.Errorf("another organisation's task: %v", err)
	}
	if la, _ := s.ListGroups(ctx, a.org.Slug); len(la) != 1 || la[0].Name != "Koret" || len(la[0].Members) != 0 {
		t.Errorf("A's group untouched: %+v", la)
	}
	if lb, _ := s.ListGroups(ctx, b.org.Slug); len(lb) != 1 || lb[0].ID != gb.ID {
		t.Errorf("B lists only its own: %+v", lb)
	}

	// the foreign keys are the backstop
	var orgA int64
	s.DB().QueryRow(`SELECT id FROM organization WHERE slug = ?`, a.org.Slug).Scan(&orgA)
	for name, c := range map[string]struct {
		stmt string
		args []any
	}{
		"member from another organisation":            {`INSERT INTO grp_member (org_id, group_id, person_id) VALUES (?, ?, ?)`, []any{orgA, ga.ID, b.people["Aslaug"].ID}},
		"group task with another organisation's task": {`INSERT INTO grp_task (org_id, group_id, task_id, fills, level) VALUES (?, ?, ?, 1, 'good')`, []any{orgA, ga.ID, b.lyd.ID}},
		"member of another organisation's group":      {`INSERT INTO grp_member (org_id, group_id, person_id) VALUES (?, ?, ?)`, []any{orgA, gb.ID, a.people["Aslaug"].ID}},
	} {
		if _, err := s.DB().Exec(c.stmt, c.args...); !isForeignKey(err) {
			t.Errorf("%s: the database must refuse it, got %v", name, err)
		}
	}
}

func TestPlanningDataCarriesTheGroups(t *testing.T) {
	s := open(t)
	e := groupWorld(t, s, "Menighet")
	plan, _ := s.CreatePlan(ctx, e.org.Slug, "Ola", "Høst 2026", "")
	g, _ := s.CreateGroup(ctx, e.org.Slug, "Ola", "Koret")
	s.SetGroupMembers(ctx, e.org.Slug, "Ola", g.ID, []int64{e.people["Aslaug"].ID})
	s.SetGroupTasks(ctx, e.org.Slug, "Ola", g.ID, []GroupTask{{e.lyd.ID, 2, "good"}})
	d, err := s.GetPlanningData(ctx, e.org.Slug, plan.Slug)
	if err != nil || len(d.Groups) != 1 || fmt.Sprint(d.Groups[0].Members) != fmt.Sprint([]int64{e.people["Aslaug"].ID}) || d.Groups[0].Tasks[0].Fills != 2 {
		t.Fatalf("%+v %v", d.Groups, err)
	}
	other := mustOrg(t, s, "Annen")
	op, _ := s.CreatePlan(ctx, other.Slug, "Eva", "Høst 2026", "")
	if d, _ = s.GetPlanningData(ctx, other.Slug, op.Slug); len(d.Groups) != 0 {
		t.Errorf("another organisation's planning data has none of them: %+v", d.Groups)
	}
	// a task in use by a group cannot be deleted
	if err := s.DeleteTask(ctx, e.org.Slug, "Ola", e.lyd.ID); !errors.Is(err, ErrConflict) {
		t.Errorf("deleting a task a group is proposed for: %v", err)
	}
}
