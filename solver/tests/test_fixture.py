"""HØST 2026-formen (oppdiktede navn): hele reglene sammen på en realistisk størrelse."""

import json
from pathlib import Path

from helpers import run

FIXTURE = Path(__file__).parent.parent / "fixtures" / "host2026.json"


def test_host2026_gir_gyldig_plan_uten_tomme_plasser():
    data = json.loads(FIXTURE.read_text(encoding="utf-8"))
    data["params"].update(time_limit=8, workers=4)
    res = run(data)  # run() kjører også den uavhengige sjekkeren
    assert res["unfilled"] == []
    assert res["warnings"] == []

    # Kirkekoret er en vanlig gruppe i manuelle ruter; kombinasjonsinnstillingen holder medlemmene unna Lyd og Bilde
    kor = [lk for lk in data["locked"] if lk.get("group") == "kirkekoret"]
    members = set(data["groups"][0]["members"])
    assert len(kor) == 2 and all(lk["task"] == "lovsang" for lk in kor)
    for lk in kor:
        on_d = [a for a in res["assignments"] if a["date"] == lk["date"] and a["task"] in ("lyd", "bilde")]
        assert on_d and not {a["person"] for a in on_d} & members
    for pool in data["pools"]:
        got = [a["person"] for a in res["assignments"] if a["date"] == pool["date"] and a["task"] == pool["task"]]
        assert got and set(got) <= set(pool["people"])
    assert {"date": "2026-09-13", "task": "taler", "label": "Gjest utenfra", "fills": 1} in res["manual"]
    assert any(a["person"] == "Ingrid" and a["date"] == "2026-12-24" and a["locked"] for a in res["assignments"])
    # ingen får mange flere «dårlig» enn nødvendig i denne fixturen
    assert len(res["concessions"]) <= 25

    # Lik fordeling per oppgave: ingen ligger langt fra sin andel, og de som kan like mange datoer
    # får omtrent like mange oppdrag. Uten dette gikk én person på 7 forbønner mens en annen fikk 0.
    for t, per in res["task_load"].items():
        assert max(abs(v["count"] + v["history"] - v["target"] - v["history"]) for v in per.values()) <= 2.0, t
        big = max(v["capacity"] for v in per.values())
        counts = [v["count"] for v in per.values() if v["capacity"] >= 0.7 * big]
        assert max(counts) - min(counts) <= 5, (t, per)
