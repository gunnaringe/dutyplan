"""Sammenlign vektsett for jevn fordeling på testplanen for høsten (host2026.json).

    uv run python scripts/fairness_experiment.py [tidsgrense] [frø...]

Måler det brukeren ser: verste avvik fra egen andel samlet (det som skal være jevnt), og per oppgave (for sammenligning).
"""

import json
import sys
from pathlib import Path

from tjenesteplan_solver import check, solve

FIXTURE = Path(__file__).parent.parent / "fixtures" / "host2026.json"
SETS = {
    "total   (samlet 10/10, oppgave 0/0)  [standard]": {},
    "total+  (samlet 10/10, oppgave 3/0)": {"w_task_fair": 3, "w_task_range": 0},
    "oppgave (samlet 3/3, oppgave 10/10)  [til 2026-11-09]": {"w_fair": 3, "w_fair_range": 3, "w_task_fair": 10, "w_task_range": 10},
}


def metrics(res):
    worst, spread = [], []
    for t, per in res["task_load"].items():
        dev = {p: v["count"] - v["target"] for p, v in per.items()}
        worst.append(max(abs(x) for x in dev.values()))
        counts = [v["count"] for v in per.values() if v["capacity"] >= 0.7 * max(w["capacity"] for w in per.values())]
        spread.append(max(counts) - min(counts) if counts else 0)
    overall = max(abs(v["count"] + v["history"] - v["target"] - v["history"]) for v in res["load"].values())
    return max(worst), max(spread), overall


if __name__ == "__main__":
    tl = float(sys.argv[1]) if len(sys.argv) > 1 else 20
    seeds = [int(x) for x in sys.argv[2:]] or [1, 2, 3]
    data = json.loads(FIXTURE.read_text(encoding="utf-8"))
    print(f"{tl:g} s per kjøring, frø {seeds}. Kolonner: verste avvik per oppgave | største spredning per oppgave | verste samlet avvik | «dårlig»")
    for name, w in SETS.items():
        rows = []
        for seed in seeds:
            d = json.loads(json.dumps(data))
            d["params"].update(time_limit=tl, seed=seed, **w)
            r = solve(d)
            assert not check(d, r) and not r["unfilled"]
            rows.append((*metrics(r), len(r["concessions"])))
        print(f"{name}: " + "  ".join(f"[{a:.1f} | {b} | {c:.1f} | {e}]" for a, b, c, e in rows), flush=True)
