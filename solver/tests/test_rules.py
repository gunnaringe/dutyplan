"""One small test per rule."""

import pytest
from helpers import NO_FAIR, base, count, dates, run, who
from dutyplan_solver import check, solve


def test_never_on_an_event_gives_no_assignment():
    d = dates(4)
    inst = base(person_dates={"a": {d[0]: "never", d[1]: "never", d[2]: "never"}})
    inst["person_tasks"]["a"]["t"] = "good"  # would like to, but it does not suit
    res = run(inst)
    assert all(a["date"] == d[3] for a in res["assignments"] if a["person"] == "a")


def test_not_taking_part_in_the_task_gives_no_assignment():
    inst = base()
    del inst["person_tasks"]["b"]["t"]
    res = run(inst)
    assert count(res, "b") == 0


def test_never_on_a_task_gives_no_assignment():
    inst = base()
    inst["person_tasks"]["c"]["t"] = "never"
    assert count(run(inst), "c") == 0


def test_several_places_give_different_people():
    inst = base(people=("a", "b", "c", "d", "e"), tasks={"cafe": 3})
    res = run(inst)
    for d in dates(4):
        ps = who(res, "cafe", d)
        assert len(ps) == 3 and len(set(ps)) == 3


def test_cannot_be_combined_is_a_hard_rule():
    inst = base(
        n_dates=1,
        tasks={"sound": 1, "video": 1},
        combinations=[{"a": "sound", "b": "video", "level": "cannot"}],
        params=NO_FAIR,
    )
    inst["person_tasks"]["a"] = {"sound": "good", "video": "good"}  # a would like both
    res = run(inst)
    assert sorted(a["person"] for a in res["assignments"] if a["person"] == "a") in ([], ["a"])
    assert who(res, "sound") != who(res, "video")


def test_preferred_gives_the_same_person():
    inst = base(
        n_dates=3,
        tasks={"x": 1, "y": 1},
        combinations=[{"a": "x", "b": "y", "level": "preferred"}],
        params=NO_FAIR,
    )
    res = run(inst)
    for d in dates(3):
        assert who(res, "x", d) == who(res, "y", d)


def test_impractical_is_avoided_when_possible():
    inst = base(n_dates=3, tasks={"x": 1, "y": 1}, people=("a", "b"))
    res = run(inst)  # the default is impractical
    for d in dates(3):
        assert who(res, "x", d) != who(res, "y", d)


def test_a_choir_is_just_a_manual_cell_plus_a_selection():
    """No rule of its own: the admin enters the Choir by hand and sets a selection for Sound that day."""
    d = dates(4)
    inst = base(
        people=("a", "b", "c", "d"),
        tasks={"sound": 1},
        locked=[{"date": d[1], "task": "singing", "label": "Choir"}],
        pools=[{"date": d[1], "task": "sound", "people": ["b", "c"]}],
    )
    inst["tasks"].append({"id": "singing", "planned": False})  # part of the setup, but not distributed
    inst["person_tasks"]["a"]["sound"] = "good"
    res = run(inst)
    assert res["manual"] == [{"date": d[1], "task": "singing", "label": "Choir", "fills": 1}]
    assert who(res, "sound", d[1])[0] in ("b", "c")
    assert not [a for a in res["assignments"] if a["task"] == "singing"]


def test_a_selection_limits_the_choice():
    d = dates(4)
    inst = base(people=("a", "b", "c", "d"), pools=[{"date": d[2], "task": "t", "people": ["c", "d"]}])
    inst["person_tasks"]["a"]["t"] = "good"
    res = run(inst)
    assert who(res, "t", d[2])[0] in ("c", "d")


def test_a_selection_with_a_never_gives_an_empty_place_with_an_explanation():
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


def test_everyone_out_gives_an_empty_place_and_an_explanation():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={p: {d: "never"} for p in "abc"})
    res = run(inst)
    assert res["unfilled"][0]["why"]["event_never"] == ["a", "b", "c"]


