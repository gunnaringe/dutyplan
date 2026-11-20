"""CP-SAT planner for Dutyplan.

Input and output are plain dicts (JSON). Levels are ASCII in the wire format:

  task and date:  never | poor | neutral | good          (not marked = neutral)
  combination:    cannot | impractical | practical | preferred

Input (everything except tasks, dates, day_type_tasks and people is optional):

  tasks:               [{"id", "planned": true}]
  dates:               [{"id": "2026-09-20", "type": "weekend"}]   events: the id is arbitrary text, and two events on the same calendar day have two different ids.
                       Everything (availability, combinations, places) applies to the event, not the calendar day.
  day_type_tasks:      {"weekend": {"shift": 1, "coffee": 3}}
  people:              [{"id", "proposable": true}]   proposable=false: the planner never chooses the person (manual cells are fine)
  person_tasks:        {person: {task: level}}         the person takes part in the task if the key exists
  person_dates:        {person: {date: level}}         one calendar for all the person's tasks
  person_task_calendars: {person: {task: {date: level}}}  "own calendar", replaces the shared one for that task
  combinations:        [{"a", "b", "level"}]
  pools:               [{"date", "task", "people": [...]}]
                       selection: only these people can be chosen that day
  locked:              manual cells: {"date", "task", "person"} | {"date", "task", "label"};
                       a label may have "fills" (places, default 1).
  history:             {person: {task: number of assignments in earlier periods}}   counts half by default
  previous_plan:       [{"date", "task", "person"}]
  params:              see DEFAULT_PARAMS

Output: see solve().
"""

from __future__ import annotations

from collections import defaultdict
from itertools import combinations as _pairs
from typing import Any

from ortools.sat.python import cp_model

TASK_LEVELS = ("never", "poor", "neutral", "good")
COMBO_LEVELS = ("cannot", "impractical", "practical", "preferred")
SCALE = 1000  # poeng -> heltall

DEFAULT_PARAMS: dict[str, Any] = {
    "time_limit": 20.0,
    "workers": 8,
    "seed": 1,
    "default_combination": "impractical",  # for task pairs that are not set
    "w_task": {"poor": -4, "neutral": 0, "good": 2},
    "w_date": {"poor": -6, "neutral": 0, "good": 3},
    "w_combo": {"impractical": -5, "practical": 2, "preferred": 6},
    # «Equal distribution» = equal TOTAL load (the user's choice, 2026-11-09): someone who only does Video must not end up with half
    # of what someone doing Audio and Video gets. Distribution per task is therefore off by default; the weights exist to try it.
    "w_fair": 10,  # points per assignment a person is outside their share of all assignments: the main term for equal distribution
    "w_fair_range": 10,  # ... and for the worst deviation
    "w_task_fair": 0,  # like w_fair, but per task (an equal share of each task, and so unequal totals for people with several tasks)
    "w_task_range": 0,  # the worst deviation per task
    "history_weight": 0.5,  # how much earlier periods count («a little less for whoever did the most»)
    "w_stability": 2,  # keep the previous plan's assignments
    "w_unfilled": 1000,  # an empty place; must come after everything else
    "solver": {},  # raw CP-SAT parameters, e.g. {"linearization_level": 2}
}


def _merge_params(raw: dict[str, Any] | None) -> dict[str, Any]:
    p = {k: (dict(v) if isinstance(v, dict) else v) for k, v in DEFAULT_PARAMS.items()}
    for k, v in (raw or {}).items():
        if k not in DEFAULT_PARAMS:
            raise ValueError(f"ukjent parameter: {k}")
        if isinstance(v, dict):
            p[k].update(v)
        else:
            p[k] = v
    if p["default_combination"] not in COMBO_LEVELS:
        raise ValueError(f"ukjent default_combination: {p['default_combination']}")
    return p


