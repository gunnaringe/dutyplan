package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

const maxEventsPerWrite = 400

// ManualCell is a place set by hand: a person, a group or a text for one task on one event. It is locked: the planner
// never moves it, and it counts as a filled place (Fills places for a group or a text).
type ManualCell struct {
	ID       int64  `json:"id"`
	EventID  int64  `json:"event_id"`
	TaskID   int64  `json:"task_id"`
	PersonID *int64 `json:"person_id,omitempty"`
	GroupID  *int64 `json:"group_id,omitempty"`
	Label    string `json:"label,omitempty"`
	Fills    int    `json:"fills"`
}

// NewCell is a request for manual cells: the same thing on every listed event. Exactly one of PersonID, GroupID, Label.
type NewCell struct {
	EventIDs []int64 `json:"events"`
	TaskID   int64   `json:"task"`
	PersonID int64   `json:"person"`
	GroupID  int64   `json:"group"`
	Label    string  `json:"label"`
	Fills    int     `json:"fills"` // places a group or a text fills (default 1); a person always fills one
}

// Pool is a selection (utvalg): only these people and groups can be chosen for the task on the event.
type Pool struct {
	EventID int64   `json:"event_id"`
	TaskID  int64   `json:"task_id"`
	People  []int64 `json:"people"`
	Groups  []int64 `json:"groups"`
}

// distinct drops repeated ids and keeps the order; it refuses a list that is too long.
func distinct(ids []int64) ([]int64, error) {
	if len(ids) > maxEventsPerWrite {
		return nil, Invalid("too_many_events", map[string]any{"max": maxEventsPerWrite}, "at most %d events at a time", maxEventsPerWrite)
	}
	seen := map[int64]bool{}
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, nil
}

// eventsOfPlan checks that every event belongs to this plan (anything else is «not found», and nothing is written)
// and returns their dates for the log.
func eventsOfPlan(ctx context.Context, q dbq, oid, pid int64, ids []int64) ([]string, error) {
	dates := make([]string, 0, len(ids))
	for _, id := range ids {
		e, err := getEvent(ctx, q, oid, pid, id)
		if err != nil {
			return nil, err
		}
		dates = append(dates, e.Date)
	}
	return dates, nil
}

