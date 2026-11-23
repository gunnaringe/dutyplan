package store

import (
	"context"
	"database/sql"
	"errors"
)

// Sections place people in the table: a heading in Availability with its people below it, in an order of their own. They
// belong to the plan, like the people, and are only placement: the planner never sees them. People without
// a section come first. Tasks have sections of their own the same way (task_section, since 2026-11-23): headings in Tasks
// and in the draft. The functions below take which (People or Tasks); the exported ones without it are for people.

const maxSections = 30

// SectionsOf says whose sections: the people's or the tasks'.
type SectionsOf struct{ table, items, what string }

var (
	People = SectionsOf{"person_section", "person", "section"}
	Tasks  = SectionsOf{"task_section", "task", "task section"}
)

type Section struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (s *Store) ListSections(ctx context.Context, orgSlug, planSlug string) ([]Section, error) {
	return s.ListSectionsOf(ctx, People, orgSlug, planSlug)
}

func (s *Store) ListSectionsOf(ctx context.Context, of SectionsOf, orgSlug, planSlug string) ([]Section, error) {
	_, pid, err := scope(ctx, s.db, orgSlug, planSlug)
	if err != nil {
		return nil, err
	}
	out := []Section{}
	err = scanAll(ctx, s.db, `SELECT id, name FROM `+of.table+` WHERE plan_id = ? ORDER BY position, id`, []any{pid}, func(r *sql.Rows) error {
		var x Section
		err := r.Scan(&x.ID, &x.Name)
		out = append(out, x)
		return err
	})
	return out, err
}

// CreateSection adds a section at the bottom.
func (s *Store) CreateSection(ctx context.Context, orgSlug, planSlug, name string) (Section, error) {
	return s.CreateSectionOf(ctx, People, orgSlug, planSlug, name)
}

func (s *Store) CreateSectionOf(ctx context.Context, of SectionsOf, orgSlug, planSlug, name string) (Section, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Section{}, err
	}
	x := Section{Name: name}
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+of.table+` WHERE plan_id = ?`, pid).Scan(&n); err != nil {
			return err
		}
		if n >= maxSections {
			return Invalid("too_many_sections", map[string]any{"max": maxSections}, "at most %d sections", maxSections)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO `+of.table+` (org_id, plan_id, name, position, created_at) VALUES (?, ?, ?, ?, ?)`, oid, pid, name, n, s.stamp())
		if isUnique(err) {
			return exists(of.what, name)
		}
		if err != nil {
			return err
		}
		x.ID, _ = res.LastInsertId()
		return nil
	})
	return x, err
}

func sectionName(ctx context.Context, q dbq, of SectionsOf, pid, id int64) (string, error) {
	var name string
	err := q.QueryRowContext(ctx, `SELECT name FROM `+of.table+` WHERE id = ? AND plan_id = ?`, id, pid).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

func (s *Store) RenameSection(ctx context.Context, orgSlug, planSlug string, id int64, name string) (Section, error) {
	return s.RenameSectionOf(ctx, People, orgSlug, planSlug, id, name)
}

func (s *Store) RenameSectionOf(ctx context.Context, of SectionsOf, orgSlug, planSlug string, id int64, name string) (Section, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Section{}, err
	}
	err = s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		old, err := sectionName(ctx, tx, of, pid, id)
		if err != nil {
			return err
		}
		if old == name {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE `+of.table+` SET name = ? WHERE id = ? AND plan_id = ?`, name, id, pid); isUnique(err) {
			return exists(of.what, name)
		} else {
			return err
		}
	})
	return Section{ID: id, Name: name}, err
}

// MoveSection puts a section at a position among the sections (0 = the first; past the end = the last).
func (s *Store) MoveSection(ctx context.Context, orgSlug, planSlug string, id int64, position int) error {
	return s.MoveSectionOf(ctx, People, orgSlug, planSlug, id, position)
}

func (s *Store) MoveSectionOf(ctx context.Context, of SectionsOf, orgSlug, planSlug string, id int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		if _, err := sectionName(ctx, tx, of, pid, id); err != nil {
			return err
		}
		return renumber(ctx, tx, of.table, pid, nil, id, position)
	})
}

