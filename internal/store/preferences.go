package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Levels a person can give a task (not marked at all = not involved) and an event (not marked = neutral).
var (
	taskLevels  = map[string]bool{"never": true, "poor": true, "neutral": true, "good": true}
	eventLevels = map[string]bool{"never": true, "poor": true, "good": true}
)

const maxAvailabilityPerCall = 400

type MeTask struct {
	TaskID int64  `json:"task_id"`
	Name   string `json:"name"`
	Level  string `json:"level"` // "" = the person has not chosen this task
}

type MeEvent struct {
	EventID int64  `json:"event_id"`
	Date    string `json:"date"`
	DayType string `json:"day_type"`
	Note    string `json:"note"`
	Level   string `json:"level"` // "" = neutral
}

// Me is everything a person's own page needs in one call.
type Me struct {
	Person Person    `json:"person"`
	Tasks  []MeTask  `json:"tasks"` // the planner's tasks only: manual-only tasks are never assigned by the planner
	Events []MeEvent `json:"events"`
}

func getPerson(ctx context.Context, q dbq, oid, id int64) (Person, error) {
	var p Person
	err := q.QueryRowContext(ctx, `SELECT id, name, active, proposable FROM person WHERE id = ? AND org_id = ?`, id, oid).
		Scan(&p.ID, &p.Name, &p.Active, &p.Proposable)
	if errors.Is(err, sql.ErrNoRows) {
		return p, ErrNotFound
	}
	return p, err
}

// GetPerson returns one person of the organisation (anyone else's is not found).
func (s *Store) GetPerson(ctx context.Context, orgSlug string, id int64) (Person, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return Person{}, err
	}
	return getPerson(ctx, s.db, oid, id)
}

func (s *Store) GetMe(ctx context.Context, orgSlug, planSlug string, personID int64) (Me, error) {
	var me Me
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return me, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return me, err
	}
	if me.Person, err = getPerson(ctx, s.db, oid, personID); err != nil {
		return me, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT t.id, t.name, COALESCE(pt.level, '')
		FROM task t LEFT JOIN person_task pt ON pt.task_id = t.id AND pt.org_id = t.org_id AND pt.person_id = ?
		WHERE t.org_id = ? AND t.planned = 1 ORDER BY t.name COLLATE NOCASE`, personID, oid)
	if err != nil {
		return me, err
	}
	me.Tasks = []MeTask{}
	for rows.Next() {
		var t MeTask
		if err := rows.Scan(&t.TaskID, &t.Name, &t.Level); err != nil {
			rows.Close()
			return me, err
		}
		me.Tasks = append(me.Tasks, t)
	}
	rows.Close()
	rows, err = s.db.QueryContext(ctx, `SELECT e.id, e.date, t.name, e.note, COALESCE(pe.level, '')
		FROM event e JOIN day_type t ON t.id = e.day_type_id AND t.org_id = e.org_id
		LEFT JOIN person_event pe ON pe.event_id = e.id AND pe.org_id = e.org_id AND pe.person_id = ?
		WHERE e.org_id = ? AND e.plan_id = ? ORDER BY e.date, e.id`, personID, oid, pid)
	if err != nil {
		return me, err
	}
	defer rows.Close()
	me.Events = []MeEvent{}
	for rows.Next() {
		var e MeEvent
		if err := rows.Scan(&e.EventID, &e.Date, &e.DayType, &e.Note, &e.Level); err != nil {
			return me, err
		}
		me.Events = append(me.Events, e)
	}
	return me, rows.Err()
}

// SetPersonTask makes the person take part in a task (or changes how much they want it).
func (s *Store) SetPersonTask(ctx context.Context, orgSlug, actor string, personID, taskID int64, level string) error {
	if !taskLevels[level] {
		return fmt.Errorf("%w: level must be ikke, poor, neutral or good", ErrInvalid)
	}
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		p, err := getPerson(ctx, tx, oid, personID)
		if err != nil {
			return logEntry{}, err
		}
		var name, before string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM task WHERE id = ? AND org_id = ? AND planned = 1`, taskID, oid).Scan(&name); errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		} else if err != nil {
			return logEntry{}, err
		}
		err = tx.QueryRowContext(ctx, `SELECT level FROM person_task WHERE person_id = ? AND task_id = ? AND org_id = ?`, personID, taskID, oid).Scan(&before)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, err
		}
		if before == level {
			return logEntry{}, fmt.Errorf("%w: nothing to change", ErrInvalid)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO person_task (org_id, person_id, task_id, level) VALUES (?, ?, ?, ?)
			ON CONFLICT (person_id, task_id) DO UPDATE SET level = excluded.level`, oid, personID, taskID, level); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "pref.task", detail: fmt.Sprintf("%s: %s %s -> %s", p.Name, name, orDash(before), level)}, nil
	})
}

// RemovePersonTask takes the task off the person's list again.
func (s *Store) RemovePersonTask(ctx context.Context, orgSlug, actor string, personID, taskID int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		p, err := getPerson(ctx, tx, oid, personID)
		if err != nil {
			return logEntry{}, err
		}
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT t.name FROM person_task pt JOIN task t ON t.id = pt.task_id AND t.org_id = pt.org_id
			WHERE pt.person_id = ? AND pt.task_id = ? AND pt.org_id = ?`, personID, taskID, oid).Scan(&name); errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		} else if err != nil {
			return logEntry{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM person_task WHERE person_id = ? AND task_id = ? AND org_id = ?`, personID, taskID, oid); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "pref.task", detail: fmt.Sprintf("%s: %s fjernet", p.Name, name)}, nil
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
func (s *Store) SetAvailability(ctx context.Context, orgSlug, planSlug, actor string, personID int64, levels map[int64]string) error {
	if len(levels) == 0 || len(levels) > maxAvailabilityPerCall {
		return fmt.Errorf("%w: give 1-%d events", ErrInvalid, maxAvailabilityPerCall)
	}
	for _, l := range levels {
		if l != "" && !eventLevels[l] {
			return fmt.Errorf("%w: level must be ikke, poor, good or empty", ErrInvalid)
		}
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
		p, err := getPerson(ctx, tx, oid, personID)
		if err != nil {
			return logEntry{}, err
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
				return logEntry{}, ErrNotFound
			} else if err != nil {
				return logEntry{}, err
			}
			var before string
			err := tx.QueryRowContext(ctx, `SELECT level FROM person_event WHERE person_id = ? AND event_id = ? AND org_id = ?`, personID, id, oid).Scan(&before)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return logEntry{}, err
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
				return logEntry{}, err
			}
			changed++
			counts[orDash(want)]++
		}
		if changed == 0 {
			return logEntry{}, fmt.Errorf("%w: nothing to change", ErrInvalid)
		}
		var parts []string
		for _, l := range []string{"never", "poor", "good", "–"} {
			if counts[l] > 0 {
				name := map[string]string{"never": "passer ikke", "poor": "dårlig", "good": "good", "–": "fjernet"}[l]
				parts = append(parts, strconv.Itoa(counts[l])+" "+name)
			}
		}
		return logEntry{orgID: oid, planID: &pid, action: "pref.availability", detail: fmt.Sprintf("%s: %s", p.Name, strings.Join(parts, ", "))}, nil
	})
}
