package store

import (
	"context"
	"database/sql"
)

// Caps on what one organisation, plan or person can hold, so one sign-up cannot fill the server's disk or slow it for
// everyone (anyone may make an account and organisations). Far above what a group needs; variables so the tests can lower
// them.
var (
	MaxOwnedOrgs     = 20  // organisations one person owns
	MaxPlansPerOrg   = 100 //
	MaxPeoplePerPlan = 1000
	MaxTasksPerPlan  = 200
	MaxEventsPerPlan = 1000
	MaxLinksPerOrg   = 200
)

// atMost refuses (code «too_many_<what>») when the query counts max or more already.
func atMost(ctx context.Context, tx *sql.Tx, max int, what, query string, args ...any) error {
	var n int
	if err := tx.QueryRowContext(ctx, query, args...).Scan(&n); err != nil {
		return err
	}
	if n >= max {
		return Invalid("too_many_"+what, map[string]any{"max": max}, "at most %d %s", max, what)
	}
	return nil
}
