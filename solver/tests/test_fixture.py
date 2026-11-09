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

    # Planleggeren foreslår faste team og par som en enhet; hvert medlem teller ett oppdrag per gang
    assert res["group_assignments"]
    members = {g["id"]: g["members"] for g in data["groups"]}
    manual = [lk for lk in data["locked"] if "group" in lk]
    for p, load in res["load"].items():
        own = sum(1 for a in res["assignments"] if a["person"] == p)
        via = sum(1 for g in res["group_assignments"] if p in g["members"])
        locked_cells = sum(1 for lk in manual if p in members[lk["group"]])
        assert load["count"] == own + via + locked_cells, p

    # Lik TOTAL belastning (brukerens valg 2026-11-09): ingen ligger langt fra sin egen andel (som følger hvor mange datoer
    # personen kan stille og historikken), uansett hvilke oppgaver de tar. Målt: verste avvik ca. 0,9.
    assert max(abs(v["count"] - v["target"]) for v in res["load"].values()) <= 2.0, res["load"]
