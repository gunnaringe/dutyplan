package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
)

// maxInPool is how many people a selection can name.
const maxInPool = 400

// ManualCell is a fixed place: a person or a text for one task on one event. The planner never moves it, and it
// counts as a filled place (Fills places for a text). Locked tells how it came about: false = set by hand,
// true = one of the planner's names that was locked so the next run keeps it. The planner treats both the same.
type ManualCell struct {
	ID       int64  `json:"id"`
	EventID  int64  `json:"event_id"`
	TaskID   int64  `json:"task_id"`
	PersonID *int64 `json:"person_id,omitempty"`
	Label    string `json:"label,omitempty"`
	Fills    int    `json:"fills"`
	Locked   bool   `json:"locked"`
}

// NewCell is a request for a manual cell on one event. Exactly one of PersonID, Label.
type NewCell struct {
	EventID  int64  `json:"event"`
	TaskID   int64  `json:"task"`
	PersonID int64  `json:"person"`
	Label    string `json:"label"`
	Fills    int    `json:"fills"`  // places a text fills (default 1); a person always fills one
	Locked   bool   `json:"locked"` // a planner's name that is locked (a person, never a text)
}

// Pool is a selection: only these people can be chosen for the task on the event.
type Pool struct {
	EventID int64   `json:"event_id"`
	TaskID  int64   `json:"task_id"`
	People  []int64 `json:"people"`
}

// distinct drops repeated ids and keeps the order; it refuses a list that is too long.
func distinct(ids []int64) ([]int64, error) {
	if len(ids) > maxInPool {
		return nil, Invalid("pool_too_large", map[string]any{"max": maxInPool}, "at most %d people in a selection", maxInPool)
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

func listCells(ctx context.Context, q dbq, oid, pid int64) ([]ManualCell, error) {
	out := []ManualCell{}
	err := scanAll(ctx, q, `SELECT c.id, c.event_id, c.task_id, c.person_id, c.label, c.fills, c.locked
		FROM manual_cell c JOIN event e ON e.id = c.event_id AND e.org_id = c.org_id
		WHERE c.org_id = ? AND c.plan_id = ? ORDER BY e.date, e.id, c.task_id, c.id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var c ManualCell
			var person sql.NullInt64
			var label sql.NullString
			if err := r.Scan(&c.ID, &c.EventID, &c.TaskID, &person, &label, &c.Fills, &c.Locked); err != nil {
				return err
			}
			if person.Valid {
				c.PersonID = &person.Int64
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

// AddCell writes a manual cell on one event of the plan. The same cell twice is a conflict.
func (s *Store) AddCell(ctx context.Context, orgSlug, planSlug string, in NewCell) (ManualCell, error) {
	var out ManualCell
	kinds := 0
	if in.PersonID != 0 {
		kinds++
	}
	label := strings.TrimSpace(in.Label)
	if label != "" {
		kinds++
		var err error
		if label, err = cleanName(label, "text"); err != nil {
			return out, err
		}
	}
	if kinds != 1 {
		return out, Invalid("cell_kind", nil, "give exactly one of person or text")
	}
	if in.Locked && label != "" {
		return out, Invalid("lock_text", nil, "only a person can be locked; a text is set by hand")
	}
	fills := in.Fills
	if fills == 0 {
		fills = 1
	}
	if fills < 1 || fills > 50 || (in.PersonID != 0 && fills != 1) {
		return out, Invalid("cell_fills", nil, "places must be 1-50 (a person fills one)")
	}
	err := s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var taskName, what string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND plan_id = ?`, in.TaskID, pid).Scan(&taskName); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		var person any
		switch {
		case in.PersonID != 0:
			if err := tx.QueryRowContext(ctx, `SELECT name FROM person WHERE id = ? AND plan_id = ?`, in.PersonID, pid).Scan(&what); errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			} else if err != nil {
				return err
			}
			person = in.PersonID
		default:
			what = label
		}
		if _, err := getEvent(ctx, tx, oid, pid, in.EventID); err != nil {
			return err
		}
		var text any
		if label != "" {
			text = label
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO manual_cell (org_id, plan_id, event_id, task_id, person_id, label, fills, locked, created_at)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, oid, pid, in.EventID, in.TaskID, person, text, fills, in.Locked, s.stamp())
		if isUnique(err) {
			return Conflict("cell_exists", nil, "that manual cell already exists")
		}
		if err != nil {
			return err
		}
		out = ManualCell{EventID: in.EventID, TaskID: in.TaskID, Label: label, Fills: fills, Locked: in.Locked}
		out.ID, _ = res.LastInsertId()
		if in.PersonID != 0 {
			out.PersonID = &in.PersonID
		}
		return nil
	})
	return out, err
}

