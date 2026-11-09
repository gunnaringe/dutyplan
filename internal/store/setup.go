package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// ---- helpers shared by the setup and date code ---------------------------------------------------------------

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
	case "task", "day_type", "person", "grp":
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

func (s *Store) CreateTask(ctx context.Context, orgSlug, actor, name string, planned bool) (Task, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Task{}, err
	}
	t := Task{Name: name, Planned: planned}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO task (org_id, name, planned, created_at) VALUES (?, ?, ?, ?)`, oid, t.Name, t.Planned, s.stamp())
		if isUnique(err) {
			return logEntry{}, exists("task", t.Name)
		}
		if err != nil {
			return logEntry{}, err
		}
		t.ID, _ = res.LastInsertId()
		return logEntry{orgID: oid, action: "task.create", detail: t.Name}, nil
	})
	return t, err
}

// CreateTasks adds many tasks in one go (pasted from a sheet, one name per line). Names that already exist are
// skipped and reported; blank lines are ignored.
func (s *Store) CreateTasks(ctx context.Context, orgSlug, actor, lines string, planned bool) (created []Task, skipped []string, err error) {
	var names []string
	seen := map[string]bool{}
	for _, l := range strings.Split(lines, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		n, err := cleanName(l, "name")
		if err != nil {
			return nil, nil, err
		}
		if !seen[strings.ToLower(n)] {
			seen[strings.ToLower(n)] = true
			names = append(names, n)
		}
	}
	if len(names) == 0 || len(names) > 200 {
		return nil, nil, Invalid("tasks_bulk_count", nil, "give 1-200 task names, one per line")
	}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		created, skipped = nil, nil
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		for _, n := range names {
			res, err := tx.ExecContext(ctx, `INSERT INTO task (org_id, name, planned, created_at) VALUES (?, ?, ?, ?) ON CONFLICT (org_id, name) DO NOTHING`, oid, n, planned, s.stamp())
			if err != nil {
				return logEntry{}, err
			}
			if k, _ := res.RowsAffected(); k == 0 {
				skipped = append(skipped, n)
				continue
			}
			id, _ := res.LastInsertId()
			created = append(created, Task{ID: id, Name: n, Planned: planned})
		}
		return logEntry{orgID: oid, action: "task.create", detail: fmt.Sprintf("%d new tasks, %d already existed", len(created), len(skipped))}, nil
	})
	return created, skipped, err
}

func (s *Store) ListTasks(ctx context.Context, orgSlug string) ([]Task, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, planned FROM task WHERE org_id = ? ORDER BY name COLLATE NOCASE`, oid)
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

func (s *Store) UpdateTask(ctx context.Context, orgSlug, actor string, id int64, patch TaskPatch) (Task, error) {
	var t Task
	if patch.Name != nil {
		n, err := cleanName(*patch.Name, "name")
		if err != nil {
			return t, err
		}
		patch.Name = &n
	}
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		err = tx.QueryRowContext(ctx, `SELECT id, name, planned FROM task WHERE id = ? AND org_id = ?`, id, oid).Scan(&t.ID, &t.Name, &t.Planned)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
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
			return logEntry{}, errNothingToChange()
		}
		_, err = tx.ExecContext(ctx, `UPDATE task SET name = ?, planned = ? WHERE id = ? AND org_id = ?`, t.Name, t.Planned, id, oid)
		if isUnique(err) {
			return logEntry{}, exists("task", t.Name)
		}
		if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "task.update", detail: fmt.Sprintf("#%d: %s", id, strings.Join(changes, "; "))}, nil
	})
	return t, err
}

// DeleteTask refuses (ErrConflict) while a day type still uses the task. Its combination settings go with it.
func (s *Store) DeleteTask(ctx context.Context, orgSlug, actor string, id int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		var name string
		err = tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND org_id = ?`, id, oid).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM task WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {
			return logEntry{}, inUse("task", name, "a day type, a group or a manual cell")
		} else if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "task.delete", detail: name}, nil
	})
}

// ---- day types -----------------------------------------------------------------------------------------------

type DayTypeTask struct {
	TaskID int64 `json:"task_id"`
	Slots  int   `json:"slots"`
}

type DayType struct {
	ID    int64         `json:"id"`
	Name  string        `json:"name"`
	Tasks []DayTypeTask `json:"tasks"`
}

func (s *Store) CreateDayType(ctx context.Context, orgSlug, actor, name string) (DayType, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return DayType{}, err
	}
	d := DayType{Name: name, Tasks: []DayTypeTask{}}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO day_type (org_id, name, created_at) VALUES (?, ?, ?)`, oid, d.Name, s.stamp())
		if isUnique(err) {
			return logEntry{}, exists("day type", d.Name)
		}
		if err != nil {
			return logEntry{}, err
		}
		d.ID, _ = res.LastInsertId()
		return logEntry{orgID: oid, action: "daytype.create", detail: d.Name}, nil
	})
	return d, err
}

func (s *Store) ListDayTypes(ctx context.Context, orgSlug string) ([]DayType, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	return listDayTypes(ctx, s.db, oid)
}

