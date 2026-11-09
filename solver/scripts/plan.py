"""Kjør planleggeren og skriv en KORT oppsummering (standard ca. 8 linjer, billig å lese for en AI).

    mise run plan                       testdataene, 20 s
    mise run plan -- --time 8 --seed 2
    mise run plan -- --info             bare hva inndataene inneholder (i stedet for å lese JSON-filen)
    mise run plan -- --full             alt: utvalg, manuelle ruter, tall per oppgave og person
    mise run plan -- --input fil.json
"""

from __future__ import annotations

import argparse
import json
from collections import defaultdict
from pathlib import Path

from dutyplan_solver import check, solve

FIXTURE = Path(__file__).resolve().parent.parent / "fixtures" / "host2026.json"


def info(data: dict) -> str:
    types = defaultdict(int)
    for d in data["dates"]:
        types[d["type"]] += 1
    slots = sum(n for d in data["dates"] for n in data["day_type_tasks"].get(d["type"], {}).values())
    return "\n".join(
        [
            f"{len(data['people'])} personer, {len(data['tasks'])} oppgaver, {len(data['dates'])} datoer "
            f"({', '.join(f'{n} {t}' for t, n in sorted(types.items()))}), ca. {slots} plasser",
            f"{len(data.get('groups', []))} grupper, {len(data.get('group_tasks', []))} gruppe-oppgaver, "
            f"{len(data.get('pools', []))} utvalg, {len(data.get('locked', []))} manuelle ruter, "
            f"{len(data.get('combinations', []))} kombinasjoner",
            f"ikke foreslåbare: {[p['id'] for p in data['people'] if not p.get('proposable', True)] or 'ingen'}",
        ]
    )


def deviations(res: dict) -> tuple[str, str, str]:
    """Verste avvik per oppgave, største spredning per oppgave, verste samlede avvik."""
    worst_t, spread_t = (0.0, "-"), (0, "-")
    for t, per in res["task_load"].items():
        for p, v in per.items():
            dev = abs(v["count"] - v["target"])
            if dev > worst_t[0]:
                worst_t = (dev, f"{t}/{p}")
        big = max(v["capacity"] for v in per.values())
        counts = [v["count"] for v in per.values() if v["capacity"] >= 0.7 * big]
        if counts and max(counts) - min(counts) > spread_t[0]:
            spread_t = (max(counts) - min(counts), t)
    worst_all = max(((abs(v["count"] - v["target"]), p) for p, v in res["load"].items()), default=(0.0, "-"))
    return (
        f"verste avvik per oppgave {worst_t[0]:.1f} ({worst_t[1]}), største spredning {spread_t[0]} ({spread_t[1]})",
        f"samlet: verste avvik {worst_all[0]:.1f} ({worst_all[1]})",
        "",
    )


def brief(data: dict, res: dict, problems: list[str]) -> str:
    teams = defaultdict(int)
    for g in res["group_assignments"]:
        teams[g["group"]] += 1
    by = defaultdict(list)
    for a in res["assignments"]:
        by[a["person"], a["date"]].append(a["task"])
    two = sum(1 for ts in by.values() if len(ts) > 1)
    per_task, overall, _ = deviations(res)
    lines = [
        f"{res['status']} {data['params']['time_limit']:g} s (frø {data['params']['seed']}) | mål {res['objective']:.1f}, "
        f"grense {res['bound']:.1f} | brudd: {problems or 'ingen'}",
        f"plasser: {len(res['assignments'])} personer + {len(res['group_assignments'])} grupper + {len(res['manual'])} manuelle"
        f" | tomme: {sum(u['missing'] for u in res['unfilled'])} | «dårlig»: {len(res['concessions'])}"
        f" | advarsler: {len(res['warnings'])}",
        per_task,
        overall,
        f"foreslåtte grupper (datoer): {dict(sorted(teams.items())) or 'ingen'}",
        f"personer med to oppgaver samme dag: {two}",
    ]
    for u in res["unfilled"][:5]:
        lines.append(f"TOM {u['date']} {u['task']} (mangler {u['missing']}): {json.dumps(u['why'], ensure_ascii=False)}")
    lines.extend(f"ADVARSEL {w}" for w in res["warnings"][:5])
    return "\n".join(lines)


def full(data: dict, res: dict) -> str:
    out = []
    for pool in data.get("pools", []):
        got = [a["person"] for a in res["assignments"] if a["date"] == pool["date"] and a["task"] == pool["task"]]
        out.append(f"utvalg {pool['date']} {pool['task']}: {pool.get('people', [])} {pool.get('groups', [])} -> {got}")
    out.append(f"manuelle ruter: {res['manual']}")
    out.append(f"låste personer: {[a for a in res['assignments'] if a['locked']]}")
    for g in res["group_assignments"]:
        out.append(f"gruppe {g['date']} {g['task']}: {g['group']} {g['members']}")
    for t, per in res["task_load"].items():
        out.append(f"{t:10} " + "  ".join(f"{p} {v['count']}/{v['target']}" for p, v in sorted(per.items())))
    return "\n".join(out)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    ap.add_argument("--input", type=Path, default=FIXTURE)
    ap.add_argument("--time", type=float, default=20)
    ap.add_argument("--seed", type=int, default=1)
    ap.add_argument("--info", action="store_true")
    ap.add_argument("--full", action="store_true")
    args = ap.parse_args()

    data = json.loads(args.input.read_text(encoding="utf-8"))
    if args.info:
        print(info(data))
        return
    data["params"].update(time_limit=args.time, seed=args.seed)
    res = solve(data)
    if res["status"] not in ("OPTIMAL", "FEASIBLE"):
        print(res["status"], "- ingen plan")
        return
    print(brief(data, res, check(data, res)))
    if args.full:
        print(full(data, res))


if __name__ == "__main__":
    main()
