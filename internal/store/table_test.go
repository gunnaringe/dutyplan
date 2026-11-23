package store

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

// tableWorld is an organisation with a plan of two events (no places yet), a second plan with an event, and a second
// organisation with its own plan, event and task.
type tableWorld struct {
	s                 *Store
	org, otherOrg     Org
	plan, other, away Plan
	sound, camera     Task
	otherSound        Task // the other plan's own Sound
	foreignTask       Task
	ev, ev2, otherEv  Event
	awayEv            Event
}

func newTableWorld(t *testing.T) tableWorld {
	t.Helper()
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	s := open(t)
	w := tableWorld{s: s, org: mustOrg(t, s, "Club"), otherOrg: mustOrg(t, s, "Elsewhere")}
	var err error
	w.plan, err = s.CreatePlan(ctx, w.org.Slug, "Spring")
	must(err)
	w.sound, err = s.CreateTask(ctx, w.org.Slug, w.plan.Slug, "Sound")
	must(err)
	w.camera, err = s.CreateTask(ctx, w.org.Slug, w.plan.Slug, "Camera")
	must(err)
	w.other, err = s.CreatePlan(ctx, w.org.Slug, "Summer")
	must(err)
	w.ev, err = s.AddEvent(ctx, w.org.Slug, w.plan.Slug, "2026-08-09")
	must(err)
	w.ev2, err = s.AddEvent(ctx, w.org.Slug, w.plan.Slug, "2026-08-16")
	must(err)
	w.otherEv, err = s.AddEvent(ctx, w.org.Slug, w.other.Slug, "2026-08-23")
	must(err)
	w.otherSound, err = s.CreateTask(ctx, w.org.Slug, w.other.Slug, "Sound")
	must(err)

	w.away, err = s.CreatePlan(ctx, w.otherOrg.Slug, "Spring")
	must(err)
	w.foreignTask, err = s.CreateTask(ctx, w.otherOrg.Slug, w.away.Slug, "Doors")
	must(err)
	w.awayEv, err = s.AddEvent(ctx, w.otherOrg.Slug, w.away.Slug, "2026-08-09")
	must(err)
	return w
}

func (w tableWorld) places(t *testing.T, org, plan string) string {
	t.Helper()
	list, err := w.s.ListEventPlaces(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, p := range list {
		out = append(out, fmt.Sprintf("%d/%d=%d", p.EventID, p.TaskID, p.Slots))
	}
	return strings.Join(out, " ")
}

func TestPlacesPerEvent(t *testing.T) {
	w := newTableWorld(t)
	s, org, plan := w.s, w.org.Slug, w.plan.Slug
	if got := w.places(t, org, plan); got != "" {
		t.Fatalf("new events have no places, got %q", got)
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev2.ID, w.camera.ID, 1); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%d/%d=2 %d/%d=1", w.ev.ID, w.sound.ID, w.ev2.ID, w.camera.ID)
	if got := w.places(t, org, plan); got != want {
		t.Fatalf("places: got %q, want %q", got, want)
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, 2); !errors.Is(err, ErrInvalid) {
		t.Errorf("the same places again: want nothing to change, got %v", err)
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, 3); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, 0); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, 0); !errors.Is(err, ErrInvalid) {
		t.Errorf("0 where the task is not needed: want nothing to change, got %v", err)
	}
	if got := w.places(t, org, plan); got != fmt.Sprintf("%d/%d=1", w.ev2.ID, w.camera.ID) {
		t.Errorf("0 takes the task off the event: %q", got)
	}
	for _, n := range []int{-1, maxSlots + 1} {
		var ie *Coded
		if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, n); !errors.As(err, &ie) || ie.Code != "places_invalid" {
			t.Errorf("places %d: want places_invalid, got %v", n, err)
		}
	}
	if err := s.SetEventPlaces(ctx, org, plan, w.ev.ID, w.sound.ID, maxSlots); err != nil {
		t.Errorf("the most places: %v", err)
	}
	if _, err := s.DB().Exec(`UPDATE event_task SET slots = 0`); err == nil || !strings.Contains(err.Error(), "CHECK") {
		t.Errorf("0 places is no row, also in the database: %v", err)
	}
}

