package store

import (
	"context"
	"database/sql"
	"errors"
)

// Writes to many cells of a plan's table at once: what a selection on the page sets (2026-11-22, the user: "drag and
// drop, and unselect selected cells + mark a whole Row. Kinda like you would in a spreedsheet"). All or nothing; every
// person and task must be the organisation's, every event this plan's (404 otherwise, as for one cell).

// MaxCellsPerCall is how many cells one bulk write may set (a selection of 250 people over 20 events fits).
const MaxCellsPerCall = 5000

// Answer is one person's answer for one event: never, poor, good, or "" (not answered).
type Answer struct {
	Person int64  `json:"person"`
	Event  int64  `json:"event"`
	Level  string `json:"level"`
}

// Places is how many places a task needs on one event (0: not needed).
type Places struct {
	Event int64 `json:"event"`
	Task  int64 `json:"task"`
	Slots int   `json:"slots"`
}

func cellsCount(n int) error {
	if n == 0 || n > MaxCellsPerCall {
		return Invalid("cells_count", map[string]any{"max": MaxCellsPerCall}, "give 1-%d cells", MaxCellsPerCall)
	}
	return nil
}

// known checks ids once each: a cache of what has been found in the organisation (and plan).
type known map[int64]bool

func (k known) check(id int64, find func() error) error {
	if k[id] {
		return nil
	}
	if err := find(); err != nil {
		return err
	}
	k[id] = true
	return nil
}

func eventInPlan(ctx context.Context, tx *sql.Tx, oid, pid, id int64) error {
	var one int
	err := tx.QueryRowContext(ctx, `SELECT 1 FROM event WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid).Scan(&one)
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

// SetAnswers sets many answers of a plan in one go.
func (s *Store) SetAnswers(ctx context.Context, orgSlug, planSlug string, answers []Answer) error {
	if err := cellsCount(len(answers)); err != nil {
		return err
	}
	for _, a := range answers {
		if a.Level != "" && !eventLevels[a.Level] {
			return Invalid("level_invalid", nil, "a level is never, poor, good or empty")
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
		people, events := known{}, known{}
		changed := 0
		for _, a := range answers {
			if err := people.check(a.Person, func() error { _, err := getPerson(ctx, tx, oid, a.Person); return err }); err != nil {
				return err
			}
			if err := events.check(a.Event, func() error { return eventInPlan(ctx, tx, oid, pid, a.Event) }); err != nil {
				return err
			}
			var res sql.Result
			if a.Level == "" {
				res, err = tx.ExecContext(ctx, `DELETE FROM person_event WHERE person_id = ? AND event_id = ? AND org_id = ?`, a.Person, a.Event, oid)
			} else {
				res, err = tx.ExecContext(ctx, `INSERT INTO person_event (org_id, plan_id, person_id, event_id, level) VALUES (?, ?, ?, ?, ?)
					ON CONFLICT (person_id, event_id) DO UPDATE SET level = excluded.level WHERE level != excluded.level`, oid, pid, a.Person, a.Event, a.Level)
			}
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				changed++
			}
		}
		if changed == 0 {
			return errNothingToChange()
		}
		return nil
	})
}

// SetPlacesMany sets the places of many cells (task × event) of a plan in one go.
func (s *Store) SetPlacesMany(ctx context.Context, orgSlug, planSlug string, places []Places) error {
	if err := cellsCount(len(places)); err != nil {
		return err
	}
	for _, p := range places {
		if p.Slots < 0 || p.Slots > maxSlots {
			return Invalid("places_invalid", map[string]any{"max": maxSlots}, "places must be 0-%d", maxSlots)
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
		tasks, events := known{}, known{}
		changed := 0
		for _, p := range places {
			if err := tasks.check(p.Task, func() error { return inOrg(ctx, tx, "task", oid, p.Task) }); err != nil {
				return err
			}
			if err := events.check(p.Event, func() error { return eventInPlan(ctx, tx, oid, pid, p.Event) }); err != nil {
				return err
			}
			var before int
			err := tx.QueryRowContext(ctx, `SELECT slots FROM event_task WHERE event_id = ? AND task_id = ? AND org_id = ?`, p.Event, p.Task, oid).Scan(&before)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if before == p.Slots {
				continue
			}
			if err := setPlaces(ctx, tx, oid, pid, p.Event, p.Task, p.Slots); err != nil {
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
