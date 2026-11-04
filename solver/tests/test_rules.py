"""Én liten test per regel."""

from helpers import NO_FAIR, base, count, dates, run, who
from tjenesteplan_solver import check, solve


def test_ikke_pa_dato_gir_aldri_oppdrag():
    d = dates(4)
    inst = base(person_dates={"a": {d[0]: "ikke", d[1]: "ikke", d[2]: "ikke"}})
    inst["person_tasks"]["a"]["t"] = "godt"  # vil gjerne, men passer ikke
    res = run(inst)
    assert all(a["date"] == d[3] for a in res["assignments"] if a["person"] == "a")


def test_ikke_med_pa_oppgaven_gir_aldri_oppdrag():
    inst = base()
    del inst["person_tasks"]["b"]["t"]
    res = run(inst)
    assert count(res, "b") == 0


def test_ikke_pa_oppgave_gir_aldri_oppdrag():
    inst = base()
    inst["person_tasks"]["c"]["t"] = "ikke"
    assert count(run(inst), "c") == 0


def test_flere_plasser_gir_ulike_personer():
    inst = base(people=("a", "b", "c", "d", "e"), tasks={"kafe": 3})
    res = run(inst)
    for d in dates(4):
        ps = who(res, "kafe", d)
        assert len(ps) == 3 and len(set(ps)) == 3


def test_kan_ikke_kombineres_er_hardt():
    inst = base(
        n_dates=1,
        tasks={"lyd": 1, "bilde": 1},
        combinations=[{"a": "lyd", "b": "bilde", "level": "kan_ikke"}],
        params=NO_FAIR,
    )
    inst["person_tasks"]["a"] = {"lyd": "godt", "bilde": "godt"}  # a vil gjerne ha begge
    res = run(inst)
    assert sorted(a["person"] for a in res["assignments"] if a["person"] == "a") in ([], ["a"])
    assert who(res, "lyd") != who(res, "bilde")


def test_bor_kombineres_gir_samme_person():
    inst = base(
        n_dates=3,
        tasks={"x": 1, "y": 1},
        combinations=[{"a": "x", "b": "y", "level": "bor"}],
        params=NO_FAIR,
    )
    res = run(inst)
    for d in dates(3):
        assert who(res, "x", d) == who(res, "y", d)


def test_upraktisk_unngas_nar_det_gar():
    inst = base(n_dates=3, tasks={"x": 1, "y": 1}, people=("a", "b"))
    res = run(inst)  # standard er upraktisk
    for d in dates(3):
        assert who(res, "x", d) != who(res, "y", d)


def test_kirkekoret_er_bare_en_manuell_rute_pluss_utvalg():
    """Ingen egen regel: admin legger Kirkekoret inn for hånd og setter et utvalg for Lyd den dagen."""
    d = dates(4)
    inst = base(
        people=("a", "b", "c", "d"),
        tasks={"lyd": 1},
        locked=[{"date": d[1], "task": "lovsang", "label": "Kirkekoret"}],
        pools=[{"date": d[1], "task": "lyd", "people": ["b", "c"]}],
    )
    inst["tasks"].append({"id": "lovsang", "planned": False})  # med i oppsettet, men fordeles ikke
    inst["person_tasks"]["a"]["lyd"] = "godt"
    res = run(inst)
    assert res["labels"] == [{"date": d[1], "task": "lovsang", "label": "Kirkekoret"}]
    assert who(res, "lyd", d[1])[0] in ("b", "c")
    assert not [a for a in res["assignments"] if a["task"] == "lovsang"]


def test_utvalg_begrenser_valget():
    d = dates(4)
    inst = base(people=("a", "b", "c", "d"), pools=[{"date": d[2], "task": "t", "people": ["c", "d"]}])
    inst["person_tasks"]["a"]["t"] = "godt"
    res = run(inst)
    assert who(res, "t", d[2])[0] in ("c", "d")


def test_utvalg_med_ikke_gir_tom_plass_med_forklaring():
    d = dates(3)
    inst = base(
        n_dates=3,
        pools=[{"date": d[1], "task": "t", "people": ["b"]}],
        person_dates={"b": {d[1]: "ikke"}},
    )
    res = run(inst)
    (u,) = res["unfilled"]
    assert (u["date"], u["task"], u["missing"]) == (d[1], "t", 1)
    assert u["why"]["dato_ikke"] == ["b"]
    assert u["why"]["utvalg"] == ["b"]
    assert u["why"]["kandidater"] == 0


def test_alle_ute_gir_tom_plass_og_forklaring():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={p: {d: "ikke"} for p in "abc"})
    res = run(inst)
    assert res["unfilled"][0]["why"]["dato_ikke"] == ["a", "b", "c"]