// DeleteSection removes a section; its people go to the end of those without a section, in their order.
func (s *Store) DeleteSection(ctx context.Context, orgSlug, planSlug string, id int64) error {
	return s.DeleteSectionOf(ctx, People, orgSlug, planSlug, id)
}

// DeleteSectionOf removes a section; its people (or tasks) go to the end of those without a section, in their order.
func (s *Store) DeleteSectionOf(ctx context.Context, of SectionsOf, orgSlug, planSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		if _, err := sectionName(ctx, tx, of, pid, id); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM `+of.items+` WHERE plan_id = ? AND section_id IS NULL`, pid).Scan(&n); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE `+of.items+` SET section_id = NULL, position = position + ? WHERE plan_id = ? AND section_id = ?`, n, pid, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM `+of.table+` WHERE id = ? AND plan_id = ?`, id, pid); err != nil {
			return err
		}
		return renumber(ctx, tx, of.table, pid, nil, 0, 0)
	})
}

// MovePerson puts a person in a section (0 = none) at a position among its people (0 = the top; past the end = the bottom).
func (s *Store) MovePerson(ctx context.Context, orgSlug, planSlug string, id, section int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		p, err := getPerson(ctx, tx, pid, id)
		if err != nil {
			return err
		}
		if section != 0 {
			if _, err := sectionName(ctx, tx, People, pid, section); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE person SET section_id = ? WHERE id = ? AND plan_id = ?`, sectionScope(section)[0], id, pid); err != nil {
			return err
		}
		if err := renumber(ctx, tx, "person", pid, sectionScope(section), id, position); err != nil {
			return err
		}
		if p.Section != section { // close the gap where the person was
			return renumber(ctx, tx, "person", pid, sectionScope(p.Section), 0, 0)
		}
		return nil
	})
}

// sectionScope is the section_id to compare with: NULL for no section.
func sectionScope(section int64) []any {
	if section == 0 {
		return []any{nil}
	}
	return []any{section}
}

// renumber gives the rows of a plan (for people: of one section) the positions 0, 1, ... in their order, with
// the row moved (if any) at position.
func renumber(ctx context.Context, tx *sql.Tx, table string, pid int64, section []any, moved int64, position int) error {
	where, args := `plan_id = ?`, []any{pid}
	switch table {
	case "person", "task":
		where, args = where+` AND section_id IS ?`, append(args, section[0])
	case "person_section", "task_section":
	default:
		panic("renumber: unknown table " + table)
	}
	var order []int64
	if err := scanAll(ctx, tx, `SELECT id FROM `+table+` WHERE `+where+` AND id <> ? ORDER BY position, id`, append(args, moved),
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
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET position = ? WHERE id = ? AND plan_id = ?`, i, id, pid); err != nil {
			return err
		}
	}
	return nil
}

// MoveTask puts a task in a section (nil: the one it is in; 0: none) at a position among its tasks (0 = the top; past the
// end = the bottom). The order is the plan's; the planner does not see it.
func (s *Store) MoveTask(ctx context.Context, orgSlug, planSlug string, id int64, section *int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		_, pid, err := scope(ctx, tx, orgSlug, planSlug)
		if err != nil {
			return err
		}
		var was sql.NullInt64
		err = tx.QueryRowContext(ctx, `SELECT section_id FROM task WHERE id = ? AND plan_id = ?`, id, pid).Scan(&was)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		to := was.Int64
		if section != nil {
			to = *section
		}
		if to != 0 {
			if _, err := sectionName(ctx, tx, Tasks, pid, to); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE task SET section_id = ? WHERE id = ? AND plan_id = ?`, sectionScope(to)[0], id, pid); err != nil {
			return err
		}
		if err := renumber(ctx, tx, "task", pid, sectionScope(to), id, position); err != nil {
			return err
		}
		if was.Int64 != to { // close the gap where the task was
			return renumber(ctx, tx, "task", pid, sectionScope(was.Int64), 0, 0)
		}
		return nil
	})
}
