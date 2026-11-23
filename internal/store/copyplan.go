package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Copying a plan into a new one (CreatePlanFrom): its people and tasks always, its table on request. Everything is
// inserted in the order of the old ids, so the new ids keep that order (combinations keep task_a < task_b).

// copySpec is one table to copy: the columns besides org_id and plan_id, which are remapped through which id map (by
// column), and whether it has an id of its own (whose map later tables use).
type copySpec struct {
	table string
	cols  []string
	remap map[string]string // column -> the table whose id map it goes through
	id    bool
	where string // more conditions on the rows copied
}

var (
	setupCopies = []copySpec{
		{table: "person_section", cols: []string{"name", "position", "created_at"}, id: true},
		{table: "person", cols: []string{"name", "proposable", "section_id", "position", "created_at"}, remap: map[string]string{"section_id": "person_section"}, id: true},
		{table: "task_section", cols: []string{"name", "position", "created_at"}, id: true},
		{table: "task", cols: []string{"name", "planned", "position", "created_at", "section_id"}, remap: map[string]string{"section_id": "task_section"}, id: true},
		{table: "task_combination", cols: []string{"task_a", "task_b", "level"}, remap: map[string]string{"task_a": "task", "task_b": "task"}},
		{table: "person_task", cols: []string{"person_id", "task_id", "level"}, remap: map[string]string{"person_id": "person", "task_id": "task"}},
	}
	tableCopies = []copySpec{
		{table: "event", cols: []string{"date", "start_time", "end_time", "created_at"}, id: true},
		{table: "plan_field", cols: []string{"name", "position", "frozen", "created_at"}, id: true},
		{table: "event_field", cols: []string{"event_id", "field_id", "value"}, remap: map[string]string{"event_id": "event", "field_id": "plan_field"}},
		{table: "event_task", cols: []string{"event_id", "task_id", "slots"}, remap: map[string]string{"event_id": "event", "task_id": "task"}},
		// names set by hand; the planner's locked names belong to the drafts, which are not copied
		{table: "manual_cell", cols: []string{"event_id", "task_id", "person_id", "label", "fills", "locked", "created_at"},
			remap: map[string]string{"event_id": "event", "task_id": "task", "person_id": "person"}, id: true, where: "locked = 0"},
		{table: "pool_member", cols: []string{"event_id", "task_id", "person_id"}, remap: map[string]string{"event_id": "event", "task_id": "task", "person_id": "person"}, id: true},
		{table: "person_event", cols: []string{"person_id", "event_id", "level"}, remap: map[string]string{"person_id": "person", "event_id": "event"}},
		{table: "answer_note", cols: []string{"person_id", "event_id", "note"}, remap: map[string]string{"person_id": "person", "event_id": "event"}},
	}
)

// copyPlan copies plan from into plan to (both the organisation's).
func copyPlan(ctx context.Context, tx *sql.Tx, oid, from, to int64, table bool) error {
	specs := setupCopies
	if table {
		specs = append(append([]copySpec{}, setupCopies...), tableCopies...)
		if _, err := tx.ExecContext(ctx, `UPDATE plan SET weights = (SELECT weights FROM plan WHERE id = ?) WHERE id = ?`, from, to); err != nil {
			return err
		}
	}
	maps := map[string]map[int64]int64{}
	for _, c := range specs {
		m, err := copyRows(ctx, tx, c, oid, from, to, maps)
		if err != nil {
			return fmt.Errorf("copy %s: %w", c.table, err)
		}
		maps[c.table] = m
	}
	return nil
}

func copyRows(ctx context.Context, tx *sql.Tx, c copySpec, oid, from, to int64, maps map[string]map[int64]int64) (map[int64]int64, error) {
	cols := strings.Join(c.cols, ", ")
	sel := cols
	if c.id {
		sel = "id, " + cols
	}
	where := "plan_id = ?"
	if c.where != "" {
		where += " AND " + c.where
	}
	rows, err := tx.QueryContext(ctx, `SELECT `+sel+` FROM `+c.table+` WHERE `+where+` ORDER BY rowid`, from)
	if err != nil {
		return nil, err
	}
	var all [][]any
	for rows.Next() {
		vals := make([]any, strings.Count(sel, ",")+1)
		ptrs := make([]any, len(vals))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			rows.Close()
			return nil, err
		}
		all = append(all, vals)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	insert := `INSERT INTO ` + c.table + ` (org_id, plan_id, ` + cols + `) VALUES (?, ?` + strings.Repeat(", ?", len(c.cols)) + `)`
	out := map[int64]int64{}
	for _, vals := range all {
		var old int64
		if c.id {
			old, vals = vals[0].(int64), vals[1:]
		}
		args := []any{oid, to}
		for i, v := range vals {
			if through, ok := c.remap[c.cols[i]]; ok && v != nil {
				n, ok := maps[through][v.(int64)]
				if !ok {
					return nil, fmt.Errorf("%s %v is not in the plan", c.cols[i], v)
				}
				v = n
			}
			args = append(args, v)
		}
		res, err := tx.ExecContext(ctx, insert, args...)
		if err != nil {
			return nil, err
		}
		if c.id {
			out[old], _ = res.LastInsertId()
		}
	}
	return out, nil
}

// claimLeftovers gives an organisation's first plan the people and tasks it had before plans had their own (rows with no
// plan, from migration 012).
func claimLeftovers(ctx context.Context, tx *sql.Tx, oid, pid int64) error {
	for _, table := range []string{"person_section", "person", "task_section", "task", "task_combination", "person_task"} {
		if _, err := tx.ExecContext(ctx, `UPDATE `+table+` SET plan_id = ? WHERE org_id = ? AND plan_id IS NULL`, pid, oid); err != nil {
			return err
		}
	}
	return nil
}
