"""Kvalitet mot tidsgrense på HØST 2026-formen.  uv run python scripts/timing.py"""

import json
from pathlib import Path

from tjenesteplan_solver import check, solve

FIXTURE = Path(__file__).parent.parent / "fixtures" / "host2026.json"

if __name__ == "__main__":
    data = json.loads(FIXTURE.read_text(encoding="utf-8"))
    for tl in (2, 5, 10, 20, 40):
        d = json.loads(json.dumps(data))
        d["params"]["time_limit"] = tl
        r = solve(d)
        print(
            f"{tl:3}s  {r['status']:9} obj {r['objective']:9.2f}  bound {r['bound']:8.2f}  "
            f"tomme {len(r['unfilled'])}  brudd {len(check(d, r))}  dårlig {len(r['concessions'])}",
            flush=True,
        )
