"""Én liten test per regel."""

import pytest
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
    assert res["manual"] == [{"date": d[1], "task": "lovsang", "label": "Kirkekoret", "fills": 1}]
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
    assert res["manual"] == [{"date": d[1], "task": "k", "label": "Egen komité", "fills": 1}]


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


def test_sjekkeren_finner_brudd():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={"a": {d: "ikke"}})
    res = run(inst)
    res["assignments"][0]["person"] = "a"  # tukle med planen
    assert any("ikke" in p for p in check(inst, res))


# --- generell gruppestøtte: ingen særregler, bare grupper + den felles kombinasjonsinnstillingen ----------


def group_inst(level=None, **extra):
    """Gruppe «g» = {a}, satt opp på «sang» første dato (oppgaven fordeles ikke). a vil gjerne ha Lyd."""
    d = dates(3)
    inst = base(
        n_dates=3,
        tasks={"lyd": 1},
        groups=[{"id": "g", "members": ["a"]}],
        locked=[{"date": d[0], "task": "sang", "group": "g"}],
        combinations=[{"a": "sang", "b": "lyd", "level": level}] if level else [],
        **extra,
    )
    inst["tasks"].append({"id": "sang", "planned": False})
    inst["person_tasks"]["a"]["lyd"] = "godt"
    return inst, d


def test_gruppe_gjor_medlemmer_opptatt_via_kombinasjonsinnstillingen():
    inst, d = group_inst("kan_ikke")
    res = run(inst)  # run() sjekker også gruppekravet i den uavhengige sjekkeren
    assert "a" not in who(res, date=d[0])
    assert res["manual"] == [{"date": d[0], "task": "sang", "group": "g", "fills": 1}]


def test_gruppe_uten_hardt_krav_er_bare_en_straff_og_kan_bli_valgt_hvis_det_trengs():
    inst, d = group_inst()  # paret er ikke satt: standard er upraktisk
    for other in "bc":
        inst.setdefault("person_dates", {})[other] = {d[0]: "ikke"}
    assert who(run(inst), "lyd", d[0]) == ["a"]  # ingen andre kan, og upraktisk er ikke forbudt


def test_gruppe_med_bor_kombineres_gir_medlemmet_oppgaven():
    inst, d = group_inst("bor", params=NO_FAIR)
    inst["person_tasks"]["a"]["lyd"] = "noytral"  # uten belønningen er b og c bedre
    inst["person_tasks"]["b"]["lyd"] = inst["person_tasks"]["c"]["lyd"] = "godt"
    assert who(run(inst), "lyd", d[0]) == ["a"]
    inst["combinations"] = []  # uten innstilling (upraktisk) velges b eller c
    assert who(run(inst), "lyd", d[0])[0] in ("b", "c")


def test_medlem_av_gruppe_kan_ikke_ta_en_ekstra_plass_pa_samme_oppgave():
    d = dates(1)[0]
    inst = base(
        people=("a", "b", "c", "d"),
        n_dates=1,
        tasks={"kafe": 3},
        groups=[{"id": "lag", "members": ["a"]}],
        locked=[{"date": d, "task": "kafe", "group": "lag"}],  # laget fyller én av tre plasser
        params={**NO_FAIR, "w_combo": {"upraktisk": 0}},  # så en straff ikke kan skjule forskjellen
    )
    inst["person_tasks"]["a"]["kafe"] = "godt"  # a vil gjerne, men er allerede med via laget
    res = run(inst)
    assert "a" not in who(res, "kafe", d) and len(who(res, "kafe", d)) == 2


def test_gruppe_kan_fylle_flere_plasser():
    d = dates(2)
    inst = base(
        people=("a", "b", "c", "d", "e"),
        n_dates=2,
        tasks={"kafe": 3},
        groups=[{"id": "lag", "members": ["a", "b", "c"]}],
        locked=[{"date": d[0], "task": "kafe", "group": "lag", "fills": 3}],
    )
    res = run(inst)
    assert who(res, "kafe", d[0]) == [] and not res["unfilled"]  # gruppen fyller alle tre
    assert len(who(res, "kafe", d[1])) == 3


def test_gruppe_medlem_med_ikke_gir_advarsel_og_manuelt_valg_beholdes():
    inst, d = group_inst("kan_ikke")
    inst.setdefault("person_dates", {})["a"] = {d[0]: "ikke"}
    res = solve(inst)
    assert any("«ikke»" in w for w in res["warnings"])
    assert res["manual"]


