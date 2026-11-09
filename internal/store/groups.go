package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// GroupTask: a task the planner may propose the group for, how many places the group fills and how willing it is.
type GroupTask struct {
	TaskID int64  `json:"task_id"`
	Fills  int    `json:"fills"`
	Level  string `json:"level"` // poor | neutral | good
}

// Group is a named set of people (a team, a choir, a couple). It has no rules of its own.
type Group struct {
	ID         int64       `json:"id"`
	Name       string      `json:"name"`
	Proposable bool        `json:"proposable"`
	Members    []int64     `json:"members"`
	Tasks      []GroupTask `json:"tasks"`
}

var groupTaskLevels = map[string]bool{"poor": true, "neutral": true, "good": true}

// listGroups reads all groups of the organisation with their members and tasks, ordered by name.
func listGroups(ctx context.Context, q dbq, oid int64) ([]Group, error) {
	out := []Group{}
	byID := map[int64]*Group{}
	if err := scanAll(ctx, q, `SELECT id, name, proposable FROM grp WHERE org_id = ? ORDER BY name, id`, []any{oid},
		func(r *sql.Rows) error {
			g := Group{Members: []int64{}, Tasks: []GroupTask{}}
			err := r.Scan(&g.ID, &g.Name, &g.Proposable)
			out = append(out, g)
			return err
		}); err != nil {
		return nil, err
	}
	for i := range out {
		byID[out[i].ID] = &out[i]
	}
	if err := scanAll(ctx, q, `SELECT group_id, person_id FROM grp_member WHERE org_id = ? ORDER BY group_id, person_id`, []any{oid},
		func(r *sql.Rows) error {
			var g, p int64
			if err := r.Scan(&g, &p); err != nil {
				return err
			}
			if x := byID[g]; x != nil {
				x.Members = append(x.Members, p)
			}
			return nil
		}); err != nil {
		return nil, err
	}
	if err := scanAll(ctx, q, `SELECT group_id, task_id, fills, level FROM grp_task WHERE org_id = ? ORDER BY group_id, task_id`, []any{oid},
		func(r *sql.Rows) error {
			var g int64
			var t GroupTask
			if err := r.Scan(&g, &t.TaskID, &t.Fills, &t.Level); err != nil {
				return err
			}
			if x := byID[g]; x != nil {
				x.Tasks = append(x.Tasks, t)
			}
			return nil
		}); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) ListGroups(ctx context.Context, orgSlug string) ([]Group, error) {
	oid, err := orgID(ctx, s.db, orgSlug)
	if err != nil {
		return nil, err
	}
	return listGroups(ctx, s.db, oid)
}

// groupIn returns one group of the organisation (ErrNotFound for anyone else's).
func groupIn(ctx context.Context, q dbq, oid, id int64) (Group, error) {
	all, err := listGroups(ctx, q, oid)
	if err != nil {
		return Group{}, err
	}
	for _, g := range all {
		if g.ID == id {
			return g, nil
		}
	}
	return Group{}, ErrNotFound
}

func (s *Store) CreateGroup(ctx context.Context, orgSlug, actor, name string) (Group, error) {
	name, err := cleanName(name, "name")
	if err != nil {
		return Group{}, err
	}
	g := Group{Name: name, Proposable: true, Members: []int64{}, Tasks: []GroupTask{}}
	err = s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		res, err := tx.ExecContext(ctx, `INSERT INTO grp (org_id, name, created_at) VALUES (?, ?, ?)`, oid, name, s.stamp())
		if isUnique(err) {
			return logEntry{}, exists("group", name)
		}
		if err != nil {
			return logEntry{}, err
		}
		g.ID, _ = res.LastInsertId()
		return logEntry{orgID: oid, action: "group.create", detail: name}, nil
	})
	return g, err
}

// GroupPatch changes only the fields that are set.
type GroupPatch struct {
	Name       *string `json:"name"`
	Proposable *bool   `json:"proposable"`
}

func (s *Store) UpdateGroup(ctx context.Context, orgSlug, actor string, id int64, patch GroupPatch) (Group, error) {
	if patch.Name != nil {
		n, err := cleanName(*patch.Name, "name")
		if err != nil {
			return Group{}, err
		}
		patch.Name = &n
	}
	var g Group
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		if g, err = groupIn(ctx, tx, oid, id); err != nil {
			return logEntry{}, err
		}
		var changes []string
		if patch.Name != nil && *patch.Name != g.Name {
			changes = append(changes, fmt.Sprintf("name %q -> %q", g.Name, *patch.Name))
			g.Name = *patch.Name
		}
		if patch.Proposable != nil && *patch.Proposable != g.Proposable {
			changes = append(changes, fmt.Sprintf("proposable %t -> %t", g.Proposable, *patch.Proposable))
			g.Proposable = *patch.Proposable
		}
		if len(changes) == 0 {
			return logEntry{}, errNothingToChange()
		}
		_, err = tx.ExecContext(ctx, `UPDATE grp SET name = ?, proposable = ? WHERE id = ? AND org_id = ?`, g.Name, g.Proposable, id, oid)
		if isUnique(err) {
			return logEntry{}, exists("group", g.Name)
		}
		if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "group.update", detail: fmt.Sprintf("#%d: %s", id, strings.Join(changes, "; "))}, nil
	})
	return g, err
}

