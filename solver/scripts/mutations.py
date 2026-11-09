"""Mutasjonssjekk: bryt én regel om gangen (planleggeren i Python og appen i Go) og se at minst én test feiler.

    mise run mutations                 alle (ca. 1-2 min); stille ved suksess
    mise run mutations -- --only gruppe   bare de som har «gruppe» i navnet
    mise run mutations -- --list       vis navnene

Skriver bare det som krever handling: mutasjoner som overlevde (mangler en test), mønstre som ikke
finnes lenger (utdatert liste) og mutasjoner som ikke lar seg kjøre. Avsluttes med én oppsummeringslinje.
Go-mutasjonene kjøres parallelt i hver sin kopi av Go-kildene i en midlertidig mappe (repoet røres ikke), Python-mutasjonene
i repoet mens de går. Python-filene gjenopprettes alltid; sikkerhetskopier ligger i solver/.mutation-backup/ mens det pågår.
"""

from __future__ import annotations

import argparse
import os
import shutil
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
MODEL = "solver/tjenesteplan_solver/model.py"
TESTS = ["tests/test_rules.py", "tests/test_random.py", "tests/test_fixture.py"]  # raskeste først, så -x stopper tidlig

# (navn, mønster i model.py, erstatning). Mønsteret erstattes bare første gang det forekommer.
PY_MUTATIONS: list[tuple[str, str, str]] = [
    ("kan ikke kombineres ignoreres", "m.add(a + b <= 1)", "pass"),
    ("ikke på dato ignoreres", 'if self.date_level(p, t, d) == "ikke":', "if False:"),
    ("ikke med på oppgaven ignoreres", "if t not in self.task_level.get(p, {}):", "if False:"),
    ("ikke på oppgave ignoreres", 'if self.task_level[p][t] == "ikke":', "if False:"),
    ("person som ikke kan foreslås", "if not self.person_proposable[p]:", "if False:"),
    ("gruppe som ikke kan foreslås", "if not self.group_proposable[g]:", "if False:"),
    ("gruppe med medlem som ikke kan foreslås", "if not self.person_proposable[mem]:", "if False:"),
    ("utvalg gjelder ikke personer", "if pool is not None and p not in pool:", "if False:"),
    ("utvalg gjelder ikke grupper", "if (d, t) in self.pool and g not in self.pool_groups[d, t]:", "if False:"),
    ("manuell gruppe forbyr ikke", "if occ is not None:", "if False:"),
    ("gruppemedlem med ikke på dato", 'if self.date_level(mem, t, d) == "ikke":', "if False:"),
    ("gruppemedlem med ikke på oppgave", 'if self.task_level.get(mem, {}).get(t) == "ikke":', "if False:"),
    ("gruppe for stor for plassene", 'if cfg["fills"] > free:', "if False:"),
    ("foreslått gruppe gjør ikke medlemmer opptatt", 'pair_penalty(f"gp_{g}_{t}_{mem}_{t2}_{d}", v, x[mem, t2, d], lvl)', "pass"),
    ("grupper med felles medlem", 'pair_penalty(f"gg_{g1}_{t1}_{g2}_{t2}_{d}", y[g1, t1, d], y[g2, t2, d], lvl, times=shared)', "pass"),
    ("samme oppgave to ganger via gruppe", 'lvl = "kan_ikke" if t2 == t else pb.combo_level(t, t2)  # samme oppgave to ganger gir ingen mening', "lvl = pb.combo_level(t, t2)"),
    ("gruppe fyller flere plasser", 'pb.group_tasks[g, t]["fills"] * v for (g, tt, dd), v in y.items()', "v for (g, tt, dd), v in y.items()"),
    ("praktisk og bør ignoreres", 'obj.append((SCALE * prm["w_combo"][level] * times, both))', "pass"),
    ("manuell gruppe: myk kombinasjon", 'obj.append((SCALE * prm["w_combo"][lvl], x[mem, t2, d]))', "pass"),
    ("preferanser ignoreres", 'w = prm["w_task"][pb.task_level[p][t]] + prm["w_date"][pb.date_level(p, t, d)]', "w = 0"),
    ("gruppens datoønsker ignoreres", 'mean_date = sum(prm["w_date"][pb.date_level(mem, t, d)] for mem in mems) / len(mems)', "mean_date = 0"),
    ("stabilitet for personer", "if (p, t, d) in pb.previous:", "if False:"),
    ("stabilitet for grupper", "if (g, t, d) in pb.previous_groups:", "if False:"),
    ("historikk samlet", "hist = {p: pb.history_total.get(p, 0) for p in persons}", "hist = {p: 0 for p in persons}"),
    ("historikk per oppgave", "h_t = {p: pb.history_task.get(p, {}).get(t, 0) for p in cand}", "h_t = {p: 0 for p in cand}"),
    ("jevn fordeling av", "m.add(under >= lo_t - load)", "m.add(under >= 0)"),
    ("manuelt gruppearbeid per oppgave", "g_t = {p: pb.gcount_task.get((p, t), 0) for p in cand}", "g_t = {p: 0 for p in cand}"),
    ("manuelt gruppearbeid samlet", "gcount = {p: pb.gcount_person.get(p, 0) for p in persons}", "gcount = {p: 0 for p in persons}"),
    ("foreslått gruppe teller ikke i fordelingen", "part[mem, t, d].append(v)  # ett oppdrag per medlem", "pass"),
    ("utdata teller ikke via gruppe", '1 for a in result["group_assignments"]', '0 for a in result["group_assignments"]'),
    ("tom plass er billig", 'obj.append((-SCALE * prm["w_unfilled"], u))', "pass"),
    ("manuelt valg vinner ikke", "m.add(v == 1)  # manuelt valg vinner over filtrene", "pass"),
]

