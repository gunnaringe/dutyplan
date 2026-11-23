package store

import (
	"context"
	"database/sql"
	"errors"
)

// Renaming changes the name only: the slug is the address, and links to a plan and to people's pages are sent around, so
// they keep working.

func (s *Store) RenameOrg(ctx context.Context, orgSlug, name string) (Org, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Org{}, err
	}
	err = s.write(ctx, func(tx *sql.Tx) error {
		var id int64
		var old string
		err := tx.QueryRowContext(ctx, `SELECT id, name FROM organization WHERE slug = ?`, orgSlug).Scan(&id, &old)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if old == name {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE organization SET name = ? WHERE id = ?`, name, id); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Org{}, err
	}
	return s.GetOrg(ctx, orgSlug)
}

func (s *Store) RenamePlan(ctx context.Context, orgSlug, planSlug, name string) (Plan, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Plan{}, err
	}
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var old string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM plan WHERE id = ? AND org_id = ?`, pid, oid).Scan(&old); err != nil {
			return err
		}
		if old == name {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plan SET name = ? WHERE id = ? AND org_id = ?`, name, pid, oid); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return Plan{}, err
	}
	return s.GetPlan(ctx, orgSlug, planSlug)
}

// DeletePlan removes a plan with everything that belongs to it: its events and what hangs on them (answers, manual cells,
// selections, places per event, info values), its info rows and all its runs, a pinned one too. People, tasks and sections
// are the organisation's and stay. A plan with a run in progress is refused: the run would have nowhere to finish.
func (s *Store) DeletePlan(ctx context.Context, orgSlug, planSlug string) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var name string
		if err := tx.QueryRowContext(ctx, `SELECT name FROM plan WHERE id = ? AND org_id = ?`, pid, oid).Scan(&name); err != nil {
			return err
		}
		var running int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan_run WHERE org_id = ? AND plan_id = ? AND status = 'running'`, oid, pid).Scan(&running); err != nil {
			return err
		}
		if running > 0 {
			return Conflict("plan_busy", map[string]any{"name": name}, "plan %q has a run in progress", name)
		}
		// The runs have no cascade, so they go first; the events take their answers, manual cells, selections, places and info
		// values with them (ON DELETE CASCADE), and then the info rows can go, and the plan's people and tasks (children
		// before parents). A table added later that refers to a plan or an event without a cascade makes this fail, and the
		// test that lists the tables with a plan_id says where to look.
		for _, table := range []string{"plan_run", "event", "plan_field", "share_link", "task_combination", "person_task", "person", "person_section", "task", "task_section"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE org_id = ? AND plan_id = ?`, oid, pid); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM plan WHERE id = ? AND org_id = ?`, pid, oid); err != nil {
			return err
		}
		return nil
	})
}

// DeleteOrg removes an organisation that has no plans, with its members and links (and people, tasks, sections and
// combinations from before plans had their own, migration 012). confirm must be
// the organisation's slug. It returns the organisation as it was.
func (s *Store) DeleteOrg(ctx context.Context, orgSlug, confirm string) (Org, error) {
	var o Org
	err := s.write(ctx, func(tx *sql.Tx) error {
		var id int64
		err := tx.QueryRowContext(ctx, `SELECT id, slug, name, created_at FROM organization WHERE slug = ?`, orgSlug).Scan(&id, &o.Slug, &o.Name, &o.CreatedAt)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if confirm != o.Slug {
			return Invalid("confirm_mismatch", nil, "the confirmation must be the organisation's address name, %q", o.Slug)
		}
		var plans int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM plan WHERE org_id = ?`, id).Scan(&plans); err != nil {
			return err
		}
		if plans > 0 {
			return Conflict("org_has_plans", map[string]any{"name": o.Name, "count": plans}, "organisation %q still has %d plan(s); delete them first", o.Name, plans)
		}
		// Children before parents: what refers to people and tasks first.
		for _, table := range []string{"task_combination", "person_task", "task", "task_section", "person", "person_section", "membership", "share_link"} {
			if _, err := tx.ExecContext(ctx, `DELETE FROM `+table+` WHERE org_id = ?`, id); err != nil {
				return err
			}
		}
		_, err = tx.ExecContext(ctx, `DELETE FROM organization WHERE id = ?`, id)
		return err
	})
	if err != nil {
		return Org{}, err
	}
	return o, nil
}
