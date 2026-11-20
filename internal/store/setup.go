package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ---- helpers shared by the setup and event code ---------------------------------------------------------------

type dbq interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func isForeignKey(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOREIGN KEY constraint failed")
}

// inOrg reports whether a row with this id exists in the organisation. Every reference in a request body is checked
// with it (a reference into another organisation is "not found"); the composite foreign keys are the backstop.
func inOrg(ctx context.Context, q dbq, table string, oid, id int64) error {
	switch table {
	case "task", "person":
	default:
		panic("inOrg: unknown table " + table)
	}
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM `+table+` WHERE id = ? AND org_id = ?`, id, oid).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func planID(ctx context.Context, q dbq, oid int64, slug string) (int64, error) {
	var id int64
	err := q.QueryRowContext(ctx, `SELECT id FROM plan WHERE org_id = ? AND slug = ?`, oid, slug).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, ErrNotFound
	}
	return id, err
}

// ---- tasks ---------------------------------------------------------------------------------------------------

type Task struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Planned bool   `json:"planned"`
}

// CreateTask adds a task the planner distributes (switch that off with UpdateTask).
func (s *Store) CreateTask(ctx context.Context, orgSlug, name string) (Task, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Task{}, err
	}
	t := Task{Name: name, Planned: true}
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task WHERE org_id = ?`, oid).Scan(&n); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO task (org_id, name, planned, position, created_at) VALUES (?, ?, ?, ?, ?)`, oid, t.Name, t.Planned, n, s.stamp())
		if isUnique(err) {
			return exists("task", t.Name)
		}
		if err != nil {
			return err
		}
		t.ID, _ = res.LastInsertId()
		return nil
	})
	return t, err
}

// ListTasks gives the organisation's tasks in their order (MoveTask), the same in every plan's table.
func (s *Store) ListTasks(ctx context.Context, orgSlug string) ([]Task, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, planned FROM task WHERE org_id = ? ORDER BY position, id`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Name, &t.Planned); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

type TaskPatch struct {
	Name    *string `json:"name"`
	Planned *bool   `json:"planned"`
}

func (s *Store) UpdateTask(ctx context.Context, orgSlug string, id int64, patch TaskPatch) (Task, error) {
	var t Task
	if patch.Name != nil {
		n, err := cleanName(*patch.Name, "name")
		if err != nil {
			return t, err
		}
		patch.Name = &n
	}
	err := s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `SELECT id, name, planned FROM task WHERE id = ? AND org_id = ?`, id, oid).Scan(&t.ID, &t.Name, &t.Planned)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		var changes []string
		if patch.Name != nil && *patch.Name != t.Name {
			changes = append(changes, fmt.Sprintf("name %q -> %q", t.Name, *patch.Name))
			t.Name = *patch.Name
		}
		if patch.Planned != nil && *patch.Planned != t.Planned {
			changes = append(changes, fmt.Sprintf("planned %t -> %t", t.Planned, *patch.Planned))
			t.Planned = *patch.Planned
		}
		if len(changes) == 0 {
			return errNothingToChange()
		}
		_, err = tx.ExecContext(ctx, `UPDATE task SET name = ?, planned = ? WHERE id = ? AND org_id = ?`, t.Name, t.Planned, id, oid)
		if isUnique(err) {
			return exists("task", t.Name)
		}
		if err != nil {
			return err
		}
		return nil
	})
	return t, err
}

// DeleteTask refuses (ErrConflict) while a manual cell or a selection uses the task. Its places on events, the
// people's answers about it and its combination settings go with it.
func (s *Store) DeleteTask(ctx context.Context, orgSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		var name string
		err = tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND org_id = ?`, id, oid).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {
			return inUse("task", name, "a manual cell or a selection")
		} else if err != nil {
			return err
		}
		return renumberTasks(ctx, tx, oid, 0, 0)
	})
}

// MoveTask puts a task at a position in the table (0 = the top; past the end = the bottom) and renumbers the others. The
// order is the organisation's, so it is the same in every plan; the planner does not see it.
func (s *Store) MoveTask(ctx context.Context, orgSlug string, id int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		if err := inOrg(ctx, tx, "task", oid, id); err != nil {
			return err
		}
		return renumberTasks(ctx, tx, oid, id, position)
	})
}

// renumberTasks gives the organisation's tasks the positions 0, 1, ... in their order, with the task moved (if any) at position.
func renumberTasks(ctx context.Context, tx *sql.Tx, oid, moved int64, position int) error {
	var order []int64
	if err := scanAll(ctx, tx, `SELECT id FROM task WHERE org_id = ? AND id <> ? ORDER BY position, id`, []any{oid, moved},
		func(r *sql.Rows) error {
			var id int64
			err := r.Scan(&id)
			order = append(order, id)
			return err
		}); err != nil {
		return err
	}
	if moved != 0 {
		order = insertAt(order, moved, position)
	}
	for i, id := range order {
		if _, err := tx.ExecContext(ctx, `UPDATE task SET position = ? WHERE id = ? AND org_id = ?`, i, id, oid); err != nil {
			return err
		}
	}
	return nil
}

// insertAt puts id into order at position (past the end = last).
func insertAt(order []int64, id int64, position int) []int64 {
	position = min(position, len(order))
	return append(order[:position], append([]int64{id}, order[position:]...)...)
}

// ---- task combinations ---------------------------------------------------------------------------------------

// Combination levels, as the planner knows them. "standard" (no row) means: use the planner's default.
// combinationLevels are what can be stored for two tasks on the same event: «can't» and «good». A pair that is not set
// («standard» removes it) is «bad»: the planner's default, impractical.
var combinationLevels = map[string]bool{"cannot": true, "practical": true}

type Combination struct {
	A     int64  `json:"a"` // always the smaller task id
	B     int64  `json:"b"`
	Level string `json:"level"`
}

func (s *Store) ListCombinations(ctx context.Context, orgSlug string) ([]Combination, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_a, task_b, level FROM task_combination WHERE org_id = ? ORDER BY task_a, task_b`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Combination{}
	for rows.Next() {
		var c Combination
		if err := rows.Scan(&c.A, &c.B, &c.Level); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// SetCombination stores the level for a pair of tasks (either order). Level "standard" removes the setting.
func (s *Store) SetCombination(ctx context.Context, orgSlug string, a, b int64, level string) error {
	if a == b {
		return Invalid("combination_self", nil, "a task cannot be combined with itself")
	}
	if level != "standard" && !combinationLevels[level] {
		return Invalid("level_invalid", nil, "level must be cannot, practical or standard")
	}
	if a > b {
		a, b = b, a
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		for _, id := range []int64{a, b} {
			if err := inOrg(ctx, tx, "task", oid, id); err != nil {
				return err
			}
		}
		if level == "standard" {
			_, err = tx.ExecContext(ctx, `DELETE FROM task_combination WHERE org_id = ? AND task_a = ? AND task_b = ?`, oid, a, b)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (?, ?, ?, ?)
				ON CONFLICT (task_a, task_b) DO UPDATE SET level = excluded.level`, oid, a, b, level)
		}
		if err != nil {
			return err
		}
		return nil
	})
}
