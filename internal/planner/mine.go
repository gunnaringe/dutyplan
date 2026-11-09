package planner

import (
	"context"
	"sort"

	"github.com/gunnaringe/dutyplan/internal/store"
)

// MyAssignment is one thing a person has been given in the plan: a place of their own, or one through a group.
type MyAssignment struct {
	EventID int64  `json:"event_id"`
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Note    string `json:"note"`
	TaskID  int64  `json:"task_id"`
	Task    string `json:"task"`
	Via     string `json:"via,omitempty"` // the group they are in, when the place is the group's
	Locked  bool   `json:"locked"`        // set by hand
	Weak    bool   `json:"weak"`          // they said «dårlig» for the event or the task
}

// MineRun says which plan the assignments come from.
type MineRun struct {
	ID         int64  `json:"id"`
	Actor      string `json:"actor"`
	FinishedAt string `json:"finished_at"`
	Stale      bool   `json:"stale"` // the plan's data has changed since
}

// Mine is «mine oppdrag»: what one person has in the plan.
type Mine struct {
	Person      store.Person   `json:"person"`
	HasPlan     bool           `json:"has_plan"`
	Run         *MineRun       `json:"run,omitempty"`
	Assignments []MyAssignment `json:"assignments"`
}

// Mine reads the plan's newest finished run (a run that is still going or failed is not a plan) and picks out what the
// person has in it, in the order of the events. The person must belong to the organisation.
func (s *Service) Mine(ctx context.Context, orgSlug, planSlug string, personID int64) (Mine, error) {
	m := Mine{Assignments: []MyAssignment{}}
	var err error
	if m.Person, err = s.st.GetPerson(ctx, orgSlug, personID); err != nil {
		return m, err
	}
	runs, err := s.st.ListRuns(ctx, orgSlug, planSlug, 0)
	if err != nil {
		return m, err
	}
	for _, r := range runs { // newest first
		if r.Status != "done" {
			continue
		}
		v, err := s.View(ctx, orgSlug, planSlug, r.ID)
		if err != nil {
			return m, err
		}
		m.HasPlan = true
		m.Run = &MineRun{ID: r.ID, Actor: r.Actor, FinishedAt: r.FinishedAt, Stale: v.Run.Stale}
		names := map[int64]string{}
		for _, t := range v.Tasks {
			names[t.ID] = t.Name
		}
		has := func(ids []int64) bool {
			for _, id := range ids {
				if id == personID {
					return true
				}
			}
			return false
		}
		for _, ev := range v.Events {
			var day []MyAssignment
			for _, c := range ev.Cells {
				mk := func() MyAssignment {
					return MyAssignment{EventID: ev.ID, Date: ev.Date, DayType: ev.DayType, Note: ev.Note, TaskID: c.TaskID, Task: names[c.TaskID]}
				}
				for _, p := range c.People {
					if p.ID == personID {
						a := mk()
						a.Locked, a.Weak = p.Locked, p.Weak
						day = append(day, a)
					}
				}
				for _, g := range c.Groups {
					if has(g.MemberIDs) {
						a := mk()
						a.Via, a.Locked, a.Weak = g.Name, g.Locked, has(g.WeakIDs)
						day = append(day, a)
					}
				}
			}
			sort.SliceStable(day, func(i, j int) bool { return day[i].Task < day[j].Task })
			m.Assignments = append(m.Assignments, day...)
		}
		break
	}
	return m, nil
}
