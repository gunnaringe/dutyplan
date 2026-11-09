"""Lager host2026.json: en realistisk høstplan (17 hendelser, 7 oppgaver, 18 personer, 4 team og par) med oppdiktede navn,
oppdiktede datoer og hendelsestyper, og tilfeldige (men faste) ønsker.

Datoene er søndagene fra 2. august, med hendelsestypene fordelt tilfeldig (eget frø), og én hendelse på en ukedag.
Ingenting her er hentet fra en ekte plan. Lovsang er ikke med i spikeren (den settes for hånd).
    python fixtures/make_host2026.py > fixtures/host2026.json
"""

import datetime
import json
import random
import sys

rng = random.Random(2026)

PEOPLE = ["Alma", "Bjarne", "Cecilie", "Dagfinn", "Elin", "Frida", "Gro", "Henrik", "Ingrid",
          "Joakim", "Kjersti", "Lasse", "Mona", "Njål", "Oda", "Pål", "Randi", "Sindre"]
CHOIR = ["Alma", "Dagfinn", "Ingrid", "Kjersti", "Lasse", "Mona"]  # en vanlig gruppe

# Hendelsene: 17 søndager fra 2. august, typene fordelt tilfeldig. «medarbeiderkveld» er en ukedag (to dager senere).
# Eget frø, så en annen fordeling ikke flytter ønskene lenger ned.
_schedule = random.Random(7)
_TYPES = ["gudstjeneste"] * 10 + ["kurskveld"] * 4 + ["fellessamling", "høytid", "medarbeiderkveld"]
_schedule.shuffle(_TYPES)
DATES = sorted(
    ((datetime.date(2026, 8, 2) + datetime.timedelta(weeks=i, days=2 if ty == "medarbeiderkveld" else 0)).isoformat(), ty)
    for i, ty in enumerate(_TYPES)
)
DAY_TYPE_TASKS = {
    "gudstjeneste": {"lyd": 1, "bilde": 1, "moteleder": 1, "taler": 1, "motevert": 1, "kafe": 3, "forbonn": 2},
    "kurskveld": {"lyd": 1, "bilde": 1, "moteleder": 1},
    "fellessamling": {"lyd": 1, "bilde": 1, "moteleder": 1, "taler": 1},
    "høytid": {"lyd": 1, "bilde": 1, "moteleder": 1, "taler": 1},
    "medarbeiderkveld": {},
}
INVOLVED = {
    "lyd": ["Alma", "Bjarne", "Dagfinn", "Elin", "Frida"],
    "bilde": ["Dagfinn", "Gro", "Henrik", "Joakim"],
    "moteleder": ["Ingrid", "Lasse", "Njål", "Oda", "Pål"],
    "taler": ["Ingrid", "Lasse", "Mona", "Pål", "Randi", "Sindre"],
    "motevert": ["Bjarne", "Cecilie", "Elin", "Frida", "Gro", "Henrik", "Joakim", "Kjersti", "Randi"],
    "kafe": ["Cecilie", "Elin", "Kjersti", "Mona", "Oda", "Randi", "Sindre"],
    "forbonn": ["Cecilie", "Dagfinn", "Gro", "Mona", "Njål", "Oda", "Pål", "Sindre"],
}
TASKS = list(INVOLVED)
COMBOS = [
    ("lyd", "bilde", "impractical"), ("moteleder", "taler", "cannot"), ("moteleder", "lyd", "cannot"),
    ("moteleder", "bilde", "cannot"), ("taler", "lyd", "cannot"), ("taler", "bilde", "cannot"),
    ("motevert", "kafe", "preferred"), ("kafe", "forbonn", "cannot"), ("kafe", "lyd", "cannot"),
    ("kafe", "bilde", "cannot"), ("motevert", "forbonn", "practical"),
    # De som synger kan ikke samtidig ha Lyd eller Bilde. Vanlig kombinasjonsinnstilling, ingen særregel.
    ("lovsang", "lyd", "cannot"), ("lovsang", "bilde", "cannot"),
]


