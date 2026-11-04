# tjenesteplan

Fordeler oppgaver på datoer med harde krav, preferanser og lik fordeling. Generelt verktøy (ikke bare kirke).
Status: bare planleggeren alene (`solver/`) finnes; appen (Go + SQLite) er ikke startet. Lokalt repo, ikke på GitHub.

## Kjør billig (les dette først)

- **Alt går via mise**: `mise install` (python, uv), `mise run setup` (pakker), så `mise tasks`. Aldri python/uv/pytest direkte.
  Skills: `verify` (tester + mutasjoner) og `run-planner` (kjør/mål). Oppgavene er stille og gir ca. 1-8 linjer.
- Vanlig løkke: endre → `mise run test:fast` (2 s) → `mise run test` (10 s) → ved regelendring `mise run mutations -- --only <ord>`
  → før commit `mise run verify`.
- **Ikke les** `PLAN.md` i sin helhet (ca. 200 linjer). `grep -n '^## ' PLAN.md`, les bare seksjonen du trenger.
  Ikke les `solver/fixtures/host2026.json` eller `solver/uv.lock` (store, genererte); bruk `mise run plan -- --info`.
- Ikke be om `--full` eller `measure:*` uten grunn; de er dyre.
- Filstier til `plan`, `solve` og `check` er relative til repoets rot. Nye Python-pakker: `mise run add -- pakke` (og `mise run lock`);
  `UV_LOCKED=1` sperrer ellers endring av `uv.lock`.

## Designregler fra brukeren (ikke bryt uten å spørre)

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
