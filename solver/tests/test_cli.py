"""The command line the way the app uses it: JSON in on stdin, JSON out on stdout."""

import json
import subprocess
import sys

from helpers import base


def run_cli(*args, data):
    r = subprocess.run([sys.executable, "-m", "dutyplan_solver", *args], input=json.dumps(data), capture_output=True, text=True, check=False)
    assert r.returncode == 0, r.stderr
    return json.loads(r.stdout)


def test_cli_with_check_adds_the_violations_list():
    data = base(n_dates=3)
    data["params"]["time_limit"] = 5
    res = run_cli("--check", data=data)
    assert res["status"] in ("OPTIMAL", "FEASIBLE") and res["violations"] == []
    assert len(res["assignments"]) == 3


def test_cli_without_check_has_no_violations_list():
    data = base(n_dates=2)
    data["params"]["time_limit"] = 5
    assert "violations" not in run_cli(data=data)


def test_cli_defaults_gives_the_default_weights():
    r = subprocess.run([sys.executable, "-m", "dutyplan_solver", "--defaults"], capture_output=True, text=True, check=False)
    assert r.returncode == 0, r.stderr
    d = json.loads(r.stdout)
    assert d["w_fair"] == 10 and d["w_task_fair"] == 0 and d["w_task"]["good"] == 2 and d["default_combination"] == "impractical"