def test_ukjent_gruppe_eller_medlem_gir_feil():
    inst, d = group_inst()
    inst["locked"][0]["group"] = "finnes_ikke"
    with pytest.raises(ValueError):
        solve(inst)
    inst, d = group_inst()
    inst["groups"][0]["members"] = ["ukjent"]
    with pytest.raises(ValueError):
        solve(inst)


def test_forklaring_nevner_gruppen_nar_medlemmer_er_opptatt():
    d = dates(1)[0]
    inst = base(
        people=("a",),
        n_dates=1,
        groups=[{"id": "g", "members": ["a"]}],
        locked=[{"date": d, "task": "sang", "group": "g"}],
        combinations=[{"a": "sang", "b": "t", "level": "kan_ikke"}],
    )
    inst["tasks"].append({"id": "sang", "planned": False})
    res = run(inst)
    assert res["unfilled"][0]["why"]["opptatt"] == {"a": ["sang (g)"]}
    assert res["unfilled"][0]["why"]["kandidater"] == 0


# --- planleggeren kan foreslå grupper -----------------------------------------------------------------


def lag_inst(**extra):
    """a og b er bare med via laget «lag»; ingen er med på oppgaven personlig."""
    inst = base(
        people=("a", "b", "c"),
        n_dates=4,
        groups=[{"id": "lag", "members": ["a", "b"]}],
        group_tasks=[{"group": "lag", "task": "t"}],
        **extra,
    )
    inst["person_tasks"] = {}
    return inst


def test_planleggeren_kan_foresla_en_gruppe():
    res = run(lag_inst())
    assert len(res["group_assignments"]) == 4 and not res["assignments"] and not res["unfilled"]
    assert all(a["members"] == ["a", "b"] for a in res["group_assignments"])


def test_ett_medlems_ikke_utelukker_gruppen_og_forklares():
    d = dates(4)
    inst = lag_inst(person_dates={"a": {d[1]: "ikke"}})
    res = run(inst)
    assert {a["date"] for a in res["group_assignments"]} == {d[0], d[2], d[3]}
    (u,) = res["unfilled"]
    assert u["date"] == d[1] and u["why"]["grupper"] == {"lag": "dato_ikke:a"}


def test_gruppe_som_trenger_flere_plasser_enn_det_er_kan_ikke_velges():
    inst = lag_inst()
    inst["group_tasks"][0]["fills"] = 2  # oppgaven har bare én plass
    res = run(inst)
    assert not res["group_assignments"] and len(res["unfilled"]) == 4
    assert res["unfilled"][0]["why"]["grupper"] == {"lag": "for_mange_plasser"}


def test_foreslatt_gruppe_kan_fylle_alle_plasser_pa_en_dag():
    d = dates(2)
    inst = base(
        people=("a", "b"),
        n_dates=2,
        tasks={"forbonn": 2},
        groups=[{"id": "par", "members": ["a", "b"]}],
        group_tasks=[{"group": "par", "task": "forbonn", "fills": 2}],
    )
    inst["person_tasks"] = {}
    res = run(inst)
    assert len(res["group_assignments"]) == 2 and not res["unfilled"]


def test_foreslatt_gruppe_gjor_medlemmer_opptatt_via_kombinasjonene():
    inst = base(
        people=("a", "b"),
        n_dates=1,
        tasks={"t": 1, "u": 1},
        groups=[{"id": "lag", "members": ["a"]}],
        group_tasks=[{"group": "lag", "task": "t", "level": "godt"}],
        combinations=[{"a": "t", "b": "u", "level": "kan_ikke"}],
    )
    inst["person_tasks"] = {"a": {"u": "godt"}, "b": {"u": "noytral"}}  # a vil gjerne ha u, men er i laget på t
    res = run(inst)
    assert [g["group"] for g in res["group_assignments"]] == ["lag"]
    assert who(res, "u") == ["b"]


def test_to_grupper_med_felles_medlem_kan_ikke_begge_velges_pa_samme_oppgave():
    inst = base(
        people=("a", "b", "c"),
        n_dates=1,
        tasks={"t": 2},
        groups=[{"id": "g1", "members": ["a", "b"]}, {"id": "g2", "members": ["b", "c"]}],
        group_tasks=[{"group": "g1", "task": "t"}, {"group": "g2", "task": "t"}],
    )
    inst["person_tasks"] = {}
    res = run(inst)
    assert len(res["group_assignments"]) == 1 and sum(u["missing"] for u in res["unfilled"]) == 1


