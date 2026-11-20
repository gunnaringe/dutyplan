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

    # The singers are set by hand on Singing; the combination setting keeps them away from Sound and Video
    singing = [lk for lk in data["locked"] if lk["task"] == "singing"]
    singers = {lk["person"] for lk in singing}
    assert len({lk["date"] for lk in singing}) == 2 and len(singers) == 6
    for d in {lk["date"] for lk in singing}:
        on_d = [a for a in res["assignments"] if a["date"] == d and a["task"] in ("sound", "video")]
        assert on_d and not {a["person"] for a in on_d} & singers
    for pool in data["pools"]:
        got = [a["person"] for a in res["assignments"] if a["date"] == pool["date"] and a["task"] == pool["task"]]
        assert got and set(got) <= set(pool["people"])
    guest = next(lk for lk in data["locked"] if lk.get("label") == "Outside guest")
    assert {**guest, "fills": 1} in res["manual"]
    fixed = next(lk for lk in data["locked"] if lk.get("person") == "Ingrid" and lk["task"] == "chair")
    assert any(a["person"] == "Ingrid" and a["date"] == fixed["date"] and a["task"] == fixed["task"] and a["locked"] for a in res["assignments"])
    # nobody gets many more «poor» than necessary in this fixture
    assert len(res["concessions"]) <= 25

    # Every assignment counts once in the load, the locked ones (Singing too) included
    for p, load in res["load"].items():
        assert load["count"] == sum(1 for a in res["assignments"] if a["person"] == p), p

    # Equal TOTAL load (the user's choice 2026-11-09): nobody is far from their own share (which follows how many dates
    # the person can make, and the history), whichever tasks they take. Measured: the worst deviation is about 0.9.
    assert max(abs(v["count"] - v["target"]) for v in res["load"].values()) <= 2.0, res["load"]
