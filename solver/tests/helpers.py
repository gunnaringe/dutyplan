from __future__ import annotations

from datetime import date, timedelta

from dutyplan_solver import check, solve

NO_FAIR = {"w_fair": 0, "w_fair_range": 0, "w_task_fair": 0, "w_task_range": 0}


def dates(n: int) -> list[str]:
    start = date(2026, 9, 6)
    return [(start + timedelta(days=7 * i)).isoformat() for i in range(n)]


def base(people=("a", "b", "c"), n_dates=4, tasks=None, **extra):
    """Små inndata: alle er med på alle oppgaver, ingen preferanser."""
    tasks = tasks or {"t": 1}
    inst = {
        "tasks": [{"id": t} for t in tasks],
        "dates": [{"id": d, "type": "g"} for d in dates(n_dates)],
        "day_type_tasks": {"g": dict(tasks)},
        "people": [{"id": p} for p in people],
        "person_tasks": {p: {t: "neutral" for t in tasks} for p in people},
        # workers: 1 gir samme svar hver gang. Flere tråder velger ulikt blant likeverdige løsninger, og da kunne en mutasjon
        # (f.eks. «jevn fordeling av») av og til overleve under CPU-last, fordi uavgjort tilfeldigvis ga riktig utfall.
        "params": {"time_limit": 10, "workers": 1},
    }
    params = extra.pop("params", {})
    inst["params"].update(params)
    inst.update(extra)
    return inst


def run(inst):
    """Løs og sjekk at planen ikke bryter noe hardt krav."""
    res = solve(inst)
    assert res["status"] in ("OPTIMAL", "FEASIBLE"), res["status"]
    problems = check(inst, res)
    assert not problems, problems
    return res


def who(res, task=None, date=None):
    return [
        a["person"]
        for a in res["assignments"]
        if (task is None or a["task"] == task) and (date is None or a["date"] == date)
    ]


def count(res, person, task=None):
    return sum(1 for a in res["assignments"] if a["person"] == person and (task is None or a["task"] == task))
