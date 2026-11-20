"""Mutation check: break one rule at a time (the planner in Python and the app in Go) and see that at least one test fails.

    mise run mutations                 all (about 7 min); quiet on success
    mise run mutations -- --only group    only those with «group» in the name
    mise run mutations -- --list       show the names

Prints only what needs action: mutations that survived (a test is missing), patterns that no longer
exist (OUTDATED) and mutations that cannot be run. Ends with one summary line.
The Go mutations run in parallel, each in its own copy of the Go sources in a temporary folder (the repo is not touched); the Python mutations
run in the repo while they go. The Python files are always restored; backups are in solver/.mutation-backup/ while it is going on.
"""

from __future__ import annotations

import argparse
import os
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import time
from concurrent.futures import ThreadPoolExecutor
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent  # solver/
REPO = ROOT.parent
BACKUP = ROOT / ".mutation-backup"
MODEL = "solver/dutyplan_solver/model.py"
TESTS = ["tests/test_rules.py", "tests/test_random.py", "tests/test_fixture.py"]  # fastest first, so that -x stops early

# (name, pattern in model.py, replacement). The pattern is replaced only the first time it occurs.
PY_MUTATIONS: list[tuple[str, str, str]] = [
    ("cannot-be-combined is ignored", "m.add(a + b <= 1)", "pass"),
    ("never on an event is ignored", 'if self.date_level(p, t, d) == "never":', "if False:"),
    ("not taking part in the task is ignored", "if t not in self.task_level.get(p, {}):", "if False:"),
    ("never on a task is ignored", 'if self.task_level[p][t] == "never":', "if False:"),
    ("a person who cannot be proposed", "if not self.person_proposable[p]:", "if False:"),
    ("a group that cannot be proposed", "if not self.group_proposable[g]:", "if False:"),
    ("a group with a member who cannot be proposed", "if not self.person_proposable[mem]:", "if False:"),
    ("a selection does not apply to people", "if pool is not None and p not in pool:", "if False:"),
    ("a selection does not apply to groups", "if (d, t) in self.pool and g not in self.pool_groups[d, t]:", "if False:"),
    ("a manual group does not forbid", "if occ is not None:", "if False:"),
    ("a group member with never on the event", 'if self.date_level(mem, t, d) == "never":', "if False:"),
    ("a group member with never on the task", 'if self.task_level.get(mem, {}).get(t) == "never":', "if False:"),
    ("a group too big for the places", 'if cfg["fills"] > free:', "if False:"),
    ("a proposed group does not make members busy", 'pair_penalty(f"gp_{g}_{t}_{mem}_{t2}_{d}", v, x[mem, t2, d], lvl)', "pass"),
    ("groups with a common member", 'pair_penalty(f"gg_{g1}_{t1}_{g2}_{t2}_{d}", y[g1, t1, d], y[g2, t2, d], lvl, times=shared)', "pass"),
    ("the same task twice through a group", 'lvl = "cannot" if t2 == t else pb.combo_level(t, t2)  # the same task twice makes no sense', "lvl = pb.combo_level(t, t2)"),
    ("a group fills several places", 'pb.group_tasks[g, t]["fills"] * v for (g, tt, dd), v in y.items()', "v for (g, tt, dd), v in y.items()"),
    ("practical and preferred are ignored", 'obj.append((SCALE * prm["w_combo"][level] * times, both))', "pass"),
    ("manual group: soft combination", 'obj.append((SCALE * prm["w_combo"][lvl], x[mem, t2, d]))', "pass"),
    ("preferences are ignored", 'w = prm["w_task"][pb.task_level[p][t]] + prm["w_date"][pb.date_level(p, t, d)]', "w = 0"),
    ("a group's date wishes are ignored", 'mean_date = sum(prm["w_date"][pb.date_level(mem, t, d)] for mem in mems) / len(mems)', "mean_date = 0"),
    ("stability for people", "if (p, t, d) in pb.previous:", "if False:"),
    ("stability for groups", "if (g, t, d) in pb.previous_groups:", "if False:"),
    ("history overall", "hist = {p: pb.history_total.get(p, 0) for p in persons}", "hist = {p: 0 for p in persons}"),
    ("history per task", "h_t = {p: pb.history_task.get(p, {}).get(t, 0) for p in cand}", "h_t = {p: 0 for p in cand}"),
    ("even distribution off", "m.add(under >= lo_t - load)", "m.add(under >= 0)"),
    ("distribution per task is the main term again", '"w_task_fair": 0,', '"w_task_fair": 10,'),
    ("overall distribution is off by default", '"w_fair": 10,', '"w_fair": 0,'),
    # (the worst-deviation terms w_fair_range and w_task_range are secondary: changing them alone does not change the outcome, so no mutation for them.)
    ("manual group work per task", "g_t = {p: pb.gcount_task.get((p, t), 0) for p in cand}", "g_t = {p: 0 for p in cand}"),
    ("manual group work overall", "gcount = {p: pb.gcount_person.get(p, 0) for p in persons}", "gcount = {p: 0 for p in persons}"),
    ("a proposed group does not count in the distribution", "part[mem, t, d].append(v)  # one assignment per member", "pass"),
    ("the output does not count through a group", '1 for a in result["group_assignments"]', '0 for a in result["group_assignments"]'),
    ("an empty place is cheap", 'obj.append((-SCALE * prm["w_unfilled"], u))', "pass"),
    ("a manual choice does not win", "m.add(v == 1)  # manuelt valg vinner over filtrene", "pass"),
]

