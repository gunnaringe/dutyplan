package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
)

// Levels a person can give a task (not marked at all = does not take part; one click on the page goes not → poor → good)
// and an event (not marked = neutral).
var (
	taskLevels  = map[string]bool{"poor": true, "good": true}
	eventLevels = map[string]bool{"never": true, "poor": true, "good": true}
)

const maxAvailabilityPerCall = 400

func getPerson(ctx context.Context, q dbq, pid, id int64) (Person, error) {
	var p Person
	err := q.QueryRowContext(ctx, `SELECT id, name, proposable, COALESCE(section_id, 0) FROM person WHERE id = ? AND plan_id = ?`, id, pid).
		Scan(&p.ID, &p.Name, &p.Proposable, &p.Section)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// SetPersonTask sets whether and how willingly a person takes part in a task, in one plan (each plan has its own).
func (s *Store) SetPersonTask(ctx context.Context, orgSlug, planSlug string, personID, taskID int64, level string) error {
	if !taskLevels[level] {
		return fmt.Errorf("%w: level must be poor or good (remove the task to not take part)", ErrInvalid)
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
		if _, err := getPerson(ctx, tx, pid, personID); err != nil {
			return err
		}
		var name, before string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND plan_id = ? AND planned = 1`, taskID, pid).Scan(&name); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		err = tx.QueryRowContext(ctx, `SELECT level FROM person_task WHERE plan_id = ? AND person_id = ? AND task_id = ? AND org_id = ?`, pid, personID, taskID, oid).Scan(&before)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if before == level {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO person_task (org_id, plan_id, person_id, task_id, level) VALUES (?, ?, ?, ?, ?)
			ON CONFLICT (person_id, task_id) DO UPDATE SET level = excluded.level`, oid, pid, personID, taskID, level); err != nil {
			return err
		}
		return nil
	})
}

// RemovePersonTask takes the task off the person's list again, in one plan.
func (s *Store) RemovePersonTask(ctx context.Context, orgSlug, planSlug string, personID, taskID int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		if _, err := getPerson(ctx, tx, pid, personID); err != nil {
			return err
		}
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT t.name FROM person_task pt JOIN task t ON t.id = pt.task_id AND t.org_id = pt.org_id
			WHERE pt.plan_id = ? AND pt.person_id = ? AND pt.task_id = ? AND pt.org_id = ?`, pid, personID, taskID, oid).Scan(&name); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM person_task WHERE plan_id = ? AND person_id = ? AND task_id = ? AND org_id = ?`, pid, personID, taskID, oid); err != nil {
			return err
		}
		return nil
	})
}

func orDash(s string) string {
	if s == "" {
		return "–"
	}
	return s
}

// SetAvailability sets the person's answer for several events of one plan at once (so a page can paint many). A level
// of "" means neutral and removes the answer. All or nothing; every event must belong to this plan.
func (s *Store) SetAvailability(ctx context.Context, orgSlug, planSlug string, personID int64, levels map[int64]string) error {
	if len(levels) == 0 || len(levels) > maxAvailabilityPerCall {
		return Invalid("levels_count", map[string]any{"max": maxAvailabilityPerCall}, "give 1-%d events", maxAvailabilityPerCall)
	}
	for _, l := range levels {
		if l != "" && !eventLevels[l] {
			return fmt.Errorf("%w: level must be never, poor, good or empty", ErrInvalid)
		}
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
		if _, err := getPerson(ctx, tx, pid, personID); err != nil {
			return err
		}
		ids := make([]int64, 0, len(levels))
		for id := range levels {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		counts := map[string]int{}
		changed := 0
		for _, id := range ids {
			var one int
			if err := tx.QueryRowContext(ctx, `SELECT 1 FROM event WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid).Scan(&one); errors.Is(err, sql.ErrNoRows) {
				return ErrNotFound
			} else if err != nil {
				return err
			}
			var before string
			err := tx.QueryRowContext(ctx, `SELECT level FROM person_event WHERE person_id = ? AND event_id = ? AND org_id = ?`, personID, id, oid).Scan(&before)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			want := levels[id]
			if before == want {
				continue
			}
			if want == "" {
				_, err = tx.ExecContext(ctx, `DELETE FROM person_event WHERE person_id = ? AND event_id = ? AND org_id = ?`, personID, id, oid)
			} else {
				_, err = tx.ExecContext(ctx, `INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES (?, ?, ?, ?, ?)
					ON CONFLICT (person_id, event_id) DO UPDATE SET level = excluded.level`, oid, pid, personID, id, want)
			}
			if err != nil {
				return err
			}
			changed++
			counts[orDash(want)]++
		}
		if changed == 0 {
			return errNothingToChange()
		}
		var parts []string
		for _, l := range []string{"never", "poor", "good", "–"} {
			if counts[l] > 0 {
				name := map[string]string{"never": "never", "poor": "poor", "good": "good", "–": "removed"}[l]
				parts = append(parts, strconv.Itoa(counts[l])+" "+name)
			}
		}
		return nil
	})
}
