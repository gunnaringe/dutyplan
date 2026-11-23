package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"unicode/utf8"
)

// A note on one answer (person × event of a plan), shown as a tab in the cell's corner (2026-11-22, the user). Its own
// table (answer_note), so it stays when the answer is cleared; seen by everyone who sees the answers.

// MaxNoteRunes is how long a note may be.
const MaxNoteRunes = 300

// AnswerNote is a note on one person's answer for one event.
type AnswerNote struct {
	PersonID int64  `json:"person_id"`
	EventID  int64  `json:"event_id"`
	Note     string `json:"note"`
}

// SetAnswerNote sets the note on a person's answer for one event; an empty note removes it.
func (s *Store) SetAnswerNote(ctx context.Context, orgSlug, planSlug string, eventID, personID int64, note string) error {
	note = strings.TrimSpace(note)
	if utf8.RuneCountInString(note) > MaxNoteRunes || !validText(strings.NewReplacer("\n", " ", "\t", " ").Replace(note)) {
		return Invalid("note_invalid", map[string]any{"max": MaxNoteRunes}, "a note must be at most %d printable characters", MaxNoteRunes)
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
		if _, err := getPerson(ctx, tx, pid, personID); err != nil {
			return err
		}
		if err := eventInPlan(ctx, tx, oid, pid, eventID); err != nil {
			return err
		}
		var before string
		err = tx.QueryRowContext(ctx, `SELECT note FROM answer_note WHERE person_id = ? AND event_id = ? AND org_id = ?`, personID, eventID, oid).Scan(&before)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if before == note {
			return errNothingToChange()
		}
		if note == "" {
			_, err = tx.ExecContext(ctx, `DELETE FROM answer_note WHERE person_id = ? AND event_id = ? AND org_id = ?`, personID, eventID, oid)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO answer_note (org_id, plan_id, person_id, event_id, note) VALUES (?, ?, ?, ?, ?)
				ON CONFLICT (person_id, event_id) DO UPDATE SET note = excluded.note`, oid, pid, personID, eventID, note)
		}
		return err
	})
}
