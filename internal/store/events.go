package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits for adding events in bulk, so a typo cannot create a flood of rows.
const (
	maxBulkSpanDays  = 366
	maxEventsPerCall = 400
)

// Limits for the free text of an event.
const maxDescriptionRunes = 2000

// Event is one occasion in a plan: a date and a day type (and a note, a time of day and a description). It is the unit that
// people are available for.
type Event struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"` // YYYY-MM-DD
	DayTypeID   int64  `json:"day_type_id"`
	DayType     string `json:"day_type"`
	Note        string `json:"note"`        // a short line shown next to the date
	StartTime   string `json:"start_time"`  // HH:MM, 24-hour, or "" when not set
	EndTime     string `json:"end_time"`    // HH:MM after StartTime, or "" (never set without a start time)
	Description string `json:"description"` // longer text, several lines allowed
}

// eventCols are the columns of an Event for a query on «event d JOIN day_type t»; scanEvent reads them. eventOrder puts the
// events of a date in time order, those with a start time first, the rest in the order they were created.
const (
	eventCols  = `d.id, d.date, d.day_type_id, t.name, d.note, d.start_time, d.end_time, d.description`
	eventOrder = `d.date, CASE WHEN d.start_time = '' THEN 1 ELSE 0 END, d.start_time, d.id`
)