func TestPlacesPerEventStayInTheirPlanAndOrganisation(t *testing.T) {
	w := newTableWorld(t)
	s, org := w.s, w.org.Slug
	for name, c := range map[string]struct {
		org, plan   string
		event, task int64
	}{
		"event of another plan":            {org, w.plan.Slug, w.otherEv.ID, w.sound.ID},
		"task of another plan":             {org, w.other.Slug, w.otherEv.ID, w.sound.ID},
		"event of another organisation":    {org, w.plan.Slug, w.awayEv.ID, w.sound.ID},
		"task of another organisation":     {org, w.plan.Slug, w.ev.ID, w.foreignTask.ID},
		"own event through another org":    {w.otherOrg.Slug, w.away.Slug, w.ev.ID, w.sound.ID},
		"foreign task through its own org": {w.otherOrg.Slug, w.away.Slug, w.awayEv.ID, w.sound.ID},
		"unknown plan":                     {org, "nope", w.ev.ID, w.sound.ID},
	} {
		if err := s.SetEventPlaces(ctx, c.org, c.plan, c.event, c.task, 2); !errors.Is(err, ErrNotFound) {
			t.Errorf("%s: want not found, got %v", name, err)
		}
	}
	if err := s.SetEventPlaces(ctx, org, w.other.Slug, w.otherEv.ID, w.otherSound.ID, 2); err != nil {
		t.Fatal(err)
	}
	if err := s.SetEventPlaces(ctx, w.otherOrg.Slug, w.away.Slug, w.awayEv.ID, w.foreignTask.ID, 3); err != nil {
		t.Fatal(err)
	}
	if got := w.places(t, org, w.plan.Slug); got != "" {
		t.Errorf("the other plans' places show in this one: %q", got)
	}
	if got := w.places(t, org, w.other.Slug); got != fmt.Sprintf("%d/%d=2", w.otherEv.ID, w.otherSound.ID) {
		t.Errorf("other plan: %q", got)
	}
	if got := w.places(t, w.otherOrg.Slug, w.away.Slug); got != fmt.Sprintf("%d/%d=3", w.awayEv.ID, w.foreignTask.ID) {
		t.Errorf("other organisation: %q", got)
	}
	if _, err := s.ListEventPlaces(ctx, org, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
}

func TestPlacesGoWithTheirEventAndTask(t *testing.T) {
	w := newTableWorld(t)
	s, org, plan := w.s, w.org.Slug, w.plan.Slug
	extra, err := s.CreateTask(ctx, org, w.plan.Slug, "Coffee")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ ev, task int64 }{{w.ev.ID, w.sound.ID}, {w.ev2.ID, w.sound.ID}, {w.ev2.ID, extra.ID}} {
		if err := s.SetEventPlaces(ctx, org, plan, c.ev, c.task, 2); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.DeleteEvent(ctx, org, plan, w.ev.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTask(ctx, org, w.plan.Slug, extra.ID); err != nil {
		t.Fatalf("a task with only places on an event can be deleted: %v", err)
	}
	if got := w.places(t, org, plan); got != fmt.Sprintf("%d/%d=2", w.ev2.ID, w.sound.ID) {
		t.Errorf("after deleting an event and a task: %q", got)
	}
}

func TestDetailsRowsOfAPlan(t *testing.T) {
	w := newTableWorld(t)
	s, org, plan := w.s, w.org.Slug, w.plan.Slug
	theme, err := s.AddField(ctx, org, plan, "  Theme ")
	if err != nil || theme.Name != "Theme" {
		t.Fatalf("add: %+v %v", theme, err)
	}
	if _, err := s.AddField(ctx, org, plan, "Theme"); !errors.Is(err, ErrConflict) {
		t.Errorf("the same name twice: want a conflict, got %v", err)
	}
	if _, err := s.AddField(ctx, org, w.other.Slug, "Theme"); err != nil {
		t.Errorf("the same name in another plan: %v", err)
	}
	for name, n := range map[string]string{"blank": " ", "too long": strings.Repeat("x", 81), "control": "a\nb"} {
		if _, err := s.AddField(ctx, org, plan, n); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s name: want invalid, got %v", name, err)
		}
	}
	speaker, err := s.AddField(ctx, org, plan, "Speaker")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev.ID, theme.ID, " Light "); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev2.ID, theme.ID, "Dark"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev.ID, speaker.ID, "Guest"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev.ID, theme.ID, "Light"); !errors.Is(err, ErrInvalid) {
		t.Errorf("the same value again: want nothing to change, got %v", err)
	}
	for name, v := range map[string]string{"too long": strings.Repeat("é", maxFieldValueRune+1), "control": "a\x00b"} {
		var ie *Coded
		if err := s.SetFieldValue(ctx, org, plan, w.ev.ID, theme.ID, v); !errors.As(err, &ie) || ie.Code != "field_value_invalid" {
			t.Errorf("%s value: want field_value_invalid, got %v", name, err)
		}
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev2.ID, theme.ID, strings.Repeat("é", maxFieldValueRune)); err != nil {
		t.Errorf("the longest value: %v", err)
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev2.ID, theme.ID, "  "); err != nil {
		t.Fatalf("an empty value removes it: %v", err)
	}
	if err := s.SetFieldValue(ctx, org, plan, w.ev2.ID, theme.ID, ""); !errors.Is(err, ErrInvalid) {
		t.Errorf("removing what is not there: want nothing to change, got %v", err)
	}

	got, err := s.ListFields(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(got.Fields) != fmt.Sprintf("[{%d Theme 0 false} {%d Speaker 1 false}]", theme.ID, speaker.ID) ||
		fmt.Sprint(got.Values) != fmt.Sprintf("[{%d %d Light} {%d %d Guest}]", w.ev.ID, theme.ID, w.ev.ID, speaker.ID) {
		t.Errorf("list: %+v", got)
	}

	renamed, err := s.RenameField(ctx, org, plan, theme.ID, "Motto")
	if err != nil || renamed.Name != "Motto" {
		t.Fatalf("rename: %+v %v", renamed, err)
	}
	if _, err := s.RenameField(ctx, org, plan, theme.ID, "Motto"); !errors.Is(err, ErrInvalid) {
		t.Errorf("rename to the same name: want nothing to change, got %v", err)
	}
	if _, err := s.RenameField(ctx, org, plan, theme.ID, "Speaker"); !errors.Is(err, ErrConflict) {
		t.Errorf("rename to a taken name: want a conflict, got %v", err)
	}
	if err := s.FreezeField(ctx, org, plan, speaker.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := s.FreezeField(ctx, org, plan, 999999, true); !errors.Is(err, ErrNotFound) {
		t.Errorf("freezing an unknown row: want not found, got %v", err)
	}
	if err := s.DeleteField(ctx, org, plan, theme.ID); err != nil {
		t.Fatal(err)
	}
	got, _ = s.ListFields(ctx, org, plan)
	if fmt.Sprint(got.Fields) != fmt.Sprintf("[{%d Speaker 0 true}]", speaker.ID) || len(got.Values) != 1 || got.Values[0].FieldID != speaker.ID {
		t.Errorf("after deleting a row its values must be gone: %+v", got)
	}
	if n := count(t, s, `SELECT COUNT(*) FROM event_field WHERE field_id = ?`, theme.ID); n != 0 {
		t.Errorf("values of the deleted row are left: %d", n)
	}
	if err := s.DeleteEvent(ctx, org, plan, w.ev.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.ListFields(ctx, org, plan); len(got.Values) != 0 || len(got.Fields) != 1 {
		t.Errorf("a deleted event takes its values, not the row: %+v", got)
	}
}

func TestMovingADetailsRow(t *testing.T) {
	w := newTableWorld(t)
	s, org, plan := w.s, w.org.Slug, w.plan.Slug
	var ids []int64
	for _, n := range []string{"Note", "Theme", "Speaker"} {
		f, err := s.AddField(ctx, org, plan, n)
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, f.ID)
	}
	order := func() string {
		t.Helper()
		got, err := s.ListFields(ctx, org, plan)
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range got.Fields {
			out = append(out, fmt.Sprintf("%s:%d", f.Name, f.Position))
		}
		return strings.Join(out, " ")
	}
	if err := s.MoveField(ctx, org, plan, ids[2], 0); err != nil || order() != "Speaker:0 Note:1 Theme:2" {
		t.Errorf("to the top: %s %v", order(), err)
	}
	if err := s.MoveField(ctx, org, plan, ids[2], 99); err != nil || order() != "Note:0 Theme:1 Speaker:2" {
		t.Errorf("past the end is the bottom: %s %v", order(), err)
	}
	if err := s.MoveField(ctx, org, plan, ids[0], 1); err != nil || order() != "Theme:0 Note:1 Speaker:2" {
		t.Errorf("one down: %s %v", order(), err)
	}
	if err := s.DeleteField(ctx, org, plan, ids[1]); err != nil || order() != "Note:0 Speaker:1" {
		t.Errorf("deleting closes the gap: %s %v", order(), err)
	}
	if f, err := s.AddField(ctx, org, plan, "Theme"); err != nil || f.Position != 2 || order() != "Note:0 Speaker:1 Theme:2" {
		t.Errorf("a new row goes to the bottom: %+v %s %v", f, order(), err)
	}
	if err := s.MoveField(ctx, org, plan, ids[0], -1); !codeIs(err, "position_invalid") {
		t.Errorf("a negative position: %v", err)
	}
	away, _ := s.AddField(ctx, org, w.other.Slug, "Away")
	if err := s.MoveField(ctx, org, plan, away.ID, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("a row of another plan: %v", err)
	}
}

func TestAPlanHasAtMostSoManyDetailsRows(t *testing.T) {
	w := newTableWorld(t)
	for i := range maxFieldsPerPlan {
		if _, err := w.s.AddField(ctx, w.org.Slug, w.plan.Slug, fmt.Sprintf("Row %d", i)); err != nil {
			t.Fatal(err)
		}
	}
	var ie *Coded
	if _, err := w.s.AddField(ctx, w.org.Slug, w.plan.Slug, "One more"); !errors.As(err, &ie) || ie.Code != "too_many_fields" {
		t.Errorf("want too_many_fields, got %v", err)
	}
	if _, err := w.s.AddField(ctx, w.org.Slug, w.other.Slug, "One more"); err != nil {
		t.Errorf("the limit is per plan: %v", err)
	}
}

func TestDetailsRowsStayInTheirPlanAndOrganisation(t *testing.T) {
	w := newTableWorld(t)
	s, org := w.s, w.org.Slug
	mine, err := s.AddField(ctx, org, w.plan.Slug, "Theme")
	if err != nil {
		t.Fatal(err)
	}
	theirs, err := s.AddField(ctx, org, w.other.Slug, "Theme")
	if err != nil {
		t.Fatal(err)
	}
	away, err := s.AddField(ctx, w.otherOrg.Slug, w.away.Slug, "Theme")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldValue(ctx, org, w.other.Slug, w.otherEv.ID, theirs.ID, "Theirs"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetFieldValue(ctx, w.otherOrg.Slug, w.away.Slug, w.awayEv.ID, away.ID, "Away"); err != nil {
		t.Fatal(err)
	}
	for name, c := range map[string]struct {
		org, plan    string
		event, field int64
	}{
		"row of another plan":         {org, w.plan.Slug, w.ev.ID, theirs.ID},
		"row of another organisation": {org, w.plan.Slug, w.ev.ID, away.ID},
		"event of another plan":       {org, w.plan.Slug, w.otherEv.ID, mine.ID},
		"own row through another org": {w.otherOrg.Slug, w.away.Slug, w.awayEv.ID, mine.ID},
	} {
		if err := s.SetFieldValue(ctx, c.org, c.plan, c.event, c.field, "x"); !errors.Is(err, ErrNotFound) {
			t.Errorf("value, %s: want not found, got %v", name, err)
		}
	}
	if _, err := s.RenameField(ctx, org, w.plan.Slug, theirs.ID, "Mine"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rename a row of another plan: %v", err)
	}
	if _, err := s.RenameField(ctx, org, w.plan.Slug, away.ID, "Mine"); !errors.Is(err, ErrNotFound) {
		t.Errorf("rename a row of another organisation: %v", err)
	}
	if err := s.DeleteField(ctx, org, w.plan.Slug, theirs.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete a row of another plan: %v", err)
	}
	if err := s.DeleteField(ctx, w.otherOrg.Slug, w.away.Slug, mine.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete a row of another organisation: %v", err)
	}
	got, err := s.ListFields(ctx, org, w.plan.Slug)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Fields) != 1 || got.Fields[0].ID != mine.ID || len(got.Values) != 0 {
		t.Errorf("this plan sees others' rows or values: %+v", got)
	}
	if got, _ := s.ListFields(ctx, w.otherOrg.Slug, w.away.Slug); len(got.Fields) != 1 || len(got.Values) != 1 || got.Values[0].Value != "Away" {
		t.Errorf("other organisation: %+v", got)
	}
	if _, err := s.ListFields(ctx, org, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
}

func TestAnswersForTheTable(t *testing.T) {
	w := newTableWorld(t)
	s, org := w.s, w.org.Slug
	kari, err := s.CreatePerson(ctx, org, w.plan.Slug, "Kari Test")
	if err != nil {
		t.Fatal(err)
	}
	ola, err := s.CreatePerson(ctx, org, w.plan.Slug, "Ola Test")
	if err != nil {
		t.Fatal(err)
	}
	eva, err := s.CreatePerson(ctx, w.otherOrg.Slug, w.away.Slug, "Eva Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAvailability(ctx, org, w.plan.Slug, kari.ID, map[int64]string{w.ev.ID: "good", w.ev2.ID: "never"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAvailability(ctx, org, w.plan.Slug, ola.ID, map[int64]string{w.ev2.ID: "poor"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAvailability(ctx, org, w.other.Slug, kari.ID, map[int64]string{w.otherEv.ID: "poor"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("a person of another plan: %v", err)
	}
	otherKari, err := s.CreatePerson(ctx, org, w.other.Slug, "Kari Test")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetAvailability(ctx, org, w.other.Slug, otherKari.ID, map[int64]string{w.otherEv.ID: "poor"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetAvailability(ctx, w.otherOrg.Slug, w.away.Slug, eva.ID, map[int64]string{w.awayEv.ID: "good"}); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonTask(ctx, org, w.plan.Slug, kari.ID, w.sound.ID, "good"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonTask(ctx, org, w.plan.Slug, ola.ID, w.camera.ID, "poor"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetPersonTask(ctx, w.otherOrg.Slug, w.away.Slug, eva.ID, w.foreignTask.ID, "good"); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListAnswers(ctx, org, w.plan.Slug)
	if err != nil {
		t.Fatal(err)
	}
	wantEvents := fmt.Sprintf("[{%d %d good} {%d %d never} {%d %d poor}]", kari.ID, w.ev.ID, kari.ID, w.ev2.ID, ola.ID, w.ev2.ID)
	wantTasks := fmt.Sprintf("[{%d %d good} {%d %d poor}]", kari.ID, w.sound.ID, ola.ID, w.camera.ID)
	if fmt.Sprint(got.Events) != wantEvents || fmt.Sprint(got.Tasks) != wantTasks {
		t.Errorf("answers:\n got %v %v\nwant %s %s", got.Events, got.Tasks, wantEvents, wantTasks)
	}
	if _, err := s.ListAnswers(ctx, org, "nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown plan: %v", err)
	}
	if got, _ := s.ListAnswers(ctx, w.otherOrg.Slug, w.away.Slug); len(got.Events) != 1 || len(got.Tasks) != 1 || got.Events[0].PersonID != eva.ID {
		t.Errorf("other organisation: %+v", got)
	}
}

func TestOneTaskWithEveryOtherAtOnce(t *testing.T) {
	w := newTableWorld(t)
	s, org := w.s, w.org.Slug
	coffee, err := s.CreateTask(ctx, org, w.plan.Slug, "Coffee")
	if err != nil {
		t.Fatal(err)
	}
	foreignToo, err := s.CreateTask(ctx, w.otherOrg.Slug, w.away.Slug, "Ticket")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetCombination(ctx, w.otherOrg.Slug, w.away.Slug, w.foreignTask.ID, foreignToo.ID, "practical"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCombination(ctx, org, w.plan.Slug, w.sound.ID, w.camera.ID, "practical"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCombination(ctx, org, w.plan.Slug, w.camera.ID, coffee.ID, "practical"); err != nil {
		t.Fatal(err)
	}
	pairs := func(org string) string {
		t.Helper()
		list, err := s.ListCombinations(ctx, org, w.plan.Slug)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprint(list)
	}
	if err := s.SetTaskCombinations(ctx, org, w.plan.Slug, w.sound.ID, "cannot"); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("[{%d %d cannot} {%d %d cannot} {%d %d practical}]", w.sound.ID, w.camera.ID, w.sound.ID, coffee.ID, w.camera.ID, coffee.ID)
	if got := pairs(org); got != want {
		t.Errorf("cannot with anything:\n got %s\nwant %s", got, want)
	}
	if err := s.SetTaskCombinations(ctx, org, w.plan.Slug, w.sound.ID, "cannot"); !errors.Is(err, ErrInvalid) {
		t.Errorf("again: want nothing to change, got %v", err)
	}
	if err := s.SetTaskCombinations(ctx, org, w.plan.Slug, w.sound.ID, "standard"); err != nil {
		t.Fatal(err)
	}
	if got := pairs(org); got != fmt.Sprintf("[{%d %d practical}]", w.camera.ID, coffee.ID) {
		t.Errorf("back to standard: %s", got)
	}
	if err := s.SetTaskCombinations(ctx, org, w.plan.Slug, w.sound.ID, "sometimes"); !errors.Is(err, ErrInvalid) {
		t.Errorf("unknown level: %v", err)
	}
	if err := s.SetTaskCombinations(ctx, org, w.plan.Slug, w.foreignTask.ID, "cannot"); !errors.Is(err, ErrNotFound) {
		t.Errorf("a task of another organisation: want not found, got %v", err)
	}
	if err := s.SetTaskCombinations(ctx, w.otherOrg.Slug, w.away.Slug, w.foreignTask.ID, "cannot"); err != nil {
		t.Fatal(err)
	}
	if got := pairs(w.otherOrg.Slug); got != fmt.Sprintf("[{%d %d cannot}]", w.foreignTask.ID, foreignToo.ID) {
		t.Errorf("other organisation: %s", got)
	}
	if got := pairs(org); got != fmt.Sprintf("[{%d %d practical}]", w.camera.ID, coffee.ID) {
		t.Errorf("another organisation's change reached this one: %s", got)
	}
}

func TestMovingATask(t *testing.T) {
	w := newTableWorld(t)
	s, org := w.s, w.org.Slug
	video, err := s.CreateTask(ctx, org, w.plan.Slug, "Video")
	if err != nil {
		t.Fatal(err)
	}
	planOf := map[string]string{w.org.Slug: w.plan.Slug, w.otherOrg.Slug: w.away.Slug}
	order := func(slug string) string {
		t.Helper()
		got, err := s.ListTasks(ctx, slug, planOf[slug])
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, tk := range got {
			out = append(out, tk.Name)
		}
		return strings.Join(out, " ")
	}
	if got := order(org); got != "Sound Camera Video" {
		t.Fatalf("new tasks go to the bottom, not by name: %s", got)
	}
	if err := s.MoveTask(ctx, org, w.plan.Slug, video.ID, nil, 0); err != nil || order(org) != "Video Sound Camera" {
		t.Errorf("to the top: %s %v", order(org), err)
	}
	if err := s.MoveTask(ctx, org, w.plan.Slug, video.ID, nil, 99); err != nil || order(org) != "Sound Camera Video" {
		t.Errorf("past the end is the bottom: %s %v", order(org), err)
	}
	if err := s.MoveTask(ctx, org, w.plan.Slug, w.sound.ID, nil, 1); err != nil || order(org) != "Camera Sound Video" {
		t.Errorf("one down: %s %v", order(org), err)
	}
	if err := s.DeleteTask(ctx, org, w.plan.Slug, w.sound.ID); err != nil || order(org) != "Camera Video" {
		t.Fatalf("delete: %s %v", order(org), err)
	}
	if err := s.DeleteTask(ctx, org, w.plan.Slug, w.camera.ID); err != nil || order(org) != "Video" {
		t.Fatalf("delete: %s %v", order(org), err)
	}
	if _, err := s.CreateTask(ctx, org, w.plan.Slug, "Lights"); err != nil || order(org) != "Video Lights" {
		t.Errorf("deleted tasks leave no gap, so a new one is last: %s %v", order(org), err)
	}
	lights := mustTask(t, s, org, w.plan.Slug, "Lights")
	if err := s.MoveTask(ctx, org, w.plan.Slug, lights, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateTask(ctx, org, w.plan.Slug, "Sound"); err != nil || order(org) != "Lights Video Sound" {
		t.Errorf("a new task goes below a moved one: %s %v", order(org), err)
	}
	if err := s.MoveTask(ctx, org, w.plan.Slug, video.ID, nil, -1); !codeIs(err, "position_invalid") {
		t.Errorf("a negative position: %v", err)
	}
	if err := s.MoveTask(ctx, org, w.plan.Slug, w.foreignTask.ID, nil, 0); !errors.Is(err, ErrNotFound) {
		t.Errorf("a task of another organisation: %v", err)
	}
	if err := s.MoveTask(ctx, w.otherOrg.Slug, w.away.Slug, w.foreignTask.ID, nil, 0); err != nil || order(org) != "Lights Video Sound" || order(w.otherOrg.Slug) != "Doors" {
		t.Errorf("each organisation has its own order: %s / %s %v", order(org), order(w.otherOrg.Slug), err)
	}
}

func mustTask(t *testing.T, s *Store, org, plan, name string) int64 {
	t.Helper()
	list, err := s.ListTasks(ctx, org, plan)
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range list {
		if tk.Name == name {
			return tk.ID
		}
	}
	t.Fatalf("no task %s", name)
	return 0
}