func listDayTypes(ctx context.Context, q dbq, oid int64) ([]DayType, error) {
	rows, err := q.QueryContext(ctx, `SELECT id, name FROM day_type WHERE org_id = ? ORDER BY name COLLATE NOCASE`, oid)
	if err != nil {
		return nil, err
	}
	out := []DayType{}
	idx := map[int64]int{}
	for rows.Next() {
		var d DayType
		if err := rows.Scan(&d.ID, &d.Name); err != nil {
			rows.Close()
			return nil, err
		}
		d.Tasks = []DayTypeTask{}
		idx[d.ID] = len(out)
		out = append(out, d)
	}
	rows.Close()
	rows, err = q.QueryContext(ctx, `SELECT day_type_id, task_id, slots FROM day_type_task WHERE org_id = ? ORDER BY task_id`, oid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var did int64
		var t DayTypeTask
		if err := rows.Scan(&did, &t.TaskID, &t.Slots); err != nil {
			return nil, err
		}
		if i, ok := idx[did]; ok { // rows of other organisations are never attached to ours
			out[i].Tasks = append(out[i].Tasks, t)
		}
	}
	return out, rows.Err()
}

func (s *Store) RenameDayType(ctx context.Context, orgSlug, actor string, id int64, name string) (DayType, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return DayType{}, err
	}
	var d DayType
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		var old string
		err = tx.QueryRowContext(ctx, `SELECT name FROM day_type WHERE id = ? AND org_id = ?`, id, oid).Scan(&old)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
		}
		if old == name {
			return logEntry{}, errNothingToChange()
		}
		if _, err = tx.ExecContext(ctx, `UPDATE day_type SET name = ? WHERE id = ? AND org_id = ?`, name, id, oid); isUnique(err) {
			return logEntry{}, exists("day type", name)
		} else if err != nil {
			return logEntry{}, err
		}
		all, err := listDayTypes(ctx, tx, oid)
		if err != nil {
			return logEntry{}, err
		}
		for _, x := range all {
			if x.ID == id {
				d = x
			}
		}
		return logEntry{orgID: oid, action: "daytype.rename", detail: fmt.Sprintf("%q -> %q", old, name)}, nil
	})
	return d, err
}

// SetDayTypeTasks replaces the day type's task list. Slots are 1-50; each task at most once; tasks must be the
// organisation's own.
func (s *Store) SetDayTypeTasks(ctx context.Context, orgSlug, actor string, id int64, tasks []DayTypeTask) (DayType, error) {
	seen := map[int64]bool{}
	for _, t := range tasks {
		if t.Slots < 1 || t.Slots > 50 {
			return DayType{}, Invalid("slots_range", nil, "slots must be 1-50")
		}
		if seen[t.TaskID] {
			return DayType{}, Invalid("task_twice", nil, "a task can be listed once")
		}
		seen[t.TaskID] = true
	}
	var d DayType
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		if err := inOrg(ctx, tx, "day_type", oid, id); err != nil {
			return logEntry{}, err
		}
		for _, t := range tasks {
			if err := inOrg(ctx, tx, "task", oid, t.TaskID); err != nil {
				return logEntry{}, err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM day_type_task WHERE day_type_id = ? AND org_id = ?`, id, oid); err != nil {
			return logEntry{}, err
		}
		for _, t := range tasks {
			if _, err := tx.ExecContext(ctx, `INSERT INTO day_type_task (org_id, day_type_id, task_id, slots) VALUES (?, ?, ?, ?)`, oid, id, t.TaskID, t.Slots); err != nil {
				return logEntry{}, err
			}
		}
		all, err := listDayTypes(ctx, tx, oid)
		if err != nil {
			return logEntry{}, err
		}
		for _, x := range all {
			if x.ID == id {
				d = x
			}
		}
		return logEntry{orgID: oid, action: "daytype.tasks", detail: fmt.Sprintf("%s: %d tasks", d.Name, len(tasks))}, nil
	})
	return d, err
}

// DeleteDayType refuses (ErrConflict) while any plan still has dates of this type.
func (s *Store) DeleteDayType(ctx context.Context, orgSlug, actor string, id int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		var name string
		err = tx.QueryRowContext(ctx, `SELECT name FROM day_type WHERE id = ? AND org_id = ?`, id, oid).Scan(&name)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM day_type WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {
			return logEntry{}, inUse("day type", name, "events in a plan")
		} else if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "daytype.delete", detail: name}, nil
	})
}

// ---- task combinations ---------------------------------------------------------------------------------------

// Combination levels, as the planner knows them. "standard" (no row) means: use the planner's default.
var combinationLevels = map[string]bool{"cannot": true, "impractical": true, "practical": true, "preferred": true}

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
func (s *Store) SetCombination(ctx context.Context, orgSlug, actor string, a, b int64, level string) error {
	if a == b {
		return Invalid("combination_self", nil, "a task cannot be combined with itself")
	}
	if level != "standard" && !combinationLevels[level] {
		return Invalid("level_invalid", nil, "level must be cannot, impractical, practical, preferred or standard")
	}
	if a > b {
		a, b = b, a
	}
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		for _, id := range []int64{a, b} {
			if err := inOrg(ctx, tx, "task", oid, id); err != nil {
				return logEntry{}, err
			}
		}
		if level == "standard" {
			_, err = tx.ExecContext(ctx, `DELETE FROM task_combination WHERE org_id = ? AND task_a = ? AND task_b = ?`, oid, a, b)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO task_combination (org_id, task_a, task_b, level) VALUES (?, ?, ?, ?)
				ON CONFLICT (task_a, task_b) DO UPDATE SET level = excluded.level`, oid, a, b, level)
		}
		if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "combination.set", detail: fmt.Sprintf("#%d + #%d: %s", a, b, level)}, nil
	})
}