def test_a_locked_person_is_kept_and_a_text_counts_as_a_place():
    d = dates(3)
    inst = base(
        tasks={"k": 2},
        n_dates=3,
        locked=[
            {"date": d[0], "task": "k", "person": "c"},
            {"date": d[1], "task": "k", "label": "Own committee"},
        ],
    )
    res = run(inst)
    assert "c" in who(res, "k", d[0]) and len(who(res, "k", d[0])) == 2
    assert len(who(res, "k", d[1])) == 1
    assert res["manual"] == [{"date": d[1], "task": "k", "label": "Own committee", "fills": 1}]


def test_a_locked_person_beats_never_but_gives_a_warning():
    d = dates(2)
    inst = base(n_dates=2, person_dates={"a": {d[0]: "never"}}, locked=[{"date": d[0], "task": "t", "person": "a"}])
    res = solve(inst)
    assert who(res, "t", d[0]) == ["a"]
    assert res["warnings"] and "event_never" in res["warnings"][0]


def test_even_distribution_between_equal_people():
    res = run(base(n_dates=6))
    assert sorted(count(res, p) for p in "abc") == [2, 2, 2]


def test_even_distribution_takes_availability_into_account():
    d = dates(6)
    never_dates = {d[i]: "never" for i in (1, 2, 3, 5)}  # a can make only two of six dates, not in a row
    inst = base(n_dates=6, person_dates={"a": never_dates})
    inst["person_tasks"]["a"]["t"] = "good"  # a is drawn towards assignments, but her share is below one
    res = run(inst)
    assert count(res, "a") <= 1
    assert sorted((count(res, "b"), count(res, "c"))) == [2, 3] or count(res, "a") == 0


def test_equal_total_load_even_if_some_take_only_one_task():
    """«Someone who only has video should not be half of someone who has sound and video»: a takes only video, b both sound and video
    and c only sound. Together that is 12 assignments, so everyone should have 4, and b does not take 3 of each."""
    def inst(**params):
        i = base(n_dates=6, tasks={"video": 1, "sound": 1}, params={"w_combo": {"impractical": 0}, **params})
        i["person_tasks"] = {"a": {"video": "neutral"}, "b": {"video": "neutral", "sound": "neutral"}, "c": {"sound": "neutral"}}
        return i

    res = run(inst())
    assert [count(res, p) for p in "abc"] == [4, 4, 4]
    assert [res["load"][p]["count"] for p in "abc"] == [4, 4, 4]
    # with an equal share of each task (the old weighting) it was 3, 6 and 3: a and c at half of b
    old = run(inst(w_fair=0, w_fair_range=0, w_task_fair=10, w_task_range=10))
    assert [count(old, p) for p in "abc"] == [3, 6, 3]


def test_history_gives_less_to_whoever_has_done_the_most():
    without = run(base(n_dates=6, history={"a": {"t": 3}}, params={"history_weight": 0}))
    assert count(without, "a") == 2  # without history they share equally
    full = run(base(n_dates=6, history={"a": {"t": 3}}, params={"history_weight": 1}))
    assert count(full, "a") == 0  # 3 + 0 against 0 + 3: catches up completely
    half = run(base(n_dates=6, history={"a": {"t": 3}}))  # default: counts half, «a little less»
    assert count(half, "a") <= 1
    assert half["task_load"]["t"]["a"]["history"] == 2


def test_history_on_one_task_does_not_affect_another():
    # Distribution per task is off by default (total load is the goal); history per task only counts when it is on.
    per_task = {"w_task_fair": 10, "w_task_range": 10}
    res = run(base(n_dates=6, tasks={"x": 1, "y": 1}, history={"a": {"x": 4}}, params={"history_weight": 1, **per_task}))
    assert count(res, "a", "x") < count(res, "b", "x")
    assert abs(count(res, "a", "y") - count(res, "b", "y")) <= 1


