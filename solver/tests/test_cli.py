"""Kommandolinjen slik appen bruker den: JSON inn på stdin, JSON ut på stdout."""

import json
import subprocess
import sys

from helpers import base


def run_cli(*args, data):
    r = subprocess.run([sys.executable, "-m", "dutyplan_solver", *args], input=json.dumps(data), capture_output=True, text=True, check=False)
    assert r.returncode == 0, r.stderr
    return json.loads(r.stdout)


def test_cli_med_check_legger_til_brudd_listen():
    data = base(n_dates=3)
    data["params"]["time_limit"] = 5
    res = run_cli("--check", data=data)
    assert res["status"] in ("OPTIMAL", "FEASIBLE") and res["violations"] == []
    assert len(res["assignments"]) == 3


def test_cli_uten_check_har_ingen_brudd_liste():
    data = base(n_dates=2)
    data["params"]["time_limit"] = 5
    assert "violations" not in run_cli(data=data)


def test_cli_defaults_gir_standardvektene():
    r = subprocess.run([sys.executable, "-m", "dutyplan_solver", "--defaults"], capture_output=True, text=True, check=False)
    assert r.returncode == 0, r.stderr
    d = json.loads(r.stdout)
    assert d["w_fair"] == 10 and d["w_task_fair"] == 0 and d["w_task"]["godt"] == 2 and d["default_combination"] == "upraktisk"
