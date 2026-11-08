package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"unicode/utf8"
)

const maxRunsPerPlan = 20 // older runs are pruned when a new one starts

type Run struct {
	ID         int64  `json:"id"`
	Status     string `json:"status"` // running | done | failed
	Seed       int    `json:"seed"`
	TimeLimit  int    `json:"time_limit"`
	Actor      string `json:"actor"`
	InputHash  string `json:"-"`
	Error      string `json:"error,omitempty"`
	StartedAt  string `json:"started_at"`
	FinishedAt string `json:"finished_at,omitempty"`
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
func (s *Store) CreateRun(ctx context.Context, orgSlug, planSlug, actor string, seed, timeLimit int, inputHash string, input, snapshot []byte) (int64, error) {
	var id int64
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		pid, err := planID(ctx, tx, oid, planSlug)
		if err != nil {
			return logEntry{}, err
		}
		clean, _ := cleanActor(actor)
		res, err := tx.ExecContext(ctx, `INSERT INTO plan_run (org_id, plan_id, status, seed, time_limit, actor, input_hash, input_json, snapshot_json, started_at)
			VALUES (?, ?, 'running', ?, ?, ?, ?, ?, ?, ?)`, oid, pid, seed, timeLimit, clean, inputHash, string(input), string(snapshot), s.stamp())
		if err != nil {
			return logEntry{}, err
		}
		id, _ = res.LastInsertId()
		if _, err := tx.ExecContext(ctx, `DELETE FROM plan_run WHERE plan_id = ? AND id NOT IN
			(SELECT id FROM plan_run WHERE plan_id = ? ORDER BY id DESC LIMIT ?)`, pid, pid, maxRunsPerPlan); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: "plan.run", detail: fmt.Sprintf("kjøring #%d, frø %d, %d s", id, seed, timeLimit)}, nil
	})
	return id, err
}

// FinishRun stores the solver's answer (output) or the reason it failed (errMsg), once. The system is the actor.
func (s *Store) FinishRun(ctx context.Context, runID int64, output []byte, errMsg string) error {
	return s.write(ctx, "Planleggeren", func(tx *sql.Tx) (logEntry, error) {
		var oid, pid int64
		var status string
		if err := tx.QueryRowContext(ctx, `SELECT org_id, plan_id, status FROM plan_run WHERE id = ?`, runID).Scan(&oid, &pid, &status); errors.Is(err, sql.ErrNoRows) {
			return logEntry{}, ErrNotFound
		} else if err != nil {
			return logEntry{}, err
		}
		if status != "running" {
			return logEntry{}, fmt.Errorf("%w: run %d is already %s", ErrConflict, runID, status)
		}
		action, detail := "plan.run.done", fmt.Sprintf("kjøring #%d er ferdig", runID)
		if errMsg != "" {
			errMsg = clip(errMsg, 500)
			if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET status = 'failed', error = ?, finished_at = ? WHERE id = ?`, errMsg, s.stamp(), runID); err != nil {
				return logEntry{}, err
			}
			action, detail = "plan.run.failed", fmt.Sprintf("kjøring #%d feilet: %s", runID, clip(errMsg, 120))
		} else if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET status = 'done', output_json = ?, finished_at = ? WHERE id = ?`, string(output), s.stamp(), runID); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, planID: &pid, action: action, detail: detail}, nil
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
		if err := s.FinishRun(ctx, id, nil, "avbrutt fordi appen ble stoppet"); err != nil {
			return 0, err
		}
	}
	return len(ids), nil
}

func scanRun(sc interface{ Scan(...any) error }, full bool) (RunFull, error) {
	var r RunFull
	var errMsg, finished, input, snap, out sql.NullString
	dest := []any{&r.ID, &r.Status, &r.Seed, &r.TimeLimit, &r.Actor, &r.InputHash, &errMsg, &r.StartedAt, &finished}
	if full {
		dest = append(dest, &input, &snap, &out)
	}
	if err := sc.Scan(dest...); err != nil {
		return r, err
	}
	r.Error, r.FinishedAt = errMsg.String, finished.String
	r.InputJSON, r.SnapshotJSON, r.OutputJSON = []byte(input.String), []byte(snap.String), []byte(out.String)
	return r, nil
}

const runCols = `id, status, seed, time_limit, actor, input_hash, error, started_at, finished_at`

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

// ListRuns returns the plan's newest runs, newest first, without the stored documents.
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
	err = scanAll(ctx, s.db, `SELECT `+runCols+` FROM plan_run WHERE org_id = ? AND plan_id = ? ORDER BY id DESC LIMIT ?`, []any{oid, pid, limit},
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