def test_last_person_beholdes_og_ledd_teller_som_plass():
    d = dates(3)
    inst = base(
        tasks={"k": 2},
        n_dates=3,
        locked=[
            {"date": d[0], "task": "k", "person": "c"},
            {"date": d[1], "task": "k", "label": "Egen komité"},
        ],
    )
    res = run(inst)
    assert "c" in who(res, "k", d[0]) and len(who(res, "k", d[0])) == 2
    assert len(who(res, "k", d[1])) == 1
    assert res["labels"] == [{"date": d[1], "task": "k", "label": "Egen komité"}]


def test_last_person_vinner_over_ikke_men_gir_advarsel():
    d = dates(2)
    inst = base(n_dates=2, person_dates={"a": {d[0]: "ikke"}}, locked=[{"date": d[0], "task": "t", "person": "a"}])
    res = solve(inst)
    assert who(res, "t", d[0]) == ["a"]
    assert res["warnings"] and "dato_ikke" in res["warnings"][0]


def test_jevn_fordeling_mellom_like_personer():
    res = run(base(n_dates=6))
    assert sorted(count(res, p) for p in "abc") == [2, 2, 2]


def test_jevn_fordeling_tar_hensyn_til_tilgjengelighet():
    d = dates(6)
    ikke = {d[i]: "ikke" for i in (1, 2, 3, 5)}  # a kan bare to av seks datoer, ikke på rad
    inst = base(n_dates=6, person_dates={"a": ikke})
    inst["person_tasks"]["a"]["t"] = "godt"  # a trekkes mot oppdrag, men andelen hennes er under én
    res = run(inst)
    assert count(res, "a") <= 1
    assert sorted((count(res, "b"), count(res, "c"))) == [2, 3] or count(res, "a") == 0


def test_historikk_gir_mindre_til_den_som_har_gjort_mest():
    uten = run(base(n_dates=6, history={"a": {"t": 3}}, params={"history_weight": 0}))
    assert count(uten, "a") == 2  # uten historikk deler de likt
    full = run(base(n_dates=6, history={"a": {"t": 3}}, params={"history_weight": 1}))
    assert count(full, "a") == 0  # 3 + 0 mot 0 + 3: tar igjen helt
    halv = run(base(n_dates=6, history={"a": {"t": 3}}))  # standard: teller halvt, «litt mindre»
    assert count(halv, "a") <= 1
    assert halv["task_load"]["t"]["a"]["history"] == 2


def test_historikk_pa_en_oppgave_pavirker_ikke_en_annen():
    res = run(base(n_dates=6, tasks={"x": 1, "y": 1}, history={"a": {"x": 4}}, params={"history_weight": 1}))
    assert count(res, "a", "x") < count(res, "b", "x")
    assert abs(count(res, "a", "y") - count(res, "b", "y")) <= 1


def test_stabilitet_beholder_forrige_plan():
    d = dates(6)
    prev = [{"date": d[i], "task": "t", "person": "abc"[i % 3]} for i in range(6)]
    res = run(base(n_dates=6, previous_plan=prev))
    got = {(a["date"], a["person"]) for a in res["assignments"]}
    assert got == {(p["date"], p["person"]) for p in prev}


def test_hvile_unngar_samme_person_to_ganger_pa_rad():
    res = run(base(people=("a", "b"), n_dates=6))
    seq = [who(res, "t", d)[0] for d in dates(6)]
    assert all(x != y for x, y in zip(seq, seq[1:])), seq


def test_egen_kalender_erstatter_den_felles():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={"a": {d: "ikke"}}, person_task_calendars={"a": {"t": {d: "godt"}}})
    inst["person_tasks"]["a"]["t"] = "godt"
    assert who(run(inst), "t", d) == ["a"]


def test_darlig_brukes_bare_nar_det_trengs_og_meldes():
    d = dates(1)[0]
    inst = base(n_dates=1, people=("a", "b"), person_dates={"a": {d: "darlig"}})
    assert who(run(inst), "t", d) == ["b"]
    inst["person_dates"]["b"] = {d: "ikke"}
    res = run(inst)
    assert who(res, "t", d) == ["a"]
    assert res["concessions"] and res["concessions"][0]["person"] == "a"


def test_maks_oppgaver_per_dag():
    inst = base(
        people=("a",),
        n_dates=1,
        tasks={"x": 1, "y": 1, "z": 1},
        combinations=[{"a": p, "b": q, "level": "bor"} for p, q in [("x", "y"), ("x", "z"), ("y", "z")]],
        params={**NO_FAIR, "max_tasks_per_day": 2},
    )
    res = run(inst)
    assert len(res["assignments"]) == 2 and sum(u["missing"] for u in res["unfilled"]) == 1


def test_sjekkeren_finner_brudd():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={"a": {d: "ikke"}})
    res = run(inst)
    res["assignments"][0]["person"] = "a"  # tukle med planen
    assert any("ikke" in p for p in check(inst, res))
