package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

const maxRunsPerPlan = 20 // older runs are pruned when a new one starts (a pinned run is kept)

type Run struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"` // running | done | failed
	Seed       int    `json:"seed"`
	TimeLimit  int    `json:"time_limit"`
	InputHash  string `json:"-"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
	Pinned     bool   `json:"pinned"` // the plan people see under «My assignments» (at most one per plan)
}

// RunFull adds the stored documents.
type RunFull struct {
	Run
	InputJSON    []byte
	SnapshotJSON []byte
	OutputJSON   []byte
}

func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// CreateRun records that a run has started. Older runs of the plan beyond the newest maxRunsPerPlan are removed.
func (s *Store) CreateRun(ctx context.Context, orgSlug, planSlug string, seed, timeLimit int, inputHash string, input, snapshot []byte) (int64, error) {
	var id int64
	err := s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO plan_run (org_id, plan_id, status, seed, time_limit, input_hash, input_json, snapshot_json, started_at)
			VALUES (?, ?, 'running', ?, ?, ?, ?, ?, ?)`, oid, pid, seed, timeLimit, inputHash, string(input), string(snapshot), s.stamp())
		if err != nil {
			return err
		}
		id, _ = res.LastInsertId()
		if _, err := tx.ExecContext(ctx, `DELETE FROM plan_run WHERE plan_id = ? AND pinned = 0 AND id NOT IN
			(SELECT id FROM plan_run WHERE plan_id = ? ORDER BY id DESC LIMIT ?)`, pid, pid, maxRunsPerPlan); err != nil {
			return err
		}
		return nil
	})
	return id, err
}

// FinishRun stores the solver's answer (output) or the reason it failed (errMsg), once.
func (s *Store) FinishRun(ctx context.Context, runID int64, output []byte, errMsg string) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		var oid, pid int64
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT org_id, plan_id, status FROM plan_run WHERE id = ?`, runID).Scan(&oid, &pid, &status); errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		} else if err != nil {
			return err
		}
		if status != "running" {
			return fmt.Errorf("%w: run %d is already %s", ErrConflict, runID, status)
		}
		if errMsg != "" {
			errMsg = clip(errMsg, 500)
			if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET status = 'failed', error = ?, finished_at = ? WHERE id = ?`, errMsg, s.stamp(), runID); err != nil {
				return err
			}
		} else if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET status = 'done', output_json = ?, finished_at = ? WHERE id = ?`, string(output), s.stamp(), runID); err != nil {
			return err
		}
		return nil
	})
}

// FailRunningRuns marks runs that were interrupted (the server stopped) as failed. Called at startup.
func (s *Store) FailRunningRuns(ctx context.Context) (int, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM plan_run WHERE status = 'running'`)
	if err != nil {
		return 0, err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return 0, err
		}
		ids = append(ids, id)
	}
	rows.Close()
	for _, id := range ids {
		if err := s.FinishRun(ctx, id, nil, "interrupted because the app was stopped"); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func scanRun(sc interface{ Scan(...any) error }, full bool) (RunFull, error) {
	var r RunFull
	var errMsg, finished, input, snap, out sql.NullString
	var pinned int
	dest := []any{&r.ID, &r.Status, &r.Seed, &r.TimeLimit, &r.InputHash, &errMsg, &r.StartedAt, &finished, &pinned}
	if full {
		dest = append(dest, &input, &snap, &out)
	}
	if err := sc.Scan(dest...); err != nil {
		return r, err
	}
	r.Error, r.FinishedAt, r.Pinned = errMsg.String, finished.String, pinned == 1
	r.InputJSON, r.SnapshotJSON, r.OutputJSON = []byte(input.String), []byte(snap.String), []byte(out.String)
	return r, nil
}

const runCols = `id, status, seed, time_limit, input_hash, error, started_at, finished_at, pinned`

// GetRun reads one run of this plan. A run of another plan or organisation is not found.
func (s *Store) GetRun(ctx context.Context, orgSlug, planSlug string, id int64) (RunFull, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return RunFull{}, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return RunFull{}, err
	}
	r, err := scanRun(s.db.QueryRowContext(ctx, `SELECT `+runCols+`, input_json, snapshot_json, output_json
		FROM plan_run WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid), true)
	if errors.Is(err, sql.ErrNoRows) {
		return RunFull{}, ErrNotFound
	}
	return r, err
}

// ListRuns returns the plan's newest runs and its pinned run (which may be older), newest first, without the stored documents.
func (s *Store) ListRuns(ctx context.Context, orgSlug, planSlug string, limit int) ([]Run, error) {
	if limit < 1 || limit > maxRunsPerPlan {
		limit = maxRunsPerPlan
	}
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return nil, err
	}
	out := []Run{}
	err = scanAll(ctx, s.db, `SELECT `+runCols+` FROM plan_run WHERE org_id = ? AND plan_id = ? AND (pinned = 1 OR id IN (SELECT id FROM plan_run WHERE org_id = ? AND plan_id = ? ORDER BY id DESC LIMIT ?)) ORDER BY id DESC`, []any{oid, pid, oid, pid, limit},
		func(r *sql.Rows) error {
			run, err := scanRun(r, false)
			out = append(out, run.Run)
			return err
		})
	return out, err
}

// NextSeed is one more than the plan's newest run (1 for the first), so «try again» gives a different plan.
func (s *Store) NextSeed(ctx context.Context, orgSlug, planSlug string) (int, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return 0, err
	}
	pid, err := planID(ctx, s.db, oid, planSlug)
	if err != nil {
		return 0, err
	}
	var last sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `SELECT MAX(seed) FROM plan_run WHERE org_id = ? AND plan_id = ?`, oid, pid).Scan(&last); err != nil {
		return 0, err
	}
	return int(last.Int64) + 1, nil
}

// PinRun makes a finished run the plan's current plan (pin) or lets the plan go back to showing its newest one (unpin).
// Only one run of a plan is pinned: pinning another moves the pin. A run that is still going or failed is not a plan.
func (s *Store) PinRun(ctx context.Context, orgSlug, planSlug string, runID int64, pin bool) error {
	return s.write(ctx, func(tx *sql.Tx) error {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return err
		}
		var status string
		var pinned int
		err = tx.QueryRowContext(ctx, `SELECT status, pinned FROM plan_run WHERE id = ? AND org_id = ? AND plan_id = ?`, runID, oid, pid).Scan(&status, &pinned)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if (pinned == 1) == pin {
			return errNothingToChange()
		}
		if pin {
			if status != "done" {
				return Invalid("run_not_done", nil, "only a finished plan can be pinned")
			}
			if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET pinned = 0 WHERE org_id = ? AND plan_id = ?`, oid, pid); err != nil {
				return err
			}
		}
		set := 0
		if pin {
			set = 1
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET pinned = ? WHERE id = ? AND org_id = ? AND plan_id = ?`, set, runID, oid, pid); err != nil {
			return err
		}
		return nil
	})
}