// SetGroupMembers replaces the member list. Every person must be the organisation's own; an inactive person may be a
// member (the planner leaves them out while they are inactive).
func (s *Store) SetGroupMembers(ctx context.Context, orgSlug, actor string, id int64, people []int64) (Group, error) {
	set := map[int64]bool{}
	for _, p := range people {
		set[p] = true
	}
	var g Group
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		old, err := groupIn(ctx, tx, oid, id)
		if err != nil {
			return logEntry{}, err
		}
		names := map[int64]string{}
		for p := range set {
			if err := inOrg(ctx, tx, "person", oid, p); err != nil {
				return logEntry{}, err
			}
		}
		if err := scanAll(ctx, tx, `SELECT id, name FROM person WHERE org_id = ?`, []any{oid}, func(r *sql.Rows) error {
			var pid int64
			var n string
			err := r.Scan(&pid, &n)
			names[pid] = n
			return err
		}); err != nil {
			return logEntry{}, err
		}
		was := map[int64]bool{}
		var removed, added []string
		for _, m := range old.Members {
			was[m] = true
			if !set[m] {
				removed = append(removed, names[m])
			}
		}
		for p := range set {
			if !was[p] {
				added = append(added, names[p])
			}
		}
		if len(removed)+len(added) == 0 {
			return logEntry{}, errNothingToChange()
		}
		sort.Strings(added)
		sort.Strings(removed)
		if _, err := tx.ExecContext(ctx, `DELETE FROM grp_member WHERE group_id = ? AND org_id = ?`, id, oid); err != nil {
			return logEntry{}, err
		}
		for p := range set {
			if _, err := tx.ExecContext(ctx, `INSERT INTO grp_member (org_id, group_id, person_id) VALUES (?, ?, ?)`, oid, id, p); err != nil {
				return logEntry{}, err
			}
		}
		if g, err = groupIn(ctx, tx, oid, id); err != nil {
			return logEntry{}, err
		}
		var parts []string
		if len(added) > 0 {
			parts = append(parts, "+"+strings.Join(added, ", +"))
		}
		if len(removed) > 0 {
			parts = append(parts, "-"+strings.Join(removed, ", -"))
		}
		return logEntry{orgID: oid, action: "group.members", detail: clip(g.Name+": "+strings.Join(parts, " "), 200)}, nil
	})
	return g, err
}

// SetGroupTasks replaces the tasks the planner may propose the group for. Only tasks the planner distributes
// can be listed; fills 1-50; each task at most once.
func (s *Store) SetGroupTasks(ctx context.Context, orgSlug, actor string, id int64, tasks []GroupTask) (Group, error) {
	seen := map[int64]bool{}
	for _, t := range tasks {
		if t.Fills < 1 || t.Fills > 50 {
			return Group{}, Invalid("fills_range", nil, "fills must be 1-50")
		}
		if !groupTaskLevels[t.Level] {
			return Group{}, Invalid("level_invalid", nil, "level must be poor, neutral or good")
		}
		if seen[t.TaskID] {
			return Group{}, Invalid("task_twice", nil, "a task can be listed once")
		}
		seen[t.TaskID] = true
	}
	var g Group
	err := s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		if _, err = groupIn(ctx, tx, oid, id); err != nil {
			return logEntry{}, err
		}
		for _, t := range tasks {
			var planned bool
			err := tx.QueryRowContext(ctx, `SELECT planned FROM task WHERE id = ? AND org_id = ?`, t.TaskID, oid).Scan(&planned)
			if errors.Is(err, sql.ErrNoRows) {
				return logEntry{}, ErrNotFound
			}
			if err != nil {
				return logEntry{}, err
			}
			if !planned {
				return logEntry{}, Invalid("task_not_distributed", nil, "the planner does not distribute that task, so it cannot propose a group for it")
			}
		}
		if _, err := tx.ExecContext(ctx, `DELETE FROM grp_task WHERE group_id = ? AND org_id = ?`, id, oid); err != nil {
			return logEntry{}, err
		}
		for _, t := range tasks {
			if _, err := tx.ExecContext(ctx, `INSERT INTO grp_task (org_id, group_id, task_id, fills, level) VALUES (?, ?, ?, ?, ?)`, oid, id, t.TaskID, t.Fills, t.Level); err != nil {
				return logEntry{}, err
			}
		}
		if g, err = groupIn(ctx, tx, oid, id); err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "group.tasks", detail: fmt.Sprintf("%s: %d tasks", g.Name, len(tasks))}, nil
	})
	return g, err
}

// DeleteGroup refuses (ErrConflict) while a manual cell or a selection (utvalg) still names the group.
func (s *Store) DeleteGroup(ctx context.Context, orgSlug, actor string, id int64) error {
	return s.write(ctx, actor, func(tx *sql.Tx) (logEntry, error) {
		oid, err := orgID(ctx, tx, orgSlug)
		if err != nil {
			return logEntry{}, err
		}
		g, err := groupIn(ctx, tx, oid, id)
		if err != nil {
			return logEntry{}, err
		}
		if _, err = tx.ExecContext(ctx, `DELETE FROM grp WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {
			return logEntry{}, inUse("group", g.Name, "a manual cell or a selection")
		} else if err != nil {
			return logEntry{}, err
		}
		return logEntry{orgID: oid, action: "group.delete", detail: g.Name}, nil
	})
}
