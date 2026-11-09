"""Random small instances: the plan must never break a hard requirement, whatever is entered."""

import random

import pytest
from helpers import dates, run

LEVELS = ["never", "poor", "neutral", "good"]
COMBOS = ["cannot", "impractical", "practical", "preferred"]


def random_instance(seed: int):
    rng = random.Random(seed)
    people = [f"p{i}" for i in range(rng.randint(3, 8))]
    tasks = {f"t{i}": rng.randint(1, 3) for i in range(rng.randint(2, 4))}
    ds = dates(rng.randint(3, 6))
    inst = {
        "tasks": [{"id": t} for t in tasks],
        "dates": [{"id": d, "type": "g"} for d in ds],
        "day_type_tasks": {"g": tasks},
        "people": [{"id": p} for p in people],
        "person_tasks": {},
        "person_dates": {},
        "person_task_calendars": {},
        "combinations": [],
        "groups": [],
        "pools": [],
        "locked": [],
        "history": {p: {t: rng.randint(0, 3) for t in tasks} for p in people},
        "params": {"time_limit": 3, "workers": 2, "seed": seed},
    }
    for person in inst["people"]:
        person["proposable"] = rng.random() > 0.15
    for p in people:
        inst["person_tasks"][p] = {t: rng.choice(LEVELS) for t in tasks if rng.random() < 0.7}
        inst["person_dates"][p] = {d: rng.choice(LEVELS) for d in ds if rng.random() < 0.4}
        if rng.random() < 0.2 and inst["person_tasks"][p]:
            t = rng.choice(list(inst["person_tasks"][p]))
            inst["person_task_calendars"][p] = {t: {d: rng.choice(LEVELS) for d in ds if rng.random() < 0.5}}
    tl = list(tasks)
    for i, a in enumerate(tl):
        for b in tl[i + 1 :]:
            if rng.random() < 0.7:
                inst["combinations"].append({"a": a, "b": b, "level": rng.choice(COMBOS)})
    for _ in range(rng.randint(0, 2)):
        inst["pools"].append(
            {"date": rng.choice(ds), "task": rng.choice(tl), "people": rng.sample(people, rng.randint(1, 3))}
        )
    if rng.random() < 0.6:
        inst["groups"].append({"id": "g1", "members": rng.sample(people, rng.randint(1, max(1, len(people) // 2)))})
    inst["group_tasks"] = []
    if inst["groups"]:
        inst["groups"][0]["proposable"] = rng.random() > 0.2
        for t in rng.sample(tl, rng.randint(0, len(tl))):
            inst["group_tasks"].append({"group": "g1", "task": t, "fills": rng.randint(1, 2), "level": rng.choice(LEVELS)})
        for pl in inst["pools"]:
            if rng.random() < 0.5:
                pl["groups"] = ["g1"]
    seen = set()
    for _ in range(rng.randint(0, 3)):
        key = (rng.choice(ds), rng.choice(tl))
        who = rng.choice(people + [None])
        if who is None and inst["groups"] and rng.random() < 0.6:
            inst["locked"].append({"date": key[0], "task": key[1], "group": "g1"})
        elif who is None or (key, who) in seen:
            inst["locked"].append({"date": key[0], "task": key[1], "label": "Gjest"})
        else:
            seen.add((key, who))
            inst["locked"].append({"date": key[0], "task": key[1], "person": who})
    return inst


@pytest.mark.parametrize("seed", range(30))
def test_random_instance_breaks_no_hard_requirement(seed):
    run(random_instance(seed))