def test_stability_keeps_the_previous_plan():
    d = dates(6)
    prev = [{"date": d[i], "task": "t", "person": "abc"[i % 3]} for i in range(6)]
    res = run(base(n_dates=6, previous_plan=prev))
    got = {(a["date"], a["person"]) for a in res["assignments"]}
    assert got == {(p["date"], p["person"]) for p in prev}


def test_an_own_calendar_replaces_the_shared_one():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={"a": {d: "never"}}, person_task_calendars={"a": {"t": {d: "good"}}})
    inst["person_tasks"]["a"]["t"] = "good"
    assert who(run(inst), "t", d) == ["a"]


def test_poor_is_used_only_when_needed_and_reported():
    d = dates(1)[0]
    inst = base(n_dates=1, people=("a", "b"), person_dates={"a": {d: "poor"}})
    assert who(run(inst), "t", d) == ["b"]
    inst["person_dates"]["b"] = {d: "never"}
    res = run(inst)
    assert who(res, "t", d) == ["a"]
    assert res["concessions"] and res["concessions"][0]["person"] == "a"


def test_the_checker_finds_violations():
    d = dates(1)[0]
    inst = base(n_dates=1, person_dates={"a": {d: "never"}})
    res = run(inst)
    res["assignments"][0]["person"] = "a"  # tamper with the plan
    assert any("never" in p for p in check(inst, res))


# --- general group support: no special rules, only groups + the shared combination setting ----------


def group_inst(level=None, **extra):
    """Group «g» = {a}, set up on «singing» on the first date (the task is not distributed). a would like Sound."""
    d = dates(3)
    inst = base(
        n_dates=3,
        tasks={"sound": 1},
        groups=[{"id": "g", "members": ["a"]}],
        locked=[{"date": d[0], "task": "singing", "group": "g"}],
        combinations=[{"a": "singing", "b": "sound", "level": level}] if level else [],
        **extra,
    )
    inst["tasks"].append({"id": "singing", "planned": False})
    inst["person_tasks"]["a"]["sound"] = "good"
    return inst, d


def test_a_group_makes_its_members_busy_through_the_combination_setting():
    inst, d = group_inst("cannot")
    res = run(inst)  # run() also checks the group requirement in the independent checker
    assert "a" not in who(res, date=d[0])
    assert res["manual"] == [{"date": d[0], "task": "singing", "group": "g", "fills": 1}]


def test_a_group_without_a_hard_rule_is_only_a_penalty_and_can_be_picked_if_needed():
    inst, d = group_inst()  # the pair is not set: the default is impractical
    for other in "bc":
        inst.setdefault("person_dates", {})[other] = {d[0]: "never"}
    assert who(run(inst), "sound", d[0]) == ["a"]  # nobody else can, and impractical is not forbidden


def test_a_group_with_preferred_gives_the_member_the_task():
    inst, d = group_inst("preferred", params=NO_FAIR)
    inst["person_tasks"]["a"]["sound"] = "neutral"  # without the reward b and c are better
    inst["person_tasks"]["b"]["sound"] = inst["person_tasks"]["c"]["sound"] = "good"
    assert who(run(inst), "sound", d[0]) == ["a"]
    inst["combinations"] = []  # with no setting (impractical) b or c is picked
    assert who(run(inst), "sound", d[0])[0] in ("b", "c")


def test_a_group_member_cannot_take_an_extra_place_on_the_same_task():
    d = dates(1)[0]
    inst = base(
        people=("a", "b", "c", "d"),
        n_dates=1,
        tasks={"cafe": 3},
        groups=[{"id": "team", "members": ["a"]}],
        locked=[{"date": d, "task": "cafe", "group": "team"}],  # the team fills one of three places
        params={**NO_FAIR, "w_combo": {"impractical": 0}},  # so that a penalty cannot hide the difference
    )
    inst["person_tasks"]["a"]["cafe"] = "good"  # a would like to, but is already in through the team
    res = run(inst)
    assert "a" not in who(res, "cafe", d) and len(who(res, "cafe", d)) == 2


