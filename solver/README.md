# dutyplan-solver (spike)

CP-SAT-planleggeren alene: JSON inn, JSON ut. Ingen app, ingen database. Se `../PLAN.md` for hvorfor,
og toppen av `dutyplan_solver/model.py` for inndataformatet. Alt kjøres via mise fra repoets rot:

    mise install && mise run setup      # python + uv + pakkene (første gang)
    mise tasks                          # alle oppgaver
    mise run test                       # alle tester, stille
    mise run plan                       # planlegg testdataene, kort oppsummering
    mise run solve < inn.json > ut.json
    mise run check -- inn.json ut.json  # uavhengig sjekk av harde krav
    mise run verify                     # tester + mutasjonssjekk
    mise run add -- pakke               # ny Python-pakke (oppdaterer uv.lock); stier over er relative til repoets rot

- `check.py` er en **uavhengig** sjekker av harde krav; den deler ikke regelkode med `model.py`.
- `scripts/mutations.py` bryter hver regel i modellen og krever at en test feiler.
- `fixtures/make_host2026.py` lager testdataene (oppdiktede navn; ikke legg ekte navn i repoet).
- `params.solver` sender rå CP-SAT-parametre videre (f.eks. `{"linearization_level": 2}`).
