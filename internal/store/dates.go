package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits for adding dates in bulk, so a typo cannot create a flood of rows.
const (
	maxBulkSpanDays = 366
	maxDatesPerCall = 400
)

type SlotDate struct {
	ID        int64  `json:"id"`
	Date      string `json:"date"` // YYYY-MM-DD
	DayTypeID int64  `json:"day_type_id"`
	DayType   string `json:"day_type"`
	Note      string `json:"note"`
}

// IsoWeekday numbers weekdays 1 = Monday ... 7 = Sunday (ISO 8601). It is computed here, in UTC, never in the browser.
func IsoWeekday(t time.Time) int { return (int(t.Weekday())+6)%7 + 1 }

func parseISODate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil && t.Format("2006-01-02") == s
}

func cleanNote(raw string) (string, error) {
	n := strings.TrimSpace(raw)
	if utf8.RuneCountInString(n) > 200 || !validText(n) {
		return "", fmt.Errorf("%w: note must be at most 200 printable characters", ErrInvalid)
	}
	return n, nil
}

func (s *Store) ListDates(ctx context.Context, orgSlug, planSlug string) ([]SlotDate, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT d.id, d.date, d.day_type_id, t.name, d.note
		FROM slot_date d JOIN day_type t ON t.id = d.day_type_id AND t.org_id = d.org_id
		WHERE d.org_id = ? AND d.plan_id = ? ORDER BY d.date`, oid, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SlotDate{}
	for rows.Next() {
		var d SlotDate
		if err := rows.Scan(&d.ID, &d.Date, &d.DayTypeID, &d.DayType, &d.Note); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func getDate(ctx context.Context, q dbq, oid, pid, id int64) (SlotDate, error) {
	var d SlotDate
	err := q.QueryRowContext(ctx, `SELECT d.id, d.date, d.day_type_id, t.name, d.note
		FROM slot_date d JOIN day_type t ON t.id = d.day_type_id AND t.org_id = d.org_id
		WHERE d.id = ? AND d.org_id = ? AND d.plan_id = ?`, id, oid, pid).Scan(&d.ID, &d.Date, &d.DayTypeID, &d.DayType, &d.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) AddDate(ctx context.Context, orgSlug, planSlug, actor, date string, dayTypeID int64, note string) (SlotDate, error) {
	if _, ok := parseISODate(date); !ok {
		return SlotDate{}, fmt.Errorf("%w: date must be YYYY-MM-DD", ErrInvalid)
	}
	note, err := cleanNote(note)
	if err != nil {
		return SlotDate{}, err
	}
	var out SlotDate
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		if err := inOrg(ctx, tx, "day_type", oid, dayTypeID); err != nil {
			return logEntry{}, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO slot_date (org_id, plan_id, date, day_type_id, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			oid, pid, date, dayTypeID, note, s.stamp())
		if isUnique(err) {
			return logEntry{}, fmt.Errorf("%w: date %s is already in the plan", ErrConflict, date)
		}
		if err != nil {
			return logEntry{}, err
		}
		id, _ := res.LastInsertId()
		if out, err = getDate(ctx, tx, oid, pid, id); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "date.add", detail: date + " " + out.DayType}, nil
	})
	return out, err
}

// AddDatesBulk adds every date from..to (inclusive) that falls on one of the ISO weekdays. Dates already in the plan
// are skipped and reported, not errors.
func (s *Store) AddDatesBulk(ctx context.Context, orgSlug, planSlug, actor, from, to string, weekdays []int, dayTypeID int64) (created []SlotDate, skipped []string, err error) {
	f, ok1 := parseISODate(from)
	t, ok2 := parseISODate(to)
	if !ok1 || !ok2 || t.Before(f) {
		return nil, nil, fmt.Errorf("%w: from and to must be YYYY-MM-DD with from <= to", ErrInvalid)
	}
	if t.Sub(f) > maxBulkSpanDays*24*time.Hour {
		return nil, nil, fmt.Errorf("%w: the range can be at most %d days", ErrInvalid, maxBulkSpanDays)
	}
	want := map[int]bool{}
	for _, w := range weekdays {
		if w < 1 || w > 7 {
			return nil, nil, fmt.Errorf("%w: weekdays are 1 (Monday) to 7 (Sunday)", ErrInvalid)
		}
		want[w] = true
	}
	if len(want) == 0 {
		return nil, nil, fmt.Errorf("%w: choose at least one weekday", ErrInvalid)
	}
	var dates []string
	for d := f; !d.After(t); d = d.AddDate(0, 0, 1) {
		if want[IsoWeekday(d)] {
			dates = append(dates, d.Format("2006-01-02"))
		}
	}
	if len(dates) == 0 {
		return nil, nil, fmt.Errorf("%w: no dates match", ErrInvalid)
	}
	if len(dates) > maxDatesPerCall {
		return nil, nil, fmt.Errorf("%w: at most %d dates at a time", ErrInvalid, maxDatesPerCall)
	}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		created, skipped = nil, nil
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		if err := inOrg(ctx, tx, "day_type", oid, dayTypeID); err != nil {
			return logEntry{}, err
		}
		for _, d := range dates {
			res, err := tx.ExecContext(ctx, `INSERT INTO slot_date (org_id, plan_id, date, day_type_id, note, created_at) VALUES (?, ?, ?, ?, '', ?)
				ON CONFLICT (plan_id, date) DO NOTHING`, oid, pid, d, dayTypeID, s.stamp())
			if err != nil {
				return logEntry{}, err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				skipped = append(skipped, d)
				continue
			}
			id, _ := res.LastInsertId()
			sd, err := getDate(ctx, tx, oid, pid, id)
			if err != nil {
				return logEntry{}, err
			}
			created = append(created, sd)
		}
		return logEntry{orgID: oid, planID: &pid, action: "date.bulk", detail: fmt.Sprintf("%d nye datoer, %d fantes fra før (%s – %s)", len(created), len(skipped), from, to)}, nil
	})
	return created, skipped, err
}

type DatePatch struct {
	DayTypeID *int64  `json:"day_type_id"`
	Note      *string `json:"note"`
}

func (s *Store) UpdateDate(ctx context.Context, orgSlug, planSlug, actor string, id int64, patch DatePatch) (SlotDate, error) {
	var out SlotDate
	if patch.Note != nil {
		n, err := cleanNote(*patch.Note)
		if err != nil {
			return out, err
		}
		patch.Note = &n
	}
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		cur, err := getDate(ctx, tx, oid, pid, id)
		if err != nil {
			return logEntry{}, err
		}
		var changes []string
		dt, note := cur.DayTypeID, cur.Note
		if patch.DayTypeID != nil && *patch.DayTypeID != dt {
			if err := inOrg(ctx, tx, "day_type", oid, *patch.DayTypeID); err != nil {
				return logEntry{}, err
			}
			dt = *patch.DayTypeID
			changes = append(changes, "day type")
		}
		if patch.Note != nil && *patch.Note != note {
			note = *patch.Note
			changes = append(changes, "note")
		}
		if len(changes) == 0 {
			return logEntry{}, fmt.Errorf("%w: nothing to change", ErrInvalid)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE slot_date SET day_type_id = ?, note = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, dt, note, id, oid, pid); err != nil {
			return logEntry{}, err
		}
		if out, err = getDate(ctx, tx, oid, pid, id); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "date.update", detail: fmt.Sprintf("%s: %s", out.Date, strings.Join(changes, ", "))}, nil
	})
	return out, err
}

func (s *Store) DeleteDate(ctx context.Context, orgSlug, planSlug, actor string, id int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		cur, err := getDate(ctx, tx, oid, pid, id)
		if err != nil {
			return logEntry{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM slot_date WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "date.delete", detail: cur.Date + " " + cur.DayType}, nil
	})
}