class Problem:
    """The input looked up and normalised, shared between the model and the explanations."""

    def __init__(self, data: dict[str, Any]) -> None:
        self.params = _merge_params(data.get("params"))
        self.data = data
        self.people = [p["id"] for p in data["people"]]
        self.person_proposable = {p["id"]: bool(p.get("proposable", True)) for p in data["people"]}
        self.planned = {t["id"] for t in data["tasks"] if t.get("planned", True)}
        self.dates = sorted(d["id"] for d in data["dates"])
        self.date_type = {d["id"]: d["type"] for d in data["dates"]}
        if len(set(self.people)) != len(self.people):
            raise ValueError("people has duplicate ids")

        self.task_level: dict[str, dict[str, str]] = {}
        for p, levels in data.get("person_tasks", {}).items():
            for t, lvl in levels.items():
                self._level(lvl)
            self.task_level[p] = dict(levels)
        self.person_dates = {
            p: {d: self._level(lvl) for d, lvl in cal.items()}
            for p, cal in data.get("person_dates", {}).items()
        }
        self.task_cal = {
            p: {t: {d: self._level(lvl) for d, lvl in cal.items()} for t, cal in tasks.items()}
            for p, tasks in data.get("person_task_calendars", {}).items()
        }

        # places per (date, task)
        self.slots: dict[tuple[str, str], int] = {}
        for d in self.dates:
            for t, n in data["day_type_tasks"].get(self.date_type[d], {}).items():
                if t in self.planned and n > 0:
                    self.slots[d, t] = n

        # manuelle ruter
        self.locked_person: dict[tuple[str, str], list[str]] = defaultdict(list)
        self.locked_other: dict[tuple[str, str], list[dict[str, Any]]] = defaultdict(list)  # text
        self.fixed_fill: dict[tuple[str, str], int] = defaultdict(int)  # places filled without a person
        for lk in data.get("locked", []):
            key = (lk["date"], lk["task"])
            if "person" in lk:
                if lk["person"] not in self.locked_person[key]:
                    self.locked_person[key].append(lk["person"])
                continue
            fills = int(lk.get("fills", 1))
            if fills < 1:
                raise ValueError("fills must be at least 1")
            self.locked_other[key].append({"label": lk["label"], "fills": fills})
            self.fixed_fill[key] += fills
        for key in set(self.locked_person) | set(self.locked_other):
            have = len(self.locked_person.get(key, [])) + self.fixed_fill.get(key, 0)
            self.slots[key] = max(self.slots.get(key, 0), have)

        # selection (pool): a manual restriction of who can be chosen for one task on one date
        self.pool: dict[tuple[str, str], set[str]] = {}
        for pl in data.get("pools", []):
            key = (pl["date"], pl["task"])
            self.pool[key] = set(pl.get("people", []))

        # combinations (symmetric)
        self.combo: dict[frozenset[str], str] = {}
        for c in data.get("combinations", []):
            if c["level"] not in COMBO_LEVELS:
                raise ValueError(f"unknown combination level: {c['level']}")
            self.combo[frozenset((c["a"], c["b"]))] = c["level"]

        hw = float(self.params["history_weight"])
        self.history_task: dict[str, dict[str, int]] = {}
        for p, per in data.get("history", {}).items():
            if not isinstance(per, dict):
                raise ValueError("history must be {person: {task: count}}")
            self.history_task[p] = {t: int(n * hw + 0.5) for t, n in per.items()}
        self.history_total = {p: sum(per.values()) for p, per in self.history_task.items()}
        self.previous = {
            (a["person"], a["task"], a["date"]) for a in data.get("previous_plan", []) if "person" in a
        }

    @staticmethod
    def _level(lvl: str) -> str:
        if lvl not in TASK_LEVELS:
            raise ValueError(f"unknown level: {lvl}")
        return lvl

    def date_level(self, p: str, t: str, d: str) -> str:
        cal = self.task_cal.get(p, {}).get(t)
        if cal is not None:
            return cal.get(d, "neutral")
        return self.person_dates.get(p, {}).get(d, "neutral")

    def combo_level(self, t1: str, t2: str) -> str:
        return self.combo.get(frozenset((t1, t2)), self.params["default_combination"])

    def reason(self, p: str, t: str, d: str) -> str | None:
        """Why p cannot take t on d, or None if it works."""
        if not self.person_proposable[p]:
            return "not_proposable"
        if t not in self.task_level.get(p, {}):
            return "not_in_task"
        if self.task_level[p][t] == "never":
            return "task_never"
        if self.date_level(p, t, d) == "never":
            return "event_never"
        pool = self.pool.get((d, t))
        if pool is not None and p not in pool:
            return "outside_selection"
        return None

