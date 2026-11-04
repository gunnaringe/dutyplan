"""Lager host2026.json: formen på HØST 2026-fanen, med oppdiktede navn og tilfeldige (men faste) ønsker.

Datoer og dagtyper følger arket. Antakelser: Alpha-gudstjenestene trenger Lyd, Bilde og Møteleder
(arket har dem tomme), og Barnekirken, Lovsang og Nattverd er ikke med i spikeren.
    python fixtures/make_host2026.py > fixtures/host2026.json
"""

import json
import random
import sys

rng = random.Random(2026)

PEOPLE = ["Alma", "Bjarne", "Cecilie", "Dagfinn", "Elin", "Frida", "Gro", "Henrik", "Ingrid",
          "Joakim", "Kjersti", "Lasse", "Mona", "Njål", "Oda", "Pål", "Randi", "Sindre"]
CHOIR = ["Alma", "Dagfinn", "Ingrid", "Kjersti", "Lasse", "Mona"]  # en vanlig gruppe

DATES = [
    ("2026-08-09", "gudstjeneste"), ("2026-08-23", "gudstjeneste"), ("2026-08-25", "medarbeiderkveld"),
    ("2026-09-06", "gudstjeneste"), ("2026-09-13", "storsamling"), ("2026-09-20", "gudstjeneste"),
    ("2026-10-04", "gudstjeneste"), ("2026-10-11", "alpha"), ("2026-10-18", "gudstjeneste"),
    ("2026-10-25", "alpha"), ("2026-11-01", "gudstjeneste"), ("2026-11-08", "alpha"),
    ("2026-11-15", "gudstjeneste"), ("2026-11-22", "alpha"), ("2026-11-29", "gudstjeneste"),
    ("2026-12-13", "gudstjeneste"), ("2026-12-24", "juleaften"),
]
DAY_TYPE_TASKS = {
    "gudstjeneste": {"lyd": 1, "bilde": 1, "moteleder": 1, "taler": 1, "motevert": 1, "kafe": 3, "forbonn": 2},
    "alpha": {"lyd": 1, "bilde": 1, "moteleder": 1},
    "storsamling": {"lyd": 1, "bilde": 1, "moteleder": 1, "taler": 1},
    "juleaften": {"lyd": 1, "bilde": 1, "moteleder": 1, "taler": 1},
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
    ("lyd", "bilde", "upraktisk"), ("moteleder", "taler", "kan_ikke"), ("moteleder", "lyd", "kan_ikke"),
    ("moteleder", "bilde", "kan_ikke"), ("taler", "lyd", "kan_ikke"), ("taler", "bilde", "kan_ikke"),
    ("motevert", "kafe", "bor"), ("kafe", "forbonn", "kan_ikke"), ("kafe", "lyd", "kan_ikke"),
    ("kafe", "bilde", "kan_ikke"), ("motevert", "forbonn", "praktisk"),
    # De som synger kan ikke samtidig ha Lyd eller Bilde. Vanlig kombinasjonsinnstilling, ingen særregel.
    ("lovsang", "lyd", "kan_ikke"), ("lovsang", "bilde", "kan_ikke"),
]


def pick(options, weights):
    return rng.choices(options, weights)[0]


person_tasks = {p: {} for p in PEOPLE}
for t, ps in INVOLVED.items():
    for p in ps:
        person_tasks[p][t] = pick(["darlig", "noytral", "godt"], [2, 5, 3])

choir_dates = ["2026-09-20", "2026-11-15"]
person_dates = {}
dates_with_tasks = [d for d, ty in DATES if DAY_TYPE_TASKS[ty]]
for p in PEOPLE:
    cal = {}
    for d in dates_with_tasks:
        lvl = pick([None, "ikke", "darlig", "godt"], [70, 8, 10, 12])
        if lvl and not (p in CHOIR and d in choir_dates):  # de som synger, er der
            cal[d] = lvl
    person_dates[p] = cal

# En lydtekniker som er på gudstjenester men helst vil ha en annen rytme på Lyd (egen kalender)
person_task_calendars = {"Bjarne": {"lyd": {d: pick(["ikke", "darlig", "godt", "noytral"], [10, 10, 30, 50]) for d in dates_with_tasks}}}

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
    "groups": [{"id": "kirkekoret", "members": CHOIR}],
    "pools": [{"date": d, "task": "lyd", "people": lyd_non_choir[:2]} for d in choir_dates],
    "locked": [
        *[{"date": d, "task": "lovsang", "group": "kirkekoret"} for d in choir_dates],
        {"date": "2026-09-13", "task": "taler", "label": "Gjest utenfra"},
        {"date": "2026-12-24", "task": "moteleder", "person": "Ingrid"},
    ],
    "history": {p: {t: rng.randint(0, 3) for t in TASKS if p in INVOLVED[t]} for p in PEOPLE},
    "params": {"time_limit": 30, "workers": 4},
}
json.dump(data, sys.stdout, ensure_ascii=False, indent=1)
sys.stdout.write("\n")