func listCells(ctx context.Context, q dbq, oid, pid int64) ([]ManualCell, error) {
	out := []ManualCell{}
	err := scanAll(ctx, q, `SELECT c.id, c.event_id, c.task_id, c.person_id, c.group_id, c.label, c.fills
		FROM manual_cell c JOIN event e ON e.id = c.event_id AND e.org_id = c.org_id
		WHERE c.org_id = ? AND c.plan_id = ? ORDER BY e.date, e.id, c.task_id, c.id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var c ManualCell
			var person, group sql.NullInt64
			var label sql.NullString
			if err := r.Scan(&c.ID, &c.EventID, &c.TaskID, &person, &group, &label, &c.Fills); err != nil {
				return err
			}
			if person.Valid {
				c.PersonID = &person.Int64
			}
			if group.Valid {
				c.GroupID = &group.Int64
			}
			c.Label = label.String
			out = append(out, c)
			return nil
		})
	return out, err
}

func (s *Store) ListCells(ctx context.Context, orgSlug, planSlug string) ([]ManualCell, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	return listCells(ctx, s.db, oid, pid)
}

// AddCells writes the same manual cell on every listed event, all or nothing. An event that already has it is skipped
// (and counted); if nothing new would be written, that is a conflict. A person must be active.
func (s *Store) AddCells(ctx context.Context, orgSlug, planSlug, actor string, in NewCell) (created []ManualCell, skipped int, err error) {
	kinds := 0
	if in.PersonID != 0 {
		kinds++
	}
	if in.GroupID != 0 {
		kinds++
	}
	label := strings.TrimSpace(in.Label)
	if label != "" {
		kinds++
		if label, err = cleanName(label, "text"); err != nil {
			return nil, 0, err
		}
	}
	if kinds != 1 {
		return nil, 0, Invalid("cell_kind", nil, "give exactly one of person, group or text")
	}
	fills := in.Fills
	if fills == 0 {
		fills = 1
	}
	if fills < 1 || fills > 50 || (in.PersonID != 0 && fills != 1) {
		return nil, 0, Invalid("cell_fills", nil, "places must be 1-50 (a person fills one)")
	}
	events, err := distinct(in.EventIDs)
	if err != nil {
		return nil, 0, err
	}
	if len(events) == 0 {
		return nil, 0, Invalid("choose_events", nil, "choose at least one event")
	}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		created, skipped = nil, 0
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		var taskName, what string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND org_id = ?`, in.TaskID, oid).Scan(&taskName); errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		} else if err != nil {
			return logEntry{}, err
		}
		var person, group any
		switch {
		case in.PersonID != 0:
			var active bool
			if err := tx.QueryRowContext(ctx, `SELECT name, active FROM person WHERE id = ? AND org_id = ?`, in.PersonID, oid).Scan(&what, &active); errors.Is(err, sql.ErrNoRows) {
				return logEntry{}, ErrNotFound
			} else if err != nil {
				return logEntry{}, err
			}
			if !active {
				return logEntry{}, Invalid("person_inactive", map[string]any{"name": what}, "%s is not active", what)
			}
			person = in.PersonID
		case in.GroupID != 0:
			if err := tx.QueryRowContext(ctx, `SELECT name FROM grp WHERE id = ? AND org_id = ?`, in.GroupID, oid).Scan(&what); errors.Is(err, sql.ErrNoRows) {
				return logEntry{}, ErrNotFound
			} else if err != nil {
				return logEntry{}, err
			}
			group = in.GroupID
		default:
			what = label
		}
		dates, err := eventsOfPlan(ctx, tx, oid, pid, events)
		if err != nil {
			return logEntry{}, err
		}
		var text any
		if label != "" {
			text = label
		}
		for _, ev := range events {
			res, err := tx.ExecContext(ctx, `INSERT INTO manual_cell (org_id, plan_id, event_id, task_id, person_id, group_id, label, fills, created_at)
				VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, oid, pid, ev, in.TaskID, person, group, text, fills, s.stamp())
			if isUnique(err) {
				skipped++
				continue
			}
			if err != nil {
				return logEntry{}, err
			}
			c := ManualCell{EventID: ev, TaskID: in.TaskID, Label: label, Fills: fills}
			c.ID, _ = res.LastInsertId()
			if in.PersonID != 0 {
				c.PersonID = &in.PersonID
			}
			if in.GroupID != 0 {
				c.GroupID = &in.GroupID
			}
			created = append(created, c)
		}
		if len(created) == 0 {
			return logEntry{}, Conflict("cell_exists", nil, "that manual cell already exists")
		}
		return logEntry{orgID: oid, planID: &pid, action: "cell.add", detail: clip(fmt.Sprintf("%s: %s, %s", taskName, what, datesText(dates)), 200)}, nil
	})
	if err != nil {
		return nil, 0, err
	}
	return created, skipped, nil
}

func datesText(dates []string) string {
	if len(dates) <= 3 {
		return strings.Join(dates, ", ")
	}
	return fmt.Sprintf("%d events (%s ...)", len(dates), strings.Join(dates[:3], ", "))
}

// DeleteCell removes one manual cell of this plan. The planner may then choose for the place again.
func (s *Store) DeleteCell(ctx context.Context, orgSlug, planSlug, actor string, id int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		var date, task string
		var who sql.NullString
		err = tx.QueryRowContext(ctx, `SELECT e.date, t.name, COALESCE(p.name, g.name, c.label)
			FROM manual_cell c JOIN event e ON e.id = c.event_id AND e.org_id = c.org_id
			JOIN task t ON t.id = c.task_id AND t.org_id = c.org_id
			LEFT JOIN person p ON p.id = c.person_id AND p.org_id = c.org_id
			LEFT JOIN grp g ON g.id = c.group_id AND g.org_id = c.org_id
			WHERE c.id = ? AND c.org_id = ? AND c.plan_id = ?`, id, oid, pid).Scan(&date, &task, &who)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM manual_cell WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "cell.delete", detail: fmt.Sprintf("%s %s: %s", task, date, who.String)}, nil
	})
}

func listPools(ctx context.Context, q dbq, oid, pid int64) ([]Pool, error) {
	out := []Pool{}
	idx := map[[2]int64]int{}
	err := scanAll(ctx, q, `SELECT m.event_id, m.task_id, m.person_id, m.group_id
		FROM pool_member m JOIN event e ON e.id = m.event_id AND e.org_id = m.org_id
		WHERE m.org_id = ? AND m.plan_id = ? ORDER BY e.date, e.id, m.task_id, m.id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var ev, task int64
			var person, group sql.NullInt64
			if err := r.Scan(&ev, &task, &person, &group); err != nil {
				return err
			}
			k := [2]int64{ev, task}
			i, ok := idx[k]
			if !ok {
				i = len(out)
				idx[k] = i
				out = append(out, Pool{EventID: ev, TaskID: task, People: []int64{}, Groups: []int64{}})
			}
			if person.Valid {
				out[i].People = append(out[i].People, person.Int64)
			}
			if group.Valid {
				out[i].Groups = append(out[i].Groups, group.Int64)
			}
			return nil
		})
	return out, err
}

