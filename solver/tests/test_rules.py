"""Én liten test per regel."""

import pytest
from helpers import NO_FAIR, base, count, dates, run, who
from dutyplan_solver import check, solve


def test_ikke_pa_dato_gir_aldri_oppdrag():
    d = dates(4)
    inst = base(person_dates={"a": {d[0]: "never", d[1]: "never", d[2]: "never"}})
    inst["person_tasks"]["a"]["t"] = "good"  # vil gjerne, men passer ikke
    res = run(inst)
    assert all(a["date"] == d[3] for a in res["assignments"] if a["person"] == "a")


def test_ikke_med_pa_oppgaven_gir_aldri_oppdrag():
    inst = base()
    del inst["person_tasks"]["b"]["t"]
    res = run(inst)
    assert count(res, "b") == 0


def test_ikke_pa_oppgave_gir_aldri_oppdrag():
    inst = base()
    inst["person_tasks"]["c"]["t"] = "never"
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
        combinations=[{"a": "lyd", "b": "bilde", "level": "cannot"}],
        params=NO_FAIR,
    )
    inst["person_tasks"]["a"] = {"lyd": "good", "bilde": "good"}  # a vil gjerne ha begge
    res = run(inst)
    assert sorted(a["person"] for a in res["assignments"] if a["person"] == "a") in ([], ["a"])
    assert who(res, "lyd") != who(res, "bilde")


def test_bor_kombineres_gir_samme_person():
    inst = base(
        n_dates=3,
        tasks={"x": 1, "y": 1},
        combinations=[{"a": "x", "b": "y", "level": "preferred"}],
        params=NO_FAIR,
    )
    res = run(inst)
    for d in dates(3):
        assert who(res, "x", d) == who(res, "y", d)


def test_upraktisk_unngas_nar_det_gar():
    inst = base(n_dates=3, tasks={"x": 1, "y": 1}, people=("a", "b"))
    res = run(inst)  # standard er impractical
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
    inst["person_tasks"]["a"]["lyd"] = "good"
    res = run(inst)
    assert res["manual"] == [{"date": d[1], "task": "lovsang", "label": "Kirkekoret", "fills": 1}]
    assert who(res, "lyd", d[1])[0] in ("b", "c")
    assert not [a for a in res["assignments"] if a["task"] == "lovsang"]


def test_utvalg_begrenser_valget():
    d = dates(4)
    inst = base(people=("a", "b", "c", "d"), pools=[{"date": d[2], "task": "t", "people": ["c", "d"]}])
    inst["person_tasks"]["a"]["t"] = "good"
    res = run(inst)
    assert who(res, "t", d[2])[0] in ("c", "d")


def test_utvalg_med_ikke_gir_tom_plass_med_forklaring():
    d = dates(3)
    inst = base(
        n_dates=3,
        pools=[{"date": d[1], "task": "t", "people": ["b"]}],
        person_dates={"b": {d[1]: "never"}},
    )
    res = run(inst)
    (u,) = res["unfilled"]
    assert (u["date"], u["task"], u["missing"]) == (d[1], "t", 1)
    assert u["why"]["event_never"] == ["b"]
    assert u["why"]["selection"] == ["b"]
    assert u["why"]["candidates"] == 0


def test_alle_ute_gir_tom_plass_og_forklaring():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={p: {d: "never"} for p in "abc"})
    res = run(inst)
    assert res["unfilled"][0]["why"]["event_never"] == ["a", "b", "c"]


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
    inst = base(n_dates=2, person_dates={"a": {d[0]: "never"}}, locked=[{"date": d[0], "task": "t", "person": "a"}])
    res = solve(inst)
    assert who(res, "t", d[0]) == ["a"]
    assert res["warnings"] and "event_never" in res["warnings"][0]


def test_jevn_fordeling_mellom_like_personer():
    res = run(base(n_dates=6))
    assert sorted(count(res, p) for p in "abc") == [2, 2, 2]


def test_jevn_fordeling_tar_hensyn_til_tilgjengelighet():
    d = dates(6)
    ikke = {d[i]: "never" for i in (1, 2, 3, 5)}  # a kan bare to av seks datoer, ikke på rad
    inst = base(n_dates=6, person_dates={"a": ikke})
    inst["person_tasks"]["a"]["t"] = "good"  # a trekkes mot oppdrag, men andelen hennes er under én
    res = run(inst)
    assert count(res, "a") <= 1
    assert sorted((count(res, "b"), count(res, "c"))) == [2, 3] or count(res, "a") == 0


