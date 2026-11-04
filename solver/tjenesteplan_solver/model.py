"""CP-SAT-planlegger for Tjenesteplan.

Inndata og utdata er vanlige dict-er (JSON). Nivåer er ASCII i ledningsformatet:

  oppgave og dato:  ikke | darlig | noytral | godt      (ikke markert = noytral)
  kombinasjon:      kan_ikke | upraktisk | praktisk | bor

Inndata (alt unntatt tasks, dates, day_type_tasks og people er valgfritt):

  tasks:               [{"id", "planned": true}]
  dates:               [{"id": "2026-09-20", "type": "helg"}]
  day_type_tasks:      {"helg": {"vakt": 1, "kafe": 3}}
  people:              [{"id"}]
  person_tasks:        {person: {task: nivå}}          person er med på oppgaven hvis nøkkelen finnes
  person_dates:        {person: {dato: nivå}}          felles kalender for personens oppgaver
  person_task_calendars: {person: {task: {dato: nivå}}}  "egen kalender", erstatter den felles for oppgaven
  combinations:        [{"a", "b", "level"}]
  pools:               [{"date", "task", "people": [...]}]   «utvalg»: bare disse kan velges den dagen
  groups:              [{"id", "members": [person...]}]   navngitt sett personer; ingen egne regler
  locked:              manuelle ruter: {"date", "task", "person"} | {"date", "task", "label"}
                       | {"date", "task", "group"}; label og group kan ha "fills" (plasser, standard 1).
                       En gruppe i en rute gjør medlemmene opptatt med oppgaven: den felles
                       kombinasjonsinnstillingen avgjør hva de kan ha samme dag (ingen særregel).
  history:             {person: {task: antall oppdrag i tidligere semestre}}   teller halvt som standard
  previous_plan:       [{"date", "task", "person"}]
  params:              se DEFAULT_PARAMS

Utdata: se solve().
"""

from __future__ import annotations

from collections import defaultdict
from itertools import combinations as _pairs
from typing import Any

from ortools.sat.python import cp_model

TASK_LEVELS = ("ikke", "darlig", "noytral", "godt")
COMBO_LEVELS = ("kan_ikke", "upraktisk", "praktisk", "bor")
SCALE = 1000  # poeng -> heltall; straffer for fordeling deles på total kapasitet

