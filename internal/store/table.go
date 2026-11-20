package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

// What the table on the plan page needs beyond events and people: places per event, the plan's details rows with a value
// per event, everyone's answers in one call, and how one task combines with all the others.

const (
	maxSlots          = 50
	maxFieldsPerPlan  = 20
	maxFieldValueRune = 200
)

// EventPlaces is how many places one task has on one event. A task without a row is not needed on the event.
type EventPlaces struct {
	EventID int64 `json:"event_id"`
	TaskID  int64 `json:"task_id"`
	Slots   int   `json:"slots"`
}

func listEventPlaces(ctx context.Context, q dbq, oid, pid int64) ([]EventPlaces, error) {
	out := []EventPlaces{}
	err := scanAll(ctx, q, `SELECT event_id, task_id, slots FROM event_task WHERE org_id = ? AND plan_id = ? ORDER BY event_id, task_id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var x EventPlaces
			err := r.Scan(&x.EventID, &x.TaskID, &x.Slots)
			out = append(out, x)
			return err
		})
	return out, err
}

func (s *Store) ListEventPlaces(ctx context.Context, orgSlug, planSlug string) ([]EventPlaces, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	return listEventPlaces(ctx, s.db, oid, pid)
}

// SetEventPlaces sets the places for a task on one event; 0 means the task is not needed there.
func (s *Store) SetEventPlaces(ctx context.Context, orgSlug, planSlug string, eventID, taskID int64, slots int) error {
	if slots < 0 || slots > maxSlots {
		return Invalid("places_invalid", map[string]any{"max": maxSlots}, "places must be 0-%d", maxSlots)
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
		if _, err := getEvent(ctx, tx, oid, pid, eventID); err != nil {
			return err
		}
		if err := inOrg(ctx, tx, "task", oid, taskID); err != nil {
			return err
		}
		var before int
		err = tx.QueryRowContext(ctx, `SELECT slots FROM event_task WHERE event_id = ? AND task_id = ? AND org_id = ?`, eventID, taskID, oid).Scan(&before)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if before == slots {
			return errNothingToChange()
		}
		return setPlaces(ctx, tx, oid, pid, eventID, taskID, slots)
	})
}

// setPlaces writes the places of a task on an event that is known to be in the plan; 0 removes the row.
func setPlaces(ctx context.Context, tx *sql.Tx, oid, pid, eventID, taskID int64, slots int) error {
	var err error
	if slots == 0 {
		_, err = tx.ExecContext(ctx, `DELETE FROM event_task WHERE event_id = ? AND task_id = ? AND org_id = ?`, eventID, taskID, oid)
	} else {
		_, err = tx.ExecContext(ctx, `INSERT INTO event_task (org_id, plan_id, event_id, task_id, slots) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (event_id, task_id) DO UPDATE SET slots = excluded.slots`, oid, pid, eventID, taskID, slots)
	}
	return err
}

// PlanField is a details row of a plan (for example «Note» or «Theme»); FieldValue is its text for one event. Position
// orders the rows from the top (0, 1, ...).
type PlanField struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Position int    `json:"position"`
}

type FieldValue struct {
	EventID int64  `json:"event_id"`
	FieldID int64  `json:"field_id"`
	Value   string `json:"value"`
}

type PlanFields struct {
	Fields []PlanField  `json:"fields"`
	Values []FieldValue `json:"values"`
}

func (s *Store) ListFields(ctx context.Context, orgSlug, planSlug string) (PlanFields, error) {
	out := PlanFields{Fields: []PlanField{}, Values: []FieldValue{}}
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return out, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return out, err
	}
	if err := scanAll(ctx, s.db, `SELECT id, name, position FROM plan_field WHERE org_id = ? AND plan_id = ? ORDER BY position, id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var f PlanField
			err := r.Scan(&f.ID, &f.Name, &f.Position)
			out.Fields = append(out.Fields, f)
			return err
		}); err != nil {
		return out, err
	}
	err = scanAll(ctx, s.db, `SELECT event_id, field_id, value FROM event_field WHERE org_id = ? AND plan_id = ? ORDER BY event_id, field_id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var v FieldValue
			err := r.Scan(&v.EventID, &v.FieldID, &v.Value)
			out.Values = append(out.Values, v)
			return err
		})
	return out, err
}

func getField(ctx context.Context, q dbq, oid, pid, id int64) (PlanField, error) {
	var f PlanField
	err := q.QueryRowContext(ctx, `SELECT id, name, position FROM plan_field WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid).Scan(&f.ID, &f.Name, &f.Position)
	if errors.Is(err, sql.ErrNoRows) {
		return f, ErrNotFound
	}
	return f, err
}

