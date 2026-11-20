---
name: run-planner
description: Run the planner on the test data or an input file and see the result cheaply. Use when someone wants to see a plan, check behaviour, measure quality/time or try weights.
---

Everything through mise. The default output is about 6 lines; ask for more only when you need it.

- `mise run plan` : run the test data (20 s) and print a short summary (status, violations, empty places, deviations).
- `mise run plan -- --info` : what the input contains, without reading the JSON file.
- `mise run plan -- --time 8 --seed 2` : faster or another seed. `-- --full` : selections, manual cells and figures per person.
- `mise run plan -- --input file.json` : your own input (a path from the repo root). Format: the top of `solver/dutyplan_solver/model.py`.
- `mise run solve < in.json > out.json` and `mise run check -- in.json out.json` : the planner as a filter, and the independent checker.
- `mise run fixture` : make the test data again after changing `solver/fixtures/make_autumn2026.py`.

Expensive measurements (80-100 s each, only after the model or weights have changed, and update the figures in PLAN.md → «Results from the spike»):
`mise run measure:time` and `mise run measure:fair`.

The result is a proposal within the time limit (FEASIBLE), not proven best. Different seeds give slightly different plans.
