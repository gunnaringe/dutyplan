package store

import (
	"context"
	"database/sql"
	"errors"
)

// RunJSON is what a run stores about its input and answer: the ids of people and tasks are in all of them.
type RunJSON struct {
	Input, Snapshot string
	Output          sql.NullString
	Hash            string
}

// RunRemapper rewrites the person and task ids in a run (old id -> new id; ids not in a map stay). Package planner sets
// it, because it knows the format; migration 012 needs it for the runs of every plan but an organisation's first.
var RunRemapper func(r RunJSON, people, tasks map[int64]int64) (RunJSON, error)

// remapRuns012 is the Go step of migration 012: the runs of the plans that got their own copies of people and tasks.
func remapRuns012(ctx context.Context, tx *sql.Tx) error {
	maps := map[int64][2]map[int64]int64{} // plan -> people, tasks
	for i, table := range []string{"map_person", "map_task"} {
		rows, err := tx.QueryContext(ctx, `SELECT plan, old, new FROM temp.`+table)
		if err != nil {
			return err
		}
		for rows.Next() {
			var plan, old, new int64
			if err := rows.Scan(&plan, &old, &new); err != nil {
				rows.Close()
				return err
			}
			m, ok := maps[plan]
			if !ok {
				m = [2]map[int64]int64{{}, {}}
				maps[plan] = m
			}
			m[i][old] = new
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return err
		}
	}
	type run struct {
		id, plan int64
		RunJSON
	}
	var runs []run
	rows, err := tx.QueryContext(ctx, `SELECT id, plan_id, input_json, snapshot_json, output_json, input_hash FROM plan_run`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var r run
		if err := rows.Scan(&r.id, &r.plan, &r.Input, &r.Snapshot, &r.Output, &r.Hash); err != nil {
			rows.Close()
			return err
		}
		if _, ok := maps[r.plan]; ok {
			runs = append(runs, r)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}
	if len(runs) > 0 && RunRemapper == nil {
		return errors.New("the runs' ids must be rewritten, and nothing has set store.RunRemapper (import package planner)")
	}
	for _, r := range runs {
		m := maps[r.plan]
		out, err := RunRemapper(r.RunJSON, m[0], m[1])
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE plan_run SET input_json = ?, snapshot_json = ?, output_json = ?, input_hash = ? WHERE id = ?`,
			out.Input, out.Snapshot, out.Output, out.Hash, r.id); err != nil {
			return err
		}
	}
	for _, table := range []string{"map_person", "map_task"} {
		if _, err := tx.ExecContext(ctx, `DROP TABLE temp.`+table); err != nil {
			return err
		}
	}
	return nil
}
