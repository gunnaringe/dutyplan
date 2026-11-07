"""Mutasjonssjekk: bryt én regel om gangen (planleggeren i Python og appen i Go) og se at minst én test feiler.

    mise run mutations                 alle (ca. 30 s); stille ved suksess
    mise run mutations -- --only gruppe   bare de som har «gruppe» i navnet
    mise run mutations -- --list       vis navnene

Skriver bare det som krever handling: mutasjoner som overlevde (mangler en test), mønstre som ikke
finnes lenger (utdatert liste) og mutasjoner som ikke lar seg kjøre. Avsluttes med én oppsummeringslinje.
Filene gjenopprettes alltid; sikkerhetskopier ligger i solver/.mutation-backup/ mens det pågår.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import time
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
    ("API krever ikke JSON", "internal/api/api.go", 'len(ct) < 16 || ct[:16] != "application/json"', "len(ct) < 0"),
    ("API godtar ukjente felt", "internal/api/api.go", "dec.DisallowUnknownFields()", ""),
    ("API har ingen størrelsesgrense", "internal/api/api.go", "r.Body = http.MaxBytesReader(w, r.Body, 64<<10)", ""),
    ("API mangler sikkerhetshoder", "internal/api/api.go", 'h.Set("Content-Security-Policy"', 'h.Set("X-Csp-Av"'),
]

MUTATIONS: list[tuple[str, str, str, str]] = [(n, MODEL, a, b) for n, a, b in PY_MUTATIONS] + GO_MUTATIONS


def run_tests(target: str) -> tuple[int, str]:
    """(0 = alle bestod; 1 = en test feilet; ellers = kunne ikke kjøres, f.eks. byggfeil)."""
    if target.endswith(".py"):
        r = subprocess.run([sys.executable, "-m", "pytest", "-x", *TESTS], cwd=ROOT, capture_output=True, text=True, check=False)
        return r.returncode, r.stdout
    r = subprocess.run(["go", "test", "-count=1", "-failfast", "./..."], cwd=REPO, capture_output=True, text=True, check=False)
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
    BACKUP.mkdir(exist_ok=True)
    for f, text in originals.items():
        (BACKUP / f.replace("/", "__")).write_text(text, encoding="utf-8")
    problems: list[str] = []
    caught = 0
    t0 = time.time()
    try:
        for name, f, old, new in chosen:
            original = originals[f]
            if old not in original:
                problems.append(f"UTDATERT  {name}: mønsteret finnes ikke i {f}")
                continue
            (REPO / f).write_text(original.replace(old, new, 1), encoding="utf-8")
            rc, _out = run_tests(f if f.endswith(".py") else "x.go")
            (REPO / f).write_text(original, encoding="utf-8")
            if rc == 1:
                caught += 1
            elif rc == 0:
                problems.append(f"OVERLEVDE {name}: ingen test feilet")
            else:
                problems.append(f"UGYLDIG   {name}: testene kunne ikke kjøres (kode {rc}); byggfeil i mutasjonen?")
    finally:
        for f, text in originals.items():
            (REPO / f).write_text(text, encoding="utf-8")
        for saved in BACKUP.iterdir():
            saved.unlink()
        BACKUP.rmdir()

    for line in problems:
        print(line)
    print(f"mutations: {caught}/{len(chosen)} fanget ({time.time() - t0:.0f} s)")
    return 1 if problems else 0


if __name__ == "__main__":
    raise SystemExit(main())
