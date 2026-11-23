package store

import (
	"context"
	"database/sql"
)

// Locking the draft (2026-11-22, the user: "La en planlagt celle som inneholder navn ha en lås-knapp … Det bør kanskje være
// enklere å låse plan?"): many of the planner's names locked or unlocked at once (a selection of draft cells, or «Lock
// all»), and «Remove unlocked», which sets the drafts aside so only what is set by hand and locked is shown.

// Lock is one of the planner's names in one place: the person for the task on the event.
type Lock struct {
	Event  int64 `json:"event"`
	Task   int64 `json:"task"`
	Person int64 `json:"person"`
}

// SetLocks locks (a locked manual cell for each) or unlocks (removes those locked cells) the names given, all or nothing.
// A name already set by hand, or already locked, stays as it is when locking; unlocking never removes one set by hand.
func (s *Store) SetLocks(ctx context.Context, orgSlug, planSlug string, locks []Lock, lock bool) error {
	if err := cellsCount(len(locks)); err != nil {
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
		people, tasks, events := known{}, known{}, known{}
		changed := 0
		for _, l := range locks {
			if err := people.check(l.Person, func() error { return inOrg(ctx, tx, "person", oid, l.Person) }); err != nil {
				return err
			}
			if err := tasks.check(l.Task, func() error { return inOrg(ctx, tx, "task", oid, l.Task) }); err != nil {
				return err
			}
			if err := events.check(l.Event, func() error { return eventInPlan(ctx, tx, oid, pid, l.Event) }); err != nil {
				return err
			}
			var res sql.Result
			if lock {
				res, err = tx.ExecContext(ctx, `INSERT INTO manual_cell (org_id, plan_id, event_id, task_id, person_id, fills, locked, created_at)
					VALUES (?, ?, ?, ?, ?, 1, 1, ?) ON CONFLICT DO NOTHING`, oid, pid, l.Event, l.Task, l.Person, s.stamp())
			} else {
				res, err = tx.ExecContext(ctx, `DELETE FROM manual_cell WHERE org_id = ? AND plan_id = ? AND event_id = ? AND task_id = ? AND person_id = ? AND locked = 1`,
					oid, pid, l.Event, l.Task, l.Person)
			}
			if err != nil {
				return err
			}
			n, _ := res.RowsAffected()
			changed += int(n)
		}
		if changed == 0 {
			return errNothingToChange()
		}
		return nil
	})
}

// SetDraftAside is «Remove unlocked»: the plan shows no draft (the pin goes too) until a new run starts or one is pinned.
// What is set by hand and locked are manual cells, so they stay.
func (s *Store) SetDraftAside(ctx context.Context, orgSlug, planSlug string) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE plan SET draft_aside = 1 WHERE id = ? AND org_id = ? AND draft_aside = 0`, pid, oid)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errNothingToChange()
		}
		_, err = tx.ExecContext(ctx, `UPDATE plan_run SET pinned = 0 WHERE org_id = ? AND plan_id = ?`, oid, pid)
		return err
	})
}

// ClearDraftAside undoes «Remove unlocked» (the page's Undo): the drafts are shown again. A pin it took away is put back by
// pinning that run (which clears this too).
func (s *Store) ClearDraftAside(ctx context.Context, orgSlug, planSlug string) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `UPDATE plan SET draft_aside = 0 WHERE id = ? AND org_id = ? AND draft_aside = 1`, pid, oid)
		if err != nil {
			return err
		}
		if n, _ := res.RowsAffected(); n == 0 {
			return errNothingToChange()
		}
		return nil
	})
}
