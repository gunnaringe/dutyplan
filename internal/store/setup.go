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

// inPlan reports whether a person or task with this id is in the plan. Every reference in a request body is checked
// with it (a reference into another plan or organisation is "not found"); the composite foreign keys are the backstop.
func inPlan(ctx context.Context, q dbq, table string, pid, id int64) error {
	switch table {
	case "task", "person":
	default:
		panic("inPlan: unknown table " + table)
	}
	var one int
	err := q.QueryRowContext(ctx, `SELECT 1 FROM `+table+` WHERE id = ? AND plan_id = ?`, id, pid).Scan(&one)
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

// scope finds the organisation and the plan, for everything that is per plan.
func scope(ctx context.Context, q dbq, orgSlug, planSlug string) (oid, pid int64, err error) {
	if oid, err = orgID(ctx, q, orgSlug); err != nil {
		return
	}
	pid, err = planID(ctx, q, oid, planSlug)
	return
}

// ---- tasks ---------------------------------------------------------------------------------------------------

type Task struct {
	ID      int64  `json:"id"`
	Name    string `json:"name"`
	Planned bool   `json:"planned"`
	Section int64  `json:"section"` // 0: no section (only placement in the table)
}

// CreateTask adds a task the planner distributes (switch that off with UpdateTask), last of those without a section.
func (s *Store) CreateTask(ctx context.Context, orgSlug, planSlug, name string) (Task, error) {
	return s.CreateTaskIn(ctx, orgSlug, planSlug, name, 0)
}

// CreateTaskIn adds a task last in a section (0: without a section).
func (s *Store) CreateTaskIn(ctx context.Context, orgSlug, planSlug, name string, section int64) (Task, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Task{}, err
	}
	t := Task{Name: name, Planned: true, Section: section}
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task WHERE plan_id = ?`, pid).Scan(&n); err != nil {
			return err
		}
		if n >= MaxTasksPerPlan {
			return Invalid("too_many_tasks", map[string]any{"max": MaxTasksPerPlan}, "at most %d tasks", MaxTasksPerPlan)
		}
		if section != 0 {
			if _, err := sectionName(ctx, tx, Tasks, pid, section); err != nil {
				return err
			}
		}
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM task WHERE plan_id = ? AND section_id IS ?`, pid, sectionScope(section)[0]).Scan(&n); err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO task (org_id, plan_id, name, planned, position, section_id, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, oid, pid, t.Name, t.Planned, n, sectionScope(section)[0], s.stamp())
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

// ListTasks gives the plan's tasks in their order (MoveTask): those without a section first, then each section's in the
// order of the sections.
func (s *Store) ListTasks(ctx context.Context, orgSlug, planSlug string) ([]Task, error) {
	_, pid, err := scope(ctx, s.db, orgSlug, planSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.name, t.planned, COALESCE(t.section_id, 0) FROM task t
		LEFT JOIN task_section x ON x.id = t.section_id AND x.plan_id = t.plan_id
		WHERE t.plan_id = ? ORDER BY COALESCE(x.position, -1), t.position, t.id`, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Task{}
	for rows.Next() {
		var t Task
		if err := rows.Scan(&t.ID, &t.Name, &t.Planned, &t.Section); err != nil {
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

func (s *Store) UpdateTask(ctx context.Context, orgSlug, planSlug string, id int64, patch TaskPatch) (Task, error) {
	var t Task
	if patch.Name != nil {
		n, err := cleanName(*patch.Name, "name")
		if err != nil {
			return t, err
		}
		patch.Name = &n
	}
	err := s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `SELECT id, name, planned, COALESCE(section_id, 0) FROM task WHERE id = ? AND plan_id = ?`, id, pid).Scan(&t.ID, &t.Name, &t.Planned, &t.Section)
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
		_, err = tx.ExecContext(ctx, `UPDATE task SET name = ?, planned = ? WHERE id = ? AND plan_id = ?`, t.Name, t.Planned, id, pid)
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
func (s *Store) DeleteTask(ctx context.Context, orgSlug, planSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		var name string
		var section sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT name, section_id FROM task WHERE id = ? AND plan_id = ?`, id, pid).Scan(&name, &section)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task WHERE id = ? AND plan_id = ?`, id, pid); isForeignKey(err) {
			return inUse("task", name, "a manual cell or a selection")
		} else if err != nil {
			return err
		}
		return renumber(ctx, tx, "task", pid, sectionScope(section.Int64), 0, 0)
	})
}

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

func (s *Store) ListCombinations(ctx context.Context, orgSlug, planSlug string) ([]Combination, error) {
	_, pid, err := scope(ctx, s.db, orgSlug, planSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT task_a, task_b, level FROM task_combination WHERE plan_id = ? ORDER BY task_a, task_b`, pid)
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
func (s *Store) SetCombination(ctx context.Context, orgSlug, planSlug string, a, b int64, level string) error {
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
		oid, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		for _, id := range []int64{a, b} {
			if err := inPlan(ctx, tx, "task", pid, id); err != nil {
				return err
			}
		}
		if level == "standard" {
			_, err = tx.ExecContext(ctx, `DELETE FROM task_combination WHERE plan_id = ? AND task_a = ? AND task_b = ?`, pid, a, b)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO task_combination (org_id, plan_id, task_a, task_b, level) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (task_a, task_b) DO UPDATE SET level = excluded.level`, oid, pid, a, b, level)
		}
		if err != nil {
			return err
		}
		return nil
	})
}