func scanEvent(sc interface{ Scan(...any) error }) (Event, error) {
	var e Event
	err := sc.Scan(&e.ID, &e.Date, &e.DayTypeID, &e.DayType, &e.Note, &e.StartTime, &e.EndTime, &e.Description)
	return e, err
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

// parseClock accepts "" (not set) or HH:MM on a 24-hour clock.
func parseClock(raw string) (string, bool) {
	t := strings.TrimSpace(raw)
	if t == "" {
		return "", true
	}
	if len(t) != 5 || t[2] != ':' {
		return "", false
	}
	_, err := time.Parse("15:04", t)
	return t, err == nil
}

// cleanTimes checks a start and an end time together: an end time needs a start time and comes after it.
func cleanTimes(start, end string) (string, string, error) {
	st, ok := parseClock(start)
	if !ok {
		return "", "", Invalid("start_time_invalid", nil, "the start time must be HH:MM (24-hour clock)")
	}
	en, ok := parseClock(end)
	if !ok {
		return "", "", Invalid("end_time_invalid", nil, "the end time must be HH:MM (24-hour clock)")
	}
	if en != "" && st == "" {
		return "", "", Invalid("end_without_start", nil, "an end time needs a start time")
	}
	if en != "" && en <= st {
		return "", "", Invalid("end_not_after_start", nil, "the end time must be after the start time")
	}
	return st, en, nil
}

// cleanDescription keeps line breaks and tabs, refuses other control characters, and trims.
func cleanDescription(raw string) (string, error) {
	d := strings.TrimSpace(strings.ReplaceAll(raw, "\r\n", "\n"))
	if utf8.RuneCountInString(d) > maxDescriptionRunes || !validText(strings.NewReplacer("\n", " ", "\t", " ").Replace(d)) {
		return "", Invalid("description_invalid", map[string]any{"max": maxDescriptionRunes}, "the description can be at most %d characters, without control characters", maxDescriptionRunes)
	}
	return d, nil
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
	rows, err := s.db.QueryContext(ctx, `SELECT `+eventCols+`
		FROM event d JOIN day_type t ON t.id = d.day_type_id AND t.org_id = d.org_id
		WHERE d.org_id = ? AND d.plan_id = ? ORDER BY `+eventOrder, oid, pid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Event{}
	for rows.Next() {
		d, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, rows.Err()
}

func getEvent(ctx context.Context, q dbq, oid, pid, id int64) (Event, error) {
	d, err := scanEvent(q.QueryRowContext(ctx, `SELECT `+eventCols+`
		FROM event d JOIN day_type t ON t.id = d.day_type_id AND t.org_id = d.org_id
		WHERE d.id = ? AND d.org_id = ? AND d.plan_id = ?`, id, oid, pid))
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// NewEvent is what AddEventFull takes: a date and a day type, and optionally a note, a time of day and a description.
type NewEvent struct {
	Date        string
	DayTypeID   int64
	Note        string
	StartTime   string
	EndTime     string
	Description string
}

// AddEvent adds an event with a note and no time of day.
func (s *Store) AddEvent(ctx context.Context, orgSlug, planSlug, date string, dayTypeID int64, note string) (Event, error) {
	return s.AddEventFull(ctx, orgSlug, planSlug, NewEvent{Date: date, DayTypeID: dayTypeID, Note: note})
}

func (s *Store) AddEventFull(ctx context.Context, orgSlug, planSlug string, in NewEvent) (Event, error) {
	if _, ok := parseISODate(in.Date); !ok {
		return Event{}, Invalid("date_invalid", nil, "date must be YYYY-MM-DD")
	}
	note, err := cleanNote(in.Note)
	if err != nil {
		return Event{}, err
	}
	start, end, err := cleanTimes(in.StartTime, in.EndTime)
	if err != nil {
		return Event{}, err
	}
	desc, err := cleanDescription(in.Description)
	if err != nil {
		return Event{}, err
	}
	var out Event
	err = s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		if err := inOrg(ctx, tx, "day_type", oid, in.DayTypeID); err != nil {
			return err
		}
		// Events are independent: the same date may have several (they differ by day type, time and note).
		res, err := tx.ExecContext(ctx, `INSERT INTO event (org_id, plan_id, date, day_type_id, note, start_time, end_time, description, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			oid, pid, in.Date, in.DayTypeID, note, start, end, desc, s.stamp())
		if err != nil {
			return err
		}
		id, _ := res.LastInsertId()
		if out, err = getEvent(ctx, tx, oid, pid, id); err != nil {
			return err
		}
		return nil
	})
	return out, err
}

// AddEventsBulk adds events without a time of day; see AddEventsBulkTimed.
func (s *Store) AddEventsBulk(ctx context.Context, orgSlug, planSlug, from, to string, weekdays []int, dayTypeID int64) (created []Event, skipped []string, err error) {
	return s.AddEventsBulkTimed(ctx, orgSlug, planSlug, from, to, weekdays, dayTypeID, "", "")
}

// AddEventsBulkTimed adds an event for every date from..to (inclusive) that falls on one of the ISO weekdays, all with the
// same start and end time (both may be empty). A date that already has an event of the same day type and start time is
// skipped and reported, not an error.
func (s *Store) AddEventsBulkTimed(ctx context.Context, orgSlug, planSlug, from, to string, weekdays []int, dayTypeID int64, startTime, endTime string) (created []Event, skipped []string, err error) {
	startTime, endTime, err = cleanTimes(startTime, endTime)
	if err != nil {
		return nil, nil, err
	}
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
	err = s.write(ctx, func(tx *sql.Tx) error {
		created, skipped = nil, nil
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		if err := inOrg(ctx, tx, "day_type", oid, dayTypeID); err != nil {
			return err
		}
		for _, d := range dates {
			// Running the same bulk add twice must not duplicate: an event with the same date, day type and start time counts as present.
			res, err := tx.ExecContext(ctx, `INSERT INTO event (org_id, plan_id, date, day_type_id, note, start_time, end_time, created_at)
				SELECT ?, ?, ?, ?, '', ?, ?, ? WHERE NOT EXISTS
				(SELECT 1 FROM event WHERE org_id = ? AND plan_id = ? AND date = ? AND day_type_id = ? AND start_time = ?)`,
				oid, pid, d, dayTypeID, startTime, endTime, s.stamp(), oid, pid, d, dayTypeID, startTime)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n == 0 {
				skipped = append(skipped, d)
				continue
			}
			id, _ := res.LastInsertId()
			sd, err := getEvent(ctx, tx, oid, pid, id)
			if err != nil {
				return err
			}
			created = append(created, sd)
		}
		return nil
	})
	return created, skipped, err
}

type EventPatch struct {
	DayTypeID   *int64  `json:"day_type_id"`
	Note        *string `json:"note"`
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
	Description *string `json:"description"`
}

func (s *Store) UpdateEvent(ctx context.Context, orgSlug, planSlug string, id int64, patch EventPatch) (Event, error) {
	var out Event
	if patch.Note != nil {
		n, err := cleanNote(*patch.Note)
		if err != nil {
			return out, err
		}
		patch.Note = &n
	}
	if patch.Description != nil {
		d, err := cleanDescription(*patch.Description)
		if err != nil {
			return out, err
		}
		patch.Description = &d
	}
	err := s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		cur, err := getEvent(ctx, tx, oid, pid, id)
		if err != nil {
			return err
		}
		var changes []string
		dt, note := cur.DayTypeID, cur.Note
		if patch.DayTypeID != nil && *patch.DayTypeID != dt {
			if err := inOrg(ctx, tx, "day_type", oid, *patch.DayTypeID); err != nil {
				return err
			}
			dt = *patch.DayTypeID
			changes = append(changes, "day type")
		}
		if patch.Note != nil && *patch.Note != note {
			note = *patch.Note
			changes = append(changes, "note")
		}
		start, end, desc := cur.StartTime, cur.EndTime, cur.Description
		if patch.StartTime != nil || patch.EndTime != nil {
			ns, ne := start, end
			if patch.StartTime != nil {
				ns = *patch.StartTime
			}
			if patch.EndTime != nil {
				ne = *patch.EndTime
			}
			ns, ne, err := cleanTimes(ns, ne)
			if err != nil {
				return err
			}
			if ns != start || ne != end {
				start, end = ns, ne
				changes = append(changes, "time")
			}
		}
		if patch.Description != nil && *patch.Description != desc {
			desc = *patch.Description
			changes = append(changes, "description")
		}
		if len(changes) == 0 {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE event SET day_type_id = ?, note = ?, start_time = ?, end_time = ?, description = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, dt, note, start, end, desc, id, oid, pid); err != nil {
			return err
		}
		if out, err = getEvent(ctx, tx, oid, pid, id); err != nil {
			return err
		}
		return nil
	})
	return out, err
}

func (s *Store) DeleteEvent(ctx context.Context, orgSlug, planSlug string, id int64) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		if _, err := getEvent(ctx, tx, oid, pid, id); err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM event WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid); err != nil {
			return err
		}
		return nil
	})
}