func (s *Store) AddField(ctx context.Context, orgSlug, planSlug, name string) (PlanField, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return PlanField{}, err
	}
	var f PlanField
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan_field WHERE org_id = ? AND plan_id = ?`, oid, pid).Scan(&n); err != nil {
			return err
		}
		if n >= maxFieldsPerPlan {
			return Invalid("too_many_fields", map[string]any{"max": maxFieldsPerPlan}, "a plan can have at most %d details rows", maxFieldsPerPlan)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO plan_field (org_id, plan_id, name, position, created_at) VALUES (?, ?, ?, ?, ?)`, oid, pid, name, n, s.stamp())
		if isUnique(err) {
			return exists("details row", name)
		} else if err != nil {
			return err
		}
		f.ID, _ = res.LastInsertId()
		f.Name, f.Position = name, n
		return nil
	})
	return f, err
}

func (s *Store) RenameField(ctx context.Context, orgSlug, planSlug string, id int64, name string) (PlanField, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return PlanField{}, err
	}
	var f PlanField
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		old, err := getField(ctx, tx, oid, pid, id)
		if err != nil {
			return err
		}
		if old.Name == name {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plan_field SET name = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, name, id, oid, pid); isUnique(err) {
			return exists("details row", name)
		} else if err != nil {
			return err
		}
		f = PlanField{ID: id, Name: name, Position: old.Position}
		return nil
	})
	return f, err
}

// MoveField puts a details row at a position (0 = the top; past the end = the bottom) and renumbers the others.
func (s *Store) MoveField(ctx context.Context, orgSlug, planSlug string, id int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
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
		if _, err := getField(ctx, tx, oid, pid, id); err != nil {
			return err
		}
		return renumberFields(ctx, tx, oid, pid, id, position)
	})
}

