"""Run the planner and print a SHORT summary (by default about 8 lines, cheap to read for an AI).

    mise run plan                       the test data, 20 s
    mise run plan -- --time 8 --seed 2
    mise run plan -- --info             only what the input contains (instead of reading the JSON file)
    mise run plan -- --full             everything: selections, manual cells, figures per task and person
    mise run plan -- --input fil.json
"""

from __future__ import annotations

import argparse
import json
from collections import defaultdict
from pathlib import Path

from dutyplan_solver import check, solve

FIXTURE = Path(__file__).resolve().parent.parent / "fixtures" / "autumn2026.json"


def info(data: dict) -> str:
    types = defaultdict(int)
    for d in data["dates"]:
        types[d["type"]] += 1
    slots = sum(n for d in data["dates"] for n in data["day_type_tasks"].get(d["type"], {}).values())
    return "\n".join(
        [
            f"{len(data['people'])} people, {len(data['tasks'])} tasks, {len(data['dates'])} dates "
            f"({', '.join(f'{n} {t}' for t, n in sorted(types.items()))}), about {slots} places",
            f"{len(data.get('groups', []))} groups, {len(data.get('group_tasks', []))} group tasks, "
            f"{len(data.get('pools', []))} selections, {len(data.get('locked', []))} manual cells, "
            f"{len(data.get('combinations', []))} combinations",
            f"not proposable: {[p['id'] for p in data['people'] if not p.get('proposable', True)] or 'none'}",
        ]
    )


def deviations(res: dict) -> tuple[str, str, str]:
    """Worst deviation per task, largest spread per task, worst overall deviation."""
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
        f"worst deviation per task {worst_t[0]:.1f} ({worst_t[1]}), largest spread {spread_t[0]} ({spread_t[1]})",
        f"overall: worst deviation {worst_all[0]:.1f} ({worst_all[1]})",
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
        f"{res['status']} {data['params']['time_limit']:g} s (seed {data['params']['seed']}) | objective {res['objective']:.1f}, "
        f"bound {res['bound']:.1f} | violations: {problems or 'none'}",
        f"places: {len(res['assignments'])} people + {len(res['group_assignments'])} groups + {len(res['manual'])} manual"
        f" | empty: {sum(u['missing'] for u in res['unfilled'])} | «poor»: {len(res['concessions'])}"
        f" | warnings: {len(res['warnings'])}",
        per_task,
        overall,
        f"proposed groups (dates): {dict(sorted(teams.items())) or 'none'}",
        f"people with two tasks the same day: {two}",
    ]
    for u in res["unfilled"][:5]:
        lines.append(f"EMPTY {u['date']} {u['task']} (missing {u['missing']}): {json.dumps(u['why'], ensure_ascii=False)}")
    lines.extend(f"WARNING {w}" for w in res["warnings"][:5])
    return "\n".join(lines)


def full(data: dict, res: dict) -> str:
    out = []
    for pool in data.get("pools", []):
        got = [a["person"] for a in res["assignments"] if a["date"] == pool["date"] and a["task"] == pool["task"]]
        out.append(f"selection {pool['date']} {pool['task']}: {pool.get('people', [])} {pool.get('groups', [])} -> {got}")
    out.append(f"manual cells: {res['manual']}")
    out.append(f"locked people: {[a for a in res['assignments'] if a['locked']]}")
    for g in res["group_assignments"]:
        out.append(f"group {g['date']} {g['task']}: {g['group']} {g['members']}")
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
        print(res["status"], "- no plan")
        return
    print(brief(data, res, check(data, res)))
    if args.full:
        print(full(data, res))


if __name__ == "__main__":
    main()