def pick(options, weights):
    return rng.choices(options, weights)[0]


person_tasks = {p: {} for p in PEOPLE}
for t, ps in INVOLVED.items():
    for p in ps:
        person_tasks[p][t] = pick(["poor", "neutral", "good"], [2, 5, 3])

_worship = [d for d, ty in DATES if ty == "gudstjeneste"]
choir_dates = [_worship[2], _worship[7]]  # koret synger to ganger
guest_date = next(d for d, ty in DATES if ty == "fellessamling")  # en gjest holder talen
leader_date = next(d for d, ty in DATES if ty == "høytid")  # en fast møteleder for hånd
person_dates = {}
dates_with_tasks = [d for d, ty in DATES if DAY_TYPE_TASKS[ty]]
for p in PEOPLE:
    cal = {}
    for d in dates_with_tasks:
        lvl = pick([None, "never", "poor", "good"], [70, 8, 10, 12])
        if lvl and not (p in CHOIR and d in choir_dates):  # de som synger, er der
            cal[d] = lvl
    person_dates[p] = cal

# En lydtekniker som er på gudstjenester men helst vil ha en annen rytme på Lyd (egen kalender)
person_task_calendars = {"Bjarne": {"lyd": {d: pick(["never", "poor", "good", "neutral"], [10, 10, 30, 50]) for d in dates_with_tasks}}}

# Faste team og par som planleggeren kan foreslå som en enhet: (medlemmer, oppgave, plasser)
TEAMS = {
    "kafeteam_a": (["Cecilie", "Elin", "Kjersti"], "kafe", 3),
    "kafeteam_b": (["Mona", "Oda", "Randi"], "kafe", 3),
    "forbonnspar_1": (["Dagfinn", "Gro"], "forbonn", 2),
    "forbonnspar_2": (["Njål", "Pål"], "forbonn", 2),
}

lyd_non_choir = [p for p in INVOLVED["lyd"] if p not in CHOIR]

data = {
    "tasks": [{"id": t} for t in TASKS] + [{"id": "lovsang", "planned": False}],
    "dates": [{"id": d, "type": ty} for d, ty in DATES],
    "day_type_tasks": DAY_TYPE_TASKS,
    "people": [{"id": p} for p in PEOPLE],
    "person_tasks": person_tasks,
    "person_dates": person_dates,
    "person_task_calendars": person_task_calendars,
    "combinations": [{"a": a, "b": b, "level": lvl} for a, b, lvl in COMBOS],
    # Kirkekoret er en vanlig gruppe. Admin legger den inn for hånd på Lovsang (som ikke fordeles) og setter et
    # utvalg for Lyd: «egen lydtekniker» blant dem som ikke synger. Medlemmene holdes unna Lyd og Bilde av
    # kombinasjonsinnstillingen over, ikke av en særregel.
    "groups": [{"id": "kirkekoret", "members": CHOIR}]
    + [{"id": g, "members": mems} for g, (mems, _t, _n) in TEAMS.items()],
    "group_tasks": [
        {"group": g, "task": t, "fills": n, "level": pick(["neutral", "good"], [2, 1])}
        for g, (_mems, t, n) in TEAMS.items()
    ],
    "pools": [{"date": d, "task": "lyd", "people": lyd_non_choir[:2]} for d in choir_dates],
    "locked": [
        *[{"date": d, "task": "lovsang", "group": "kirkekoret"} for d in choir_dates],
        {"date": guest_date, "task": "taler", "label": "Gjest utenfra"},
        {"date": leader_date, "task": "moteleder", "person": "Ingrid"},
    ],
    "history": {p: {t: rng.randint(0, 3) for t in TASKS if p in INVOLVED[t]} for p in PEOPLE},
    "params": {"time_limit": 30, "workers": 4},
}
json.dump(data, sys.stdout, ensure_ascii=False, indent=1)
sys.stdout.write("\n")
