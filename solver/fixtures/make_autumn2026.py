"""Makes autumn2026.json: a realistic autumn plan (17 events, 7 tasks, 18 people) with invented names,
invented dates and event types, and random (but fixed) wishes.

The dates are the Sundays from 2 August, with the event types spread randomly (a seed of their own), and one event on a weekday.
Nothing here is taken from a real plan. Singing is not part of the spike (it is entered by hand).
    python fixtures/make_autumn2026.py > fixtures/autumn2026.json
"""

import datetime
import json
import random
import sys

rng = random.Random(2026)

PEOPLE = ["Alma", "Bjarne", "Cecilie", "Dagfinn", "Elin", "Frida", "Gro", "Henrik", "Ingrid",
          "Joakim", "Kjersti", "Lasse", "Mona", "Njål", "Oda", "Pål", "Randi", "Sindre"]
CHOIR = ["Alma", "Dagfinn", "Ingrid", "Kjersti", "Lasse", "Mona"]  # set by hand on Singing, one by one

# The events: 17 Sundays from 2 August, the types spread randomly. «staff evening» is on a weekday (two days later).
# A seed of its own, so that another spread does not move the wishes further down.
_schedule = random.Random(7)
_TYPES = ["service"] * 10 + ["course evening"] * 4 + ["gathering", "festival", "staff evening"]
_schedule.shuffle(_TYPES)
DATES = sorted(
    ((datetime.date(2026, 8, 2) + datetime.timedelta(weeks=i, days=2 if ty == "staff evening" else 0)).isoformat(), ty)
    for i, ty in enumerate(_TYPES)
)
DAY_TYPE_TASKS = {
    "service": {"sound": 1, "video": 1, "chair": 1, "speaker": 1, "host": 1, "cafe": 3, "prayer": 2},
    "course evening": {"sound": 1, "video": 1, "chair": 1},
    "gathering": {"sound": 1, "video": 1, "chair": 1, "speaker": 1},
    "festival": {"sound": 1, "video": 1, "chair": 1, "speaker": 1},
    "staff evening": {},
}
INVOLVED = {
    "sound": ["Alma", "Bjarne", "Dagfinn", "Elin", "Frida"],
    "video": ["Dagfinn", "Gro", "Henrik", "Joakim"],
    "chair": ["Ingrid", "Lasse", "Njål", "Oda", "Pål"],
    "speaker": ["Ingrid", "Lasse", "Mona", "Pål", "Randi", "Sindre"],
    "host": ["Bjarne", "Cecilie", "Elin", "Frida", "Gro", "Henrik", "Joakim", "Kjersti", "Randi"],
    "cafe": ["Cecilie", "Elin", "Kjersti", "Mona", "Oda", "Randi", "Sindre"],
    "prayer": ["Cecilie", "Dagfinn", "Gro", "Mona", "Njål", "Oda", "Pål", "Sindre"],
}
TASKS = list(INVOLVED)
COMBOS = [
    ("sound", "video", "impractical"), ("chair", "speaker", "cannot"), ("chair", "sound", "cannot"),
    ("chair", "video", "cannot"), ("speaker", "sound", "cannot"), ("speaker", "video", "cannot"),
    ("host", "cafe", "preferred"), ("cafe", "prayer", "cannot"), ("cafe", "sound", "cannot"),
    ("cafe", "video", "cannot"), ("host", "prayer", "practical"),
    # Those who sing cannot also have Sound or Video. An ordinary combination setting, no special rule.
    ("singing", "sound", "cannot"), ("singing", "video", "cannot"),
]


def pick(options, weights):
    return rng.choices(options, weights)[0]


person_tasks = {p: {} for p in PEOPLE}
for t, ps in INVOLVED.items():
    for p in ps:
        person_tasks[p][t] = pick(["poor", "neutral", "good"], [2, 5, 3])
for p in CHOIR:
    person_tasks[p]["singing"] = "neutral"  # they take part in Singing, which is set by hand

_services = [d for d, ty in DATES if ty == "service"]
choir_dates = [_services[2], _services[7]]  # the choir sings twice
guest_date = next(d for d, ty in DATES if ty == "gathering")  # a guest gives the talk
leader_date = next(d for d, ty in DATES if ty == "festival")  # a fixed chair, entered by hand
person_dates = {}
dates_with_tasks = [d for d, ty in DATES if DAY_TYPE_TASKS[ty]]
for p in PEOPLE:
    cal = {}
    for d in dates_with_tasks:
        lvl = pick([None, "never", "poor", "good"], [70, 8, 10, 12])
        if lvl and not (p in CHOIR and d in choir_dates):  # those who sing are there
            cal[d] = lvl
    person_dates[p] = cal

# A sound engineer who is at the services but would rather have another rhythm on Sound (a calendar of their own)
person_task_calendars = {"Bjarne": {"sound": {d: pick(["never", "poor", "good", "neutral"], [10, 10, 30, 50]) for d in dates_with_tasks}}}

sound_non_choir = [p for p in INVOLVED["sound"] if p not in CHOIR]

data = {
    "tasks": [{"id": t} for t in TASKS] + [{"id": "singing", "planned": False}],
    "dates": [{"id": d, "type": ty} for d, ty in DATES],
    "day_type_tasks": DAY_TYPE_TASKS,
    "people": [{"id": p} for p in PEOPLE],
    "person_tasks": person_tasks,
    "person_dates": person_dates,
    "person_task_calendars": person_task_calendars,
    "combinations": [{"a": a, "b": b, "level": lvl} for a, b, lvl in COMBOS],
    # The admin enters the singers by hand on Singing (which is not distributed) and sets a selection for Sound: «their own sound
    # engineer» among those who do not sing. The singers are kept away from Sound and Video by the combination setting above, not
    # by a special rule.
    "pools": [{"date": d, "task": "sound", "people": sound_non_choir[:2]} for d in choir_dates],
    "locked": [
        *[{"date": d, "task": "singing", "person": p} for d in choir_dates for p in CHOIR],
        {"date": guest_date, "task": "speaker", "label": "Outside guest"},
        {"date": leader_date, "task": "chair", "person": "Ingrid"},
    ],
    "history": {p: {t: rng.randint(0, 3) for t in TASKS if p in INVOLVED[t]} for p in PEOPLE},
    "params": {"time_limit": 30, "workers": 4},
}
json.dump(data, sys.stdout, ensure_ascii=False, indent=1)
sys.stdout.write("\n")