# Go-appen: (navn, fil fra repoets rot, mønster, erstatning). Tenant-vaktene først: det er det viktigste å ikke miste.
GO_MUTATIONS: list[tuple[str, str, str, str]] = [
    ("tenant: person oppdateres uten org-vakt", "internal/store/store.go", "FROM person WHERE id = ? AND org_id = ?`, id, oid)", "FROM person WHERE id = ? AND ? > 0`, id, oid)"),
    ("tenant: plan hentes uten org", "internal/store/store.go", "WHERE o.slug = ? AND p.slug = ?", "WHERE ? != '' AND p.slug = ?"),
    ("tenant: personer listes uten org", "internal/store/store.go", "FROM person WHERE org_id = ? ORDER BY", "FROM person WHERE ? > 0 ORDER BY"),
    ("tenant: planer listes uten org", "internal/store/store.go", "FROM plan WHERE org_id = ? ORDER BY id DESC", "FROM plan WHERE ? > 0 ORDER BY id DESC"),
    ("tenant: logg uten org", "internal/store/store.go", "LEFT JOIN plan p ON p.id = l.plan_id WHERE l.org_id = ?", "LEFT JOIN plan p ON p.id = l.plan_id WHERE ? > 0"),
    ("fremmednøkler av", "internal/store/store.go", 'q.Add("_pragma", "foreign_keys(1)")', 'q.Add("_pragma", "foreign_keys(0)")'),
    ("aktør er ikke påkrevd", "internal/store/store.go", 'if dec == "" {', "if false {"),
    ("aktør lengde og tegn", "internal/store/store.go", "utf8.RuneCountInString(dec) > 80 || !validText(dec)", "false"),
    ("organisasjon kan dupliseres", "internal/store/migrations/001_init.sql", "slug       TEXT NOT NULL UNIQUE,", "slug       TEXT NOT NULL,"),
    ("plan kan dupliseres i organisasjonen", "internal/store/migrations/001_init.sql", "UNIQUE (org_id, slug)", "UNIQUE (id)"),
    ("person kan dupliseres i organisasjonen", "internal/store/migrations/001_init.sql", "UNIQUE (org_id, name)", "UNIQUE (id)"),
    ("endring uten ny verdi godtas", "internal/store/store.go", "if len(changes) == 0 {", "if false {"),
    # oppsett og datoer (fase 2)
    ("tenant: referanse kontrolleres uten org", "internal/store/setup.go", "WHERE id = ? AND org_id = ?`, id, oid).Scan(&one)", "WHERE id = ? AND ? > 0`, id, oid).Scan(&one)"),
    ("tenant: plan slås opp uten org", "internal/store/setup.go", "SELECT id FROM plan WHERE org_id = ? AND slug = ?", "SELECT id FROM plan WHERE ? > 0 AND slug = ?"),
    ("tenant: oppgave oppdateres uten org-vakt", "internal/store/setup.go", "SELECT id, name, planned FROM task WHERE id = ? AND org_id = ?", "SELECT id, name, planned FROM task WHERE id = ? AND ? > 0"),
    ("tenant: oppgaver listes uten org", "internal/store/setup.go", "FROM task WHERE org_id = ? ORDER BY name", "FROM task WHERE ? > 0 ORDER BY name"),
    ("tenant: dagtyper listes uten org", "internal/store/setup.go", "SELECT id, name FROM day_type WHERE org_id = ? ORDER BY", "SELECT id, name FROM day_type WHERE ? > 0 ORDER BY"),
    # (listDayTypes har to lag: org-filteret i SQL og «bare rader for våre dagtyper» i koden. Å fjerne ett endrer ingenting.)
    ("tenant: kombinasjoner listes uten org", "internal/store/setup.go", "FROM task_combination WHERE org_id = ? ORDER BY", "FROM task_combination WHERE ? > 0 ORDER BY"),
    ("tenant: hendelser listes uten plan", "internal/store/events.go", "WHERE d.org_id = ? AND d.plan_id = ? ORDER BY d.date, d.id", "WHERE d.org_id = ? AND ? > 0 ORDER BY d.date, d.id"),
    ("tenant: hendelse hentes uten plan", "internal/store/events.go", "WHERE d.id = ? AND d.org_id = ? AND d.plan_id = ?", "WHERE d.id = ? AND d.org_id = ? AND ? > 0"),
    ("kombinasjon lagres ikke med minste id først", "internal/store/setup.go", "if a > b {", "if false {"),
    ("kombinasjon med seg selv", "internal/store/setup.go", "if a == b {", "if false {"),
    ("«standard» fjerner ikke innstillingen", "internal/store/setup.go", 'if level == "standard" {', "if false {"),
    ("plasser utenfor 1-50", "internal/store/setup.go", "if t.Slots < 1 || t.Slots > 50 {", "if false {"),
    ("oppgave i bruk kan slettes", "internal/store/setup.go", "DELETE FROM task WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {", "DELETE FROM task WHERE id = ? AND org_id = ?`, id, oid); false {"),
    ("dagtype i bruk kan slettes", "internal/store/setup.go", "DELETE FROM day_type WHERE id = ? AND org_id = ?`, id, oid); isForeignKey(err) {", "DELETE FROM day_type WHERE id = ? AND org_id = ?`, id, oid); false {"),
    ("ugyldig dato i hendelse godtas", "internal/store/events.go", "if _, ok := parseISODate(date); !ok {", "if false {"),
    ("to hendelser samme dag nektes", "internal/store/migrations/003_events.sql", "CREATE INDEX event_plan ON event (plan_id, date, id);", "CREATE UNIQUE INDEX event_plan ON event (plan_id, date);"),
    ("masseoppretting dupliserer ved gjentakelse", "internal/store/events.go", "(SELECT 1 FROM event WHERE org_id = ? AND plan_id = ? AND date = ? AND day_type_id = ?)", "(SELECT 1 WHERE 0)"),
    ("ukedager nummereres feil", "internal/store/events.go", "(int(t.Weekday())+6)%7 + 1", "int(t.Weekday())"),
    ("masseoppretting uten grense", "internal/store/events.go", "if t.Sub(f) > maxBulkSpanDays*24*time.Hour {", "if false {"),
    # planlegging (fase 4)
    ("tenant: kjøring hentes uten org og plan", "internal/store/runs.go", "FROM plan_run WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid)", "FROM plan_run WHERE id = ? AND ? > 0 AND ? > 0`, id, oid, pid)"),
    ("tenant: kjøring hentes uten plan", "internal/store/runs.go", "FROM plan_run WHERE id = ? AND org_id = ? AND plan_id = ?`, id, oid, pid)", "FROM plan_run WHERE id = ? AND org_id = ? AND ? > 0`, id, oid, pid)"),
    ("tenant: kjøringer listes uten org og plan", "internal/store/runs.go", "FROM plan_run WHERE org_id = ? AND plan_id = ? ORDER BY id DESC LIMIT ?", "FROM plan_run WHERE ? > 0 AND ? > 0 ORDER BY id DESC LIMIT ?"),
    ("tenant: kjøringer listes uten plan", "internal/store/runs.go", "FROM plan_run WHERE org_id = ? AND plan_id = ? ORDER BY id DESC LIMIT ?", "FROM plan_run WHERE org_id = ? AND ? > 0 ORDER BY id DESC LIMIT ?"),
    ("frøet telles ikke opp", "internal/store/runs.go", "return int(last.Int64) + 1, nil", "return int(last.Int64), nil"),
    ("gamle kjøringer ryddes ikke", "internal/store/runs.go", "DELETE FROM plan_run WHERE plan_id = ? AND id NOT IN", "DELETE FROM plan_run WHERE plan_id = ? AND 0 = 1 AND id NOT IN"),
    ("ferdig kjøring kan avsluttes på nytt", "internal/store/runs.go", 'if status != "running" {', "if false {"),
    ("bare én kjøring om gangen", "internal/planner/service.go", "if s.running {", "if false {"),
    ("tidsgrense uten nedre grense", "internal/planner/service.go", "if p.TimeLimit < MinTimeLimit || p.TimeLimit > MaxTimeLimit {", "if p.TimeLimit > MaxTimeLimit {"),
    ("tidsgrense uten øvre grense", "internal/planner/service.go", "if p.TimeLimit < MinTimeLimit || p.TimeLimit > MaxTimeLimit {", "if p.TimeLimit < MinTimeLimit {"),
    ("inaktiv person planlegges", "internal/planner/input.go", "if !person.Active { // an inactive person is not planned", "if false { // an inactive person is not planned"),
    ("«kan ikke foreslås» sendes ikke til planleggeren", "internal/planner/input.go", "}{pid(person.ID), person.Proposable})", "}{pid(person.ID), true})"),
    ("utdatert plan oppdages ikke", "internal/planner/view.go", "v.Run.Stale = hash != r.InputHash", "v.Run.Stale = hash == \"\""),
    ("kontrollen leses ikke fra svaret", "internal/planner/view.go", 'v.Run.Violations = out.Violations', 'v.Run.Violations = []string{}'),
    ("API: planlegging uten planlegger gir ikke 503", "internal/api/runs.go", "case errors.Is(err, planner.ErrUnavailable):", "case false:"),
    ("API: opptatt gir ikke 409", "internal/api/runs.go", "case errors.Is(err, planner.ErrBusy):", "case false:"),
    # grupper (omgang 2)
    ("tenant: grupper listes uten org", "internal/store/groups.go", "FROM grp WHERE org_id = ? ORDER BY name, id", "FROM grp WHERE ? > 0 ORDER BY name, id"),
    ("tenant: gruppens oppgave slås opp uten org", "internal/store/groups.go", "SELECT planned FROM task WHERE id = ? AND org_id = ?", "SELECT planned FROM task WHERE id = ? AND ? > 0"),
    ("gruppemedlem kontrolleres ikke mot organisasjonen", "internal/store/groups.go", 'if err := inOrg(ctx, tx, "person", oid, p); err != nil {', 'if err := inOrg(ctx, tx, "person", oid, p); err != nil && p < 0 {'),
    ("gruppeoppgave for en oppgave som ikke fordeles", "internal/store/groups.go", "if !planned {", "if false {"),
    ("gruppens plasser utenfor 1-50", "internal/store/groups.go", "if t.Fills < 1 || t.Fills > 50 {", "if false {"),
    ("gruppens nivå valideres ikke", "internal/store/groups.go", "if !groupTaskLevels[t.Level] {", "if false {"),
    ("uendret medlemsliste godtas", "internal/store/groups.go", "if len(removed)+len(added) == 0 {", "if false {"),
    ("gruppe kan dupliseres i organisasjonen", "internal/store/migrations/006_groups.sql", "UNIQUE (org_id, name),", "UNIQUE (id),"),
    ("gruppe uten aktive medlemmer sendes", "internal/planner/input.go", "if len(sg.Members) == 0 {", "if false {"),
    ("inaktivt gruppemedlem sendes", "internal/planner/input.go", "if active[m] {", "if true {"),
    ("gruppe som skulle vært med mangler advarsel", "internal/planner/input.go", "if len(g.Tasks) > 0 {", "if false {"),
    ("ubrukt tom gruppe gir advarsel", "internal/planner/input.go", "if len(g.Tasks) > 0 {", "if true {"),
    ("gruppeoppgave for oppgave som ikke fordeles sendes", "internal/planner/input.go", "if planned[t.TaskID] { // the task may have been", "if true { // the task may have been"),
    ("«kan foreslås» for gruppe sendes ikke", "internal/planner/input.go", "Proposable: g.Proposable}", "Proposable: true}"),
    ("plasser for oppgaver som ikke fordeles", "internal/planner/input.go", "if planned[t.TaskID] {\n\t\t\t\tsnap.Slots", "if true {\n\t\t\t\tsnap.Slots"),
    ("låst person vises ikke som låst", "internal/planner/view.go", "Locked: a.Locked})", "Locked: false})"),
    ("gruppemedlem i kompromisslisten uten gruppen", "internal/planner/view.go", 'who += " (som medlem av " + groupName(c.Group) + ")"', 'who += ""'),
    ("manuell tekst vises ikke", "internal/planner/view.go", "c.texts = append(c.texts, CellText{Text: m.Label, Fills: fills})", "c.texts = c.texts[:0]"),
    ("plasser teller ikke grupper", "internal/planner/view.go", "c.Slots += g.Fills", "c.Slots += g.Fills * 0"),
    ("plasser teller ikke tekster", "internal/planner/view.go", "c.Slots += x.Fills", "c.Slots += x.Fills * 0"),
    ("plass uten svar fra planleggeren droppes", "internal/planner/view.go", "if n > 0 && cells[key{eid(e.ID), tid(task)}] == nil {", "if false {"),
    ("advarsel om «kan ikke foreslås» vises", "internal/planner/explain.go", 'case "ikke_foreslaas":\n\t\t\treturn ""', 'case "ikke_foreslaas":\n\t\t\treturn "x"'),
    ("utvalget nevnes ikke i forklaringen", "internal/planner/explain.go", 's = "utvalget er " + strings.Join(names, ", ")', 's = "utvalget er"'),
    ("gruppeforklaring mangler", "internal/planner/explain.go", 'if r, _ := groups[g].(string); r != "" && r != "kandidat" {', 'if r, _ := groups[g].(string); false && r != "" {'),
    # manuelle ruter og utvalg (omgang 2)
    ("tenant: ruter listes uten org og plan", "internal/store/cells.go", "WHERE c.org_id = ? AND c.plan_id = ? ORDER BY", "WHERE ? > 0 AND ? > 0 ORDER BY"),
    ("tenant: ruter listes uten plan", "internal/store/cells.go", "WHERE c.org_id = ? AND c.plan_id = ? ORDER BY", "WHERE c.org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: rute slettes uten plan", "internal/store/cells.go", "WHERE c.id = ? AND c.org_id = ? AND c.plan_id = ?`, id, oid, pid)", "WHERE c.id = ? AND c.org_id = ? AND ? > 0`, id, oid, pid)"),
    ("tenant: utvalg listes uten plan", "internal/store/cells.go", "WHERE m.org_id = ? AND m.plan_id = ? ORDER BY", "WHERE m.org_id = ? AND ? > 0 ORDER BY"),
    ("tenant: oppgave for rute slås opp uten org", "internal/store/cells.go", "SELECT name FROM task WHERE id = ? AND org_id = ?`, in.TaskID, oid)", "SELECT name FROM task WHERE id = ? AND ? > 0`, in.TaskID, oid)"),
    ("tenant: person for rute slås opp uten org", "internal/store/cells.go", "SELECT name, active FROM person WHERE id = ? AND org_id = ?`, in.PersonID, oid)", "SELECT name, active FROM person WHERE id = ? AND ? > 0`, in.PersonID, oid)"),
    ("tenant: gruppe for rute slås opp uten org", "internal/store/cells.go", "SELECT name FROM grp WHERE id = ? AND org_id = ?`, in.GroupID, oid)", "SELECT name FROM grp WHERE id = ? AND ? > 0`, in.GroupID, oid)"),
    ("rute uten hendelse godtas", "internal/store/cells.go", "if len(events) == 0 {\n\t\treturn nil, 0, fmt.Errorf", "if false {\n\t\treturn nil, 0, fmt.Errorf"),
    ("utvalg uten hendelse godtas", "internal/store/cells.go", 'if len(events) == 0 {\n\t\treturn fmt.Errorf("%w: choose at least one event"', 'if false {\n\t\treturn fmt.Errorf("%w: choose at least one event"'),
    ("rute som nevner to ting eller ingen godtas", "internal/store/cells.go", "if kinds != 1 {", "if kinds > 5 {"),
    ("en person fyller flere plasser i en rute", "internal/store/cells.go", " || (in.PersonID != 0 && fills != 1)", ""),
    ("inaktiv person i en rute", "internal/store/cells.go", 'if !active {\n\t\t\t\treturn logEntry{}, fmt.Errorf("%w: %s is not active", ErrInvalid, what)', 'if false {\n\t\t\t\treturn logEntry{}, fmt.Errorf("%w: %s is not active", ErrInvalid, what)'),
    ("inaktiv person i et utvalg", "internal/store/cells.go", 'if !active {\n\t\t\t\treturn logEntry{}, fmt.Errorf("%w: %s is not active", ErrInvalid, n)', 'if false {\n\t\t\t\treturn logEntry{}, fmt.Errorf("%w: %s is not active", ErrInvalid, n)'),
    ("utvalg for oppgave som ikke fordeles", "internal/store/cells.go", "if !planned {", "if false {"),
    ("samme rute to ganger gir ikke konflikt", "internal/store/cells.go", "if len(created) == 0 {", "if false {"),
    ("hoppet over telles ikke", "internal/store/cells.go", "skipped++", "skipped += 0"),
    ("utvalget erstattes ikke", "internal/store/cells.go", "AND event_id = ? AND task_id = ?`, oid, pid, ev, taskID)", "AND event_id = ? AND task_id = ? AND 0`, oid, pid, ev, taskID)"),
    ("utvalg for én oppgave fjerner andres utvalg", "internal/store/cells.go", "AND event_id = ? AND task_id = ?`, oid, pid, ev, taskID)", "AND event_id = ? AND ? > 0`, oid, pid, ev, taskID)"),
    ("samme person to ganger i samme rute", "internal/store/migrations/007_cells.sql", "CREATE UNIQUE INDEX manual_cell_person", "CREATE INDEX manual_cell_person"),
    ("rute som nevner flere ting (CHECK)", "internal/store/migrations/007_cells.sql", "CHECK ((person_id IS NOT NULL) + (group_id IS NOT NULL) + (label IS NOT NULL) = 1),", "CHECK (1),"),
    ("rute følger ikke hendelsen ved sletting", "internal/store/migrations/007_cells.sql", "REFERENCES event (org_id, plan_id, id) ON DELETE CASCADE,", "REFERENCES event (org_id, plan_id, id),"),
    ("manuell rute for inaktiv person sendes", "internal/planner/input.go", "if !active[*c.PersonID] {", "if false {"),
    ("manuell gruppe-rute uten aktive medlemmer sendes", "internal/planner/input.go", "if !groups[*c.GroupID] {", "if false {"),
    ("plasser for tekst-rute sendes ikke", "internal/planner/input.go", "l.Label, l.Fills = c.Label, c.Fills", "l.Label = c.Label"),
    ("plasser for gruppe-rute sendes ikke", "internal/planner/input.go", "l.Group, l.Fills = gid(*c.GroupID), c.Fills", "l.Group = gid(*c.GroupID)"),
    ("inaktiv person i utvalg sendes", "internal/planner/input.go", "for _, person := range pl.People {\n\t\t\tif active[person] {", "for _, person := range pl.People {\n\t\t\tif true {"),
    ("gruppe uten aktive medlemmer i utvalg sendes", "internal/planner/input.go", "for _, g := range pl.Groups {\n\t\t\tif groups[g] {", "for _, g := range pl.Groups {\n\t\t\tif true {"),
    ("tomt utvalg uten advarsel", "internal/planner/input.go", "if len(sp.People)+len(sp.Groups) == 0 {", "if false {"),
    ("utvalg med null i stedet for tom liste", "internal/planner/input.go", "People: []string{}, Groups: []string{}}", "People: nil, Groups: nil}"),
    ("tomt utvalg forklares ikke", "internal/planner/explain.go", 'if _, has := why["utenfor_utvalg"]; has || why["utvalg"] != nil || why["utvalg_grupper"] != nil {', 'if why["utvalg"] != nil || why["utvalg_grupper"] != nil {'),
    # personens svar (fase 3)
    ("tenant: person hentes uten org", "internal/store/preferences.go", "FROM person WHERE id = ? AND org_id = ?`, id, oid).", "FROM person WHERE id = ? AND ? > 0`, id, oid)."),
    ("tenant: oppgave hentes uten org ved valg", "internal/store/preferences.go", "WHERE id = ? AND org_id = ? AND planned = 1`, taskID, oid)", "WHERE id = ? AND ? > 0 AND planned = 1`, taskID, oid)"),
    ("tenant: hendelsen hører ikke til planen", "internal/store/preferences.go", "SELECT 1 FROM event WHERE id = ? AND org_id = ? AND plan_id = ?", "SELECT 1 FROM event WHERE id = ? AND org_id = ? AND ? > 0"),
    ("pref: andres oppgavevalg vises", "internal/store/preferences.go", "pt.org_id = t.org_id AND pt.person_id = ?", "pt.org_id = t.org_id AND ? > 0"),
    ("pref: andres tilgjengelighet vises", "internal/store/preferences.go", "pe.org_id = e.org_id AND pe.person_id = ?", "pe.org_id = e.org_id AND ? > 0"),
    ("pref: oppgave som ikke fordeles kan velges", "internal/store/preferences.go", "WHERE id = ? AND org_id = ? AND planned = 1`, taskID, oid)", "WHERE id = ? AND org_id = ?`, taskID, oid)"),
    ("pref: oppgave som ikke fordeles vises", "internal/store/preferences.go", "WHERE t.org_id = ? AND t.planned = 1", "WHERE t.org_id = ?"),
    ("pref: oppgavenivå uten kontroll", "internal/store/preferences.go", "if !taskLevels[level] {", "if false {"),
    ("pref: hendelsesnivå uten kontroll", "internal/store/preferences.go", 'if l != "" && !eventLevels[l] {', 'if l == "aldri" {'),
    ("pref: nøytral fjerner ikke svaret", "internal/store/preferences.go", 'if want == "" {', "if false {"),
    ("pref: uendret svar godtas", "internal/store/preferences.go", "if changed == 0 {", "if false {"),
    ("pref: uendret oppgavevalg godtas", "internal/store/preferences.go", "if before == level {", "if false {"),
    ("API krever ikke JSON", "internal/api/api.go", 'len(ct) < 16 || ct[:16] != "application/json"', "len(ct) < 0"),
    ("API godtar ukjente felt", "internal/api/api.go", "dec.DisallowUnknownFields()", ""),
    ("API har ingen størrelsesgrense", "internal/api/api.go", "r.Body = http.MaxBytesReader(w, r.Body, 64<<10)", ""),
    ("API mangler sikkerhetshoder", "internal/api/api.go", 'h.Set("Content-Security-Policy"', 'h.Set("X-Csp-Av"'),
]

