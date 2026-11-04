"""Se på en plan: utvalg og låste ruter, fordeling per oppgave, dobbeltbookinger.

    uv run python scripts/look_at_plan.py [tidsgrense] [frø]
"""

import json
import sys
from collections import defaultdict
from pathlib import Path

from tjenesteplan_solver import check, solve

FIXTURE = Path(__file__).parent.parent / "fixtures" / "host2026.json"

if __name__ == "__main__":
    data = json.loads(FIXTURE.read_text(encoding="utf-8"))
    data["params"]["time_limit"] = float(sys.argv[1]) if len(sys.argv) > 1 else 20
    data["params"]["seed"] = int(sys.argv[2]) if len(sys.argv) > 2 else 1
    r = solve(data)
    A = r["assignments"]
    print(r["status"], "mål", r["objective"], "grense", r["bound"], "brudd:", check(data, r) or "ingen")
    print("tomme plasser:", r["unfilled"] or "ingen", "| advarsler:", r["warnings"] or "ingen")

    print("\nUtvalg:")
    for pool in data["pools"]:
        got = [a["person"] for a in A if a["date"] == pool["date"] and a["task"] == pool["task"]]
        print(f"  {pool['date']} {pool['task']:6} utvalg {pool['people']} -> {got}")
    print("Manuelle ruter:", r["manual"], [a for a in A if a["locked"]])

    print("\nPer oppgave (antall / mål):")
    for t, per in r["task_load"].items():
        print(f"  {t:10}", "  ".join(f"{p} {v['count']}/{v['target']}" for p, v in sorted(per.items())))

    by = defaultdict(list)
    for a in A:
        by[a["person"], a["date"]].append(a["task"])
    print("\nTo oppgaver samme dag:", {f"{p} {d}": ts for (p, d), ts in by.items() if len(ts) > 1} or "ingen")
    print("«dårlig» brukt:", len(r["concessions"]))