def test_lik_total_belastning_selv_om_noen_bare_tar_en_oppgave():
    """«En som har bare bilde bør ikke være halvparten av en som har lyd og bilde»: a tar bare bilde, b både lyd og bilde
    og c bare lyd. Samlet er det 12 oppdrag, så alle skal ha 4, og b tar altså ikke 3 av hver."""
    def inst(**params):
        i = base(n_dates=6, tasks={"bilde": 1, "lyd": 1}, params={"w_combo": {"impractical": 0}, **params})
        i["person_tasks"] = {"a": {"bilde": "neutral"}, "b": {"bilde": "neutral", "lyd": "neutral"}, "c": {"lyd": "neutral"}}
        return i

    res = run(inst())
    assert [count(res, p) for p in "abc"] == [4, 4, 4]
    assert [res["load"][p]["count"] for p in "abc"] == [4, 4, 4]
    # med lik andel av hver oppgave (den gamle vektingen) ble det 3, 6 og 3: a og c på halvparten av b
    gammel = run(inst(w_fair=0, w_fair_range=0, w_task_fair=10, w_task_range=10))
    assert [count(gammel, p) for p in "abc"] == [3, 6, 3]


def test_historikk_gir_mindre_til_den_som_har_gjort_mest():
    uten = run(base(n_dates=6, history={"a": {"t": 3}}, params={"history_weight": 0}))
    assert count(uten, "a") == 2  # uten historikk deler de likt
    full = run(base(n_dates=6, history={"a": {"t": 3}}, params={"history_weight": 1}))
    assert count(full, "a") == 0  # 3 + 0 mot 0 + 3: tar igjen helt
    halv = run(base(n_dates=6, history={"a": {"t": 3}}))  # standard: teller halvt, «litt mindre»
    assert count(halv, "a") <= 1
    assert halv["task_load"]["t"]["a"]["history"] == 2


def test_historikk_pa_en_oppgave_pavirker_ikke_en_annen():
    # Fordeling per oppgave er av som standard (total belastning er målet); historikk per oppgave er bare med når den er på.
    per_oppgave = {"w_task_fair": 10, "w_task_range": 10}
    res = run(base(n_dates=6, tasks={"x": 1, "y": 1}, history={"a": {"x": 4}}, params={"history_weight": 1, **per_oppgave}))
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
    inst = base(n_dates=1, person_dates={"a": {d: "never"}}, person_task_calendars={"a": {"t": {d: "good"}}})
    inst["person_tasks"]["a"]["t"] = "good"
    assert who(run(inst), "t", d) == ["a"]


def test_darlig_brukes_bare_nar_det_trengs_og_meldes():
    d = dates(1)[0]
    inst = base(n_dates=1, people=("a", "b"), person_dates={"a": {d: "poor"}})
    assert who(run(inst), "t", d) == ["b"]
    inst["person_dates"]["b"] = {d: "never"}
    res = run(inst)
    assert who(res, "t", d) == ["a"]
    assert res["concessions"] and res["concessions"][0]["person"] == "a"


def test_sjekkeren_finner_brudd():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={"a": {d: "never"}})
    res = run(inst)
    res["assignments"][0]["person"] = "a"  # tukle med planen
    assert any("never" in p for p in check(inst, res))


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
    inst["person_tasks"]["a"]["lyd"] = "good"
    return inst, d


def test_gruppe_gjor_medlemmer_opptatt_via_kombinasjonsinnstillingen():
    inst, d = group_inst("cannot")
    res = run(inst)  # run() sjekker også gruppekravet i den uavhengige sjekkeren
    assert "a" not in who(res, date=d[0])
    assert res["manual"] == [{"date": d[0], "task": "sang", "group": "g", "fills": 1}]


def test_gruppe_uten_hardt_krav_er_bare_en_straff_og_kan_bli_valgt_hvis_det_trengs():
    inst, d = group_inst()  # paret er ikke satt: standard er impractical
    for other in "bc":
        inst.setdefault("person_dates", {})[other] = {d[0]: "never"}
    assert who(run(inst), "lyd", d[0]) == ["a"]  # ingen andre kan, og impractical er ikke forbudt


def test_gruppe_med_bor_kombineres_gir_medlemmet_oppgaven():
    inst, d = group_inst("preferred", params=NO_FAIR)
    inst["person_tasks"]["a"]["lyd"] = "neutral"  # uten belønningen er b og c bedre
    inst["person_tasks"]["b"]["lyd"] = inst["person_tasks"]["c"]["lyd"] = "good"
    assert who(run(inst), "lyd", d[0]) == ["a"]
    inst["combinations"] = []  # uten innstilling (impractical) velges b eller c
    assert who(run(inst), "lyd", d[0])[0] in ("b", "c")


