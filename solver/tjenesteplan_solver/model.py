"""CP-SAT-planlegger for Tjenesteplan.

Inndata og utdata er vanlige dict-er (JSON). Nivåer er ASCII i ledningsformatet:

  oppgave og dato:  ikke | darlig | noytral | godt      (ikke markert = noytral)
  kombinasjon:      kan_ikke | upraktisk | praktisk | bor

Inndata (alt unntatt tasks, dates, day_type_tasks og people er valgfritt):

  tasks:               [{"id", "planned": true}]
  dates:               [{"id": "2026-09-20", "type": "helg"}]
  day_type_tasks:      {"helg": {"vakt": 1, "kafe": 3}}
  people:              [{"id", "proposable": true}]   proposable=false: planleggeren velger aldri personen (manuelle ruter går an)
  person_tasks:        {person: {task: nivå}}          person er med på oppgaven hvis nøkkelen finnes
  person_dates:        {person: {dato: nivå}}          felles kalender for personens oppgaver
  person_task_calendars: {person: {task: {dato: nivå}}}  "egen kalender", erstatter den felles for oppgaven
  combinations:        [{"a", "b", "level"}]
  groups:              [{"id", "members": [person...], "proposable": true}]   navngitt sett personer; ingen egne regler.
                       proposable=false: planleggeren foreslår aldri gruppen. Den foreslås heller ikke hvis et medlem
                       har proposable=false. Manuelle ruter går an.
  group_tasks:         [{"group", "task", "fills": 1, "level": "noytral"}]
                       oppgaver planleggeren kan foreslå gruppen til; gruppen fyller "fills" plasser.
                       Dato-ønsker kommer fra medlemmene (snitt); "ikke" hos ett medlem utelukker gruppen.
  pools:               [{"date", "task", "people": [...], "groups": [...]}]
                       «utvalg»: bare disse kan velges den dagen (personer og/eller grupper)
  locked:              manuelle ruter: {"date", "task", "person"} | {"date", "task", "label"}
                       | {"date", "task", "group"}; label og group kan ha "fills" (plasser, standard 1).
  history:             {person: {task: antall oppdrag i tidligere semestre}}   teller halvt som standard
  previous_plan:       [{"date", "task", "person"} | {"date", "task", "group"}]
  params:              se DEFAULT_PARAMS

En gruppe i en rute (manuell eller foreslått) gjør medlemmene opptatt med oppgaven den dagen. Den felles
kombinasjonsinnstillingen avgjør hva de kan ha i tillegg (ingen særregel). Gruppearbeid teller som ett oppdrag
per medlem i fordelingen.

Utdata: se solve().
"""

from __future__ import annotations

from collections import defaultdict
from itertools import combinations as _pairs
from typing import Any

from ortools.sat.python import cp_model

