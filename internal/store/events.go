package store

import (
	"context"
	"database/sql"
	"errors"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// Limits for adding events several at a time (every week or every few weeks until a date, or a copy to several dates), so
// a typo cannot create a flood of rows. maxEveryWeeks is the longest step the page offers («every 8 weeks»).
const (
	maxRepeatDays    = 366
	maxEventsPerCall = 200
	maxEveryWeeks    = 8
)

// Limits for the free text of an event.
const maxDescriptionRunes = 2000

// Event is one occasion in a plan: a date, and optionally a time of day and a description. It is the unit that people are
// available for. What else is said about it is in the plan's details rows (event_field), and its places in event_task.
type Event struct {
	ID          int64  `json:"id"`
	Date        string `json:"date"`        // YYYY-MM-DD
	StartTime   string `json:"start_time"`  // HH:MM, 24-hour, or "" when not set
	EndTime     string `json:"end_time"`    // HH:MM after StartTime, or "" (never set without a start time)
	Description string `json:"description"` // longer text, several lines allowed
}

// eventCols are the columns of an Event for a query on «event d»; scanEvent reads them. eventOrder puts the events of a date
// in time order, those with a start time first, the rest in the order they were created.
const (
	eventCols  = `d.id, d.date, d.start_time, d.end_time, d.description`
	eventOrder = `d.date, CASE WHEN d.start_time = '' THEN 1 ELSE 0 END, d.start_time, d.id`
)

func scanEvent(sc interface{ Scan(...any) error }) (Event, error) {
	var e Event
	err := sc.Scan(&e.ID, &e.Date, &e.StartTime, &e.EndTime, &e.Description)
	return e, err
}

func parseISODate(s string) (time.Time, bool) {
	t, err := time.Parse("2006-01-02", s)
	return t, err == nil && t.Format("2006-01-02") == s
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
		FROM event d
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
		FROM event d
		WHERE d.id = ? AND d.org_id = ? AND d.plan_id = ?`, id, oid, pid))
	if errors.Is(err, sql.ErrNoRows) {
		return d, ErrNotFound
	}
	return d, err
}

// repeatEvery gives every date in dates, and with until each of them again every `every` weeks (0 means 1) up to and
// including until, in order and without repeats. The dates must be YYYY-MM-DD, until on or after each of them and at most
// maxRepeatDays later. Only the dates are made: nothing remembers the rhythm, and each event is on its own afterwards.
func repeatEvery(dates []string, until string, every int) ([]string, error) {
	if len(dates) == 0 {
		return nil, Invalid("no_dates", nil, "give at least one date")
	}
	if every == 0 {
		every = 1
	}
	if every < 1 || every > maxEveryWeeks {
		return nil, Invalid("every_invalid", map[string]any{"max": maxEveryWeeks}, "every must be 1 to %d weeks", maxEveryWeeks)
	}
	var last time.Time
	if until != "" {
		u, ok := parseISODate(until)
		if !ok {
			return nil, Invalid("date_invalid", nil, "date must be YYYY-MM-DD")
		}
		last = u
	}
	seen := map[string]bool{}
	var out []string
	for _, d := range dates {
		day, ok := parseISODate(d)
		if !ok {
			return nil, Invalid("date_invalid", nil, "date must be YYYY-MM-DD")
		}
		if until == "" {
			if !seen[d] {
				seen[d] = true
				out = append(out, d)
			}
			continue
		}
		if last.Before(day) {
			return nil, Invalid("range_invalid", nil, "the last date must be on or after the first")
		}
		if last.Sub(day) > maxRepeatDays*24*time.Hour {
			return nil, Invalid("range_too_long", map[string]any{"max": maxRepeatDays}, "the range can be at most %d days", maxRepeatDays)
		}
		for x := day; !x.After(last); x = x.AddDate(0, 0, 7*every) {
			if k := x.Format("2006-01-02"); !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	if len(out) > maxEventsPerCall {
		return nil, Invalid("too_many_dates", map[string]any{"max": maxEventsPerCall}, "at most %d dates at a time", maxEventsPerCall)
	}
	sort.Strings(out)
	return out, nil
}

// NewEvents is what AddEvents takes: a date (and with Until the same weekday every week, or every Every weeks, up to and
// including that date), and optionally a time of day. The places, the details and the description are set afterwards,
// or come with a copy.
type NewEvents struct {
	Date      string `json:"date"`
	Until     string `json:"until"`
	Every     int    `json:"every"`
	StartTime string `json:"start_time"`
	EndTime   string `json:"end_time"`
}

// AddEvent adds one event on a date, with nothing else.
func (s *Store) AddEvent(ctx context.Context, orgSlug, planSlug, date string) (Event, error) {
	evs, err := s.AddEvents(ctx, orgSlug, planSlug, NewEvents{Date: date})
	if err != nil {
		return Event{}, err
	}
	return evs[0], nil
}

// AddEvents adds the events, all or nothing. Events are independent: a date may already have others.
func (s *Store) AddEvents(ctx context.Context, orgSlug, planSlug string, in NewEvents) ([]Event, error) {
	dates, err := repeatEvery([]string{in.Date}, in.Until, in.Every)
	if err != nil {
		return nil, err
	}
	start, end, err := cleanTimes(in.StartTime, in.EndTime)
	if err != nil {
		return nil, err
	}
	var out []Event
	err = s.write(ctx, func(tx *sql.Tx) error {
		out = nil
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		for _, d := range dates {
			e, err := s.insertEvent(ctx, tx, oid, pid, Event{Date: d, StartTime: start, EndTime: end})
			if err != nil {
				return err
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

func (s *Store) insertEvent(ctx context.Context, tx *sql.Tx, oid, pid int64, e Event) (Event, error) {
	res, err := tx.ExecContext(ctx, `INSERT INTO event (org_id, plan_id, date, start_time, end_time, description, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		oid, pid, e.Date, e.StartTime, e.EndTime, e.Description, s.stamp())
	if err != nil {
		return Event{}, err
	}
	e.ID, _ = res.LastInsertId()
	return e, nil
}

// CopyOf says what a copy of an event takes along. Dates are the new dates (and with Until each of them every week, or
// every Every weeks, up to and including that date). Times copies the start and end time; without it the copies get StartTime and EndTime.
// Details copies the values of the details rows and the description, Places the places, and Hand the cells set by hand
// (never the planner's names that were locked: a copy has no draft yet). Answers are never copied.
type CopyOf struct {
	Dates     []string `json:"dates"`
	Until     string   `json:"until"`
	Every     int      `json:"every"`
	Times     bool     `json:"times"`
	StartTime string   `json:"start_time"`
	EndTime   string   `json:"end_time"`
	Details   bool     `json:"details"`
	Places    bool     `json:"places"`
	Hand      bool     `json:"hand"`
}

// CopyEvent makes copies of an event on other dates, all or nothing, and returns them in date order.
func (s *Store) CopyEvent(ctx context.Context, orgSlug, planSlug string, id int64, in CopyOf) ([]Event, error) {
	dates, err := repeatEvery(in.Dates, in.Until, in.Every)
	if err != nil {
		return nil, err
	}
	start, end := in.StartTime, in.EndTime
	if !in.Times {
		if start, end, err = cleanTimes(start, end); err != nil {
			return nil, err
		}
	}
	var out []Event
	err = s.write(ctx, func(tx *sql.Tx) error {
		out = nil
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		src, err := getEvent(ctx, tx, oid, pid, id)
		if err != nil {
			return err
		}
		tmpl := Event{StartTime: start, EndTime: end}
		if in.Times {
			tmpl.StartTime, tmpl.EndTime = src.StartTime, src.EndTime
		}
		if in.Details {
			tmpl.Description = src.Description
		}
		for _, d := range dates {
			e := tmpl
			e.Date = d
			if e, err = s.insertEvent(ctx, tx, oid, pid, e); err != nil {
				return err
			}
			if in.Details {
				if _, err := tx.ExecContext(ctx, `INSERT INTO event_field (org_id, plan_id, event_id, field_id, value)
					SELECT org_id, plan_id, ?, field_id, value FROM event_field WHERE event_id = ? AND org_id = ? AND plan_id = ?`, e.ID, src.ID, oid, pid); err != nil {
					return err
				}
			}
			if in.Places {
				if _, err := tx.ExecContext(ctx, `INSERT INTO event_task (org_id, plan_id, event_id, task_id, slots)
					SELECT org_id, plan_id, ?, task_id, slots FROM event_task WHERE event_id = ? AND org_id = ? AND plan_id = ?`, e.ID, src.ID, oid, pid); err != nil {
					return err
				}
			}
			if in.Hand {
				if _, err := tx.ExecContext(ctx, `INSERT INTO manual_cell (org_id, plan_id, event_id, task_id, person_id, label, fills, locked, created_at)
					SELECT org_id, plan_id, ?, task_id, person_id, label, fills, 0, ? FROM manual_cell
					WHERE event_id = ? AND org_id = ? AND plan_id = ? AND locked = 0`, e.ID, s.stamp(), src.ID, oid, pid); err != nil {
					return err
				}
			}
			out = append(out, e)
		}
		return nil
	})
	return out, err
}

// EventPatch changes the time of day or the description of an event. The date stays: copy the event to move it.
type EventPatch struct {
	StartTime   *string `json:"start_time"`
	EndTime     *string `json:"end_time"`
	Description *string `json:"description"`
}

func (s *Store) UpdateEvent(ctx context.Context, orgSlug, planSlug string, id int64, patch EventPatch) (Event, error) {
	var out Event
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
		changed := false
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
				changed = true
			}
		}
		if patch.Description != nil && *patch.Description != desc {
			desc = *patch.Description
			changed = true
		}
		if !changed {
			return errNothingToChange()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE event SET start_time = ?, end_time = ?, description = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, start, end, desc, id, oid, pid); err != nil {
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
