package store

import (
	"context"
	"database/sql"
)

// PlanPerson is a person as the planner sees them.
type PlanPerson struct {
	ID         int64
	Name       string
	Active     bool
	Proposable bool
}

// PlanningData is everything the planner needs for one plan, read in one transaction so it is consistent.
// Everything is scoped by the organisation (and the plan for events and availability).
type PlanningData struct {
	Tasks        []Task
	Events       []Event
	People       []PlanPerson
	PersonTasks  map[int64]map[int64]string // person -> task -> level (a row means: takes part)
	PersonEvents map[int64]map[int64]string // person -> event -> level (no row = neutral)
	Combinations []Combination
	Groups       []Group
	Cells        []ManualCell            // manual cells of the plan
	Pools        []Pool                  // selections of the plan
	Weights      map[string]any          // what the plan changed in the planner's weighting (the rest is the default)
	EventPlaces  map[int64]map[int64]int // event -> task -> places (a task without an entry is not needed on the event)
}

func (s *Store) GetPlanningData(ctx context.Context, orgSlug, planSlug string) (PlanningData, error) {
	var d PlanningData
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return d, err
	}
	defer tx.Rollback()
	oid, err := orgID(ctx, tx, orgSlug)
	if err != nil {
		return d, err
	}
	pid, err := planID(ctx, tx, oid, planSlug)
	if err != nil {
		return d, err
	}

	d.Tasks = []Task{}
	if err := scanAll(ctx, tx, `SELECT id, name, planned FROM task WHERE org_id = ? ORDER BY id`, []any{oid},
		func(r *sql.Rows) error {
			var t Task
			err := r.Scan(&t.ID, &t.Name, &t.Planned)
			d.Tasks = append(d.Tasks, t)
			return err
		}); err != nil {
		return d, err
	}
	d.Events = []Event{}
	// In the order of date and creation, not time of day: the planner's input (and so a run's hash) must not change when a time is set.
	if err := scanAll(ctx, tx, `SELECT `+eventCols+` FROM event d
		WHERE d.org_id = ? AND d.plan_id = ? ORDER BY d.date, d.id`, []any{oid, pid},
		func(r *sql.Rows) error {
			e, err := scanEvent(r)
			d.Events = append(d.Events, e)
			return err
		}); err != nil {
		return d, err
	}
	d.People = []PlanPerson{}
	if err := scanAll(ctx, tx, `SELECT id, name, active, proposable FROM person WHERE org_id = ? ORDER BY id`, []any{oid},
		func(r *sql.Rows) error {
			var p PlanPerson
			err := r.Scan(&p.ID, &p.Name, &p.Active, &p.Proposable)
			d.People = append(d.People, p)
			return err
		}); err != nil {
		return d, err
	}
	d.PersonTasks = map[int64]map[int64]string{}
	if err := scanAll(ctx, tx, `SELECT person_id, task_id, level FROM person_task WHERE org_id = ?`, []any{oid},
		func(r *sql.Rows) error {
			var p, t int64
			var l string
			if err := r.Scan(&p, &t, &l); err != nil {
				return err
			}
			if d.PersonTasks[p] == nil {
				d.PersonTasks[p] = map[int64]string{}
			}
			d.PersonTasks[p][t] = l
			return nil
		}); err != nil {
		return d, err
	}
	d.PersonEvents = map[int64]map[int64]string{}
	if err := scanAll(ctx, tx, `SELECT person_id, event_id, level FROM person_event WHERE org_id = ? AND plan_id = ?`, []any{oid, pid},
		func(r *sql.Rows) error {
			var p, e int64
			var l string
			if err := r.Scan(&p, &e, &l); err != nil {
				return err
			}
			if d.PersonEvents[p] == nil {
				d.PersonEvents[p] = map[int64]string{}
			}
			d.PersonEvents[p][e] = l
			return nil
		}); err != nil {
		return d, err
	}
	d.Combinations = []Combination{}
	if err := scanAll(ctx, tx, `SELECT task_a, task_b, level FROM task_combination WHERE org_id = ? ORDER BY task_a, task_b`, []any{oid},
		func(r *sql.Rows) error {
			var c Combination
			err := r.Scan(&c.A, &c.B, &c.Level)
			d.Combinations = append(d.Combinations, c)
			return err
		}); err != nil {
		return d, err
	}
	if d.Groups, err = listGroups(ctx, tx, oid); err != nil {
		return d, err
	}
	if d.Cells, err = listCells(ctx, tx, oid, pid); err != nil {
		return d, err
	}
	if d.Pools, err = listPools(ctx, tx, oid, pid); err != nil {
		return d, err
	}
	if d.Weights, err = getWeights(ctx, tx, oid, pid); err != nil {
		return d, err
	}
	places, err := listEventPlaces(ctx, tx, oid, pid)
	if err != nil {
		return d, err
	}
	d.EventPlaces = map[int64]map[int64]int{}
	for _, x := range places {
		if d.EventPlaces[x.EventID] == nil {
			d.EventPlaces[x.EventID] = map[int64]int{}
		}
		d.EventPlaces[x.EventID][x.TaskID] = x.Slots
	}
	return d, nil
}

func scanAll(ctx context.Context, q dbq, query string, args []any, each func(*sql.Rows) error) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		if err := each(rows); err != nil {
			return err
		}
	}
	return rows.Err()
}
