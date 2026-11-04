"""Mutasjonssjekk: bryt én regel om gangen i model.py og se at minst én test feiler.

    mise run mutations                 alle (ca. 30 s); stille ved suksess
    mise run mutations -- --only gruppe   bare de som har «gruppe» i navnet
    mise run mutations -- --list       vis navnene

Skriver bare det som krever handling: mutasjoner som overlevde (mangler en test), mønstre som ikke
finnes lenger (utdatert liste) og mutasjoner som ikke lar seg kjøre. Avsluttes med én oppsummeringslinje.
model.py gjenopprettes alltid; en sikkerhetskopi ligger i .mutation-backup-model.py mens det pågår.
"""

from __future__ import annotations

import argparse
import subprocess
import sys
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
MODEL = ROOT / "tjenesteplan_solver" / "model.py"
BACKUP = ROOT / ".mutation-backup-model.py"
TESTS = ["tests/test_rules.py", "tests/test_random.py", "tests/test_fixture.py"]  # raskeste først, så -x stopper tidlig

# (navn, mønster i model.py, erstatning). Mønsteret erstattes bare første gang det forekommer.
MUTATIONS: list[tuple[str, str, str]] = [
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


def tests_pass() -> int:
    r = subprocess.run(
        [sys.executable, "-m", "pytest", "-x", *TESTS], cwd=ROOT, capture_output=True, text=True, check=False
    )
    return r.returncode


def main() -> int:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--only", help="bare mutasjoner med denne teksten i navnet")
    ap.add_argument("--list", action="store_true", help="vis navnene og avslutt")
    args = ap.parse_args()

    chosen = [m for m in MUTATIONS if not args.only or args.only.lower() in m[0].lower()]
    if args.list:
        print("\n".join(name for name, _a, _b in chosen))
        return 0
    if not chosen:
        print(f"ingen mutasjoner matcher «{args.only}»")
        return 1

    if BACKUP.exists():  # forrige kjøring ble avbrutt midt i en mutasjon
        MODEL.write_text(BACKUP.read_text(encoding="utf-8"), encoding="utf-8")
        BACKUP.unlink()
        print("gjenopprettet model.py etter en avbrutt kjøring")
    original = MODEL.read_text(encoding="utf-8")
    if tests_pass() != 0:
        print("testene feiler allerede uten mutasjoner; rett det først (mise run test)")
        return 1

    BACKUP.write_text(original, encoding="utf-8")
    problems: list[str] = []
    caught = 0
    t0 = time.time()
    try:
        for name, old, new in chosen:
            if old not in original:
                problems.append(f"UTDATERT  {name}: mønsteret finnes ikke i model.py")
                continue
            MODEL.write_text(original.replace(old, new, 1), encoding="utf-8")
            rc = tests_pass()
            if rc == 1:
                caught += 1
            elif rc == 0:
                problems.append(f"OVERLEVDE {name}: ingen test feilet")
            else:
                problems.append(f"UGYLDIG   {name}: pytest ga kode {rc} (syntaksfeil i mutasjonen?)")
    finally:
        MODEL.write_text(original, encoding="utf-8")
        BACKUP.unlink(missing_ok=True)

    for line in problems:
        print(line)
    print(f"mutations: {caught}/{len(chosen)} fanget ({time.time() - t0:.0f} s)")
    return 1 if problems else 0


if __name__ == "__main__":
    raise SystemExit(main())