def test_a_group_can_fill_several_places():
    d = dates(2)
    inst = base(
        people=("a", "b", "c", "d", "e"),
        n_dates=2,
        tasks={"cafe": 3},
        groups=[{"id": "team", "members": ["a", "b", "c"]}],
        locked=[{"date": d[0], "task": "cafe", "group": "team", "fills": 3}],
    )
    res = run(inst)
    assert who(res, "cafe", d[0]) == [] and not res["unfilled"]  # the group fills all three
    assert len(who(res, "cafe", d[1])) == 3


def test_a_member_with_never_gives_a_warning_and_the_manual_choice_is_kept():
    inst, d = group_inst("cannot")
    inst.setdefault("person_dates", {})["a"] = {d[0]: "never"}
    res = solve(inst)
    assert any("«never»" in w for w in res["warnings"])
    assert res["manual"]


def test_an_unknown_group_or_member_is_an_error():
    inst, d = group_inst()
    inst["locked"][0]["group"] = "does_not_exist"
    with pytest.raises(ValueError):
        solve(inst)
    inst, d = group_inst()
    inst["groups"][0]["members"] = ["unknown"]
    with pytest.raises(ValueError):
        solve(inst)


def test_the_explanation_names_the_group_when_members_are_busy():
    d = dates(1)[0]
    inst = base(
        people=("a",),
        n_dates=1,
        groups=[{"id": "g", "members": ["a"]}],
        locked=[{"date": d, "task": "singing", "group": "g"}],
        combinations=[{"a": "singing", "b": "t", "level": "cannot"}],
    )
    inst["tasks"].append({"id": "singing", "planned": False})
    res = run(inst)
    assert res["unfilled"][0]["why"]["busy"] == {"a": ["singing (g)"]}
    assert res["unfilled"][0]["why"]["candidates"] == 0


# --- the planner can propose groups -----------------------------------------------------------------


def team_inst(**extra):
    """a and b are in only through the team «team»; nobody takes part in the task personally."""
    inst = base(
        people=("a", "b", "c"),
        n_dates=4,
        groups=[{"id": "team", "members": ["a", "b"]}],
        group_tasks=[{"group": "team", "task": "t"}],
        **extra,
    )
    inst["person_tasks"] = {}
    return inst


def test_the_planner_can_propose_a_group():
    res = run(team_inst())
    assert len(res["group_assignments"]) == 4 and not res["assignments"] and not res["unfilled"]
    assert all(a["members"] == ["a", "b"] for a in res["group_assignments"])


def test_one_members_never_rules_the_group_out_and_is_explained():
    d = dates(4)
    inst = team_inst(person_dates={"a": {d[1]: "never"}})
    res = run(inst)
    assert {a["date"] for a in res["group_assignments"]} == {d[0], d[2], d[3]}
    (u,) = res["unfilled"]
    assert u["date"] == d[1] and u["why"]["groups"] == {"team": "event_never:a"}


def test_a_group_that_needs_more_places_than_there_are_cannot_be_picked():
    inst = team_inst()
    inst["group_tasks"][0]["fills"] = 2  # the task has only one place
    res = run(inst)
    assert not res["group_assignments"] and len(res["unfilled"]) == 4
    assert res["unfilled"][0]["why"]["groups"] == {"team": "too_many_places"}


def test_a_proposed_group_can_fill_all_the_places_on_a_day():
    d = dates(2)
    inst = base(
        people=("a", "b"),
        n_dates=2,
        tasks={"prayer": 2},
        groups=[{"id": "pair", "members": ["a", "b"]}],
        group_tasks=[{"group": "pair", "task": "prayer", "fills": 2}],
    )
    inst["person_tasks"] = {}
    res = run(inst)
    assert len(res["group_assignments"]) == 2 and not res["unfilled"]


