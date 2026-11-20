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
