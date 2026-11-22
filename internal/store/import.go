package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// Import puts a whole plan into an organisation in one go, for example a semester moved from a spreadsheet (2026-11-22,
// the user: "import the Vår 2027 tab"). All or nothing: every write joins one transaction, with the same checks and limits
// as on the page. The plan must be new; tasks are matched by name (case does not matter) and only the missing ones are
// made. No people and no answers: those are for the people themselves. The format is in docs/llms.md.
type Import struct {
	Plan   string        `json:"plan"`
	Tasks  []string      `json:"tasks"`
	Fields []string      `json:"fields"`
	Events []ImportEvent `json:"events"`
}

// ImportEvent is one event of an Import: places per task name, details per field name.
type ImportEvent struct {
	Date        string            `json:"date"`
	StartTime   string            `json:"start_time"`
	EndTime     string            `json:"end_time"`
	Description string            `json:"description"`
	Fields      map[string]string `json:"fields"`
	Places      map[string]int    `json:"places"`
}

// ImportResult says what an Import made.
type ImportResult struct {
	Plan     Plan     `json:"plan"`
	NewTasks []string `json:"new_tasks"`
	Events   int      `json:"events"`
	Places   int      `json:"places"`
}

// With dryRun everything is checked and made, then taken back: nothing is stored.
func (s *Store) Import(ctx context.Context, orgSlug string, in Import, dryRun bool) (ImportResult, error) {
	var out ImportResult
	if len(in.Events) == 0 {
		return out, Invalid("import_empty", nil, "an import needs at least one event")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	ctx = context.WithValue(ctx, txKey{}, tx)

	tasks := map[string]int64{}
	if err := scanAll(ctx, tx, `SELECT t.id, t.name FROM task t JOIN organization o ON o.id = t.org_id WHERE o.slug = ?`, []any{orgSlug},
		func(r *sql.Rows) error {
			var id int64
			var name string
			err := r.Scan(&id, &name)
			tasks[strings.ToLower(name)] = id
			return err
		}); err != nil {
		return out, err
	}
	if out.Plan, err = s.CreatePlan(ctx, orgSlug, in.Plan); err != nil {
		return out, err
	}
	for _, name := range in.Tasks {
		if _, ok := tasks[strings.ToLower(strings.TrimSpace(name))]; ok {
			continue
		}
		t, err := s.CreateTask(ctx, orgSlug, name)
		if err != nil {
			return out, err
		}
		tasks[strings.ToLower(t.Name)] = t.ID
		out.NewTasks = append(out.NewTasks, t.Name)
	}
	fields := map[string]int64{}
	for _, name := range in.Fields {
		f, err := s.AddField(ctx, orgSlug, out.Plan.Slug, name)
		if err != nil {
			return out, err
		}
		fields[strings.ToLower(f.Name)] = f.ID
	}
	for i, e := range in.Events {
		where := fmt.Sprintf("event %d (%s)", i+1, e.Date)
		evs, err := s.AddEvents(ctx, orgSlug, out.Plan.Slug, NewEvents{Date: e.Date, StartTime: e.StartTime, EndTime: e.EndTime})
		if err != nil {
			return out, fmt.Errorf("%s: %w", where, err)
		}
		ev := evs[0]
		if strings.TrimSpace(e.Description) != "" {
			if _, err := s.UpdateEvent(ctx, orgSlug, out.Plan.Slug, ev.ID, EventPatch{Description: &e.Description}); err != nil {
				return out, fmt.Errorf("%s: %w", where, err)
			}
		}
		for name, value := range e.Fields {
			id, ok := fields[strings.ToLower(strings.TrimSpace(name))]
			if !ok {
				return out, Invalid("import_unknown", map[string]any{"name": name}, "%s: the field %q is not in fields", where, name)
			}
			if strings.TrimSpace(value) == "" {
				continue
			}
			if err := s.SetFieldValue(ctx, orgSlug, out.Plan.Slug, ev.ID, id, value); err != nil {
				return out, fmt.Errorf("%s: %w", where, err)
			}
		}
		for name, n := range e.Places {
			id, ok := tasks[strings.ToLower(strings.TrimSpace(name))]
			if !ok {
				return out, Invalid("import_unknown", map[string]any{"name": name}, "%s: the task %q is not in tasks", where, name)
			}
			if n == 0 {
				continue
			}
			if err := s.SetEventPlaces(ctx, orgSlug, out.Plan.Slug, ev.ID, id, n); err != nil {
				return out, fmt.Errorf("%s: %w", where, err)
			}
			out.Places++
		}
		out.Events++
	}
	if dryRun {
		return out, nil
	}
	return out, tx.Commit()
}
