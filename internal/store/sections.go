package store

import (
	"context"
	"database/sql"
	"errors"
)

// Sections place people in the table: a heading in Availability with its people below it, in an order of their own. They
// belong to the organisation (the same in every plan) and are only placement: the planner never sees them. People without
// a section come first.

const maxSections = 30

type Section struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
}

func (s *Store) ListSections(ctx context.Context, orgSlug string) ([]Section, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	out := []Section{}
	err = scanAll(ctx, s.db, `SELECT id, name FROM person_section WHERE org_id = ? ORDER BY position, id`, []any{oid}, func(r *sql.Rows) error {
		var x Section
		err := r.Scan(&x.ID, &x.Name)
		out = append(out, x)
		return err
	})
	return out, err
}

// CreateSection adds a section at the bottom.
func (s *Store) CreateSection(ctx context.Context, orgSlug, name string) (Section, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Section{}, err
	}
	x := Section{Name: name}
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM person_section WHERE org_id = ?`, oid).Scan(&n); err != nil {
			return err
		}
		if n >= maxSections {
			return Invalid("too_many_sections", map[string]any{"max": maxSections}, "at most %d sections", maxSections)
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO person_section (org_id, name, position, created_at) VALUES (?, ?, ?, ?)`, oid, name, n, s.stamp())
		if isUnique(err) {
			return exists("section", name)
		}
		if err != nil {
			return err
		}
		x.ID, _ = res.LastInsertId()
		return nil
	})
	return x, err
}

func sectionName(ctx context.Context, q dbq, oid, id int64) (string, error) {
	var name string
	err := q.QueryRowContext(ctx, `SELECT name FROM person_section WHERE id = ? AND org_id = ?`, id, oid).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", ErrNotFound
	}
	return name, err
}

func (s *Store) RenameSection(ctx context.Context, orgSlug string, id int64, name string) (Section, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Section{}, err
	}
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		old, err := sectionName(ctx, tx, oid, id)
		if err != nil {
			return err
		}
		if old == name {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE person_section SET name = ? WHERE id = ? AND org_id = ?`, name, id, oid); isUnique(err) {
			return exists("section", name)
		} else {
			return err
		}
	})
	return Section{ID: id, Name: name}, err
}

// MoveSection puts a section at a position among the sections (0 = the first; past the end = the last).
func (s *Store) MoveSection(ctx context.Context, orgSlug string, id int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		if _, err := sectionName(ctx, tx, oid, id); err != nil {
			return err
		}
		return renumber(ctx, tx, "person_section", oid, nil, id, position)
	})
}

// DeleteSection removes a section; its people go to the end of those without a section, in their order.
func (s *Store) DeleteSection(ctx context.Context, orgSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		if _, err := sectionName(ctx, tx, oid, id); err != nil {
			return err
		}
		var n int
		if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM person WHERE org_id = ? AND section_id IS NULL`, oid).Scan(&n); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE person SET section_id = NULL, position = position + ? WHERE org_id = ? AND section_id = ?`, n, oid, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM person_section WHERE id = ? AND org_id = ?`, id, oid); err != nil {
			return err
		}
		return renumber(ctx, tx, "person_section", oid, nil, 0, 0)
	})
}

// MovePerson puts a person in a section (0 = none) at a position among its people (0 = the top; past the end = the bottom).
func (s *Store) MovePerson(ctx context.Context, orgSlug string, id, section int64, position int) error {
	if position < 0 {
		return Invalid("position_invalid", nil, "the position must be 0 or more")
	}
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		p, err := getPerson(ctx, tx, oid, id)
		if err != nil {
			return err
		}
		if section != 0 {
			if _, err := sectionName(ctx, tx, oid, section); err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE person SET section_id = ? WHERE id = ? AND org_id = ?`, sectionScope(section)[0], id, oid); err != nil {
			return err
		}
		if err := renumber(ctx, tx, "person", oid, sectionScope(section), id, position); err != nil {
			return err
		}
		if p.Section != section { // close the gap where the person was
			return renumber(ctx, tx, "person", oid, sectionScope(p.Section), 0, 0)
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

// renumber gives the rows of an organisation (for people: of one section) the positions 0, 1, ... in their order, with
// the row moved (if any) at position.
func renumber(ctx context.Context, tx *sql.Tx, table string, oid int64, section []any, moved int64, position int) error {
	where, args := `org_id = ?`, []any{oid}
	switch table {
	case "person":
		where, args = where+` AND section_id IS ?`, append(args, section[0])
	case "person_section":
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
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET position = ? WHERE id = ? AND org_id = ?`, i, id, oid); err != nil {
			return err
		}
	}
	return nil
}
