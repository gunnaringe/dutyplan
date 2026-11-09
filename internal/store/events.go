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

// Limits for adding events in bulk, so a typo cannot create a flood of rows.
const (
	maxBulkSpanDays  = 366
	maxEventsPerCall = 400
)

// Event is one occasion in a plan: a date and a day type (and a note). It is the unit that people are available for.
type Event struct {
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
		return "", Invalid("note_too_long", nil, "note must be at most 200 printable characters")
	}
	return n, nil
}

func (s *Store) ListEvents(ctx context.Context, orgSlug, planSlug string) ([]Event, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT d.id, d.date, d.day_type_id, t.name, d.note
		FROM event d JOIN day_type t ON t.id = d.day_type_id AND t.org_id = d.org_id
		WHERE d.org_id = ? AND d.plan_id = ? ORDER BY d.date, d.id`, oid, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		var d Event
		if err := rows.Scan(&d.ID, &d.Date, &d.DayTypeID, &d.DayType, &d.Note); err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func getEvent(ctx context.Context, q dbq, oid, pid, id int64) (Event, error) {
	var d Event
	err := q.QueryRowContext(ctx, `SELECT d.id, d.date, d.day_type_id, t.name, d.note
		FROM event d JOIN day_type t ON t.id = d.day_type_id AND t.org_id = d.org_id
		WHERE d.id = ? AND d.org_id = ? AND d.plan_id = ?`, id, oid, pid).Scan(&d.ID, &d.Date, &d.DayTypeID, &d.DayType, &d.Note)
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

func (s *Store) AddEvent(ctx context.Context, orgSlug, planSlug, actor, date string, dayTypeID int64, note string) (Event, error) {
	if _, ok := parseISODate(date); !ok {
		return Event{}, Invalid("date_invalid", nil, "date must be YYYY-MM-DD")
	}
	note, err := cleanNote(note)
	if err != nil {
		return Event{}, err
	}
	var out Event
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
		// Events are independent: the same date may have several (they differ by day type and note).
		res, err := tx.ExecContext(ctx, `INSERT INTO event (org_id, plan_id, date, day_type_id, note, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
			oid, pid, date, dayTypeID, note, s.stamp())
		if err != nil {
			return logEntry{}, err
		}
		id, _ := res.LastInsertId()
		if out, err = getEvent(ctx, tx, oid, pid, id); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "event.add", detail: date + " " + out.DayType}, nil
	})
	return out, err
}

// AddEventsBulk adds an event for every date from..to (inclusive) that falls on one of the ISO weekdays. A date that
// already has an event of the same day type is skipped and reported, not an error.
func (s *Store) AddEventsBulk(ctx context.Context, orgSlug, planSlug, actor, from, to string, weekdays []int, dayTypeID int64) (created []Event, skipped []string, err error) {
	f, ok1 := parseISODate(from)
	t, ok2 := parseISODate(to)
	if !ok1 || !ok2 || t.Before(f) {
		return nil, nil, Invalid("range_invalid", nil, "from and to must be YYYY-MM-DD with from <= to")
	}
	if t.Sub(f) > maxBulkSpanDays*24*time.Hour {
		return nil, nil, Invalid("range_too_long", map[string]any{"max": maxBulkSpanDays}, "the range can be at most %d days", maxBulkSpanDays)
	}
	want := map[int]bool{}
	for _, w := range weekdays {
		if w < 1 || w > 7 {
			return nil, nil, Invalid("weekdays_invalid", nil, "weekdays are 1 (Monday) to 7 (Sunday)")
		}
		want[w] = true
	}
	if len(want) == 0 {
		return nil, nil, Invalid("weekdays_none", nil, "choose at least one weekday")
	}
	var dates []string
	for d := f; !d.After(t); d = d.AddDate(0, 0, 1) {
		if want[IsoWeekday(d)] {
			dates = append(dates, d.Format("2006-01-02"))
		}
	}
	if len(dates) == 0 {
		return nil, nil, Invalid("no_dates", nil, "no dates match")
	}
	if len(dates) > maxEventsPerCall {
		return nil, nil, Invalid("too_many_dates", map[string]any{"max": maxEventsPerCall}, "at most %d dates at a time", maxEventsPerCall)
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
			// Running the same bulk add twice must not duplicate: an event with the same date and day type counts as present.
			res, err := tx.ExecContext(ctx, `INSERT INTO event (org_id, plan_id, date, day_type_id, note, created_at)
				SELECT ?, ?, ?, ?, '', ? WHERE NOT EXISTS
				(SELECT 1 FROM event WHERE org_id = ? AND plan_id = ? AND date = ? AND day_type_id = ?)`,
				oid, pid, d, dayTypeID, s.stamp(), oid, pid, d, dayTypeID)
			if err != nil {
				return logEntry{}, err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				skipped = append(skipped, d)
				continue
			}
			id, _ := res.LastInsertId()
			sd, err := getEvent(ctx, tx, oid, pid, id)
			if err != nil {
				return logEntry{}, err
			}
			created = append(created, sd)
		}
		return logEntry{orgID: oid, planID: &pid, action: "event.bulk", detail: fmt.Sprintf("%d new events, %d already existed (%s – %s)", len(created), len(skipped), from, to)}, nil
	})
	return created, skipped, err
}

type EventPatch struct {
	DayTypeID *int64  `json:"day_type_id"`
	Note      *string `json:"note"`
}

func (s *Store) UpdateEvent(ctx context.Context, orgSlug, planSlug, actor string, id int64, patch EventPatch) (Event, error) {
	var out Event
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
		cur, err := getEvent(ctx, tx, oid, pid, id)
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
			return logEntry{}, errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE event SET day_type_id = ?, note = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, dt, note, id, oid, pid); err != nil {
			return logEntry{}, err
		}
		if out, err = getEvent(ctx, tx, oid, pid, id); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "event.update", detail: fmt.Sprintf("%s: %s", out.Date, strings.Join(changes, ", "))}, nil
	})
	return out, err
}

func (s *Store) DeleteEvent(ctx context.Context, orgSlug, planSlug, actor string, id int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		cur, err := getEvent(ctx, tx, oid, pid, id)
		if err != nil {
			return logEntry{}, err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM event WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "event.delete", detail: cur.Date + " " + cur.DayType}, nil
	})
}
