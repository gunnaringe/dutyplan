---
name: verify
description: Verifiser endringer (Go-appen og Python-planleggeren) billigst mulig med mise-oppgavene. Bruk etter å ha endret kode eller tester, og før commit.
---

Kjør bare via mise (aldri python/uv/pytest direkte). Alle oppgaver er stille: én linje ved suksess.

1. Mens du jobber: `mise run test:fast` (ca. 5 s: Go-testene + Python-regeltestene). Bare Go: `mise run test:app -- -run Navn`.
2. Før du er ferdig: `mise run test` (ca. 15 s, alt).
3. Endret eller la du til en **regel**: `mise run mutations -- --only <ord fra navnet>` (sekunder), og til slutt
   `mise run mutations` (ca. 30 s). Stille ved suksess; skriver bare OVERLEVDE / UTDATERT / UGYLDIG.
4. Før commit: `mise run verify` (tester + alle mutasjoner + røyktest av den bygde appen).
5. Endret du frontenden (`web/`): `mise run app:ui` (headless nettleser, ca. 10 s; feiler også om en side blir bredere enn en mobilskjerm). Hopp over den for endringer som ikke er synlige.

Overlevde en mutasjon, mangler en test som avhenger av regelen. Skriv den slik at uavhengige ledd ikke dekker for den:
skru av andre ledd med parametre (`NO_FAIR` i tests/helpers.py, `w_combo: {upraktisk: 0}`), og test begge retninger
når utfallet ellers kan bli riktig ved uavgjort.

Ny planleggerregel = fire steder: `model.py`, den uavhengige `check.py`, en test, og en linje i `PY_MUTATIONS` i
`scripts/mutations.py`. Ny Go-regel (særlig organisasjonsavgrensning): test + en linje i `GO_MUTATIONS`; hvert oppslag skal avgrenses av organisasjonen. Oppdater `PLAN.md` bare i den seksjonen det gjelder.

Ikke les `fixtures/host2026.json` eller `uv.lock` (stor, generert). Bruk `mise run plan -- --info`.