# The Go app: (name, file from the repo root, pattern, replacement). The tenant guards first: they matter most not to lose.
GO_MUTATIONS: list[tuple[str, str, str, str]] = [
    ("tenant: a person is updated without the org guard", "internal/store/store.go", "FROM person WHERE id = ? AND org_id = ?`, id, oid)", "FROM person WHERE id = ? AND ? > 0`, id, oid)"),
    ("tenant: a plan is fetched without the org", "internal/store/store.go", "WHERE o.slug = ? AND p.slug = ?", "WHERE ? != '' AND p.slug = ?"),
    ("tenant: people are listed without the org", "internal/store/store.go", "WHERE p.org_id = ? ORDER BY x.position", "WHERE ? > 0 ORDER BY x.position"),
    ("tenant: plans are listed without the org", "internal/store/store.go", "FROM plan WHERE org_id = ? ORDER BY id DESC", "FROM plan WHERE ? > 0 ORDER BY id DESC"),
    ("foreign keys off", "internal/store/store.go", 'q.Add("_pragma", "foreign_keys(1)")', 'q.Add("_pragma", "foreign_keys(0)")'),
    ("cross-site writes are allowed", "internal/api/api.go", "http.NewCrossOriginProtection().Handler(recoverer(mux, log))", "recoverer(mux, log)"),
    ("a database with unknown migrations is opened", "internal/store/store.go", "if !known[v] {", "if false {"),
    ("an organisation can be duplicated", "internal/store/migrations/001_schema.sql", "slug       TEXT NOT NULL UNIQUE,", "slug       TEXT NOT NULL,"),
    ("a plan can be duplicated in the organisation", "internal/store/migrations/001_schema.sql", "UNIQUE (org_id, slug)", "UNIQUE (id)"),
    ("a person can be duplicated in the organisation", "internal/store/migrations/001_schema.sql", "    UNIQUE (org_id, name),\n    FOREIGN KEY (org_id, section_id)", "    UNIQUE (id),\n    FOREIGN KEY (org_id, section_id)"),
    ("a change without a new value is accepted", "internal/store/store.go", "if len(changes) == 0 {", "if false {"),
    # setup and dates (phase 2)
    ("tenant: a reference is checked without the org", "internal/store/setup.go", "WHERE id = ? AND org_id = ?`, id, oid).Scan(&one)", "WHERE id = ? AND ? > 0`, id, oid).Scan(&one)"),
    ("tenant: a plan is looked up without the org", "internal/store/setup.go", "SELECT id FROM plan WHERE org_id = ? AND slug = ?", "SELECT id FROM plan WHERE ? > 0 AND slug = ?"),
    ("tenant: a task is updated without the org guard", "internal/store/setup.go", "SELECT id, name, planned FROM task WHERE id = ? AND org_id = ?", "SELECT id, name, planned FROM task WHERE id = ? AND ? > 0"),
    ("tenant: tasks are listed without the org", "internal/store/setup.go", "FROM task WHERE org_id = ? ORDER BY position, id`, oid)", "FROM task WHERE ? > 0 ORDER BY position, id`, oid)"),
    ("tenant: combinations are listed without the org", "internal/store/setup.go", "FROM task_combination WHERE org_id = ? ORDER BY", "FROM task_combination WHERE ? > 0 ORDER BY"),
    ("tenant: events are listed without the plan", "internal/store/events.go", "WHERE d.org_id = ? AND d.plan_id = ? ORDER BY ", "WHERE d.org_id = ? AND ? > 0 ORDER BY "),
    ("tenant: an event is fetched without the plan", "internal/store/events.go", "WHERE d.id = ? AND d.org_id = ? AND d.plan_id = ?", "WHERE d.id = ? AND d.org_id = ? AND ? > 0"),
    ("a combination is not stored with the smallest id first", "internal/store/setup.go", "if a > b {", "if false {"),
    ("a combination with itself", "internal/store/setup.go", "if a == b {", "if false {"),
    ("«standard» does not remove the setting", "internal/store/setup.go", 'if level == "standard" {', "if false {"),
    ("a task in use can be deleted", "internal/store/setup.go", "DELETE FROM task WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {", "DELETE FROM task WHERE id = ? AND org_id = ?`, id, oid); false {"),
    ("an invalid date in an event is accepted", "internal/store/events.go", "day, ok := parseISODate(d)\n\t\tif !ok {", "day, ok := parseISODate(d)\n\t\tif false && !ok {"),
    ("two events on the same day are refused", "internal/store/migrations/001_schema.sql", "CREATE INDEX event_plan ON event (plan_id, date, id);", "CREATE UNIQUE INDEX event_plan ON event (plan_id, date);"),
    ("every week until: a date that comes twice gives two events", "internal/store/events.go", "if k := x.Format(\"2006-01-02\"); !seen[k] {", "if k := x.Format(\"2006-01-02\"); true {"),
    ("every week until: no limit on the range", "internal/store/events.go", "if last.Sub(day) > maxRepeatDays*24*time.Hour {", "if false {"),
    # planning (phase 4)
    ("tenant: a run is fetched without org and plan", "internal/store/runs.go", "FROM plan_run WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid)", "FROM plan_run WHERE id = ? AND ? > 0 AND ? > 0`, id, oid, pid)"),
    ("tenant: a run is fetched without the plan", "internal/store/runs.go", "FROM plan_run WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid)", "FROM plan_run WHERE id = ? AND org_id = ? AND ? > 0`, id, oid, pid)"),
    ("tenant: runs are listed without org and plan", "internal/store/runs.go", "FROM plan_run WHERE org_id = ? AND plan_id = ? AND (pinned = 1 OR", "FROM plan_run WHERE ? > 0 AND ? > 0 AND (pinned = 1 OR"),
    ("tenant: runs are listed without the plan", "internal/store/runs.go", "FROM plan_run WHERE org_id = ? AND plan_id = ? AND (pinned = 1 OR", "FROM plan_run WHERE org_id = ? AND ? > 0 AND (pinned = 1 OR"),
    # rename and delete an organisation or a plan
    ("delete plan: the plan's runs are kept", "internal/store/manage.go", '[]string{"plan_run", "event", "plan_field"}', '[]string{"event", "plan_field"}'),
    ("delete plan: the plan's events are kept", "internal/store/manage.go", '[]string{"plan_run", "event", "plan_field"}', '[]string{"plan_run", "plan_field"}'),
    ("delete plan: manual cells do not follow their event", "internal/store/migrations/001_schema.sql", "ON DELETE CASCADE, -- cells follow their event", ", -- cells follow their event"),
    ("delete plan: answers do not follow their event", "internal/store/migrations/001_schema.sql", "ON DELETE CASCADE -- answers follow their event", "-- answers follow their event"),
    ("delete plan: a run in progress does not block", "internal/store/manage.go", "if running > 0 {", "if false {"),
    ("delete organisation: plans do not block", "internal/store/manage.go", "if plans > 0 {", "if false {"),
    ("delete organisation: the confirmation is not checked", "internal/store/manage.go", "if confirm != o.Slug {", "if false {"),
    ("delete organisation: its groups are kept", "internal/store/manage.go", "\"person_task\", \"grp\", \"task\"", "\"person_task\", \"task\""),
    ("rename organisation: the same name is accepted", "internal/store/manage.go", "SELECT id, name FROM organization WHERE slug = ?`, orgSlug).Scan(&id, &old)", "SELECT id, name || '~' FROM organization WHERE slug = ?`, orgSlug).Scan(&id, &old)"),
    # a time of day and a description on an event
    ("event time: any text is accepted as a time", "internal/store/events.go", '_, err := time.Parse("15:04", t)', '_, _ = time.Parse("15:04", t); var err error'),
    ("event time: a one-digit hour is accepted", "internal/store/events.go", "if len(t) != 5 || t[2] != ':' {", "if false {"),
    ("event time: an end time without a start time is accepted", "internal/store/events.go", 'if en != "" && st == "" {', "if false {"),
    ("event time: an end time before the start is accepted", "internal/store/events.go", 'if en != "" && en <= st {', "if false {"),
    ("event time: an end time equal to the start is accepted", "internal/store/events.go", "en <= st {", "en < st {"),
    ("event description: no length limit", "internal/store/events.go", "utf8.RuneCountInString(d) > maxDescriptionRunes ||", "utf8.RuneCountInString(d) > 1<<30 ||"),
    ("event description: control characters are accepted", "internal/store/events.go", '|| !validText(strings.NewReplacer("\\n", " ", "\\t", " ").Replace(d))', "|| false"),
    ("event description: line endings are not normalised", "internal/store/events.go", 'strings.ReplaceAll(raw, "\\r\\n", "\\n")', "raw"),
    ("events: the time of day is ignored in the order", "internal/store/events.go", "eventOrder = `d.date, CASE WHEN d.start_time = '' THEN 1 ELSE 0 END, d.start_time, d.id`", "eventOrder = `d.date, d.id`"),
    ("events: those without a time come first", "internal/store/events.go", "CASE WHEN d.start_time = '' THEN 1 ELSE 0 END", "CASE WHEN d.start_time = '' THEN 0 ELSE 1 END"),
    ("events: the planner's input follows the time of day", "internal/store/planning_data.go", "ORDER BY d.date, d.id`, []any{oid, pid},", "ORDER BY `+eventOrder, []any{oid, pid},"),
    ('added events: the times are not stored', 'internal/store/events.go', 'Event{Date: d, StartTime: start, EndTime: end}', 'Event{Date: d, StartTime: start[:0], EndTime: end[:0]}'),
    ("event update: a changed time is not saved", "internal/store/events.go", "start, end = ns, ne", "_, _ = ns, ne"),
    ("event update: a changed description is not saved", "internal/store/events.go", "desc = *patch.Description", "desc = desc"),
    ("view: an event shows the time it had when the run was made", "internal/planner/view.go", "row.StartTime, row.EndTime, row.Description = cur.StartTime, cur.EndTime, cur.Description", "_, _, _ = cur.StartTime, cur.EndTime, cur.Description"),
    ("view: events of a date are not in time order", "internal/planner/view.go", "return a.StartTime < b.StartTime", "return false"),
    ("view: events without a time come first", "internal/planner/view.go", "return a.StartTime != \"\"", "return a.StartTime == \"\""),
    ("event time: a malformed time can be stored", "internal/store/migrations/001_schema.sql", "CHECK (start_time = '' OR start_time GLOB '[0-2][0-9]:[0-5][0-9]')", ""),
    ("event time: an end time without a start can be stored", "internal/store/migrations/001_schema.sql", "AND start_time <> '' AND end_time > start_time", ""),
    # pin a run as the plan's current plan
    ("pin: a pinned run is pruned", "internal/store/runs.go", "WHERE plan_id = ? AND pinned = 0 AND id NOT IN", "WHERE plan_id = ? AND id NOT IN"),
    ("pin: a pinned run drops out of the list", "internal/store/runs.go", "AND (pinned = 1 OR id IN (SELECT", "AND (0 = 1 OR id IN (SELECT"),
    ("pin: a run that is not finished can be pinned", "internal/store/runs.go", 'if status != "done" {', "if false {"),
    ("pin: pinning another run does not move the pin", "internal/store/runs.go", "UPDATE plan_run SET pinned = 0 WHERE org_id = ? AND plan_id = ?", "UPDATE plan_run SET pinned = 0 WHERE org_id = ? AND plan_id = ? AND 0 = 1"),
    ("pin: pinning twice is not refused", "internal/store/runs.go", "if (pinned == 1) == pin {", "if false {"),
    ("tenant: pin finds the run without org and plan", "internal/store/runs.go", "WHERE id = ? AND org_id = ? AND plan_id = ?`, runID, oid, pid).Scan(&status, &pinned)", "WHERE id = ? AND ? > 0 AND ? > 0`, runID, oid, pid).Scan(&status, &pinned)"),
    ("tenant: pin finds the run without plan", "internal/store/runs.go", "WHERE id = ? AND org_id = ? AND plan_id = ?`, runID, oid, pid).Scan(&status, &pinned)", "WHERE id = ? AND org_id = ? AND ? > 0`, runID, oid, pid).Scan(&status, &pinned)"),
    ("pin: a plan can have several pinned runs", "internal/store/migrations/001_schema.sql", "CREATE UNIQUE INDEX plan_run_pinned", "CREATE INDEX plan_run_pinned"),
    ("the seed is not counted up", "internal/store/runs.go", "return int(last.Int64) + 1, nil", "return int(last.Int64), nil"),
    ("old runs are not pruned", "internal/store/runs.go", "DELETE FROM plan_run WHERE plan_id = ? AND pinned = 0 AND id NOT IN", "DELETE FROM plan_run WHERE plan_id = ? AND pinned = 0 AND 0 = 1 AND id NOT IN"),
    ("a finished run can be finished again", "internal/store/runs.go", 'if status != "running" {', "if false {"),
    ("only one run at a time", "internal/planner/service.go", "if s.running {", "if false {"),
    ("time limit without a lower limit", "internal/planner/service.go", "if p.TimeLimit < MinTimeLimit || p.TimeLimit > MaxTimeLimit {", "if p.TimeLimit > MaxTimeLimit {"),
    ("time limit without an upper limit", "internal/planner/service.go", "if p.TimeLimit < MinTimeLimit || p.TimeLimit > MaxTimeLimit {", "if p.TimeLimit < MinTimeLimit {"),
    ("«cannot be proposed» is not sent to the planner", "internal/planner/input.go", "}{pid(person.ID), person.Proposable})", "}{pid(person.ID), true})"),
    ("an out-of-date plan is not detected", "internal/planner/view.go", "v.Run.Stale = hash != r.InputHash", "v.Run.Stale = hash == \"\""),
    ("the check is not read from the answer", "internal/planner/view.go", 'v.Run.Violations = out.Violations', 'v.Run.Violations = []string{}'),
    ("API: planning without a planner does not give 503", "internal/api/runs.go", "case errors.Is(err, planner.ErrUnavailable):", "case false:"),
    ("API: busy does not give 409", "internal/api/runs.go", "case errors.Is(err, planner.ErrBusy):", "case false:"),
    # groups (round 2)
    ("tenant: groups are listed without the org", "internal/store/groups.go", "FROM grp WHERE org_id = ? ORDER BY name, id", "FROM grp WHERE ? > 0 ORDER BY name, id"),
    ("tenant: a group's task is looked up without the org", "internal/store/groups.go", "SELECT planned FROM task WHERE id = ? AND org_id = ?", "SELECT planned FROM task WHERE id = ? AND ? > 0"),
    ("a group member is not checked against the organisation", "internal/store/groups.go", 'if err := inOrg(ctx, tx, "person", oid, p); err != nil {', 'if err := inOrg(ctx, tx, "person", oid, p); err != nil && p < 0 {'),
    ("a group task for a task that is not distributed", "internal/store/groups.go", "if !planned {", "if false {"),
    ("a group's places outside 1-50", "internal/store/groups.go", "if t.Fills < 1 || t.Fills > 50 {", "if false {"),
    ("a group's level is not validated", "internal/store/groups.go", "if !groupTaskLevels[t.Level] {", "if false {"),
    ("an unchanged member list is accepted", "internal/store/groups.go", "if len(removed)+len(added) == 0 {", "if false {"),
    ("a group can be duplicated in the organisation", "internal/store/migrations/001_schema.sql", "UNIQUE (org_id, name),\n    UNIQUE (org_id, id)\n);\n\nCREATE TABLE grp_member", "UNIQUE (id),\n    UNIQUE (org_id, id)\n);\n\nCREATE TABLE grp_member"),
    ("a group without active members is sent", "internal/planner/input.go", "if len(sg.Members) == 0 {", "if false {"),
    ("a group that should have been included has no warning", "internal/planner/input.go", "if len(g.Tasks) > 0 {", "if false {"),
    ("an unused empty group gives a warning", "internal/planner/input.go", "if len(g.Tasks) > 0 {", "if true {"),
    ("a group task for a task that is not distributed is sent", "internal/planner/input.go", "if planned[t.TaskID] { // the task may have been", "if true { // the task may have been"),
    ("«can be proposed» for a group is not sent", "internal/planner/input.go", "Proposable: g.Proposable}", "Proposable: true}"),
    ("places for tasks that are not distributed", "internal/planner/input.go", "if planned[task] {\n\t\t\t\tsnap.Places", "if true {\n\t\t\t\tsnap.Places"),
    ("a locked person is not shown as locked", "internal/planner/view.go", "Locked: a.Locked})", "Locked: false})"),
    ("a group member in the compromise list without the group", "internal/planner/view.go", 'who += i18n.T(l, "compromise.member_of", groupName(c.Group))', 'who += ""'),
    ("a manual text is not shown", "internal/planner/view.go", "c.texts = append(c.texts, CellText{Text: m.Label, Fills: fills})", "c.texts = c.texts[:0]"),
    ("places do not count groups", "internal/planner/view.go", "c.Slots += g.Fills", "c.Slots += g.Fills * 0"),
    ("places do not count texts", "internal/planner/view.go", "c.Slots += x.Fills", "c.Slots += x.Fills * 0"),
    ("a place without an answer from the planner is dropped", "internal/planner/view.go", "if n > 0 && cells[key{eid(e.ID), tid(task)}] == nil {", "if false {"),
    ("a warning about «cannot be proposed» is shown", "internal/planner/explain.go", 'case "not_proposable":\n\t\t\treturn ""', 'case "not_proposable":\n\t\t\treturn "x"'),
    ("the selection is not named in the explanation", "internal/planner/explain.go", 's = n.t("explain.selection", strings.Join(names, ", "))', 's = n.t("explain.selection_empty")'),
    ("the group explanation is missing", "internal/planner/explain.go", 'if r, _ := groups[g].(string); r != "" && r != "candidate" {', 'if r, _ := groups[g].(string); false && r != "" {'),
    # manual cells and selections (round 2)
    ("tenant: cells are listed without org and plan", "internal/store/cells.go", "WHERE c.org_id = ? AND c.plan_id = ? ORDER BY", "WHERE ? > 0 AND ? > 0 ORDER BY"),
    ("tenant: cells are listed without the plan", "internal/store/cells.go", "WHERE c.org_id = ? AND c.plan_id = ? ORDER BY", "WHERE c.org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: a cell is deleted without the plan", "internal/store/cells.go", "WHERE c.id = ? AND c.org_id = ? AND c.plan_id = ?`, id, oid, pid)", "WHERE c.id = ? AND c.org_id = ? AND ? > 0`, id, oid, pid)"),
    ("tenant: selections are listed without the plan", "internal/store/cells.go", "WHERE m.org_id = ? AND m.plan_id = ? ORDER BY", "WHERE m.org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: a cell's task is looked up without the org", "internal/store/cells.go", "SELECT name FROM task WHERE id = ? AND org_id = ?`, in.TaskID, oid)", "SELECT name FROM task WHERE id = ? AND ? > 0`, in.TaskID, oid)"),
    ("tenant: a cell's person is looked up without the org", "internal/store/cells.go", "SELECT name FROM person WHERE id = ? AND org_id = ?`, in.PersonID, oid)", "SELECT name FROM person WHERE id = ? AND ? > 0`, in.PersonID, oid)"),
    ("tenant: a selection's person is looked up without the org", "internal/store/cells.go", "SELECT name FROM person WHERE id = ? AND org_id = ?`, p, oid)", "SELECT name FROM person WHERE id = ? AND ? > 0`, p, oid)"),
    ("one click: a person's task can be neutral again", "internal/store/preferences.go", 'taskLevels  = map[string]bool{"poor": true, "good": true}', 'taskLevels  = map[string]bool{"poor": true, "neutral": true, "good": true}'),
    ("one click: a pair can be stored as impractical", "internal/store/setup.go", 'var combinationLevels = map[string]bool{"cannot": true, "practical": true}', 'var combinationLevels = map[string]bool{"cannot": true, "impractical": true, "practical": true}'),
    ("tenant: a cell's group is looked up without the org", "internal/store/cells.go", "SELECT name FROM grp WHERE id = ? AND org_id = ?`, in.GroupID, oid)", "SELECT name FROM grp WHERE id = ? AND ? > 0`, in.GroupID, oid)"),
    ('a cell without an event of the plan is accepted', 'internal/store/cells.go', 'if _, err := getEvent(ctx, tx, oid, pid, in.EventID); err != nil {', 'if _, err := getEvent(ctx, tx, oid, pid, in.EventID); false && err != nil {'),
    ('a selection without an event of the plan is accepted', 'internal/store/cells.go', 'if _, err := getEvent(ctx, tx, oid, pid, eventID); err != nil {', 'if _, err := getEvent(ctx, tx, oid, pid, eventID); false && err != nil {'),
    ("a cell naming two things or none is accepted", "internal/store/cells.go", "if kinds != 1 {", "if kinds > 5 {"),
    ("a person fills several places in a cell", "internal/store/cells.go", " || (in.PersonID != 0 && fills != 1)", ""),
    ("a selection for a task that is not distributed", "internal/store/cells.go", "if !planned {", "if false {"),
    ('the same cell twice gives no conflict', 'internal/store/cells.go', 'if isUnique(err) {\n\t\t\treturn Conflict("cell_exists"', 'if false {\n\t\t\treturn Conflict("cell_exists"'),
    ('the selection is not replaced', 'internal/store/cells.go', 'AND event_id = ? AND task_id = ?`, oid, pid, eventID, taskID)', 'AND event_id = ? AND task_id = ? AND 0`, oid, pid, eventID, taskID)'),
    ("a selection for one task removes others' selections", 'internal/store/cells.go', 'AND event_id = ? AND task_id = ?`, oid, pid, eventID, taskID)', 'AND event_id = ? AND ? > 0`, oid, pid, eventID, taskID)'),
    ("the same person twice in the same cell", "internal/store/migrations/001_schema.sql", "CREATE UNIQUE INDEX manual_cell_person", "CREATE INDEX manual_cell_person"),
    ("a cell naming several things (CHECK)", "internal/store/migrations/001_schema.sql", "CHECK ((person_id IS NOT NULL) + (group_id IS NOT NULL) + (label IS NOT NULL) = 1),", "CHECK (1),"),
    ("a selection does not follow its event on deletion", "internal/store/migrations/001_schema.sql", "ON DELETE CASCADE, -- selections follow their event", ", -- selections follow their event"),
    ("a manual group cell without active members is sent", "internal/planner/input.go", "if !groups[*c.GroupID] {", "if false {"),
    ("places for a text cell are not sent", "internal/planner/input.go", "l.Label, l.Fills = c.Label, c.Fills", "l.Label = c.Label"),
    ("places for a group cell are not sent", "internal/planner/input.go", "l.Group, l.Fills = gid(*c.GroupID), c.Fills", "l.Group = gid(*c.GroupID)"),
    ("a group without active members in a selection is sent", "internal/planner/input.go", "for _, g := range pl.Groups {\n\t\t\tif groups[g] {", "for _, g := range pl.Groups {\n\t\t\tif true {"),
    ("an empty selection without a warning", "internal/planner/input.go", "if len(sp.People)+len(sp.Groups) == 0 {", "if false {"),
    ("a selection with null instead of an empty list", "internal/planner/input.go", "People: []string{}, Groups: []string{}}", "People: nil, Groups: nil}"),
    ("an empty selection is not explained", "internal/planner/explain.go", 'if _, has := why["outside_selection"]; has || why["selection"] != nil || why["selection_groups"] != nil {', 'if why["selection"] != nil || why["selection_groups"] != nil {'),
    # weighting per plan
    ("an unknown weight is accepted", "internal/store/weights.go", "if !ok {\n\t\t\treturn nil, Invalid(\"weight_unknown\"", "if !ok && false {\n\t\t\treturn nil, Invalid(\"weight_unknown\""),
    ("a weight over the limit is accepted", "internal/store/weights.go", "v < spec.Min || v > spec.Max", "v < spec.Min"),
    ("a weight under the limit is accepted", "internal/store/weights.go", "v < spec.Min || v > spec.Max", "v > spec.Max"),
    ("a number equal to the default is stored", "internal/store/weights.go", "if v != spec.Default {", "if true {"),
    ("a weighting without change is accepted", "internal/store/weights.go", "if len(changes) == 0 {", "if false {"),
    ("tenant: weights are fetched without the plan", "internal/store/weights.go", "SELECT weights FROM plan WHERE org_id = ? AND id = ?`, oid, pid)", "SELECT weights FROM plan WHERE org_id = ? AND ? > 0`, oid, pid)"),
    ("tenant: weights are written without the plan", "internal/store/weights.go", "UPDATE plan SET weights = ? WHERE org_id = ? AND id = ?`, string(b), oid, pid)", "UPDATE plan SET weights = ? WHERE org_id = ? AND ? > 0`, string(b), oid, pid)"),
    ("weights are not sent to the planner", "internal/planner/input.go", "in.Params = weightParams(d.Weights) // the weights", "in.Params = nil // the weights"),
    ("weights with a dot are not nested", "internal/planner/input.go", 'if group, name, nested := strings.Cut(key, "."); nested {', 'if group, name, nested := strings.Cut(key, "."); nested && false {'),
    ("empty lists are sent as null", "internal/planner/input.go", "if v := reflect.ValueOf(p).Elem(); v.IsNil() {", "if v := reflect.ValueOf(p).Elem(); false {"),
    # server texts in two languages
    ("i18n: nn is not Norwegian", "internal/i18n/i18n.go", 'case "nb", "no", "nn":', 'case "nb", "no":'),
    ("i18n: a missing key falls back to Norwegian", "internal/i18n/i18n.go", "if f, ok = en[key]; !ok {", "if f, ok = nb[key]; !ok {"),
    ("API: the language of a run view is ignored", "internal/api/runs.go", 'v, err := s.planner.View(r.Context(), org, plan, id, i18n.Parse(r.URL.Query().Get("lang")))', 'v, err := s.planner.View(r.Context(), org, plan, id, i18n.EN)'),
    ("API: the code and parameters of an error are dropped", "internal/api/api.go", "e.Code, e.Params = c.Code, c.Params", "_ = c"),
    ("view: notes are always English", "internal/planner/view.go", 'v.Warnings = append(v.Warnings, i18n.T(l, "note."+note.Code, args...))', 'v.Warnings = append(v.Warnings, i18n.T(i18n.EN, "note."+note.Code, args...))'),
    ("view: compromises are always English", "internal/planner/view.go", 'v.Weak = append(v.Weak, i18n.T(l, "compromise",', 'v.Weak = append(v.Weak, i18n.T(i18n.EN, "compromise",'),
    ("view: explanations are always English", "internal/planner/view.go", "Event: eventName, L: l}", "Event: eventName, L: i18n.EN}"),
    # migration 9: English codes
    # the person's answers (phase 3)
    ("tenant: a person is fetched without the org", "internal/store/preferences.go", "FROM person WHERE id = ? AND org_id = ?`, id, oid).", "FROM person WHERE id = ? AND ? > 0`, id, oid)."),
    ("tenant: a task is fetched without the org on choice", "internal/store/preferences.go", "WHERE id = ? AND org_id = ? AND planned = 1`, taskID, oid)", "WHERE id = ? AND ? > 0 AND planned = 1`, taskID, oid)"),
    ("tenant: the event does not belong to the plan", "internal/store/preferences.go", "SELECT 1 FROM event WHERE id = ? AND org_id = ? AND plan_id = ?", "SELECT 1 FROM event WHERE id = ? AND org_id = ? AND ? > 0"),
    ("pref: a task that is not distributed can be chosen", "internal/store/preferences.go", "WHERE id = ? AND org_id = ? AND planned = 1`, taskID, oid)", "WHERE id = ? AND org_id = ?`, taskID, oid)"),
    ("pref: task level without a check", "internal/store/preferences.go", "if !taskLevels[level] {", "if false {"),
    ("pref: event level without a check", "internal/store/preferences.go", 'if l != "" && !eventLevels[l] {', 'if l == "aldri" {'),
    ("pref: neutral does not remove the answer", "internal/store/preferences.go", 'if want == "" {', "if false {"),
    ("pref: an unchanged answer is accepted", "internal/store/preferences.go", "if changed == 0 {", "if false {"),
    ("pref: an unchanged task choice is accepted", "internal/store/preferences.go", "if before == level {", "if false {"),
    ("API does not require JSON", "internal/api/api.go", 'len(ct) < 16 || ct[:16] != "application/json"', "len(ct) < 0"),
    ("API accepts unknown fields", "internal/api/api.go", "dec.DisallowUnknownFields()", ""),
    ("API has no size limit", "internal/api/api.go", "r.Body = http.MaxBytesReader(w, r.Body, 64<<10)", ""),
    ("API lacks security headers", "internal/api/api.go", 'h.Set("Content-Security-Policy"', 'h.Set("X-Csp-Av"'),
    # the table: places per event, info rows, everyone's answers, one task against all the others
    ("tenant: places per event are listed without the plan", "internal/store/table.go", "FROM event_task WHERE org_id = ? AND plan_id = ? ORDER BY", "FROM event_task WHERE org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: a task gets places without the org", "internal/store/table.go", "if err := inOrg(ctx, tx, \"task\", oid, taskID); err != nil {", "if err := error(nil); err != nil {"),
    ("tenant: details rows are listed without the plan", "internal/store/table.go", "SELECT id, name, position FROM plan_field WHERE org_id = ? AND plan_id = ? ORDER BY", "SELECT id, name, position FROM plan_field WHERE org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: info values are listed without the plan", "internal/store/table.go", "FROM event_field WHERE org_id = ? AND plan_id = ? ORDER BY", "FROM event_field WHERE org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: a details row is fetched without the plan", "internal/store/table.go", "SELECT id, name, position FROM plan_field WHERE id = ? AND org_id = ? AND plan_id = ?", "SELECT id, name, position FROM plan_field WHERE id = ? AND org_id = ? AND ? > 0"),
    ("tenant: the table's answers are listed without the plan", "internal/store/table.go", "FROM person_event WHERE org_id = ? AND plan_id = ? ORDER BY", "FROM person_event WHERE org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: the table's task choices are listed without the org", "internal/store/table.go", "FROM person_task WHERE org_id = ? ORDER BY", "FROM person_task WHERE ? > 0 ORDER BY"),
    ("tenant: one task against all is looked up without the org", "internal/store/table.go", "SELECT name FROM task WHERE id = ? AND org_id = ?`, taskID, oid).Scan(&name)", "SELECT name FROM task WHERE id = ? AND ? > 0`, taskID, oid).Scan(&name)"),
    ("tenant: one task against all reaches other organisations' tasks", "internal/store/table.go", "SELECT id FROM task WHERE org_id = ? AND id <> ? ORDER BY id", "SELECT id FROM task WHERE ? > 0 AND id <> ? ORDER BY id"),
    ("places: any number is accepted", "internal/store/table.go", "if slots < 0 || slots > maxSlots {", "if false {"),
    ("places: the same places again are accepted", "internal/store/table.go", "if before == slots {", "if false {"),
    ("places: a deleted event keeps its places", "internal/store/migrations/001_schema.sql", "ON DELETE CASCADE, -- places follow their event", ", -- places follow their event"),
    ("places: a task with places on an event cannot be deleted", "internal/store/migrations/001_schema.sql", "ON DELETE CASCADE -- and their task", "-- and their task"),
    ("places: the API resets when the number is missing", "internal/api/table.go", "if b.Slots == nil {", "if false {"),
    ("fields: a plan has any number of details rows", "internal/store/table.go", "if n >= maxFieldsPerPlan {", "if false {"),
    ("fields: an info value of any length", "internal/store/table.go", "utf8.RuneCountInString(value) > maxFieldValueRune ||", "utf8.RuneCountInString(value) > 1<<20 ||"),
    ("fields: an empty value is stored", "internal/store/table.go", 'if value == "" {', "if false {"),
    ("fields: the same value again is accepted", "internal/store/table.go", "if before == value {", "if false {"),
    ("fields: a deleted row keeps its values", "internal/store/migrations/001_schema.sql", "REFERENCES plan_field (org_id, plan_id, id) ON DELETE CASCADE", "REFERENCES plan_field (org_id, plan_id, id)"),
    ("fields: a deleted plan keeps its details rows", "internal/store/manage.go", "[]string{\"plan_run\", \"event\", \"plan_field\"}", "[]string{\"plan_run\", \"event\"}"),
    ("copy: the names that were locked are copied too", "internal/store/events.go", "WHERE event_id = ? AND org_id = ? AND plan_id = ? AND locked = 0`", "WHERE event_id = ? AND org_id = ? AND plan_id = ?`"),
    ("copy: the places are not copied", "internal/store/events.go", "if in.Places {", "if false {"),
    ("copy: the details are not copied", "internal/store/events.go", "if in.Details {\n\t\t\t\tif _, err", "if false {\n\t\t\t\tif _, err"),
    ("copy: the times are not copied", "internal/store/events.go", "tmpl.StartTime, tmpl.EndTime = src.StartTime, src.EndTime", "_ = src"),
    ("copy: the cells set by hand are not copied", "internal/store/events.go", "if in.Hand {", "if false {"),
    ("every week until: the step is not a week", "internal/store/events.go", "x = x.AddDate(0, 0, 7*every)", "x = x.AddDate(0, 0, 6*every)"),
    ("every few weeks: the step ignores the choice", "internal/store/events.go", "x = x.AddDate(0, 0, 7*every)", "x = x.AddDate(0, 0, 7)"),
    ("every few weeks: no upper limit", "internal/store/events.go", "if every < 1 || every > maxEveryWeeks {", "if every < 1 {"),
    ("every few weeks: adding events loses the choice", "internal/store/events.go", 'Every     int    `json:"every"`', 'Every     int    `json:"every_lost"`'),
    ("every few weeks: a copy loses the choice", "internal/store/events.go", 'Every     int      `json:"every"`', 'Every     int      `json:"every_lost"`'),
    ("every week until: any number of dates", "internal/store/events.go", "if len(out) > maxEventsPerCall {", "if false {"),
    ("places: 0 places can be stored", "internal/store/migrations/001_schema.sql", "CHECK (slots BETWEEN 1 AND 50)", "CHECK (slots BETWEEN 0 AND 50)"),
    ("fields: a new row is not put last", "internal/store/table.go", "oid, pid, name, n, s.stamp())", "oid, pid, name, 0, s.stamp())"),
    ("fields: a moved row is not put at its position", "internal/store/table.go", "order = insertAt(order, moved, position)", "order = append(order, moved)"),
    ("fields: deleting a row leaves a gap", "internal/store/table.go", "return renumberFields(ctx, tx, oid, pid, 0, 0)", "return nil"),
    ("fields: a negative position is accepted", "internal/store/table.go", "if position < 0 {", "if false {"),
    ("tasks: listed by name, not in their order", "internal/store/setup.go", "FROM task WHERE org_id = ? ORDER BY position, id`, oid)", "FROM task WHERE org_id = ? ORDER BY name COLLATE NOCASE`, oid)"),
    ("tasks: a new task is not put last", "internal/store/setup.go", "oid, t.Name, t.Planned, n, s.stamp())", "oid, t.Name, t.Planned, 0, s.stamp())"),
    ("tasks: a moved task is not put at its position", "internal/store/setup.go", "order = insertAt(order, moved, position)", "order = append(order, moved)"),
    ("tasks: deleting a task leaves a gap", "internal/store/setup.go", "return renumberTasks(ctx, tx, oid, 0, 0)", "return nil"),
    ("tasks: a negative position is accepted", "internal/store/setup.go", "if position < 0 {", "if false {"),
    ("tenant: a task of another organisation can be moved", "internal/store/setup.go", "if err := inOrg(ctx, tx, \"task\", oid, id); err != nil {\n\t\t\treturn err\n\t\t}\n\t\treturn renumberTasks", "return renumberTasks"),
    ("input: the tasks follow the table's order", "internal/store/planning_data.go", "FROM task WHERE org_id = ? ORDER BY id`", "FROM task WHERE org_id = ? ORDER BY position DESC, id`"),
    ("fill row: other plans' events are filled too", "internal/store/table.go", "WHERE e.org_id = ? AND e.plan_id = ? AND COALESCE", "WHERE e.org_id = ? AND ? > 0 AND COALESCE"),
    ("tenant: a row is filled with another organisation's task", "internal/store/table.go", "if err := inOrg(ctx, tx, \"task\", oid, taskID); err != nil {\n\t\t\treturn err\n\t\t}\n\t\tvar events", "var events"),
    ("fill row: the same number again is accepted", "internal/store/table.go", "if len(events) == 0 {\n\t\t\treturn errNothingToChange()", "if false {\n\t\t\treturn errNothingToChange()"),
    ("fill row: an event with that number already counts as a change", "internal/store/table.go", "COALESCE(x.slots, 0) <> ?", "COALESCE(x.slots, 0) >= 0 - ?"),
    ("fill row: no upper limit", "internal/store/table.go", "taskID int64, slots int) error {\n\tif slots < 0 || slots > maxSlots {", "taskID int64, slots int) error {\n\tif slots < 0 {"),
    ("fill row: the route is not there", "internal/api/table.go", "mux.HandleFunc(\"PUT /api/orgs/{org}/plans/{plan}/tasks/{id}/places\", s.fillPlaces)\n", ""),
    ("people: listed by name, not in their order", "internal/store/store.go", "ORDER BY x.position IS NOT NULL, x.position, p.position, p.id`", "ORDER BY p.name`"),
    ("people: those without a section are not first", "internal/store/store.go", "ORDER BY x.position IS NOT NULL, x.position,", "ORDER BY x.position IS NULL, x.position,"),
    ("people: a new person is not put last", "internal/store/store.go", "oid, p.Name, n, s.stamp())", "oid, p.Name, 0, s.stamp())"),
    ("people: a moved person is not put at their position", "internal/store/sections.go", "order = insertAt(order, moved, position)", "order = append(order, moved)"),
    ("people: the section a person left keeps a gap", "internal/store/sections.go", "if p.Section != section { // close", "if false { // close"),
    ("people: the section is not stored", "internal/store/sections.go", "`UPDATE person SET section_id = ? WHERE id = ? AND org_id = ?`, sectionScope(section)[0]", "`UPDATE person SET section_id = ? WHERE id = ? AND org_id = ?`, nil"),
    ("people: the order ignores the section", "internal/store/sections.go", "where, args = where+` AND section_id IS ?`, append(args, section[0])", "_ = section"),
    ("people: a negative position is accepted", "internal/store/sections.go", "section int64, position int) error {\n\tif position < 0 {", "section int64, position int) error {\n\tif false {"),
    ("tenant: a person is moved into another organisation's section", "internal/store/sections.go", "if section != 0 {\n\t\t\tif _, err := sectionName", "if false {\n\t\t\tif _, err := sectionName"),
    ("tenant: another organisation's person can be moved", "internal/store/sections.go", "if err != nil {\n\t\t\treturn err\n\t\t}\n\t\tif section != 0 {", "_ = err\n\t\tif section != 0 {"),
    ("tenant: sections are listed without the org", "internal/store/sections.go", "FROM person_section WHERE org_id = ? ORDER BY position, id`, []any{oid}", "FROM person_section WHERE ? > 0 ORDER BY position, id`, []any{oid}"),
    ("tenant: a section is found without the org", "internal/store/sections.go", "FROM person_section WHERE id = ? AND org_id = ?`, id, oid)", "FROM person_section WHERE id = ? AND ? > 0`, id, oid)"),
    ("sections: a new section is not put last", "internal/store/sections.go", "oid, name, n, s.stamp())", "oid, name, 0, s.stamp())"),
    ("sections: no upper limit", "internal/store/sections.go", "if n >= maxSections {", "if false {"),
    ("sections: a negative position is accepted", "internal/store/sections.go", "id int64, position int) error {\n\tif position < 0 {", "id int64, position int) error {\n\tif false {"),
    ("sections: a moved section is not put at its position", "internal/store/sections.go", "return renumber(ctx, tx, \"person_section\", oid, nil, id, position)", "return nil"),
    ("sections: deleting one leaves its people in it", "internal/store/sections.go", "SET section_id = NULL, position = position + ?", "SET position = position + ?"),
    ("sections: a deleted section's people are mixed in", "internal/store/sections.go", "position = position + ? WHERE", "position = position + 0 * ? WHERE"),
    ("sections: deleting one leaves a gap", "internal/store/sections.go", "\t\treturn renumber(ctx, tx, \"person_section\", oid, nil, 0, 0)", "\t\treturn nil"),
    ("sections: the same name twice", "internal/store/migrations/001_schema.sql", "    position   INTEGER NOT NULL,  -- the order of the headings (0, 1, ...); people without a section come first\n    created_at TEXT NOT NULL,\n    UNIQUE (org_id, name),", "    position   INTEGER NOT NULL,  -- the order of the headings (0, 1, ...); people without a section come first\n    created_at TEXT NOT NULL,"),
    ("input: the people follow the table's order", "internal/store/planning_data.go", "FROM person WHERE org_id = ? ORDER BY id`", "FROM person WHERE org_id = ? ORDER BY position DESC, id`"),
    ("sections: the person route is not there", "internal/api/sections.go", "mux.HandleFunc(\"PUT /api/orgs/{org}/people/{id}/position\", s.movePerson)\n", ""),
    ("cells: a text can be locked", "internal/store/cells.go", "if in.Locked && label != \"\" {", "if false {"),
    ("cells: the lock is not stored", "internal/store/cells.go", "text, fills, in.Locked, s.stamp())", "text, fills, false, s.stamp())"),
    ("combinations: one against all keeps standard as a row", "internal/store/table.go", 'if level == "standard" {', "if false {"),
    ("combinations: one against all with nothing new is accepted", "internal/store/table.go", "if changed == 0 {", "if false {"),
    ("input: an event's own places are ignored", "internal/planner/input.go", "for task, n := range d.EventPlaces[e.ID] {", "for task, n := range map[int64]int{} {"),
    ("places: 0 places does not take the task off the event", "internal/store/table.go", "if slots == 0 {", "if false {"),
    ("input: all events share one type", "internal/planner/input.go", "typ := eid(e.ID)", "typ := \"all\""),
    ("input: places are not work", "internal/planner/input.go", "for task, n := range eventSlots(d, planned, e) {", "for task, n := range eventSlots(d, map[int64]bool{}, e) {"),
    ("view: places nobody fills are not shown", "internal/planner/view.go", "for task, n := range snap.SlotsFor(e) {", "for task, n := range snap.SlotsFor(SnapEvent{}) {"),
]

