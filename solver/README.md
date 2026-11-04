# tjenesteplan-solver (spike)

CP-SAT-planleggeren alene: JSON inn, JSON ut. Ingen app, ingen database. Se `PLAN.md` for hvorfor,
og `tjenesteplan_solver/model.py` (toppen) for inndataformatet.

    export UV_PYTHON_PREFERENCE=only-managed     # mise-shimmen for python er ødelagt her
    mise exec uv@0.13.5 -- uv sync --python 3.12
    mise exec uv@0.13.5 -- uv run pytest -q                                  # 72 tester, ca. 10 s
    mise exec uv@0.13.5 -- uv run python -m tjenesteplan_solver < fixtures/host2026.json > /tmp/ut.json
    mise exec uv@0.13.5 -- uv run python -m tjenesteplan_solver check fixtures/host2026.json /tmp/ut.json

- `check.py` er en **uavhengig** sjekker av harde krav; den deler ikke regelkode med `model.py`.
- `fixtures/make_host2026.py` lager HØST 2026-formen med oppdiktede navn. Ikke legg ekte navn i repoet.
- `params.solver` sender rå CP-SAT-parametre videre (f.eks. `{"linearization_level": 2}`).
- `scripts/timing.py`, `scripts/fairness_experiment.py` og `scripts/look_at_plan.py` er målingene som står i `PLAN.md`.
