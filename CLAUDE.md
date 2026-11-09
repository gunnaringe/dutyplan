# tjenesteplan

Fordeler oppgaver på hendelser med harde krav, preferanser og lik fordeling. Generelt verktøy (ikke bare kirke).
Lokalt repo, ikke på GitHub. Status: planleggeren (`solver/`, Python) er koblet til appen (Go + SQLite + vanilla JS): organisasjoner, planer,
personer, endringslogg, oppsett (oppgaver, dagtyper med plasser, kombinasjoner, hendelser, grupper), «Min side», «Planlegg» på planen, manuelle ruter,
utvalg og «Lås» (fase 4 omgang 1-2). Rutenett og «mine oppdrag» (fase 5) er bygget. Utrulling på kate (fase 6) er utsatt etter brukerens ønske (2026-11-09): ikke start på den uten å bli bedt om det. Publisering er ikke bygget (siste kjøring er planen).

Oppbygging: `cmd/tjenesteplan` (main), `internal/store` (SQLite, migreringer, all tilgang), `internal/planner` (bygger inndata fra databasen,
starter Python-planleggeren, gjør svaret om til en visning med forklaringer), `internal/api` (JSON-API + serverer frontenden),
`web/` (index.html, app.js, style.css, innebygd i binæren), `solver/` (Python/CP-SAT), `scripts/` (røyktest, nettlesersjekk).

## Kjør billig (les dette først)

- **Alt går via mise**: `mise install` (python, uv, go), `mise run setup` (Python-pakker), så `mise tasks`. Aldri python/uv/pytest/go direkte.
  Skills: `verify` (tester + mutasjoner), `run-planner` (planleggeren) og `run-app` (appen). Oppgavene er stille og gir ca. 1-8 linjer.
- Vanlig løkke: endre → `mise run test:fast` (Go + Python-regler, ca. 5 s) → `mise run test` (alt, ca. 15 s) → ved regelendring
  `mise run mutations -- --only <ord>` (hele settet ca. 3 min) → før commit `mise run verify` (tester + alle mutasjoner + røyktest av appen).
- **Ikke les** `PLAN.md` i sin helhet (ca. 200 linjer). `grep -n '^## ' PLAN.md`, les bare seksjonen du trenger.
  Ikke les `solver/fixtures/host2026.json` eller `solver/uv.lock` (store, genererte); bruk `mise run plan -- --info`.
- Ikke be om `--full`, `measure:*` eller `app:ui` uten grunn; de er dyre eller trenger sibling-avhengigheter.
- Appen finner planleggeren via `TJENESTEPLAN_SOLVER` (satt i `mise.toml`, krever `mise run setup`); uten den hoppes de to testene mot den ekte
  løseren over og «Planlegg» er avslått. Mutasjonene kjører Go-testene uten den, så regler må fanges av testene med falsk løser.
- Filstier til `plan`, `solve` og `check` er relative til repoets rot. Nye Python-pakker: `mise run add -- pakke` (og `mise run lock`);
  `UV_LOCKED=1` sperrer ellers endring av `uv.lock`.

## Designregler fra brukeren (ikke bryt uten å spørre)

- **Organisasjon → planer.** Alt tilhører en organisasjon; en organisasjon har flere planer (f.eks. «Høst 2026»). Organisasjonen ligger i adressen
  (`/o/<org>/p/<plan>`, `/api/orgs/{org}/...`), tjeneren har ingen «gjeldende organisasjon». Hvert oppslag avgrenses av organisasjonen, og
  en plan/person fra en annen organisasjon gir 404 (isolasjonstestene og `tenant:`-mutasjonene vokter dette). Ingen innlogging.
- **Alle skrivinger går gjennom `store.write`** (krever aktør i `X-Actor`, logger i samme transaksjon). Ikke skriv til databasen utenom den.
- **Hendelsen er enheten, ikke datoen.** Hendelser er selvstendige: to kan ligge på samme dag. Tilgjengelighet («kan jeg»), kombinasjoner, plasser
  og gruppe-opptatthet gjelder hendelsen. Planleggerens «dates» er hendelser (id er vilkårlig tekst).
- **Ingen regnearkstøtte** (verken import eller eksport). Brukeren vil ikke ha det; ikke foreslå det på nytt.
- **Ingen kalenderfeed** (`.ics`, abonnement eller nedlasting; droppet 2026-11-09). Ikke foreslå det på nytt uten at publisering og klokkeslett på hendelser finnes.
- **Generelt verktøy.** Oppgaver, personer, grupper, hendelser og dagtyper er data admin lager. Ingen kirkebegreper i kode, datamodell eller regler.
- **Ingen særregler for enkeltting.** En **gruppe** er et navngitt sett personer. Den kan stå i en manuell rute eller foreslås av planleggeren
  (`group_tasks`); medlemmene er da opptatt, og den felles kombinasjonsinnstillingen (oppgave × oppgave) avgjør hva de kan ha samme dag.
  Et **utvalg** (bare disse personene/gruppene kan velges) dekker «egen lydtekniker» og liknende. Ingen kompetanser eller betingede krav.
- **Kan foreslås** er en innstilling per person og per gruppe (standard ja). Nei = planleggeren velger dem aldri; manuelle ruter går an.
- **Gruppearbeid = ett oppdrag per medlem** i fordelingen (samlet og per oppgave).
- **Optimaliser kun innenfor én plan** (2026-11-09). Ingen historikk fra tidligere planer: appen sender aldri `history` eller `previous_plan` til planleggeren.
- **Tatt ut etter ønske:** maks oppgaver per person per dag, og hvile mellom hendelser. Ikke legg dem tilbake uten å spørre.
- **Ingen innlogging i første versjon.** Ingen lenker med hemmelighet, ingen roller. Endringslogg i stedet.
- Hver person velger selv hvilke oppgaver hen er med på; én liste per person med svar per hendelse (*passer ikke / dårlig / godt*), valgfri egen liste per oppgave.
- «Lik fordeling» = **lik total belastning** (brukerens svar 2026-11-09): en som bare tar Bilde skal ikke ende på halvparten av en som tar Lyd og Bilde.
  Fordeling per oppgave er av som standard (`w_task_fair`/`w_task_range` 0). Ikke gjør oppgaveleddet til hovedledd igjen uten å spørre.
- **Vektene er konfigurasjon per plan** (2026-11-09): `store.WeightSpecs` (standard og grenser), lagret som det som avviker fra standard, og med i hashen. Standardene skal være kopier av
  planleggerens (`--defaults`; en test sjekker det). Ny vekt = en linje i `WeightSpecs`, en tekst i `WEIGHT_LABELS` i `web/app.js`.

## Testing og data

- `check.py` er en uavhengig sjekker; hold den uavhengig av `model.py` (bare standardverdiene deles).
- Hver regel skal ha en test som feiler når regelen brytes i modellen: `scripts/mutations.py` (`MUTATIONS`) kontrollerer det.
- Ingen ekte navn, e-post eller telefon i repoet. Testdataene har oppdiktede navn og lages av `mise run fixture`.
- Nivåer på ledningen er ASCII: `ikke|darlig|noytral|godt`, `kan_ikke|upraktisk|praktisk|bor`.
- Ikke gi et skript navn som kolliderer med standardbiblioteket (`inspect.py` ødela numpy-importen).
- Ikke kjør `mise lock` her: `lockfile` i denne konfigurasjonen skrev en `mise.lock` i `~/code` (overordnet konfig). Versjonene er pinnet i `mise.toml`.