// DeleteCell removes one manual cell of this plan. The planner may then choose for the place again.
func (s *Store) DeleteCell(ctx context.Context, orgSlug, planSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var date, task string
		var who sql.NullString
		err = tx.QueryRowContext(ctx, `SELECT e.date, t.name, COALESCE(p.name, c.label)
			FROM manual_cell c JOIN event e ON e.id = c.event_id AND e.org_id = c.org_id
			JOIN task t ON t.id = c.task_id AND t.org_id = c.org_id
			LEFT JOIN person p ON p.id = c.person_id AND p.org_id = c.org_id
			WHERE c.id = ? AND c.org_id = ? AND c.plan_id = ?`, id, oid, pid).Scan(&date, &task, &who)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM manual_cell WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid); err != nil {
			return err
		}
		return nil
	})
}

func listPools(ctx context.Context, q dbq, oid, pid int64) ([]Pool, error) {
	out := []Pool{}
	idx := map[[2]int64]int{}
	err := scanAll(ctx, q, `SELECT m.event_id, m.task_id, m.person_id
		FROM pool_member m JOIN event e ON e.id = m.event_id AND e.org_id = m.org_id
		WHERE m.org_id = ? AND m.plan_id = ? ORDER BY e.date, e.id, m.task_id, m.id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var ev, task int64
			var person int64
			if err := r.Scan(&ev, &task, &person); err != nil {
				return err
			}
			k := [2]int64{ev, task}
			i, ok := idx[k]
			if !ok {
				i = len(out)
				idx[k] = i
				out = append(out, Pool{EventID: ev, TaskID: task, People: []int64{}})
			}
			out[i].People = append(out[i].People, person)
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

// SetPool replaces the selection for one task on one event; no people removes it. Only a task the planner distributes
// can have one.
func (s *Store) SetPool(ctx context.Context, orgSlug, planSlug string, eventID, taskID int64, people []int64) error {
	people, err := distinct(people)
	if err != nil {
		return err
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var taskName string
		var planned bool
		if err := tx.QueryRowContext(ctx, `SELECT name, planned FROM task WHERE id = ? AND plan_id = ?`, taskID, pid).Scan(&taskName, &planned); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if !planned {
			return Invalid("task_not_distributed", nil, "the planner does not distribute %q, so there is nothing to select for", taskName)
		}
		for _, p := range people {
			var n string
			if err := tx.QueryRowContext(ctx, `SELECT name FROM person WHERE id = ? AND plan_id = ?`, p, pid).Scan(&n); errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			} else if err != nil {
				return err
			}
		}
		if _, err := getEvent(ctx, tx, oid, pid, eventID); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM pool_member WHERE org_id = ? AND plan_id = ? AND event_id = ? AND task_id = ?`, oid, pid, eventID, taskID); err != nil {
			return err
		}
		for _, p := range people {
			if _, err := tx.ExecContext(ctx, `INSERT INTO pool_member (org_id, plan_id, event_id, task_id, person_id) VALUES (?, ?, ?, ?, ?)`, oid, pid, eventID, taskID, p); err != nil {
				return err
			}
		}
		return nil
	})
}