MUTATIONS: list[tuple[str, str, str, str]] = [(n, MODEL, a, b) for n, a, b in PY_MUTATIONS] + GO_MUTATIONS


GO_SOURCES = ["cmd", "internal", "web", "go.mod", "go.sum"]  # what the Go tests need
_local = threading.local()
_copies: list[Path] = []


def go_copy() -> Path:
    """This thread's own copy of the Go sources (made the first time), so that mutations can run at the same time."""
    if not hasattr(_local, "dir"):
        d = Path(tempfile.mkdtemp(prefix="tp-mut-"))
        _copies.append(d)
        for name in GO_SOURCES:
            src = REPO / name
            if src.is_dir():
                shutil.copytree(src, d / name)
            else:
                shutil.copy2(src, d / name)
        _local.dir = d
    return _local.dir


def run_tests(target: str, go_dir: Path | None = None) -> tuple[int, str]:
    """(0 = all passed; 1 = a test failed; otherwise = could not be run, for example a build error)."""
    if target.endswith(".py"):
        r = subprocess.run([sys.executable, "-m", "pytest", "-x", *TESTS], cwd=ROOT, capture_output=True, text=True, check=False)
        return r.returncode, r.stdout
    # Without a planner in the environment the two tests against the real solver are skipped (about 1.3 s each time). The rules must be caught
    # by the tests with a fake solver; a mutation only the real tests catch is to be shown as SURVIVED.
    env = {**os.environ, "DUTYPLAN_SOLVER": ""}
    r = subprocess.run(["go", "test", "-count=1", "-failfast", "./..."], cwd=go_dir or REPO, capture_output=True, text=True, check=False, env=env)
    out = r.stdout + r.stderr
    if r.returncode != 0 and ("build failed" in out or "setup failed" in out):
        return 99, out
    return r.returncode, out