def test_medlem_av_gruppe_kan_ikke_ta_en_ekstra_plass_pa_samme_oppgave():
    d = dates(1)[0]
    inst = base(
        people=("a", "b", "c", "d"),
        n_dates=1,
        tasks={"kafe": 3},
        groups=[{"id": "lag", "members": ["a"]}],
        locked=[{"date": d, "task": "kafe", "group": "lag"}],  # laget fyller én av tre plasser
        params={**NO_FAIR, "w_combo": {"impractical": 0}},  # så en straff ikke kan skjule forskjellen
    )
    inst["person_tasks"]["a"]["kafe"] = "good"  # a vil gjerne, men er allerede med via laget
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
    inst, d = group_inst("cannot")
    inst.setdefault("person_dates", {})["a"] = {d[0]: "never"}
    res = solve(inst)
    assert any("«never»" in w for w in res["warnings"])
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
        combinations=[{"a": "sang", "b": "t", "level": "cannot"}],
    )
    inst["tasks"].append({"id": "sang", "planned": False})
    res = run(inst)
    assert res["unfilled"][0]["why"]["busy"] == {"a": ["sang (g)"]}
    assert res["unfilled"][0]["why"]["candidates"] == 0


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
    inst = lag_inst(person_dates={"a": {d[1]: "never"}})
    res = run(inst)
    assert {a["date"] for a in res["group_assignments"]} == {d[0], d[2], d[3]}
    (u,) = res["unfilled"]
    assert u["date"] == d[1] and u["why"]["groups"] == {"lag": "event_never:a"}


def test_gruppe_som_trenger_flere_plasser_enn_det_er_kan_ikke_velges():
    inst = lag_inst()
    inst["group_tasks"][0]["fills"] = 2  # oppgaven har bare én plass
    res = run(inst)
    assert not res["group_assignments"] and len(res["unfilled"]) == 4
    assert res["unfilled"][0]["why"]["groups"] == {"lag": "too_many_places"}


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
        group_tasks=[{"group": "lag", "task": "t", "level": "good"}],
        combinations=[{"a": "t", "b": "u", "level": "cannot"}],
    )
    inst["person_tasks"] = {"a": {"u": "good"}, "b": {"u": "neutral"}}  # a vil gjerne ha u, men er i laget på t
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
        group_tasks=[{"group": "g1", "task": "t", "level": "good"}, {"group": "g2", "task": "t", "level": "good"}],
        pools=[{"date": d[0], "task": "t", "groups": ["g2"]}, {"date": d[1], "task": "t", "people": ["c"]}],
    )
    inst["person_tasks"] = {"c": {"t": "neutral"}}
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
    # det manuelle gruppearbeidet er med i målet per oppgave (6 oppdrag, a kan fire av seks datoer: 6 * 4/16), også når fordeling per oppgave er av
    assert med["task_load"]["t"]["a"]["target"] == 1.5
    assert sorted(count(med, p) for p in "bc") == [2, 2]
    assert not med["unfilled"]


def test_oppdrag_teller_ett_per_medlem_for_foreslatte_grupper():
    inst = base(
        people=("a", "b", "c"),
        n_dates=4,
        groups=[{"id": "lag", "members": ["a", "b"]}],
        group_tasks=[{"group": "lag", "task": "t", "level": "good"}],
    )
    res = run(inst)
    assert res["group_assignments"]
    for p in "abc":
        via = sum(1 for g in res["group_assignments"] if p in g["members"])
        assert res["load"][p]["count"] == count(res, p) + via
        assert res["task_load"]["t"][p]["count"] == count(res, p, "t") + via


def test_foreslatt_gruppe_teller_i_fordelingen_slik_at_den_ikke_tar_alt():
    """a og b er bare med via laget, som alle vil ha (good). Teller medlemmene ett oppdrag hver per gang,
    må laget dele datoene med c; ellers ville laget tatt alle seks."""
    inst = base(
        people=("a", "b", "c"),
        n_dates=6,
        groups=[{"id": "lag", "members": ["a", "b"]}],
        group_tasks=[{"group": "lag", "task": "t", "level": "good"}],
    )
    inst["person_tasks"] = {"c": {"t": "neutral"}}
    res = run(inst)
    assert 1 <= len(res["group_assignments"]) <= 3
    assert count(res, "c") >= 3


def test_manuelt_gruppearbeid_teller_ogsa_i_det_samlede_leddet():
    """Gruppearbeid på en annen oppgave (sang) påvirker bare det samlede leddet; skru av oppgaveleddet for å se det."""
    d = dates(6)
    params = {"w_combo": {"impractical": 0}, "w_task_fair": 0, "w_task_range": 0}

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


# --- «kan foreslås»: innstilling per person og per gruppe --------------------------------------------


