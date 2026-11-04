# tjenesteplan

Fordeler oppgaver (lyd, bilde, møteleder, …) på datoer i et semester, med harde krav, preferanser og lik fordeling.
**`PLAN.md` er fasit** for hva som skal bygges og hvorfor; les den først. Status: bare planleggeren alene (`solver/`)
finnes. Appen (Go + SQLite) er ikke startet. Lokalt repo, ikke på GitHub ennå.

## Designregler fra brukeren (ikke bryt uten å spørre)

- **Generelt verktøy, ikke kirke.** Oppgaver, personer, grupper, datoer og dagtyper er data admin lager selv. Ingen
  navn eller begreper fra kirken i kode, datamodell eller regler; de finnes bare i testdataene og som eksempler.
- **Ingen særregler for enkeltting.** Kirkekoret er ikke et begrep i planleggeren. En **gruppe** er bare et navngitt sett
  personer. Den kan stå i en manuell rute eller foreslås av planleggeren (`group_tasks`); i begge tilfeller er medlemmene
  opptatt, og den felles kombinasjonsinnstillingen (oppgave × oppgave) avgjør hva de kan ha samme dag. Et **utvalg**
  (bare disse personene/gruppene kan velges) dekker «egen lydtekniker» og liknende. Ingen kompetanser eller betingede krav.
- **Gruppearbeid = ett oppdrag per medlem** i fordelingen (samlet, per oppgave, historikk). Brukerens valg 2026-11-04.
- **Tatt ut etter ønske:** maks antall oppgaver per person per dag, og hvile mellom datoer. Ikke legg dem tilbake uten å spørre.
- **Ingen innlogging i første versjon.** Ingen lenker med hemmelighet, ingen roller. Endringslogg i stedet.
- Hver person velger selv hvilke oppgaver hen er med på; resten skjules. Én dato-kalender per person (*passer ikke /
  dårlig / godt*), valgfri egen kalender per oppgave. Kombinasjoner av oppgaver er én felles innstilling.
- «Lik fordeling» = per oppgave (hovedleddet); samlet antall er et svakere vern. Det er en avveining brukeren eier.

## Verktøy (felle)

`mise`-shimmen for python er ødelagt her. Bruk uv med egen Python, fra `solver/`:

    export UV_PYTHON_PREFERENCE=only-managed
    mise exec uv@0.13.5 -- uv sync --python 3.12
    mise exec uv@0.13.5 -- uv run pytest -q          # 72 tester, ca. 10 s

Ikke gi et skript navn som kolliderer med standardbiblioteket (`inspect.py` ødela numpy-importen).

## Testing

- `check.py` er en uavhengig sjekker av harde krav. Hold den uavhengig av `model.py` (bare standardverdiene deles).
- Når en regel endres: bryt den i modellen og se at en test feiler. Tester som består uten regelen er verdiløse.
- Ingen ekte navn, e-post eller telefon i repoet. Fixturen `solver/fixtures/host2026.json` har oppdiktede navn
  og lages av `make_host2026.py`.
- Nivåer på ledningen er ASCII: `ikke|darlig|noytral|godt`, `kan_ikke|upraktisk|praktisk|bor`.