// renumberFields gives the plan's rows the positions 0, 1, ... in their order, with the row moved (if any) at position.
func renumberFields(ctx context.Context, tx *sql.Tx, oid, pid, moved int64, position int) error {
	var order []int64
	if err := scanAll(ctx, tx, `SELECT id FROM plan_field WHERE org_id = ? AND plan_id = ? AND id <> ? ORDER BY position, id`, []any{oid, pid, moved},
		func(r *sql.Rows) error {
			var id int64
			err := r.Scan(&id)
			order = append(order, id)
			return err
		}); err != nil {
		return err
	}
	if moved != 0 {
		position = min(position, len(order))
		order = append(order[:position], append([]int64{moved}, order[position:]...)...)
	}
	for i, id := range order {
		if _, err := tx.ExecContext(ctx, `UPDATE plan_field SET position = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, i, id, oid, pid); err != nil {
			return err
		}
	}
	return nil
}

// DeleteField removes a details row with its values on every event; the rows below move up.
func (s *Store) DeleteField(ctx context.Context, orgSlug, planSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		if _, err := getField(ctx, tx, oid, pid, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM plan_field WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid); err != nil {
			return err
		}
		return renumberFields(ctx, tx, oid, pid, 0, 0)
	})
}

// SetFieldValue sets the text of a details row for one event; an empty text removes it.
func (s *Store) SetFieldValue(ctx context.Context, orgSlug, planSlug string, eventID, fieldID int64, value string) error {
	value = strings.TrimSpace(value)
	if utf8.RuneCountInString(value) > maxFieldValueRune || !validText(value) {
		return Invalid("field_value_invalid", map[string]any{"max": maxFieldValueRune}, "a details value must be at most %d printable characters", maxFieldValueRune)
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
		if _, err := getEvent(ctx, tx, oid, pid, eventID); err != nil {
			return err
		}
		if _, err := getField(ctx, tx, oid, pid, fieldID); err != nil {
			return err
		}
		var before string
		err = tx.QueryRowContext(ctx, `SELECT value FROM event_field WHERE event_id = ? AND field_id = ? AND org_id = ?`, eventID, fieldID, oid).Scan(&before)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if before == value {
			return errNothingToChange()
		}
		if value == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM event_field WHERE event_id = ? AND field_id = ? AND org_id = ?`, eventID, fieldID, oid)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO event_field (org_id, plan_id, event_id, field_id, value) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (event_id, field_id) DO UPDATE SET value = excluded.value`, oid, pid, eventID, fieldID, value)
		}
		if err != nil {
			return err
		}
		return nil
	})
}

// Answers is what everyone has said, for the table: per event of the plan («can I») and per task («do I take part»).
type Answers struct {
	Events []EventAnswer `json:"events"`
	Tasks  []TaskAnswer  `json:"tasks"`
}

type EventAnswer struct {
	PersonID int64  `json:"person_id"`
	EventID  int64  `json:"event_id"`
	Level    string `json:"level"` // never | poor | good (no row = neutral)
}

type TaskAnswer struct {
	PersonID int64  `json:"person_id"`
	TaskID   int64  `json:"task_id"`
	Level    string `json:"level"` // poor | good (no row = does not take part)
}

func (s *Store) ListAnswers(ctx context.Context, orgSlug, planSlug string) (Answers, error) {
	out := Answers{Events: []EventAnswer{}, Tasks: []TaskAnswer{}}
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return out, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return out, err
	}
	if err := scanAll(ctx, s.db, `SELECT person_id, event_id, level FROM person_event WHERE org_id = ? AND plan_id = ? ORDER BY person_id, event_id`, []any{oid, pid},
		func(r *sql.Rows) error {
			var a EventAnswer
			err := r.Scan(&a.PersonID, &a.EventID, &a.Level)
			out.Events = append(out.Events, a)
			return err
		}); err != nil {
		return out, err
	}
	err = scanAll(ctx, s.db, `SELECT person_id, task_id, level FROM person_task WHERE org_id = ? ORDER BY person_id, task_id`, []any{oid},
		func(r *sql.Rows) error {
			var a TaskAnswer
			err := r.Scan(&a.PersonID, &a.TaskID, &a.Level)
			out.Tasks = append(out.Tasks, a)
			return err
		})
	return out, err
}

// SetTaskCombinations sets how one task combines with every other task of the organisation at once, for example «cannot be
// combined with anything». "standard" removes the settings, so the pairs are «bad» (impractical) again.
func (s *Store) SetTaskCombinations(ctx context.Context, orgSlug string, taskID int64, level string) error {
	if level != "standard" && !combinationLevels[level] {
		return Invalid("level_invalid", nil, "level must be cannot, practical or standard")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND org_id = ?`, taskID, oid).Scan(&name); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		var others []int64
		if err := scanAll(ctx, tx, `SELECT id FROM task WHERE org_id = ? AND id <> ? ORDER BY id`, []any{oid, taskID}, func(r *sql.Rows) error {
			var id int64
			err := r.Scan(&id)
			others = append(others, id)
			return err
		}); err != nil {
			return err
		}
		changed := 0
		for _, other := range others {
			a, b := min(taskID, other), max(taskID, other)
			before := "standard"
			if err := tx.QueryRowContext(ctx, `SELECT level FROM task_combination WHERE org_id = ? AND task_a = ? AND task_b = ?`, oid, a, b).Scan(&before); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if before == level {
				continue
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
			changed++
		}
		if changed == 0 {
			return errNothingToChange()
		}
		return nil
	})
}