func (s *Store) ListPools(ctx context.Context, orgSlug, planSlug string) ([]Pool, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	return listPools(ctx, s.db, oid, pid)
}

// SetPool replaces the selection for one task on every listed event, all or nothing; no people and no groups removes
// it. Only a task the planner distributes can have one, and a person in it must be active.
func (s *Store) SetPool(ctx context.Context, orgSlug, planSlug, actor string, eventIDs []int64, taskID int64, people, groups []int64) error {
	events, err := distinct(eventIDs)
	if err != nil {
		return err
	}
	if len(events) == 0 {
		return Invalid("choose_events", nil, "choose at least one event")
	}
	people, err = distinct(people)
	if err != nil {
		return err
	}
	groups, err = distinct(groups)
	if err != nil {
		return err
	}
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		var taskName string
		var planned bool
		if err := tx.QueryRowContext(ctx, `SELECT name, planned FROM task WHERE id = ? AND org_id = ?`, taskID, oid).Scan(&taskName, &planned); errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		} else if err != nil {
			return logEntry{}, err
		}
		if !planned {
			return logEntry{}, Invalid("task_not_distributed", nil, "the planner does not distribute %q, so there is nothing to select for", taskName)
		}
		var names []string
		for _, p := range people {
			var n string
			var active bool
			if err := tx.QueryRowContext(ctx, `SELECT name, active FROM person WHERE id = ? AND org_id = ?`, p, oid).Scan(&n, &active); errors.Is(err, sql.ErrNoRows) {
				return logEntry{}, ErrNotFound
			} else if err != nil {
				return logEntry{}, err
			}
			if !active {
				return logEntry{}, Invalid("person_inactive", map[string]any{"name": n}, "%s is not active", n)
			}
			names = append(names, n)
		}
		for _, g := range groups {
			var n string
			if err := tx.QueryRowContext(ctx, `SELECT name FROM grp WHERE id = ? AND org_id = ?`, g, oid).Scan(&n); errors.Is(err, sql.ErrNoRows) {
				return logEntry{}, ErrNotFound
			} else if err != nil {
				return logEntry{}, err
			}
			names = append(names, n)
		}
		dates, err := eventsOfPlan(ctx, tx, oid, pid, events)
		if err != nil {
			return logEntry{}, err
		}
		for _, ev := range events {
			if _, err := tx.ExecContext(ctx, `DELETE FROM pool_member WHERE org_id = ? AND plan_id = ? AND event_id = ? AND task_id = ?`, oid, pid, ev, taskID); err != nil {
				return logEntry{}, err
			}
			for _, p := range people {
				if _, err := tx.ExecContext(ctx, `INSERT INTO pool_member (org_id, plan_id, event_id, task_id, person_id) VALUES (?, ?, ?, ?, ?)`, oid, pid, ev, taskID, p); err != nil {
					return logEntry{}, err
				}
			}
			for _, g := range groups {
				if _, err := tx.ExecContext(ctx, `INSERT INTO pool_member (org_id, plan_id, event_id, task_id, group_id) VALUES (?, ?, ?, ?, ?)`, oid, pid, ev, taskID, g); err != nil {
					return logEntry{}, err
				}
			}
		}
		sort.Strings(names)
		if len(names) == 0 {
			return logEntry{orgID: oid, planID: &pid, action: "pool.clear", detail: clip(fmt.Sprintf("%s: %s", taskName, datesText(dates)), 200)}, nil
		}
		return logEntry{orgID: oid, planID: &pid, action: "pool.set", detail: clip(fmt.Sprintf("%s: %s, %s", taskName, strings.Join(names, ", "), datesText(dates)), 200)}, nil
	})
}