DEFAULT_PARAMS: dict[str, Any] = {
    "time_limit": 20.0,
    "workers": 8,
    "seed": 1,
    "default_combination": "upraktisk",  # for oppgavepar som ikke er satt
    "w_task": {"darlig": -4, "noytral": 0, "godt": 2},
    "w_date": {"darlig": -6, "noytral": 0, "godt": 3},
    "w_combo": {"upraktisk": -5, "praktisk": 2, "bor": 6},
    "w_fair": 3,  # poeng per tildeling en person ligger utenfor sin samlede andel (vern mot overbelastning)
    "w_fair_range": 3,  # ... og for det verste avviket
    "w_task_fair": 10,  # som w_fair, men per oppgave: hovedleddet for «lik fordeling»
    "w_task_range": 10,  # det verste avviket per oppgave
    "history_weight": 0.5,  # hvor mye tidligere semestre teller («litt mindre til den som har gjort mest»)
    "w_stability": 2,  # behold forrige plans tildeling
    "w_unfilled": 1000,  # tom plass; skal komme etter alt annet
    "solver": {},  # rå CP-SAT-parametre, f.eks. {"linearization_level": 2}
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
    """Inndata slått opp og normalisert, delt mellom modellen og forklaringene."""

    def __init__(self, data: dict[str, Any]) -> None:
        self.params = _merge_params(data.get("params"))
        self.data = data
        self.people = [p["id"] for p in data["people"]]
        self.planned = {t["id"] for t in data["tasks"] if t.get("planned", True)}
        self.dates = sorted(d["id"] for d in data["dates"])
        self.date_type = {d["id"]: d["type"] for d in data["dates"]}
        if len(set(self.people)) != len(self.people):
            raise ValueError("people har doble id-er")

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

        # plasser per (dato, oppgave)
        self.slots: dict[tuple[str, str], int] = {}
        for d in self.dates:
            for t, n in data["day_type_tasks"].get(self.date_type[d], {}).items():
                if t in self.planned and n > 0:
                    self.slots[d, t] = n

        # låste ruter
        self.locked_person: dict[tuple[str, str], list[str]] = defaultdict(list)
        self.locked_other: dict[tuple[str, str], list[dict[str, Any]]] = defaultdict(list)  # tekst/gruppe
        self.fixed_fill: dict[tuple[str, str], int] = defaultdict(int)  # plasser fylt uten en person
        self.group_members: dict[str, list[str]] = {}
        for g in data.get("groups", []):
            unknown = [mem for mem in g["members"] if mem not in self.people]
            if unknown:
                raise ValueError(f"gruppe {g['id']} har ukjente medlemmer: {unknown}")
            self.group_members[g["id"]] = list(g["members"])
        # (person, dato) -> [(oppgave, gruppe)]: medlemmer som er opptatt via en gruppe i en manuell rute
        self.occupied: dict[tuple[str, str], list[tuple[str, str]]] = defaultdict(list)
        for lk in data.get("locked", []):
            key = (lk["date"], lk["task"])
            if "person" in lk:
                if lk["person"] not in self.locked_person[key]:
                    self.locked_person[key].append(lk["person"])
                continue
            fills = int(lk.get("fills", 1))
            if fills < 1:
                raise ValueError("fills må være minst 1")
            if "group" in lk:
                if lk["group"] not in self.group_members:
                    raise ValueError(f"ukjent gruppe: {lk['group']}")
                for mem in self.group_members[lk["group"]]:
                    self.occupied[mem, lk["date"]].append((lk["task"], lk["group"]))
            self.locked_other[key].append({**{k: lk[k] for k in ("label", "group") if k in lk}, "fills": fills})
            self.fixed_fill[key] += fills
        for key in set(self.locked_person) | set(self.locked_other):
            have = len(self.locked_person.get(key, [])) + self.fixed_fill.get(key, 0)
            self.slots[key] = max(self.slots.get(key, 0), have)

        # utvalg: manuell begrensning av hvem som kan velges for én oppgave på én dato
        self.pool: dict[tuple[str, str], set[str]] = {}
        for pl in data.get("pools", []):
            self.pool[pl["date"], pl["task"]] = set(pl["people"])

        # kombinasjoner (symmetriske)
        self.combo: dict[frozenset[str], str] = {}
        for c in data.get("combinations", []):
            if c["level"] not in COMBO_LEVELS:
                raise ValueError(f"ukjent kombinasjonsnivå: {c['level']}")
            self.combo[frozenset((c["a"], c["b"]))] = c["level"]

        hw = float(self.params["history_weight"])
        self.history_task: dict[str, dict[str, int]] = {}
        for p, per in data.get("history", {}).items():
            if not isinstance(per, dict):
                raise ValueError("history må være {person: {oppgave: antall}}")
            self.history_task[p] = {t: int(n * hw + 0.5) for t, n in per.items()}
        self.history_total = {p: sum(per.values()) for p, per in self.history_task.items()}
        self.previous = {
            (a["person"], a["task"], a["date"]) for a in data.get("previous_plan", [])
        }

    @staticmethod
    def _level(lvl: str) -> str:
        if lvl not in TASK_LEVELS:
            raise ValueError(f"ukjent nivå: {lvl}")
        return lvl

    def date_level(self, p: str, t: str, d: str) -> str:
        cal = self.task_cal.get(p, {}).get(t)
        if cal is not None:
            return cal.get(d, "noytral")
        return self.person_dates.get(p, {}).get(d, "noytral")

    def reason(self, p: str, t: str, d: str) -> str | None:
        """Hvorfor p ikke kan ta t på d, eller None hvis det går."""
        if t not in self.task_level.get(p, {}):
            return "ikke_med"
        if self.task_level[p][t] == "ikke":
            return "oppgave_ikke"
        if self.date_level(p, t, d) == "ikke":
            return "dato_ikke"
        pool = self.pool.get((d, t))
        if pool is not None and p not in pool:
            return "utenfor_utvalg"
        return None

    def combo_level(self, t1: str, t2: str) -> str:
        return self.combo.get(frozenset((t1, t2)), self.params["default_combination"])


def solve(data: dict[str, Any]) -> dict[str, Any]:
    pb = Problem(data)
    prm = pb.params
    m = cp_model.CpModel()

    # --- variabler ---------------------------------------------------------
    x: dict[tuple[str, str, str], cp_model.IntVar] = {}
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
        labels = pb.fixed_fill.get((d, t), 0)
        fills = [x[p, t, d] for p in pb.people if (p, t, d) in x]
        u = m.new_int_var(0, n, f"u_{t}_{d}")
        unfilled[d, t] = u
        m.add(sum(fills) + u + labels == n)

    by_pd: dict[tuple[str, str], list[str]] = defaultdict(list)  # (p, d) -> [t]
    for (p, t, d) in x:
        by_pd[p, d].append(t)

    locked_pd: dict[tuple[str, str], set[str]] = defaultdict(set)
    for (d, t), ps in pb.locked_person.items():
        for p in ps:
            locked_pd[p, d].add(t)

    obj: list[tuple[int, cp_model.IntVar]] = []  # (koeffisient, variabel), maksimeres

    # --- harde og myke krav per person og dag ------------------------------
    for (p, d), ts in by_pd.items():
        for t1, t2 in _pairs(sorted(ts), 2):
            if t1 in locked_pd[p, d] and t2 in locked_pd[p, d]:
                continue  # begge låst for hånd: ikke vårt valg
            lvl = pb.combo_level(t1, t2)
            a, b = x[p, t1, d], x[p, t2, d]
            if lvl == "kan_ikke":
                m.add(a + b <= 1)
            else:
                both = m.new_bool_var(f"both_{p}_{t1}_{t2}_{d}")
                m.add(both <= a)
                m.add(both <= b)
                m.add(both >= a + b - 1)
                obj.append((SCALE * prm["w_combo"][lvl], both))

    # --- gruppe i en manuell rute: medlemmene er opptatt med oppgaven -------------
    pre_warnings: list[str] = []
    for (mem, d), occ in pb.occupied.items():
        for t, g in occ:
            if pb.date_level(mem, t, d) == "ikke":
                pre_warnings.append(
                    f"gruppe {g} står på {t} {d}, men {mem} har «ikke» den dagen (manuelt valg beholdt)"
                )
            for t2 in by_pd.get((mem, d), []):
                v = x[mem, t2, d]
                lvl = "kan_ikke" if t2 == t else pb.combo_level(t, t2)  # samme oppgave to ganger gir ingen mening
                if mem in pb.locked_person.get((d, t2), ()):
                    pre_warnings.append(f"{mem} er låst på {t2} {d} samtidig som gruppe {g} står på {t}")
                elif lvl == "kan_ikke":
                    m.add(v == 0)
                else:
                    obj.append((SCALE * prm["w_combo"][lvl], v))

    # --- preferanser og stabilitet -----------------------------------------
    for (p, t, d), v in x.items():
        if p in pb.locked_person.get((d, t), ()):
            continue
        w = prm["w_task"][pb.task_level[p][t]] + prm["w_date"][pb.date_level(p, t, d)]
        if (p, t, d) in pb.previous:
            w += prm["w_stability"]
        if w:
            obj.append((SCALE * w, v))

    # --- jevn fordeling, samlet --------------------------------------------
    # Hver persons mål er sin andel av alle plasser (inkl. historikk), regnet etter hvor
    # mange datoer personen kan stille. Målet er sjelden et helt tall, så det er et bånd
    # [nedre, øvre] av hele tall som er «jevnt nok»; bare det utenfor båndet koster.
    dates_of: dict[str, set[str]] = defaultdict(set)
    for (p, t, d) in x:
        dates_of[p].add(d)
    capacity = {p: len(ds) for p, ds in dates_of.items()}
    persons = [p for p in pb.people if capacity.get(p)]
    total_slots = sum(
        n - pb.fixed_fill.get(key, 0) for key, n in pb.slots.items()
    )  # plasser som skal fylles av personer
    hist = {p: pb.history_total.get(p, 0) for p in persons}
    A = sum(capacity[p] for p in persons)
    H = sum(hist.values())
    load_terms: dict[str, list[cp_model.IntVar]] = defaultdict(list)
    task_terms: dict[tuple[str, str], list[cp_model.IntVar]] = defaultdict(list)
    for (p, t, _d), v in x.items():
        load_terms[p].append(v)
        task_terms[p, t].append(v)

    def band(name: str, load: Any, target_num: int, target_den: int, w: float, worst: Any) -> None:
        """Straff for å ligge utenfor [gulv, tak] av target_num/target_den."""
        lo_t = target_num // target_den
        hi_t = -(-target_num // target_den)
        top = total_slots + sum(pb.history_total.values()) + 1
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
        worst = m.new_int_var(0, total_slots + H + 1, "fair_worst")
        for p in persons:
            band(p, sum(load_terms[p]) + hist[p], (total_slots + H) * capacity[p], A, prm["w_fair"], worst)
        obj.append((-round(SCALE * prm["w_fair_range"]), worst))

    # --- jevn fordeling per oppgave ----------------------------------------
    task_info: dict[str, tuple[dict[str, int], int, int, dict[str, int]]] = {}
    for t in sorted(pb.planned):
        cand = sorted({p for (p, tt, _d) in x if tt == t})
        if len(cand) < 2:
            continue
        cap_t = {p: len({d for (q, tt, d) in x if q == p and tt == t}) for p in cand}
        A_t = sum(cap_t.values())
        T_t = sum(
            n - pb.fixed_fill.get((d, tt), 0)
            for (d, tt), n in pb.slots.items()
            if tt == t
        )
        if A_t == 0 or T_t == 0:
            continue
        h_t = {p: pb.history_task.get(p, {}).get(t, 0) for p in cand}
        H_t = sum(h_t.values())
        worst_t = m.new_int_var(0, T_t + H_t + 1, f"worst_{t}")
        for p in cand:
            band(f"{p}_{t}", sum(task_terms[p, t]) + h_t[p], (T_t + H_t) * cap_t[p], A_t, prm["w_task_fair"], worst_t)
        obj.append((-round(SCALE * prm["w_task_range"]), worst_t))
        task_info[t] = (cap_t, A_t, T_t, h_t)

    # --- tom plass ---------------------------------------------------------
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
        "warnings": list(pre_warnings),
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
            tl = pb.task_level.get(p, {}).get(t, "noytral")
            dl = pb.date_level(p, t, d)
            result["assignments"].append(
                {"date": d, "task": t, "person": p, "locked": locked, "task_level": tl, "date_level": dl}
            )
            assigned[p, d].append(t)
            if locked and pb.reason(p, t, d) is not None:
                result["warnings"].append(
                    f"låst: {p} på {t} {d} bryter «{pb.reason(p, t, d)}» (manuelt valg beholdt)"
                )
            if "darlig" in (tl, dl):
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

    for p in persons:
        count = sum(1 for a in result["assignments"] if a["person"] == p)
        target = (total_slots + H) * capacity[p] / A - hist[p] if A else 0.0
        result["load"][p] = {
            "count": count,
            "target": round(target, 2),
            "capacity": capacity[p],
            "history": hist[p],
        }
    for t, (cap_t, A_t, T_t, h_t) in sorted(task_info.items()):
        H_t = sum(h_t.values())
        result["task_load"][t] = {
            p: {
                "count": sum(1 for a in result["assignments"] if a["person"] == p and a["task"] == t),
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
    """Hvorfor en plass ble stående tom: hvem som er ute og av hvilken grunn."""
    why: dict[str, Any] = {"ikke_med": 0, "oppgave_ikke": [], "dato_ikke": [], "opptatt": {}}
    outside_pool: list[str] = []
    candidates = 0
    for p in pb.people:
        r = pb.reason(p, t, d)
        if r == "ikke_med":
            why["ikke_med"] += 1
        elif r == "oppgave_ikke":
            why["oppgave_ikke"].append(p)
        elif r == "dato_ikke":
            why["dato_ikke"].append(p)
        elif r == "utenfor_utvalg":
            outside_pool.append(p)
        elif blocked_by := [
            f"{gt} ({g})" for gt, g in pb.occupied.get((p, d), []) if gt == t or pb.combo_level(gt, t) == "kan_ikke"
        ]:
            why["opptatt"][p] = blocked_by
        else:
            candidates += 1
            others = [o for o in assigned.get((p, d), []) if o != t]
            if others:
                why["opptatt"][p] = others
    if (d, t) in pb.pool:
        why["utvalg"] = sorted(pb.pool[d, t])
        why["utenfor_utvalg"] = len(outside_pool)
    why["kandidater"] = candidates
    return {k: v for k, v in why.items() if v or k == "kandidater"}
