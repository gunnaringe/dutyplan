package planner

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gunnaringe/dutyplan/internal/store"
)

func TestTheViewShowsManualTextsAndLockedPeopleWithNames(t *testing.T) {
	w := newWorld(t, "Club")
	e0 := eid(w.events[0].ID)
	answer, _ := json.Marshal(map[string]any{
		"status": "OPTIMAL", "wall_time": 0.2,
		"assignments": []map[string]any{
			{"date": e0, "task": tid(w.camera.ID), "person": pid(w.people["Cecilie"].ID), "locked": true, "task_level": "neutral", "date_level": "neutral"},
		},
		"manual":      []map[string]any{{"date": e0, "task": tid(w.singing.ID), "label": "Outside guest", "fills": 2}},
		"concessions": []map[string]any{}, "unfilled": []map[string]any{}, "warnings": []string{},
		"load": map[string]any{}, "violations": []string{},
	})
	s := newService(w, func(context.Context, []byte) ([]byte, error) { return answer, nil })
	id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	if err != nil {
		t.Fatal(err)
	}
	cell := func(task store.Task) *Cell {
		for i := range v.Events[0].Cells {
			if v.Events[0].Cells[i].TaskID == task.ID {
				return &v.Events[0].Cells[i]
			}
		}
		return nil
	}
	if c := cell(w.singing); c == nil || len(c.Texts) != 1 || c.Texts[0].Text != "Outside guest" || c.Texts[0].Fills != 2 || c.Slots != 2 || c.Missing != 0 {
		t.Errorf("manual text: %+v", c)
	}
	if c := cell(w.camera); c == nil || len(c.People) != 1 || !c.People[0].Locked || c.People[0].Name != "Cecilie" {
		t.Errorf("locked person: %+v", c)
	}
}

// A name locked when the run was made is kept in the snapshot, so the page can still show it as the planner's after it is
// unlocked; a name set by hand is not.
func TestTheViewTellsALockedNameFromOneSetByHand(t *testing.T) {
	w := newWorld(t, "Club")
	ev := w.events[0]
	locked, hand := w.people["Cecilie"], w.people["Dina"]
	if _, err := w.st.AddCell(ctx, w.org.Slug, w.plan.Slug, store.NewCell{EventID: ev.ID, TaskID: w.camera.ID, PersonID: locked.ID, Locked: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.st.AddCell(ctx, w.org.Slug, w.plan.Slug, store.NewCell{EventID: ev.ID, TaskID: w.sound.ID, PersonID: hand.ID}); err != nil {
		t.Fatal(err)
	}
	e0 := eid(ev.ID)
	answer, _ := json.Marshal(map[string]any{
		"status": "OPTIMAL", "wall_time": 0.2,
		"assignments": []map[string]any{
			{"date": e0, "task": tid(w.camera.ID), "person": pid(locked.ID), "locked": true, "task_level": "good", "date_level": "good"},
			{"date": e0, "task": tid(w.sound.ID), "person": pid(hand.ID), "locked": true, "task_level": "good", "date_level": "good"},
		},
		"manual": []map[string]any{}, "concessions": []map[string]any{}, "unfilled": []map[string]any{}, "warnings": []string{},
		"load": map[string]any{}, "violations": []string{},
	})
	s := newService(w, func(context.Context, []byte) ([]byte, error) { return answer, nil })
	id, err := s.Start(ctx, w.org.Slug, w.plan.Slug, Params{TimeLimit: 5})
	if err != nil {
		t.Fatal(err)
	}
	waitDone(t, s)
	v, err := s.View(ctx, w.org.Slug, w.plan.Slug, id)
	if err != nil {
		t.Fatal(err)
	}
	got := map[int64]CellPerson{}
	for _, c := range v.Events[0].Cells {
		for _, p := range c.People {
			got[p.ID] = p
		}
	}
	if p := got[locked.ID]; !p.Locked || !p.Kept {
		t.Errorf("the locked name: %+v, want locked and kept", p)
	}
	if p := got[hand.ID]; !p.Locked || p.Kept {
		t.Errorf("the name set by hand: %+v, want locked and not kept", p)
	}
}
