"""Independent checker: is a finished plan allowed to look like this?

Written from the input alone (shares no rule code with model.py), so a bug in the
model is not also here. Checks hard requirements only, not whether the plan is the best possible.
"""

from __future__ import annotations

from collections import Counter, defaultdict
from itertools import combinations
from typing import Any

from .model import DEFAULT_PARAMS  # the default values only; the rule logic is written again here


def check(data: dict[str, Any], result: dict[str, Any]) -> list[str]:
    bad: list[str] = []
    if result.get("status") not in ("OPTIMAL", "FEASIBLE"):
        return [f"no plan to check (status {result.get('status')})"]

    people = {p["id"] for p in data["people"]}
    proposable_p = {p["id"]: bool(p.get("proposable", True)) for p in data["people"]}
    planned = {t["id"] for t in data["tasks"] if t.get("planned", True)}
    dtype = {d["id"]: d["type"] for d in data["dates"]}
    default_combo = (data.get("params") or {}).get("default_combination", DEFAULT_PARAMS["default_combination"])

    slots: dict[tuple[str, str], int] = {}
    for d, ty in dtype.items():
        for t, n in data["day_type_tasks"].get(ty, {}).items():
            if t in planned and n > 0:
                slots[d, t] = n

    locked_p: dict[tuple[str, str], list[str]] = defaultdict(list)
    locked_l: dict[tuple[str, str], int] = defaultdict(int)
    for lk in data.get("locked", []):
        if "person" in lk:
            locked_p[lk["date"], lk["task"]].append(lk["person"])
        else:
            locked_l[lk["date"], lk["task"]] += int(lk.get("fills", 1))
    for key in set(locked_p) | set(locked_l):
        slots[key] = max(slots.get(key, 0), len(locked_p.get(key, [])) + locked_l.get(key, 0))

    pools = {(pl["date"], pl["task"]): set(pl.get("people", [])) for pl in data.get("pools", [])}
    combo = {
        frozenset((c["a"], c["b"])): c["level"] for c in data.get("combinations", [])
    }

    def date_lvl(p: str, t: str, d: str) -> str:
        cal = data.get("person_task_calendars", {}).get(p, {}).get(t)
        src = cal if cal is not None else data.get("person_dates", {}).get(p, {})
        return src.get(d, "neutral")

    assigns = result["assignments"]
    seen = Counter((a["person"], a["task"], a["date"]) for a in assigns)
    for k, n in seen.items():
        if n > 1:
            bad.append(f"duplicate: {k}")

    per_cell: Counter[tuple[str, str]] = Counter()
    for a in assigns:
        p, t, d = a["person"], a["task"], a["date"]
        key = (d, t)
        per_cell[key] += 1
        if p not in people:
            bad.append(f"unknown person {p}")
            continue
        if key not in slots:
            bad.append(f"{p} is assigned to {t} on {d}, but the task does not apply that day")
            continue
        if p in locked_p.get(key, []):
            continue  # a manual choice wins over the filters
        if a.get("locked"):
            bad.append(f"{p} {t} {d} is marked locked, but is not in the input")
        if not proposable_p[p]:
            bad.append(f"{p} cannot be proposed, but is assigned to {t} on {d}")
        if t not in data.get("person_tasks", {}).get(p, {}):
            bad.append(f"{p} does not take part in {t} ({d})")
        elif data["person_tasks"][p][t] == "never":
            bad.append(f"{p} said 'never' to {t} ({d})")
        if date_lvl(p, t, d) == "never":
            bad.append(f"{p} has 'never' on {d} for {t}")
        if key in pools and p not in pools[key]:
            bad.append(f"{p} is not in the selection for {t} {d}")

    for key, ps in locked_p.items():
        for p in ps:
            if seen[p, key[1], key[0]] == 0:
                bad.append(f"locked {p} {key[1]} {key[0]} is missing from the plan")

    missing = {(u["date"], u["task"]): u["missing"] for u in result["unfilled"]}
    for key, n in slots.items():
        filled = per_cell[key] + locked_l.get(key, 0) + missing.get(key, 0)
        if filled != n:
            bad.append(f"{key}: {per_cell[key]} people + {locked_l.get(key, 0)} manual + {missing.get(key, 0)} empty != {n} places")

    by_pd: dict[tuple[str, str], set[str]] = defaultdict(set)
    for a in assigns:
        by_pd[a["person"], a["date"]].add(a["task"])
    for (p, d), ts in by_pd.items():
        lk = {t for t in ts if p in locked_p.get((d, t), [])}
        for t1, t2 in combinations(sorted(ts), 2):
            if t1 in lk and t2 in lk:
                continue
            if combo.get(frozenset((t1, t2)), default_combo) == "cannot":
                bad.append(f"{p} has {t1} and {t2} on {d}, which cannot be combined")
    return bad