def test_person_som_ikke_kan_foreslaas_velges_aldri_men_kan_lases():
    d = dates(4)
    inst = base(n_dates=4, locked=[{"date": d[0], "task": "t", "person": "a"}])
    inst["people"][0]["proposable"] = False
    inst["person_tasks"]["a"]["t"] = "good"  # alle andre ville vært bedre uten innstillingen
    res = run(inst)
    assert who(res, "t", d[0]) == ["a"] and count(res, "a") == 1  # bare det manuelle valget


def test_gruppe_som_ikke_kan_foreslaas_velges_aldri_men_kan_stå_i_en_manuell_rute():
    d = dates(4)
    inst = lag_inst(locked=[{"date": d[0], "task": "t", "group": "lag"}])
    inst["groups"][0]["proposable"] = False
    inst["group_tasks"][0]["level"] = "good"
    res = run(inst)
    assert not res["group_assignments"]
    assert res["manual"] == [{"date": d[0], "task": "t", "group": "lag", "fills": 1}]
    assert res["unfilled"][0]["why"]["groups"] == {"lag": "not_proposable"}


def test_gruppe_med_medlem_som_ikke_kan_foreslaas_utelukkes():
    inst = lag_inst()
    inst["people"][0]["proposable"] = False  # a er medlem av «lag»
    res = run(inst)
    assert not res["group_assignments"]
    assert res["unfilled"][0]["why"]["groups"] == {"lag": "not_proposable:a"}


def test_forklaringen_nevner_dem_som_ikke_kan_foreslaas():
    inst = base(people=("a",), n_dates=1)
    inst["people"][0]["proposable"] = False
    res = run(inst)
    assert res["unfilled"][0]["why"]["not_proposable"] == ["a"] and res["unfilled"][0]["why"]["candidates"] == 0


def test_foreslatt_gruppe_velges_etter_medlemmenes_datoonsker():
    d = dates(4)
    for good, bad in (("a", "b"), ("b", "a")):  # begge retninger, så en tilfeldig uavgjort ikke gir riktig svar
        inst = base(
            people=("a", "b"),
            n_dates=4,
            groups=[{"id": "g1", "members": ["a"]}, {"id": "g2", "members": ["b"]}],
            group_tasks=[{"group": "g1", "task": "t"}, {"group": "g2", "task": "t"}],
            person_dates={bad: {x: "poor" for x in d}, good: {x: "good" for x in d}},
            params=NO_FAIR,
        )
        inst["person_tasks"] = {}
        res = run(inst)
        assert {g["group"] for g in res["group_assignments"]} == {"g1" if good == "a" else "g2"}


def test_historikk_virker_ogsa_i_det_samlede_leddet():
    """Skru av oppgaveleddet, så er det bare det samlede leddet som kan ta hensyn til historikken."""
    inst = base(
        n_dates=6,
        history={"a": {"t": 6}},
        params={"history_weight": 0.5, "w_task_fair": 0, "w_task_range": 0},
    )
    assert count(run(inst), "a") == 0  # a har allerede 3 (halvt av 6) og får sin andel dekket


def test_to_hendelser_samme_dag_er_uavhengige():
    """Tilgjengelighet gjelder hendelsen, ikke kalenderdagen: a kan morgen men ikke kveld, b omvendt."""
    morgen, kveld = "2026-08-09#1", "2026-08-09#2"
    inst = base(people=("a", "b"), n_dates=1)
    inst["dates"] = [{"id": morgen, "type": "g"}, {"id": kveld, "type": "g"}]
    inst["person_dates"] = {"a": {kveld: "never"}, "b": {morgen: "never"}}
    res = run(inst)
    assert who(res, "t", morgen) == ["a"] and who(res, "t", kveld) == ["b"]


def test_kombinasjoner_gjelder_hendelsen_ikke_kalenderdagen():
    """Samme person kan ha Lyd på én hendelse og Bilde på en annen samme dag, selv om de ikke kan kombineres i en hendelse."""
    e1, e2 = "2026-08-09#1", "2026-08-09#2"
    inst = base(people=("a",), n_dates=1, tasks={"lyd": 1, "bilde": 1},
                combinations=[{"a": "lyd", "b": "bilde", "level": "cannot"}], params=NO_FAIR)
    inst["dates"] = [{"id": e1, "type": "g"}, {"id": e2, "type": "g"}]
    inst["person_tasks"]["a"] = {"lyd": "good", "bilde": "good"}
    res = run(inst)
    per_event = {e: [a["task"] for a in res["assignments"] if a["date"] == e] for e in (e1, e2)}
    assert all(len(ts) == 1 for ts in per_event.values())  # aldri begge i samme hendelse
    assert len(res["assignments"]) == 2 and sum(u["missing"] for u in res["unfilled"]) == 2
