# dutyplan-solver

The CP-SAT planner on its own: JSON in, JSON out. No app, no database. See `../PLAN.md` for why, and the top of
`dutyplan_solver/model.py` for the input format. Everything runs through mise from the repo root:

    mise install && mise run setup      # python + uv + the packages (first time)
    mise tasks                          # all tasks
    mise run test                       # all tests, quiet
    mise run plan                       # plan the test data, short summary
    mise run solve < in.json > out.json
    mise run check -- in.json out.json  # independent check of the hard requirements
    mise run verify                     # tests + mutation check
    mise run add -- package             # a new Python package (updates uv.lock); paths above are relative to the repo root

- `check.py` is an **independent** checker of the hard requirements; it shares no rule code with `model.py`.
- `scripts/mutations.py` breaks every rule in the model and requires a test to fail.
- `fixtures/make_autumn2026.py` makes the test data (invented names; do not put real names in the repo).
- `params.solver` passes raw CP-SAT parameters on (for example `{"linearization_level": 2}`).