def test_utvalg_kan_navngi_grupper_og_et_utvalg_uten_grupper_utelukker_dem():
    d = dates(2)
    inst = base(
        people=("a", "b", "c"),
        n_dates=2,
        groups=[{"id": "g1", "members": ["a"]}, {"id": "g2", "members": ["b"]}],
        group_tasks=[{"group": "g1", "task": "t", "level": "godt"}, {"group": "g2", "task": "t", "level": "godt"}],
        pools=[{"date": d[0], "task": "t", "groups": ["g2"]}, {"date": d[1], "task": "t", "people": ["c"]}],
    )
    inst["person_tasks"] = {"c": {"t": "noytral"}}
    res = run(inst)
    assert [g["group"] for g in res["group_assignments"] if g["date"] == d[0]] == ["g2"]
    assert who(res, "t", d[1]) == ["c"] and not [g for g in res["group_assignments"] if g["date"] == d[1]]


def test_gruppearbeid_teller_ett_oppdrag_per_medlem_for_manuelle_ruter():
    d = dates(6)

    def run_with(manual):
        inst = base(n_dates=6, groups=[{"id": "lag", "members": ["a"]}],
                    locked=[{"date": d[i], "task": "t", "group": "lag"} for i in range(manual)])
        return run(inst)

    uten = run_with(0)
    assert (count(uten, "a"), uten["load"]["a"]["count"]) == (2, 2)  # uten gruppearbeid deler de likt
    med = run_with(2)  # laget (kun a) fyller t to ganger for hånd
    assert count(med, "a") == 0  # de to rutene er a sin andel: ett oppdrag hver
    assert med["load"]["a"]["count"] == 2 and med["task_load"]["t"]["a"]["count"] == 2
    assert sorted(count(med, p) for p in "bc") == [2, 2]
    assert not med["unfilled"]


def test_oppdrag_teller_ett_per_medlem_for_foreslatte_grupper():
    inst = base(
        people=("a", "b", "c"),
        n_dates=4,
        groups=[{"id": "lag", "members": ["a", "b"]}],
        group_tasks=[{"group": "lag", "task": "t", "level": "godt"}],
    )
    res = run(inst)
    assert res["group_assignments"]
    for p in "abc":
        via = sum(1 for g in res["group_assignments"] if p in g["members"])
        assert res["load"][p]["count"] == count(res, p) + via
        assert res["task_load"]["t"][p]["count"] == count(res, p, "t") + via


def test_foreslatt_gruppe_teller_i_fordelingen_slik_at_den_ikke_tar_alt():
    """a og b er bare med via laget, som alle vil ha (godt). Teller medlemmene ett oppdrag hver per gang,
    må laget dele datoene med c; ellers ville laget tatt alle seks."""
    inst = base(
        people=("a", "b", "c"),
        n_dates=6,
        groups=[{"id": "lag", "members": ["a", "b"]}],
        group_tasks=[{"group": "lag", "task": "t", "level": "godt"}],
    )
    inst["person_tasks"] = {"c": {"t": "noytral"}}
    res = run(inst)
    assert 1 <= len(res["group_assignments"]) <= 3
    assert count(res, "c") >= 3


def test_manuelt_gruppearbeid_teller_ogsa_i_det_samlede_leddet():
    """Gruppearbeid på en annen oppgave (sang) påvirker bare det samlede leddet; skru av oppgaveleddet for å se det."""
    d = dates(6)
    params = {"w_combo": {"upraktisk": 0}, "w_task_fair": 0, "w_task_range": 0}

    def planned_for_a(manual):
        inst = base(n_dates=6, groups=[{"id": "lag", "members": ["a"]}], params=params,
                    locked=[{"date": d[i], "task": "sang", "group": "lag"} for i in range(manual)])
        inst["tasks"].append({"id": "sang", "planned": False})
        return count(run(inst), "a")

    assert planned_for_a(0) == 2
    assert planned_for_a(2) <= 1


def test_stabilitet_gjelder_ogsa_foreslatte_grupper():
    d = dates(4)
    inst = base(
        people=("a", "b"),
        n_dates=4,
        groups=[{"id": "g1", "members": ["a"]}, {"id": "g2", "members": ["b"]}],
        group_tasks=[{"group": "g1", "task": "t"}, {"group": "g2", "task": "t"}],
        previous_plan=[{"date": d[i], "task": "t", "group": "g1" if i % 2 == 0 else "g2"} for i in range(4)],
    )
    inst["person_tasks"] = {}
    res = run(inst)
    assert [g["group"] for g in res["group_assignments"]] == ["g1", "g2", "g1", "g2"]