TASK_LEVELS = ("ikke", "darlig", "noytral", "godt")
COMBO_LEVELS = ("kan_ikke", "upraktisk", "praktisk", "bor")
SCALE = 1000  # poeng -> heltall

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
        self.person_proposable = {p["id"]: bool(p.get("proposable", True)) for p in data["people"]}
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

        # grupper
        self.group_members: dict[str, list[str]] = {}
        for g in data.get("groups", []):
            unknown = [mem for mem in g["members"] if mem not in self.people]
            if unknown:
                raise ValueError(f"gruppe {g['id']} har ukjente medlemmer: {unknown}")
            self.group_members[g["id"]] = list(g["members"])
        self.group_proposable = {g["id"]: bool(g.get("proposable", True)) for g in data.get("groups", [])}
        self.group_tasks: dict[tuple[str, str], dict[str, Any]] = {}
        for gt in data.get("group_tasks", []):
            if gt["group"] not in self.group_members:
                raise ValueError(f"ukjent gruppe i group_tasks: {gt['group']}")
            if gt["task"] not in self.planned:
                raise ValueError(f"group_tasks gjelder en oppgave som ikke fordeles: {gt['task']}")
            fills = int(gt.get("fills", 1))
            if fills < 1:
                raise ValueError("fills må være minst 1")
            self.group_tasks[gt["group"], gt["task"]] = {
                "fills": fills,
                "level": self._level(gt.get("level", "noytral")),
            }

        # manuelle ruter
        self.locked_person: dict[tuple[str, str], list[str]] = defaultdict(list)
        self.locked_other: dict[tuple[str, str], list[dict[str, Any]]] = defaultdict(list)  # tekst/gruppe
        self.fixed_fill: dict[tuple[str, str], int] = defaultdict(int)  # plasser fylt uten en person
        # (person, dato) -> [(oppgave, gruppe)]: medlemmer som er opptatt via en gruppe i en manuell rute
        self.occupied: dict[tuple[str, str], list[tuple[str, str]]] = defaultdict(list)
        # gruppearbeid teller som ett oppdrag per medlem og rute
        self.gcount_person: dict[str, int] = defaultdict(int)
        self.gcount_task: dict[tuple[str, str], int] = defaultdict(int)
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
                    self.gcount_person[mem] += 1
                    self.gcount_task[mem, lk["task"]] += 1
            self.locked_other[key].append({**{k: lk[k] for k in ("label", "group") if k in lk}, "fills": fills})
            self.fixed_fill[key] += fills
        for key in set(self.locked_person) | set(self.locked_other):
            have = len(self.locked_person.get(key, [])) + self.fixed_fill.get(key, 0)
            self.slots[key] = max(self.slots.get(key, 0), have)

        # utvalg: manuell begrensning av hvem som kan velges for én oppgave på én dato
        self.pool: dict[tuple[str, str], set[str]] = {}
        self.pool_groups: dict[tuple[str, str], set[str]] = {}
        for pl in data.get("pools", []):
            key = (pl["date"], pl["task"])
            self.pool[key] = set(pl.get("people", []))
            self.pool_groups[key] = set(pl.get("groups", []))
            for g in self.pool_groups[key]:
                if g not in self.group_members:
                    raise ValueError(f"ukjent gruppe i utvalg: {g}")

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
            (a["person"], a["task"], a["date"]) for a in data.get("previous_plan", []) if "person" in a
        }
        self.previous_groups = {
            (a["group"], a["task"], a["date"]) for a in data.get("previous_plan", []) if "group" in a
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

    def combo_level(self, t1: str, t2: str) -> str:
        return self.combo.get(frozenset((t1, t2)), self.params["default_combination"])

    def occupied_block(self, p: str, t: str, d: str) -> tuple[str, str] | None:
        """(oppgave, gruppe) hvis en gruppe i en manuell rute forbyr p å ta t på d (samme oppgave eller «kan ikke»)."""
        for gt, g in self.occupied.get((p, d), ()):
            if gt == t or self.combo_level(gt, t) == "kan_ikke":
                return gt, g
        return None

    def reason(self, p: str, t: str, d: str) -> str | None:
        """Hvorfor p ikke kan ta t på d, eller None hvis det går."""
        if not self.person_proposable[p]:
            return "ikke_foreslaas"
        if t not in self.task_level.get(p, {}):
            return "ikke_med"
        if self.task_level[p][t] == "ikke":
            return "oppgave_ikke"
        if self.date_level(p, t, d) == "ikke":
            return "dato_ikke"
        pool = self.pool.get((d, t))
        if pool is not None and p not in pool:
            return "utenfor_utvalg"
        occ = self.occupied_block(p, t, d)
        if occ is not None:
            return f"opptatt:{occ[0]} ({occ[1]})"
        return None

    def group_reason(self, g: str, t: str, d: str) -> str | None:
        """Hvorfor gruppen g ikke kan foreslås for t på d, eller None. Medlemmenes egne «ikke» utelukker gruppen."""
        cfg = self.group_tasks[g, t]
        if not self.group_proposable[g]:
            return "ikke_foreslaas"
        free = self.slots.get((d, t), 0) - self.fixed_fill.get((d, t), 0) - len(self.locked_person.get((d, t), ()))
        if cfg["fills"] > free:
            return "for_mange_plasser"
        if (d, t) in self.pool and g not in self.pool_groups[d, t]:
            return "utenfor_utvalg"
        for mem in self.group_members[g]:
            if not self.person_proposable[mem]:
                return f"ikke_foreslaas:{mem}"
            if self.task_level.get(mem, {}).get(t) == "ikke":
                return f"oppgave_ikke:{mem}"
            if self.date_level(mem, t, d) == "ikke":
                return f"dato_ikke:{mem}"
            if self.occupied_block(mem, t, d) is not None:
                return f"opptatt:{mem}"
        return None


def solve(data: dict[str, Any]) -> dict[str, Any]:
    pb = Problem(data)
    prm = pb.params
    m = cp_model.CpModel()

    # --- variabler ---------------------------------------------------------
    x: dict[tuple[str, str, str], cp_model.IntVar] = {}  # (person, oppgave, dato)
    for (d, t) in pb.slots:
        locked_here = set(pb.locked_person.get((d, t), ()))
        for p in pb.people:
            is_locked = p in locked_here
            if is_locked or pb.reason(p, t, d) is None:
                v = m.new_bool_var(f"x_{p}_{t}_{d}")
                x[p, t, d] = v
                if is_locked:
                    m.add(v == 1)  # manuelt valg vinner over filtrene

    y: dict[tuple[str, str, str], cp_model.IntVar] = {}  # (gruppe, oppgave, dato): planleggeren foreslår gruppen
    for (g, t), cfg in pb.group_tasks.items():
        if cfg["level"] == "ikke":
            continue
        for d in pb.dates:
            if (d, t) in pb.slots and pb.group_reason(g, t, d) is None:
                y[g, t, d] = m.new_bool_var(f"y_{g}_{t}_{d}")

    unfilled: dict[tuple[str, str], cp_model.IntVar] = {}
    for (d, t), n in pb.slots.items():
        person_part = [x[p, t, d] for p in pb.people if (p, t, d) in x]
        group_part = [pb.group_tasks[g, t]["fills"] * v for (g, tt, dd), v in y.items() if tt == t and dd == d]
        u = m.new_int_var(0, n, f"u_{t}_{d}")
        unfilled[d, t] = u
        m.add(sum(person_part) + sum(group_part) + u + pb.fixed_fill.get((d, t), 0) == n)

    by_pd: dict[tuple[str, str], list[str]] = defaultdict(list)  # (p, d) -> [t]
    for (p, t, d) in x:
        by_pd[p, d].append(t)

    locked_pd: dict[tuple[str, str], set[str]] = defaultdict(set)
    for (d, t), ps in pb.locked_person.items():
        for p in ps:
            locked_pd[p, d].add(t)

    obj: list[tuple[int, cp_model.IntVar]] = []  # (koeffisient, variabel), maksimeres

    def pair_penalty(name: str, a: Any, b: Any, level: str, times: int = 1) -> None:
        """Hardt forbud eller myk straff/bonus for at a og b er sanne samtidig."""
        if level == "kan_ikke":
            m.add(a + b <= 1)
            return
        both = m.new_bool_var(name)
        m.add(both <= a)
        m.add(both <= b)
        m.add(both >= a + b - 1)
        obj.append((SCALE * prm["w_combo"][level] * times, both))

    # --- kombinasjoner: samme person, samme dag ----------------------------
    for (p, d), ts in by_pd.items():
        for t1, t2 in _pairs(sorted(ts), 2):
            if t1 in locked_pd[p, d] and t2 in locked_pd[p, d]:
                continue  # begge låst for hånd: ikke vårt valg
            pair_penalty(f"both_{p}_{t1}_{t2}_{d}", x[p, t1, d], x[p, t2, d], pb.combo_level(t1, t2))

    # --- foreslått gruppe: medlemmene er opptatt med gruppens oppgave ----------
    for (g, t, d), v in y.items():
        for mem in pb.group_members[g]:
            for t2 in by_pd.get((mem, d), []):
                lvl = "kan_ikke" if t2 == t else pb.combo_level(t, t2)  # samme oppgave to ganger gir ingen mening
                pair_penalty(f"gp_{g}_{t}_{mem}_{t2}_{d}", v, x[mem, t2, d], lvl)
    y_by_date: dict[str, list[tuple[str, str]]] = defaultdict(list)
    for (g, t, d) in y:
        y_by_date[d].append((g, t))
    for d, cells in y_by_date.items():
        for (g1, t1), (g2, t2) in _pairs(sorted(cells), 2):
            shared = len(set(pb.group_members[g1]) & set(pb.group_members[g2]))
            if shared:
                lvl = "kan_ikke" if t1 == t2 else pb.combo_level(t1, t2)
                pair_penalty(f"gg_{g1}_{t1}_{g2}_{t2}_{d}", y[g1, t1, d], y[g2, t2, d], lvl, times=shared)

    # --- gruppe i en manuell rute: medlemmene er opptatt med oppgaven -------------
    # Forbudte kombinasjoner finnes ikke som variabler (se Problem.reason). Her er det de myke.
    pre_warnings: list[str] = []
    for (mem, d), occ in pb.occupied.items():
        for t, g in occ:
            if pb.date_level(mem, t, d) == "ikke":
                pre_warnings.append(
                    f"gruppe {g} står på {t} {d}, men {mem} har «ikke» den dagen (manuelt valg beholdt)"
                )
            for t2 in by_pd.get((mem, d), []):
                lvl = "kan_ikke" if t2 == t else pb.combo_level(t, t2)
                if mem in pb.locked_person.get((d, t2), ()):
                    if lvl == "kan_ikke":
                        pre_warnings.append(f"{mem} er låst på {t2} {d} samtidig som gruppe {g} står på {t}")
                elif lvl != "kan_ikke":
                    obj.append((SCALE * prm["w_combo"][lvl], x[mem, t2, d]))

    # --- preferanser og stabilitet -----------------------------------------
    for (p, t, d), v in x.items():
        if p in pb.locked_person.get((d, t), ()):
            continue
        w = prm["w_task"][pb.task_level[p][t]] + prm["w_date"][pb.date_level(p, t, d)]
        if (p, t, d) in pb.previous:
            w += prm["w_stability"]
        if w:
            obj.append((SCALE * w, v))
    for (g, t, d), v in y.items():
        mems = pb.group_members[g]
        mean_date = sum(prm["w_date"][pb.date_level(mem, t, d)] for mem in mems) / len(mems)
        w = prm["w_task"][pb.group_tasks[g, t]["level"]] + mean_date
        if (g, t, d) in pb.previous_groups:
            w += prm["w_stability"]
        obj.append((round(SCALE * w), v))

    # --- hvem som deltar hvor: personlig eller via en foreslått gruppe -------------
    part: dict[tuple[str, str, str], list[cp_model.IntVar]] = defaultdict(list)
    for (p, t, d), v in x.items():
        part[p, t, d].append(v)
    for (g, t, d), v in y.items():
        for mem in pb.group_members[g]:
            part[mem, t, d].append(v)  # ett oppdrag per medlem

    # --- jevn fordeling, samlet --------------------------------------------
    # Hver persons mål er sin andel av alle oppdrag (inkl. historikk og gruppearbeid), regnet etter hvor
    # mange datoer personen kan stille. Målet er sjelden et helt tall, så det er et bånd
    # [nedre, øvre] av hele tall som er «jevnt nok»; bare det utenfor båndet koster.
    dates_of: dict[str, set[str]] = defaultdict(set)
    for (p, _t, d) in part:
        dates_of[p].add(d)
    capacity = {p: len(ds) for p, ds in dates_of.items()}
    persons = [p for p in pb.people if capacity.get(p)]
    total_slots = sum(n - pb.fixed_fill.get(key, 0) for key, n in pb.slots.items())  # plasser som fylles av personer/grupper
    hist = {p: pb.history_total.get(p, 0) for p in persons}
    gcount = {p: pb.gcount_person.get(p, 0) for p in persons}
    A = sum(capacity[p] for p in persons)
    H = sum(hist.values())
    G = sum(gcount.values())
    load_terms: dict[str, list[cp_model.IntVar]] = defaultdict(list)
    task_terms: dict[tuple[str, str], list[cp_model.IntVar]] = defaultdict(list)
    for (p, t, _d), vs in part.items():
        load_terms[p].extend(vs)
        task_terms[p, t].extend(vs)
    top = sum(len(vs) for vs in part.values()) + total_slots + sum(pb.history_total.values()) + sum(pb.gcount_person.values()) + 1

    def band(name: str, load: Any, target_num: int, target_den: int, w: float, worst: Any) -> None:
        """Straff for å ligge utenfor [gulv, tak] av target_num/target_den."""
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
            band(p, sum(load_terms[p]) + hist[p] + gcount[p], (total_slots + G + H) * capacity[p], A, prm["w_fair"], worst)
        obj.append((-round(SCALE * prm["w_fair_range"]), worst))

    # --- jevn fordeling per oppgave ----------------------------------------
    task_info: dict[str, tuple[dict[str, int], int, int, dict[str, int], dict[str, int]]] = {}
    for t in sorted(pb.planned):
        cand = sorted({p for (p, tt, _d) in part if tt == t})
        if len(cand) < 2:
            continue
        cap_t = {p: len({d for (q, tt, d) in part if q == p and tt == t}) for p in cand}
        A_t = sum(cap_t.values())
        T_t = sum(n - pb.fixed_fill.get((d, tt), 0) for (d, tt), n in pb.slots.items() if tt == t)
        g_t = {p: pb.gcount_task.get((p, t), 0) for p in cand}
        D_t = T_t + sum(g_t.values())
        if A_t == 0 or D_t == 0:
            continue
        h_t = {p: pb.history_task.get(p, {}).get(t, 0) for p in cand}
        H_t = sum(h_t.values())
        worst_t = m.new_int_var(0, top, f"worst_{t}")
        for p in cand:
            band(f"{p}_{t}", sum(task_terms[p, t]) + h_t[p] + g_t[p], (D_t + H_t) * cap_t[p], A_t, prm["w_task_fair"], worst_t)
        obj.append((-round(SCALE * prm["w_task_range"]), worst_t))
        task_info[t] = (cap_t, A_t, D_t, h_t, g_t)

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
        "group_assignments": [],
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

    assigned: dict[tuple[str, str], list[str]] = defaultdict(list)  # (p, d) -> [t], også via grupper
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
    for (g, t, d), v in sorted(y.items(), key=lambda kv: (kv[0][2], kv[0][1], kv[0][0])):
        if solver.value(v):
            gl = pb.group_tasks[g, t]["level"]
            result["group_assignments"].append(
                {"date": d, "task": t, "group": g, "members": list(pb.group_members[g]),
                 "fills": pb.group_tasks[g, t]["fills"], "locked": False}
            )
            for mem in pb.group_members[g]:
                assigned[mem, d].append(t)
                dl = pb.date_level(mem, t, d)
                if "darlig" in (gl, dl):
                    result["concessions"].append(
                        {"date": d, "task": t, "person": mem, "group": g, "task_level": gl, "date_level": dl}
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
        """Oppdrag for p (i oppgave t, eller alle): personlige, via foreslåtte grupper og via manuelle grupper."""
        own = sum(1 for a in result["assignments"] if a["person"] == p and (t is None or a["task"] == t))
        via = sum(
            1 for a in result["group_assignments"]
            if p in a["members"] and (t is None or a["task"] == t)
        )
        manual = pb.gcount_person.get(p, 0) if t is None else pb.gcount_task.get((p, t), 0)
        return own + via + manual

    for p in persons:
        target = (total_slots + G + H) * capacity[p] / A - hist[p] if A else 0.0
        result["load"][p] = {
            "count": count_for(p, None),
            "target": round(target, 2),
            "capacity": capacity[p],
            "history": hist[p],
        }
    for t, (cap_t, A_t, D_t, h_t, _g_t) in sorted(task_info.items()):
        H_t = sum(h_t.values())
        result["task_load"][t] = {
            p: {
                "count": count_for(p, t),
                "target": round((D_t + H_t) * cap_t[p] / A_t - h_t[p], 2),
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
    why: dict[str, Any] = {"ikke_foreslaas": [], "ikke_med": 0, "oppgave_ikke": [], "dato_ikke": [], "opptatt": {}}
    outside_pool: list[str] = []
    candidates = 0
    for p in pb.people:
        r = pb.reason(p, t, d)
        if r == "ikke_foreslaas":
            why["ikke_foreslaas"].append(p)
        elif r == "ikke_med":
            why["ikke_med"] += 1
        elif r == "oppgave_ikke":
            why["oppgave_ikke"].append(p)
        elif r == "dato_ikke":
            why["dato_ikke"].append(p)
        elif r == "utenfor_utvalg":
            outside_pool.append(p)
        elif r is not None and r.startswith("opptatt:"):
            why["opptatt"][p] = [r.split(":", 1)[1]]
        else:
            candidates += 1
            others = [o for o in assigned.get((p, d), []) if o != t]
            if others:
                why["opptatt"][p] = others
    if (d, t) in pb.pool:
        why["utvalg"] = sorted(pb.pool[d, t])
        if pb.pool_groups[d, t]:
            why["utvalg_grupper"] = sorted(pb.pool_groups[d, t])
        why["utenfor_utvalg"] = len(outside_pool)
    groups = {g: (pb.group_reason(g, t, d) or "kandidat") for (g, tt) in pb.group_tasks if tt == t}
    if groups:
        why["grupper"] = groups
    why["kandidater"] = candidates
    return {k: v for k, v in why.items() if v or k == "kandidater"}
