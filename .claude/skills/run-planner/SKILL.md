---
name: run-planner
description: Kjør planleggeren på testdataene eller en inndatafil og se resultatet billig. Bruk når noen vil se en plan, sjekke atferd, måle kvalitet/tid eller teste vekter.
---

Alt via mise. Standardutdata er ca. 6 linjer; be bare om mer når du trenger det.

- `mise run plan` : kjør testdataene (20 s) og skriv en kort oppsummering (status, brudd, tomme plasser, avvik, grupper).
- `mise run plan -- --info` : hva inndataene inneholder, uten å lese JSON-filen.
- `mise run plan -- --time 8 --seed 2` : raskere eller annet frø. `-- --full` : utvalg, manuelle ruter og tall per person.
- `mise run plan -- --input fil.json` : egen inndata (sti fra repoets rot). Format: toppen av `solver/tjenesteplan_solver/model.py`.
- `mise run solve < inn.json > ut.json` og `mise run check -- inn.json ut.json` : planleggeren som filter, og den uavhengige sjekkeren.
- `mise run fixture` : lag testdataene på nytt etter å ha endret `solver/fixtures/make_host2026.py`.

Dyre målinger (80-100 s hver, bare etter at modell eller vekter er endret, og oppdater tallene i PLAN.md → «Resultater fra spiken»):
`mise run measure:time` og `mise run measure:fair`.

Resultatet er et forslag innenfor tidsgrensen (FEASIBLE), ikke bevist best. Ulike frø gir litt ulike planer.