def tests_pass() -> int:
    py, _ = run_tests(MODEL)
    go, _ = run_tests("x.go")
    return py or go


def main() -> int:
    # A task runner that stops us sends SIGTERM, which would end Python without the finally blocks that put the Python files back
    # (a mutation was left in model.py once, 2026-11-13). Exit normally instead, so they run.
    signal.signal(signal.SIGTERM, lambda *_: sys.exit(143))
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--only", help="only mutations with this text in the name")
    ap.add_argument("--list", action="store_true", help="show the names and exit")
    args = ap.parse_args()

    chosen = [m for m in MUTATIONS if not args.only or args.only.lower() in m[0].lower()]
    if args.list:
        print("\n".join(name for name, *_ in chosen))
        return 0
    if not chosen:
        print(f"no mutations match «{args.only}»")
        return 1

    if BACKUP.exists():  # the previous run was interrupted in the middle of a mutation
        for saved in BACKUP.iterdir():
            (REPO / saved.name.replace("__", "/")).write_text(saved.read_text(encoding="utf-8"), encoding="utf-8")
            saved.unlink()
        BACKUP.rmdir()
        print("restored files after an interrupted run")
    if tests_pass() != 0:
        print("the tests already fail without mutations; fix that first (mise run test)")
        return 1

    files = sorted({f for _n, f, _a, _b in chosen})
    originals = {f: (REPO / f).read_text(encoding="utf-8") for f in files}
    py_files = [f for f in files if f.endswith(".py")]
    if py_files:
        BACKUP.mkdir(exist_ok=True)
        for f in py_files:
            (BACKUP / f.replace("/", "__")).write_text(originals[f], encoding="utf-8")
    t0 = time.time()

    def verdict(name: str, rc: int) -> str | None:
        if rc == 1:
            return None
        if rc == 0:
            return f"SURVIVED  {name}: no test failed"
        return f"INVALID   {name}: the tests could not be run (code {rc}); a build error in the mutation?"

    def try_go(item: tuple[str, str, str, str]) -> str | None:
        name, f, old, new = item
        original = originals[f]
        if old not in original:
            return f"OUTDATED  {name}: the pattern is not in {f}"
        path = go_copy() / f
        path.write_text(original.replace(old, new, 1), encoding="utf-8")
        try:
            rc, _out = run_tests("x.go", go_copy())
        finally:
            path.write_text(original, encoding="utf-8")
        return verdict(name, rc)

    def try_py(item: tuple[str, str, str, str]) -> str | None:
        name, f, old, new = item
        original = originals[f]
        if old not in original:
            return f"OUTDATED  {name}: the pattern is not in {f}"
        (REPO / f).write_text(original.replace(old, new, 1), encoding="utf-8")
        try:
            rc, _out = run_tests(f)
        finally:
            (REPO / f).write_text(original, encoding="utf-8")
        return verdict(name, rc)

    go_items = [m for m in chosen if not m[1].endswith(".py")]
    py_items = [m for m in chosen if m[1].endswith(".py")]
    results: dict[str, str | None] = {}
    try:
        with ThreadPoolExecutor(max_workers=max(1, min(4, os.cpu_count() or 1))) as pool:
            futures = {m[0]: pool.submit(try_go, m) for m in go_items}  # Go in parallel, each in its own copy
            for m in py_items:  # Python in the repo, one at a time, while Go runs
                results[m[0]] = try_py(m)
            for name, fut in futures.items():
                results[name] = fut.result()
    finally:
        for f in py_files:
            (REPO / f).write_text(originals[f], encoding="utf-8")
        if BACKUP.exists():
            for saved in BACKUP.iterdir():
                saved.unlink()
            BACKUP.rmdir()
        for d in _copies:
            shutil.rmtree(d, ignore_errors=True)
    problems = [results[m[0]] for m in chosen if results[m[0]]]
    caught = len(chosen) - len(problems)

    for line in problems:
        print(line)
    print(f"mutations: {caught}/{len(chosen)} caught ({time.time() - t0:.0f} s)")
    return 1 if problems else 0


if __name__ == "__main__":
    raise SystemExit(main())