def test_a_proposed_group_makes_members_busy_through_the_combinations():
    inst = base(
        people=("a", "b"),
        n_dates=1,
        tasks={"t": 1, "u": 1},
        groups=[{"id": "team", "members": ["a"]}],
        group_tasks=[{"group": "team", "task": "t", "level": "good"}],
        combinations=[{"a": "t", "b": "u", "level": "cannot"}],
    )
    inst["person_tasks"] = {"a": {"u": "good"}, "b": {"u": "neutral"}}  # a would like u, but is in the team on t
    res = run(inst)
    assert [g["group"] for g in res["group_assignments"]] == ["team"]
    assert who(res, "u") == ["b"]


def test_two_groups_with_a_common_member_cannot_both_be_picked_on_the_same_task():
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


def test_a_selection_can_name_groups_and_a_selection_without_groups_rules_them_out():
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


def test_group_work_counts_one_assignment_per_member_for_manual_cells():
    d = dates(6)

    def run_with(manual):
        inst = base(n_dates=6, groups=[{"id": "team", "members": ["a"]}],
                    locked=[{"date": d[i], "task": "t", "group": "team"} for i in range(manual)])
        return run(inst)

    without = run_with(0)
    assert (count(without, "a"), without["load"]["a"]["count"]) == (2, 2)  # without group work they share equally
    with_cells = run_with(2)  # the team (only a) fills t twice by hand
    assert count(with_cells, "a") == 0  # the two cells are a's share: one assignment each
    assert with_cells["load"]["a"]["count"] == 2 and with_cells["task_load"]["t"]["a"]["count"] == 2
    # the manual group work is part of the target per task (6 assignments, a can make four of six dates: 6 * 4/16), also when distribution per task is off
    assert with_cells["task_load"]["t"]["a"]["target"] == 1.5
    assert sorted(count(with_cells, p) for p in "bc") == [2, 2]
    assert not with_cells["unfilled"]


def test_assignments_count_one_per_member_for_proposed_groups():
    inst = base(
        people=("a", "b", "c"),
        n_dates=4,
        groups=[{"id": "team", "members": ["a", "b"]}],
        group_tasks=[{"group": "team", "task": "t", "level": "good"}],
    )
    res = run(inst)
    assert res["group_assignments"]
    for p in "abc":
        via = sum(1 for g in res["group_assignments"] if p in g["members"])
        assert res["load"][p]["count"] == count(res, p) + via
        assert res["task_load"]["t"][p]["count"] == count(res, p, "t") + via


def test_a_proposed_group_counts_in_the_distribution_so_that_it_does_not_take_everything():
    """a and b are in only through the team, which everyone likes (good). Since the members count one assignment each time,
    the team has to share the dates with c; otherwise the team would have taken all six."""
    inst = base(
        people=("a", "b", "c"),
        n_dates=6,
        groups=[{"id": "team", "members": ["a", "b"]}],
        group_tasks=[{"group": "team", "task": "t", "level": "good"}],
    )
    inst["person_tasks"] = {"c": {"t": "neutral"}}
    res = run(inst)
    assert 1 <= len(res["group_assignments"]) <= 3
    assert count(res, "c") >= 3


def test_manual_group_work_also_counts_in_the_overall_term():
    """Group work on another task (singing) affects only the overall term; switch off the per-task term to see it."""
    d = dates(6)
    params = {"w_combo": {"impractical": 0}, "w_task_fair": 0, "w_task_range": 0}

    def planned_for_a(manual):
        inst = base(n_dates=6, groups=[{"id": "team", "members": ["a"]}], params=params,
                    locked=[{"date": d[i], "task": "singing", "group": "team"} for i in range(manual)])
        inst["tasks"].append({"id": "singing", "planned": False})
        return count(run(inst), "a")

    assert planned_for_a(0) == 2
    assert planned_for_a(2) <= 1


def test_stability_also_applies_to_proposed_groups():
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


