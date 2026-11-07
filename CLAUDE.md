# tjenesteplan

Fordeler oppgaver på datoer med harde krav, preferanser og lik fordeling. Generelt verktøy (ikke bare kirke).
Lokalt repo, ikke på GitHub. Status: planleggeren (`solver/`, Python) virker alene; appens første del (Go + SQLite + vanilla JS)
har organisasjoner, planer, personer og endringslogg. Oppsett av oppgaver/datoer, preferanser og kobling til planleggeren er neste faser.

Oppbygging: `cmd/tjenesteplan` (main), `internal/store` (SQLite, migreringer, all tilgang), `internal/api` (JSON-API + serverer frontenden),
`web/` (index.html, app.js, style.css, innebygd i binæren), `solver/` (Python/CP-SAT), `scripts/` (røyktest, nettlesersjekk).

## Kjør billig (les dette først)

- **Alt går via mise**: `mise install` (python, uv, go), `mise run setup` (Python-pakker), så `mise tasks`. Aldri python/uv/pytest/go direkte.
  Skills: `verify` (tester + mutasjoner), `run-planner` (planleggeren) og `run-app` (appen). Oppgavene er stille og gir ca. 1-8 linjer.
- Vanlig løkke: endre → `mise run test:fast` (Go + Python-regler, ca. 5 s) → `mise run test` (alt, ca. 15 s) → ved regelendring
  `mise run mutations -- --only <ord>` → før commit `mise run verify` (tester + alle mutasjoner + røyktest av appen).
- **Ikke les** `PLAN.md` i sin helhet (ca. 200 linjer). `grep -n '^## ' PLAN.md`, les bare seksjonen du trenger.
  Ikke les `solver/fixtures/host2026.json` eller `solver/uv.lock` (store, genererte); bruk `mise run plan -- --info`.
- Ikke be om `--full`, `measure:*` eller `app:ui` uten grunn; de er dyre eller trenger sibling-avhengigheter.
- Filstier til `plan`, `solve` og `check` er relative til repoets rot. Nye Python-pakker: `mise run add -- pakke` (og `mise run lock`);
  `UV_LOCKED=1` sperrer ellers endring av `uv.lock`.

## Designregler fra brukeren (ikke bryt uten å spørre)

- **Organisasjon → planer.** Alt tilhører en organisasjon; en organisasjon har flere planer (f.eks. «Høst 2026»). Organisasjonen ligger i adressen
  (`/o/<org>/p/<plan>`, `/api/orgs/{org}/...`), tjeneren har ingen «gjeldende organisasjon». Hvert oppslag avgrenses av organisasjonen, og
  en plan/person fra en annen organisasjon gir 404 (isolasjonstestene og `tenant:`-mutasjonene vokter dette). Ingen innlogging.
- **Alle skrivinger går gjennom `store.write`** (krever aktør i `X-Actor`, logger i samme transaksjon). Ikke skriv til databasen utenom den.
- **Generelt verktøy.** Oppgaver, personer, grupper, datoer og dagtyper er data admin lager. Ingen kirkebegreper i kode, datamodell eller regler.
- **Ingen særregler for enkeltting.** En **gruppe** er et navngitt sett personer. Den kan stå i en manuell rute eller foreslås av planleggeren
  (`group_tasks`); medlemmene er da opptatt, og den felles kombinasjonsinnstillingen (oppgave × oppgave) avgjør hva de kan ha samme dag.
  Et **utvalg** (bare disse personene/gruppene kan velges) dekker «egen lydtekniker» og liknende. Ingen kompetanser eller betingede krav.
- **Kan foreslås** er en innstilling per person og per gruppe (standard ja). Nei = planleggeren velger dem aldri; manuelle ruter går an.
- **Gruppearbeid = ett oppdrag per medlem** i fordelingen (samlet, per oppgave, historikk).
- **Tatt ut etter ønske:** maks oppgaver per person per dag, og hvile mellom datoer. Ikke legg dem tilbake uten å spørre.
- **Ingen innlogging i første versjon.** Ingen lenker med hemmelighet, ingen roller. Endringslogg i stedet.
- Hver person velger selv hvilke oppgaver hen er med på; én dato-kalender per person (*passer ikke / dårlig / godt*), valgfri egen per oppgave.
- «Lik fordeling» = per oppgave (hovedleddet); samlet antall er et svakere vern. En avveining brukeren eier.

## Testing og data

- `check.py` er en uavhengig sjekker; hold den uavhengig av `model.py` (bare standardverdiene deles).
- Hver regel skal ha en test som feiler når regelen brytes i modellen: `scripts/mutations.py` (`MUTATIONS`) kontrollerer det.
- Ingen ekte navn, e-post eller telefon i repoet. Testdataene har oppdiktede navn og lages av `mise run fixture`.
- Nivåer på ledningen er ASCII: `ikke|darlig|noytral|godt`, `kan_ikke|upraktisk|praktisk|bor`.
- Ikke gi et skript navn som kolliderer med standardbiblioteket (`inspect.py` ødela numpy-importen).
- Ikke kjør `mise lock` her: `lockfile` i denne konfigurasjonen skrev en `mise.lock` i `~/code` (overordnet konfig). Versjonene er pinnet i `mise.toml`.
