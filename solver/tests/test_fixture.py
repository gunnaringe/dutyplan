"""The autumn plan (invented names and dates): all the rules together at a realistic size."""

import json
from pathlib import Path

from helpers import run

FIXTURE = Path(__file__).parent.parent / "fixtures" / "autumn2026.json"


def test_autumn2026_gives_a_valid_plan_without_empty_places():
    data = json.loads(FIXTURE.read_text(encoding="utf-8"))
    data["params"].update(time_limit=8, workers=4)
    res = run(data)  # run() also runs the independent checker
    assert res["unfilled"] == []
    assert res["warnings"] == []

    # The choir is an ordinary group in manual cells; the combination setting keeps its members away from Sound and Video
    choir = [lk for lk in data["locked"] if lk.get("group") == "choir"]
    members = set(data["groups"][0]["members"])
    assert len(choir) == 2 and all(lk["task"] == "singing" for lk in choir)
    for lk in choir:
        on_d = [a for a in res["assignments"] if a["date"] == lk["date"] and a["task"] in ("sound", "video")]
        assert on_d and not {a["person"] for a in on_d} & members
    for pool in data["pools"]:
        got = [a["person"] for a in res["assignments"] if a["date"] == pool["date"] and a["task"] == pool["task"]]
        assert got and set(got) <= set(pool["people"])
    guest = next(lk for lk in data["locked"] if lk.get("label") == "Outside guest")
    assert {**guest, "fills": 1} in res["manual"]
    fixed = next(lk for lk in data["locked"] if lk.get("person") == "Ingrid")
    assert any(a["person"] == "Ingrid" and a["date"] == fixed["date"] and a["task"] == fixed["task"] and a["locked"] for a in res["assignments"])
    # nobody gets many more «poor» than necessary in this fixture
    assert len(res["concessions"]) <= 25

    # The planner proposes fixed teams and couples as a unit; each member counts one assignment each time
    assert res["group_assignments"]
    members = {g["id"]: g["members"] for g in data["groups"]}
    manual = [lk for lk in data["locked"] if "group" in lk]
    for p, load in res["load"].items():
        own = sum(1 for a in res["assignments"] if a["person"] == p)
        via = sum(1 for g in res["group_assignments"] if p in g["members"])
        locked_cells = sum(1 for lk in manual if p in members[lk["group"]])
        assert load["count"] == own + via + locked_cells, p

    # Equal TOTAL load (the user's choice 2026-11-09): nobody is far from their own share (which follows how many dates
    # the person can make, and the history), whichever tasks they take. Measured: the worst deviation is about 0.9.
    assert max(abs(v["count"] - v["target"]) for v in res["load"].values()) <= 2.0, res["load"]