def solve(data: dict[str, Any]) -> dict[str, Any]:
    pb = Problem(data)
    prm = pb.params
    m = cp_model.CpModel()

    # --- variabler ---------------------------------------------------------
    x: dict[tuple[str, str, str], cp_model.IntVar] = {}  # (person, task, date)
    for (d, t) in pb.slots:
        locked_here = set(pb.locked_person.get((d, t), ()))
        for p in pb.people:
            is_locked = p in locked_here
            if is_locked or pb.reason(p, t, d) is None:
                v = m.new_bool_var(f"x_{p}_{t}_{d}")
                x[p, t, d] = v
                if is_locked:
                    m.add(v == 1)  # manuelt valg vinner over filtrene

    unfilled: dict[tuple[str, str], cp_model.IntVar] = {}
    for (d, t), n in pb.slots.items():
        person_part = [x[p, t, d] for p in pb.people if (p, t, d) in x]
        u = m.new_int_var(0, n, f"u_{t}_{d}")
        unfilled[d, t] = u
        m.add(sum(person_part) + u + pb.fixed_fill.get((d, t), 0) == n)

    by_pd: dict[tuple[str, str], list[str]] = defaultdict(list)  # (p, d) -> [t]
    for (p, t, d) in x:
        by_pd[p, d].append(t)

    locked_pd: dict[tuple[str, str], set[str]] = defaultdict(set)
    for (d, t), ps in pb.locked_person.items():
        for p in ps:
            locked_pd[p, d].add(t)

    obj: list[tuple[int, cp_model.IntVar]] = []  # (koeffisient, variabel), maksimeres

    def pair_penalty(name: str, a: Any, b: Any, level: str, times: int = 1) -> None:
        """A hard ban or a soft penalty/bonus for a and b being true at the same time."""
        if level == "cannot":
            m.add(a + b <= 1)
            return
        both = m.new_bool_var(name)
        m.add(both <= a)
        m.add(both <= b)
        m.add(both >= a + b - 1)
        obj.append((SCALE * prm["w_combo"][level] * times, both))

    # --- combinations: the same person, the same day ----------------------------
    for (p, d), ts in by_pd.items():
        for t1, t2 in _pairs(sorted(ts), 2):
            if t1 in locked_pd[p, d] and t2 in locked_pd[p, d]:
                continue  # both locked by hand: not our choice
            pair_penalty(f"both_{p}_{t1}_{t2}_{d}", x[p, t1, d], x[p, t2, d], pb.combo_level(t1, t2))

    # --- preferences and stability -----------------------------------------
    for (p, t, d), v in x.items():
        if p in pb.locked_person.get((d, t), ()):
            continue
        w = prm["w_task"][pb.task_level[p][t]] + prm["w_date"][pb.date_level(p, t, d)]
        if (p, t, d) in pb.previous:
            w += prm["w_stability"]
        if w:
            obj.append((SCALE * w, v))
    # --- who takes part where -------------------------------------------------
    part: dict[tuple[str, str, str], list[cp_model.IntVar]] = defaultdict(list)
    for (p, t, d), v in x.items():
        part[p, t, d].append(v)

    # --- even distribution, in total --------------------------------------------
    # Each person's target is their share of all assignments (including history), calculated from how
    # many dates the person can attend. The target is rarely a whole number, so a band [lower, upper] of whole
    # numbers counts as «even enough»; only what is outside the band costs.
    dates_of: dict[str, set[str]] = defaultdict(set)
    for (p, _t, d) in part:
        dates_of[p].add(d)
    capacity = {p: len(ds) for p, ds in dates_of.items()}
    persons = [p for p in pb.people if capacity.get(p)]
    total_slots = sum(n - pb.fixed_fill.get(key, 0) for key, n in pb.slots.items())  # places filled by people
    hist = {p: pb.history_total.get(p, 0) for p in persons}
    A = sum(capacity[p] for p in persons)
    H = sum(hist.values())
    load_terms: dict[str, list[cp_model.IntVar]] = defaultdict(list)
    task_terms: dict[tuple[str, str], list[cp_model.IntVar]] = defaultdict(list)
    for (p, t, _d), vs in part.items():
        load_terms[p].extend(vs)
        task_terms[p, t].extend(vs)
    top = sum(len(vs) for vs in part.values()) + total_slots + sum(pb.history_total.values()) + 1

    def band(name: str, load: Any, target_num: int, target_den: int, w: float, worst: Any) -> None:
        """Penalty for being outside [floor, ceiling] of target_num/target_den."""
        lo_t = target_num // target_den
        hi_t = -(-target_num // target_den)
        under = m.new_int_var(0, top, f"under_{name}")
        over = m.new_int_var(0, top, f"over_{name}")
        m.add(under >= lo_t - load)
        m.add(over >= load - hi_t)
        obj.append((-round(SCALE * w), under))
        obj.append((-round(SCALE * w), over))
        if worst is not None:
            m.add(worst >= under)
            m.add(worst >= over)

    if len(persons) >= 2 and total_slots > 0:
        worst = m.new_int_var(0, top, "fair_worst")
        for p in persons:
            band(p, sum(load_terms[p]) + hist[p], (total_slots + H) * capacity[p], A, prm["w_fair"], worst)
        obj.append((-round(SCALE * prm["w_fair_range"]), worst))

    # --- even distribution per task ----------------------------------------
    task_info: dict[str, tuple[dict[str, int], int, int, dict[str, int]]] = {}
    for t in sorted(pb.planned):
        cand = sorted({p for (p, tt, _d) in part if tt == t})
        if len(cand) < 2:
            continue
        cap_t = {p: len({d for (q, tt, d) in part if q == p and tt == t}) for p in cand}
        A_t = sum(cap_t.values())
        T_t = sum(n - pb.fixed_fill.get((d, tt), 0) for (d, tt), n in pb.slots.items() if tt == t)
        if A_t == 0 or T_t == 0:
            continue
        h_t = {p: pb.history_task.get(p, {}).get(t, 0) for p in cand}
        H_t = sum(h_t.values())
        if prm["w_task_fair"] or prm["w_task_range"]:  # off by default: no variables to build
            worst_t = m.new_int_var(0, top, f"worst_{t}")
            for p in cand:
                band(f"{p}_{t}", sum(task_terms[p, t]) + h_t[p], (T_t + H_t) * cap_t[p], A_t, prm["w_task_fair"], worst_t)
            obj.append((-round(SCALE * prm["w_task_range"]), worst_t))
        task_info[t] = (cap_t, A_t, T_t, h_t)

    # --- empty place ---------------------------------------------------------
    for u in unfilled.values():
        obj.append((-SCALE * prm["w_unfilled"], u))

    m.maximize(sum(c * v for c, v in obj))

    solver = cp_model.CpSolver()
    solver.parameters.max_time_in_seconds = float(prm["time_limit"])
    solver.parameters.num_workers = int(prm["workers"])
    solver.parameters.random_seed = int(prm["seed"])
    for key, val in prm["solver"].items():
        setattr(solver.parameters, key, val)
    status = solver.solve(m)
    status_name = solver.status_name(status)

    result: dict[str, Any] = {
        "status": status_name,
        "wall_time": round(solver.wall_time, 3),
        "assignments": [],
        "manual": [],
        "unfilled": [],
        "concessions": [],
        "warnings": [],
        "load": {},
        "task_load": {},
    }
    if status not in (cp_model.OPTIMAL, cp_model.FEASIBLE):
        return result

    result["objective"] = solver.objective_value / SCALE
    result["bound"] = solver.best_objective_bound / SCALE

    assigned: dict[tuple[str, str], list[str]] = defaultdict(list)  # (p, d) -> [t]
    for (p, t, d), v in sorted(x.items(), key=lambda kv: (kv[0][2], kv[0][1], kv[0][0])):
        if solver.value(v):
            locked = p in pb.locked_person.get((d, t), ())
            tl = pb.task_level.get(p, {}).get(t, "neutral")
            dl = pb.date_level(p, t, d)
            result["assignments"].append(
                {"date": d, "task": t, "person": p, "locked": locked, "task_level": tl, "date_level": dl}
            )
            assigned[p, d].append(t)
            if locked and pb.reason(p, t, d) is not None:
                result["warnings"].append(
                    f"locked: {p} on {t} {d} breaks «{pb.reason(p, t, d)}» (manual choice kept)"
                )
            if "poor" in (tl, dl):
                result["concessions"].append(
                    {"date": d, "task": t, "person": p, "task_level": tl, "date_level": dl}
                )
    for (d, t), cells in sorted(pb.locked_other.items()):
        for cell in cells:
            result["manual"].append({"date": d, "task": t, **cell})

    for (d, t), u in sorted(unfilled.items()):
        missing = solver.value(u)
        if missing:
            result["unfilled"].append(
                {"date": d, "task": t, "missing": missing, "why": _explain(pb, assigned, d, t)}
            )

    def count_for(p: str, t: str | None) -> int:
        """Assignments for p (in task t, or all)."""
        return sum(1 for a in result["assignments"] if a["person"] == p and (t is None or a["task"] == t))

    for p in persons:
        target = (total_slots + H) * capacity[p] / A - hist[p] if A else 0.0
        result["load"][p] = {
            "count": count_for(p, None),
            "target": round(target, 2),
            "capacity": capacity[p],
            "history": hist[p],
        }
    for t, (cap_t, A_t, T_t, h_t) in sorted(task_info.items()):
        H_t = sum(h_t.values())
        result["task_load"][t] = {
            p: {
                "count": count_for(p, t),
                "target": round((T_t + H_t) * cap_t[p] / A_t - h_t[p], 2),
                "capacity": cap_t[p],
                "history": h_t[p],
            }
            for p in sorted(cap_t)
        }
    return result


def _explain(
    pb: Problem, assigned: dict[tuple[str, str], list[str]], d: str, t: str
) -> dict[str, Any]:
    """Why a place was left empty: who is out and for what reason."""
    why: dict[str, Any] = {"not_proposable": [], "not_in_task": 0, "task_never": [], "event_never": [], "busy": {}}
    outside_pool: list[str] = []
    candidates = 0
    for p in pb.people:
        r = pb.reason(p, t, d)
        if r == "not_proposable":
            why["not_proposable"].append(p)
        elif r == "not_in_task":
            why["not_in_task"] += 1
        elif r == "task_never":
            why["task_never"].append(p)
        elif r == "event_never":
            why["event_never"].append(p)
        elif r == "outside_selection":
            outside_pool.append(p)
        else:
            candidates += 1
            others = [o for o in assigned.get((p, d), []) if o != t]
            if others:
                why["busy"][p] = others
    if (d, t) in pb.pool:
        why["selection"] = sorted(pb.pool[d, t])
        why["outside_selection"] = len(outside_pool)
    why["candidates"] = candidates
    return {k: v for k, v in why.items() if v or k == "candidates"}