MUTATIONS: list[tuple[str, str, str, str]] = [(n, MODEL, a, b) for n, a, b in PY_MUTATIONS] + GO_MUTATIONS


GO_SOURCES = ["cmd", "internal", "web", "go.mod", "go.sum"]  # det Go-testene trenger
_local = threading.local()
_copies: list[Path] = []


def go_copy() -> Path:
    """Denne trådens egen kopi av Go-kildene (laget første gang), så mutasjoner kan kjøre samtidig."""
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
    """(0 = alle bestod; 1 = en test feilet; ellers = kunne ikke kjøres, f.eks. byggfeil)."""
    if target.endswith(".py"):
        r = subprocess.run([sys.executable, "-m", "pytest", "-x", *TESTS], cwd=ROOT, capture_output=True, text=True, check=False)
        return r.returncode, r.stdout
    # Uten planlegger i miljøet hoppes de to testene mot den ekte løseren over (ca. 1,3 s hver gang). Reglene skal fanges
    # av testene med falsk løser; en mutasjon bare de ekte testene fanger, skal vises som OVERLEVDE.
    env = {**os.environ, "TJENESTEPLAN_SOLVER": ""}
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
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--only", help="bare mutasjoner med denne teksten i navnet")
    ap.add_argument("--list", action="store_true", help="vis navnene og avslutt")
    args = ap.parse_args()

    chosen = [m for m in MUTATIONS if not args.only or args.only.lower() in m[0].lower()]
    if args.list:
        print("\n".join(name for name, *_ in chosen))
        return 0
    if not chosen:
        print(f"ingen mutasjoner matcher «{args.only}»")
        return 1

    if BACKUP.exists():  # forrige kjøring ble avbrutt midt i en mutasjon
        for saved in BACKUP.iterdir():
            (REPO / saved.name.replace("__", "/")).write_text(saved.read_text(encoding="utf-8"), encoding="utf-8")
            saved.unlink()
        BACKUP.rmdir()
        print("gjenopprettet filer etter en avbrutt kjøring")
    if tests_pass() != 0:
        print("testene feiler allerede uten mutasjoner; rett det først (mise run test)")
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
            return f"OVERLEVDE {name}: ingen test feilet"
        return f"UGYLDIG   {name}: testene kunne ikke kjøres (kode {rc}); byggfeil i mutasjonen?"

    def try_go(item: tuple[str, str, str, str]) -> str | None:
        name, f, old, new = item
        original = originals[f]
        if old not in original:
            return f"UTDATERT  {name}: mønsteret finnes ikke i {f}"
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
            return f"UTDATERT  {name}: mønsteret finnes ikke i {f}"
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
            futures = {m[0]: pool.submit(try_go, m) for m in go_items}  # Go i parallell, hver i sin kopi
            for m in py_items:  # Python i repoet, én om gangen, mens Go går
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
    print(f"mutations: {caught}/{len(chosen)} fanget ({time.time() - t0:.0f} s)")
    return 1 if problems else 0


if __name__ == "__main__":
    raise SystemExit(main())
