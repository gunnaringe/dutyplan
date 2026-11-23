package store

import (
	"errors"
	"strconv"
	"strings"
	"testing"
)

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// Sections for tasks (2026-11-23): headings in Tasks with their tasks below, the same in every plan; tasks without a
// section first. Only placement: the planner's input (ordered by id) does not change.
func TestTaskSections(t *testing.T) {
	s := open(t)
	org := mustOrg(t, s, "Club").Slug
	other := mustOrg(t, s, "Elsewhere").Slug
	task := map[string]int64{}
	pl1 := mustPlan(t, s, org)
	for _, n := range []string{"Sound", "Video", "Coffee", "Doors"} {
		x, err := s.CreateTask(ctx, org, pl1.Slug, n)
		if err != nil {
			t.Fatal(err)
		}
		task[n] = x.ID
	}
	order := func() string {
		ts, err := s.ListTasks(ctx, org, pl1.Slug)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, x := range ts {
			out = append(out, x.Name+"/"+itoa(x.Section))
		}
		return strings.Join(out, " ")
	}
	tech, err := s.CreateSectionOf(ctx, Tasks, org, pl1.Slug, "Tech")
	if err != nil {
		t.Fatal(err)
	}
	hall, err := s.CreateSectionOf(ctx, Tasks, org, pl1.Slug, "Hall")
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s.ListSections(ctx, org, pl1.Slug); len(got) != 0 {
		t.Errorf("task sections are not the people's: %+v", got)
	}
	if got, _ := s.ListSectionsOf(ctx, Tasks, org, pl1.Slug); len(got) != 2 || got[0].Name != "Tech" || got[1].Name != "Hall" {
		t.Errorf("the task sections in their order: %+v", got)
	}
	// into a section, at a position; the others close the gap
	if err := s.MoveTask(ctx, org, pl1.Slug, task["Video"], &tech.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask(ctx, org, pl1.Slug, task["Sound"], &tech.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask(ctx, org, pl1.Slug, task["Coffee"], &hall.ID, 5); err != nil {
		t.Fatal(err)
	}
	want := "Doors/0 Sound/" + itoa(tech.ID) + " Video/" + itoa(tech.ID) + " Coffee/" + itoa(hall.ID)
	if got := order(); got != want {
		t.Errorf("tasks without a section first, then each section's: %s\nwant %s", got, want)
	}
	// nil keeps the section and moves within it
	if err := s.MoveTask(ctx, org, pl1.Slug, task["Sound"], nil, 9); err != nil {
		t.Fatal(err)
	}
	if got := order(); !strings.HasPrefix(got, "Doors/0 Video/"+itoa(tech.ID)+" Sound/"+itoa(tech.ID)) {
		t.Errorf("moved within its section: %s", got)
	}
	// the sections' order is the order of their tasks
	if err := s.MoveSectionOf(ctx, Tasks, org, pl1.Slug, hall.ID, 0); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "Doors/0 Coffee/"+itoa(hall.ID)+" Video/"+itoa(tech.ID)+" Sound/"+itoa(tech.ID) {
		t.Errorf("the sections moved: %s", got)
	}
	// a new task in a section goes last in it
	if x, err := s.CreateTaskIn(ctx, org, pl1.Slug, "Lights", tech.ID); err != nil || x.Section != tech.ID {
		t.Fatalf("a task made in a section: %+v %v", x, err)
	} else {
		task["Lights"] = x.ID
	}
	if got := order(); !strings.HasSuffix(got, "Sound/"+itoa(tech.ID)+" Lights/"+itoa(tech.ID)) {
		t.Errorf("the new task is last in its section: %s", got)
	}
	// deleting a task in a section closes the gap there (positions 0, 1, ... in the section)
	tmp, err := s.CreateTaskIn(ctx, org, pl1.Slug, "Temp", tech.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask(ctx, org, pl1.Slug, tmp.ID, nil, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTask(ctx, org, pl1.Slug, tmp.ID); err != nil {
		t.Fatal(err)
	}
	var positions string
	if err := s.DB().QueryRow(`SELECT group_concat(position, ',') FROM (SELECT position FROM task WHERE section_id = ? ORDER BY position)`, tech.ID).Scan(&positions); err != nil || positions != "0,1,2" {
		t.Errorf("the section's positions after deleting a task: %q %v", positions, err)
	}
	// renaming; the same name twice; another organisation's section or task is not found
	if x, err := s.RenameSectionOf(ctx, Tasks, org, pl1.Slug, tech.ID, "Technique"); err != nil || x.Name != "Technique" {
		t.Errorf("rename: %+v %v", x, err)
	}
	if _, err := s.CreateSectionOf(ctx, Tasks, org, pl1.Slug, "Hall"); !codeIs(err, "already_exists") {
		t.Errorf("the same name twice: %v", err)
	}
	pl2 := mustPlan(t, s, other)
	away, err := s.CreateSectionOf(ctx, Tasks, other, pl2.Slug, "Away")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask(ctx, org, pl1.Slug, task["Doors"], &away.ID, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("into another organisation's section: want not found, got %v", err)
	}
	if _, err := s.CreateTaskIn(ctx, org, pl1.Slug, "Bar", away.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("made in another organisation's section: want not found, got %v", err)
	}
	if err := s.MoveTask(ctx, other, pl2.Slug, task["Doors"], nil, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("another organisation's task: want not found, got %v", err)
	}
	if err := s.DeleteSectionOf(ctx, Tasks, other, pl2.Slug, tech.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("deleting another organisation's section: want not found, got %v", err)
	}
	// deleting a section puts its tasks last of those without one, in their order; deleting a task closes the gap
	if err := s.DeleteSectionOf(ctx, Tasks, org, pl1.Slug, tech.ID); err != nil {
		t.Fatal(err)
	}
	if got := order(); got != "Doors/0 Video/0 Sound/0 Lights/0 Coffee/"+itoa(hall.ID) {
		t.Errorf("after deleting a section: %s", got)
	}
	if err := s.DeleteTask(ctx, org, pl1.Slug, task["Video"]); err != nil {
		t.Fatal(err)
	}
	if err := s.MoveTask(ctx, org, pl1.Slug, task["Lights"], nil, 0); err != nil || order() != "Lights/0 Doors/0 Sound/0 Coffee/"+itoa(hall.ID) {
		t.Errorf("after deleting a task: %s %v", order(), err)
	}
}
