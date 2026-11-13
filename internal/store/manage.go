package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// Renaming changes the name only: the slug is the address, and links to a plan and to people's pages are sent around, so
// they keep working.

func (s *Store) RenameOrg(ctx context.Context, actor, orgSlug, name string) (Org, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Org{}, err
	}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		var id int64
		var old string
		err := tx.QueryRowContext(ctx, `SELECT id, name FROM organization WHERE slug = ?`, orgSlug).Scan(&id, &old)
		if errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		}
		if err != nil {
			return logEntry{}, err
		}
		if old == name {
			return logEntry{}, errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE organization SET name = ? WHERE id = ?`, name, id); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: id, action: "org.rename", detail: fmt.Sprintf("%q -> %q", old, name)}, nil
	})
	if err != nil {
		return Org{}, err
	}
	return s.GetOrg(ctx, orgSlug)
}

func (s *Store) RenamePlan(ctx context.Context, orgSlug, planSlug, actor, name string) (Plan, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Plan{}, err
	}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		var old string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM plan WHERE id = ? AND org_id = ?`, pid, oid).Scan(&old); err != nil {
			return logEntry{}, err
		}
		if old == name {
			return logEntry{}, errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plan SET name = ? WHERE id = ? AND org_id = ?`, name, pid, oid); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "plan.rename", detail: fmt.Sprintf("%q -> %q", old, name)}, nil
	})
	if err != nil {
		return Plan{}, err
	}
	return s.GetPlan(ctx, orgSlug, planSlug)
}

// DeletePlan removes a plan with everything that belongs to it: its events and what hangs on them (answers, manual cells,
// selections, places per event, info values), its info rows and all its runs, a pinned one too. People, tasks, day types
// and groups are the organisation's and stay. The change log stays too, so the history of the organisation is whole: lines about the plan lose their link to it and get the
// plan's name in front of the text, and the deletion itself is a line without a plan. A plan with a run in progress is
// refused: the run would have nowhere to finish.
func (s *Store) DeletePlan(ctx context.Context, orgSlug, planSlug, actor string) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM plan WHERE id = ? AND org_id = ?`, pid, oid).Scan(&name); err != nil {
			return logEntry{}, err
		}
		var running int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan_run WHERE org_id = ? AND plan_id = ? AND status = 'running'`, oid, pid).Scan(&running); err != nil {
			return logEntry{}, err
		}
		if running > 0 {
			return logEntry{}, Conflict("plan_busy", map[string]any{"name": name}, "plan %q has a run in progress", name)
		}
		// The runs have no cascade, so they go first; the events take their answers, manual cells, selections, places and info
		// values with them (ON DELETE CASCADE), and then the info rows can go. A table added later that refers to a plan or an
		// event without a cascade makes this fail, and the test that lists the tables with a plan_id says where to look.
		for _, table := range []string{"plan_run", "event", "plan_field"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE org_id = ? AND plan_id = ?`, oid, pid); err != nil {
				return logEntry{}, err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE change_log SET plan_id = NULL, detail = ? || ': ' || detail WHERE org_id = ? AND plan_id = ?`, name, oid, pid); err != nil {
			return logEntry{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM plan WHERE id = ? AND org_id = ?`, pid, oid); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "plan.delete", detail: name}, nil
	})
}

// DeleteOrg removes an organisation that has no plans, with its people, tasks, day types, groups, combinations and its
// change log. confirm must be the organisation's slug. This is the one change that does not go through write: the log
// lines belong to the organisation and go with it, so there is nowhere to record the deletion; the caller logs it
// instead. It returns the organisation as it was.
func (s *Store) DeleteOrg(ctx context.Context, actor, orgSlug, confirm string) (Org, error) {
	if _, err := cleanActor(actor); err != nil {
		return Org{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Org{}, err
	}
	defer tx.Rollback()
	var o Org
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id, slug, name, created_at FROM organization WHERE slug = ?`, orgSlug).Scan(&id, &o.Slug, &o.Name, &o.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Org{}, ErrNotFound
	}
	if err != nil {
		return Org{}, err
	}
	if confirm != o.Slug {
		return Org{}, Invalid("confirm_mismatch", nil, "the confirmation must be the organisation's address name, %q", o.Slug)
	}
	var plans int
	if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan WHERE org_id = ?`, id).Scan(&plans); err != nil {
		return Org{}, err
	}
	if plans > 0 {
		return Org{}, Conflict("org_has_plans", map[string]any{"name": o.Name, "count": plans}, "organisation %q still has %d plan(s); delete them first", o.Name, plans)
	}
	// Children before parents: what refers to people, tasks, day types and groups first.
	for _, table := range []string{"grp_task", "grp_member", "day_type_task", "task_combination", "person_task", "grp", "day_type", "task", "person", "change_log"} {
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE org_id = ?`, id); err != nil {
			return Org{}, err
		}
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM organization WHERE id = ?`, id); err != nil {
		return Org{}, err
	}
	if err := tx.Commit(); err != nil {
		return Org{}, err
	}
	return o, nil
}