# --- «can be proposed»: a setting per person and per group --------------------------------------------


def test_a_person_who_cannot_be_proposed_is_never_picked_but_can_be_locked():
    d = dates(4)
    inst = base(n_dates=4, locked=[{"date": d[0], "task": "t", "person": "a"}])
    inst["people"][0]["proposable"] = False
    inst["person_tasks"]["a"]["t"] = "good"  # everyone else would be better without the setting
    res = run(inst)
    assert who(res, "t", d[0]) == ["a"] and count(res, "a") == 1  # only the manual choice


def test_a_group_that_cannot_be_proposed_is_never_picked_but_can_be_in_a_manual_cell():
    d = dates(4)
    inst = team_inst(locked=[{"date": d[0], "task": "t", "group": "team"}])
    inst["groups"][0]["proposable"] = False
    inst["group_tasks"][0]["level"] = "good"
    res = run(inst)
    assert not res["group_assignments"]
    assert res["manual"] == [{"date": d[0], "task": "t", "group": "team", "fills": 1}]
    assert res["unfilled"][0]["why"]["groups"] == {"team": "not_proposable"}


def test_a_group_with_a_member_who_cannot_be_proposed_is_ruled_out():
    inst = team_inst()
    inst["people"][0]["proposable"] = False  # a is a member of «team»
    res = run(inst)
    assert not res["group_assignments"]
    assert res["unfilled"][0]["why"]["groups"] == {"team": "not_proposable:a"}


def test_the_explanation_names_those_who_cannot_be_proposed():
    inst = base(people=("a",), n_dates=1)
    inst["people"][0]["proposable"] = False
    res = run(inst)
    assert res["unfilled"][0]["why"]["not_proposable"] == ["a"] and res["unfilled"][0]["why"]["candidates"] == 0


def test_a_proposed_group_is_picked_by_its_members_date_wishes():
    d = dates(4)
    for good, bad in (("a", "b"), ("b", "a")):  # both directions, so that a random tie does not give the right answer
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


def test_history_also_works_in_the_overall_term():
    """Switch off the per-task term, and only the overall term can take the history into account."""
    inst = base(
        n_dates=6,
        history={"a": {"t": 6}},
        params={"history_weight": 0.5, "w_task_fair": 0, "w_task_range": 0},
    )
    assert count(run(inst), "a") == 0  # a already has 3 (half of 6) and gets her share covered


def test_two_events_on_the_same_day_are_independent():
    """Availability applies to the event, not the calendar day: a can make the morning but not the evening, b the other way round."""
    morning, evening = "2026-08-09#1", "2026-08-09#2"
    inst = base(people=("a", "b"), n_dates=1)
    inst["dates"] = [{"id": morning, "type": "g"}, {"id": evening, "type": "g"}]
    inst["person_dates"] = {"a": {evening: "never"}, "b": {morning: "never"}}
    res = run(inst)
    assert who(res, "t", morning) == ["a"] and who(res, "t", evening) == ["b"]


def test_combinations_apply_to_the_event_not_the_calendar_day():
    """The same person can have Sound at one event and Video at another the same day, although they cannot be combined within one event."""
    e1, e2 = "2026-08-09#1", "2026-08-09#2"
    inst = base(people=("a",), n_dates=1, tasks={"sound": 1, "video": 1},
                combinations=[{"a": "sound", "b": "video", "level": "cannot"}], params=NO_FAIR)
    inst["dates"] = [{"id": e1, "type": "g"}, {"id": e2, "type": "g"}]
    inst["person_tasks"]["a"] = {"sound": "good", "video": "good"}
    res = run(inst)
    per_event = {e: [a["task"] for a in res["assignments"] if a["date"] == e] for e in (e1, e2)}
    assert all(len(ts) == 1 for ts in per_event.values())  # never both in the same event
    assert len(res["assignments"]) == 2 and sum(u["missing"] for u in res["unfilled"]) == 2
