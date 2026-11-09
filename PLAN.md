# Tjenesteplan: plan

Arbeidstittel. Status 2026-11-08: **planleggeren (`solver/`) virker alene; appen har organisasjoner, planer, personer, endringslogg og oppsett (oppgaver, dagtyper, kombinasjoner, hendelser). Lokalt git-repo, ikke på GitHub.**
Første bruksområde og testdata: en kirkes tjenesteplan for høsten 2026 (oppdiktede navn i repoet). Verktøyet er generelt; se *Generelt*.

## Mål

Fordele oppgaver (lyd, bilde, møteleder, vakter, … hva som helst) på hendelser i en periode slik at

1. **harde krav aldri brytes** (passer ikke, kan ikke kombineres, manuelt utvalg),
2. **preferanser følges så godt det går**, og
3. **fordelingen blir mest mulig lik**, også på tvers av semestre.

Folk legger selv inn når det passer. Planleggeren gir et *forslag*; det som er
lagt inn for hånd (f.eks. at et kor synger på en bestemt hendelse) er *låst* og flyttes aldri.

## Generelt

Bruksområdet er generelt. **Alt er data som admin lager selv**: oppgaver, personer, grupper (team), hendelser og dagtyper,
og innstillingene mellom dem. Ingenting i planleggeren, datamodellen eller språket i appen er bundet til kirke.
Navn som Lyd, Kirkekoret, Gudstjeneste og Alpha finnes bare i testdataene og som eksempler her. Appen får norsk
bokmål først, men tekstene skal kunne byttes ut.

## Avgrensning i første versjon

- **Ingen innlogging, ingen lenker med hemmelighet, ingen roller.** Se *Tilgang uten innlogging*.
- **Grupper er generelle.** En gruppe er et navngitt sett personer (et team, et kor, et par, et lag). Ingen gruppe har
  særregler. Admin kan sette en gruppe inn i en manuell rute, og **planleggeren kan også foreslå grupper** som kandidater
  for oppgaver admin har satt opp for gruppen (`group_task`: hvilken oppgave, hvor mange plasser den fyller, og et nivå).
  I begge tilfeller er medlemmene opptatt med oppgaven, og den felles kombinasjonsinnstillingen avgjør hva de kan ha samme
  dag (se *Manuelle ruter, grupper og utvalg*).
- **Gruppearbeid teller som ett oppdrag per medlem** i fordelingen (samlet og per oppgave), både for
  manuelle og foreslåtte grupper.
- **Ingen regnearkstøtte**: verken import fra eller eksport til regneark (bestemt 2026-11-08). Alt legges inn i appen.
- Ingen kompetanser og ingen betingede krav («når X, så Y»). Utvalg dekker det.
- Ingen øvre grense for antall oppgaver per person per dag, og ingen hvilekrav mellom hendelser. Det er kombinasjonsinnstillingen
  som styrer hva som kan, bør og ikke bør samme dag. (Begge ble prøvd og tatt ut etter ønske 2026-11-04.)

## Hvordan det brukes

**Hvem som helst med adressen**
- Åpner siden og velger navnet sitt.
- **Velger selv hvilke oppgaver hen er med på.** Alle oppgaver er skjult til å begynne med; «vis alle» lar
  personen krysse av for flere. Det som ikke er avkrysset, vises ikke og blir aldri tildelt personlig (unntatt via en gruppe admin
  har satt opp for oppgaven, se *Manuelle ruter, grupper og utvalg*).
- Velger preferanse per oppgave: *ikke / dårlig / nøytral / godt*. Folk er ulike, og det er opp til dem.
- Velger for hver **hendelse**: *passer ikke / dårlig / godt* (ikke markert = nøytral). **Én liste gjelder
  alle oppgavene personen er med på.** Lyd og Bilde har samme preferanse, så det er ett valg per hendelse.
  Det er hendelsen som gjelder, ikke kalenderdagen: to hendelser samme dag har hvert sitt svar.
- Har en oppgave et annet mønster, slår personen på **«egen liste for denne oppgaven»** (av som standard).
  Da får oppgaven sin egen liste, som starter som en kopi av den felles.
- Ser planen og egne oppdrag.

**Oppsett og planlegging** (en del av samme app, ikke et eget område)
- Oppgaver, personer, grupper, hendelser og dagtyper, **felles innstilling for hvilke oppgaver som kan kombineres**,
  manuelle ruter og utvalg. Kjøre planleggeren, se forklaringer, låse forslag.

**Manuelle ruter, grupper og utvalg** (to generelle verktøy som dekker «kor synger», «egen lydtekniker» og alt liknende)
- Admin kan legge inn en **manuell rute** for en oppgave og hendelse: en person, en **gruppe** eller en tekst
  (f.eks. «Gjest utenfra»). Den er låst og teller som en fylt plass. En gruppe eller tekst kan fylle flere plasser (`fills`).
- En **gruppe** i en manuell rute gjør medlemmene opptatt med den oppgaven den dagen. Hva de kan ha i tillegg, styres av den
  felles kombinasjonsinnstillingen, uten særregel. Eksempel: gruppen «Kirkekoret» står på oppgaven «Lovsang» 20.9, og
  *Lovsang × Lyd* og *Lovsang × Bilde* er satt til «kan ikke kombineres». Da er medlemmene ute av Lyd og Bilde den dagen.
  Står paret på «upraktisk» eller «praktisk» i stedet, blir det en straff eller en bonus, ikke et forbud.
- Admin kan sette et **utvalg** for en oppgave og hendelse: bare disse kan velges. Planleggeren velger blant dem.
  «Egen lydtekniker når koret synger» er et utvalg for Lyd på to personer som er med på oppgaven og ikke synger.
  Er antallet i utvalget like stort som antall plasser, er det i praksis en manuell tildeling.
- Utvalget er et vanlig hardt krav. Står noen i utvalget som har «passer ikke» den dagen eller ikke er med på
  oppgaven, kommer det som forklaring (og en tom plass hvis ingen i utvalget går).
- Står et gruppemedlem på «passer ikke» på hendelsen en manuell gruppe er satt opp for, beholdes det manuelle valget, med en advarsel.
- **Foreslåtte grupper, det admin må sette opp:** for hver gruppe hvilke oppgaver den kan foreslås til, hvor mange plasser den
  fyller og et nivå. Én som har «passer ikke» på en hendelse, utelukker hele gruppen fra den hendelsen; et par er altså ikke brukbart på en hendelse
  noen av dem ikke kan.
- **Kan foreslås, per person og per gruppe** (standard ja). Står den på nei, velger planleggeren aldri personen eller gruppen selv
  (heller ikke gjennom et utvalg), men admin kan fortsatt legge dem inn for hånd. En gruppe foreslås heller ikke hvis et medlem
  ikke kan foreslås. Gruppen foreslås til de oppgavene admin har satt opp for den, uavhengig av om medlemmene selv har krysset av
  for oppgaven; det er denne innstillingen som styrer det.
- **Foreslåtte grupper:** admin setter opp hvilke oppgaver en gruppe kan foreslås til (`group_task`). Gruppen er en kandidat
  på lik linje med en person og fyller `fills` plasser. Den er utelukket fra en hendelse hvis ett medlem har «passer ikke» (eller har
  sagt «ikke» til oppgaven), hvis gruppen står utenfor et utvalg, eller hvis medlemmene er opptatt via en annen gruppe.
  Poengene for hendelsen er snittet av medlemmenes ønsker; oppgavenivået settes på gruppen. Medlemmenes egen deltakelse i oppgaven
  trengs ikke. Et utvalg kan nevne både personer og grupper.
- To grupper med felles medlem kan ikke begge velges samme dag hvis det er samme oppgave eller «kan ikke kombineres».

## Datamodell (SQLite)

**En organisasjon har flere planer.** `organization` → `plan` (f.eks. «Høst 2026», «Vår 2027»). Det som er felles for en
organisasjon, deles av alle planene; det som hører til en periode, ligger under planen.

*Bygget (migrering 001–008):* `organization`, `plan`, `person`, `change_log`, `task`, `day_type`, `day_type_task`, `task_combination`, `event`, `person_task`, `person_event`, `plan_run`, `grp`, `grp_member`, `grp_task`, `manual_cell`, `pool_member`.
Alle barnetabeller har `org_id` og sammensatte fremmednøkler mot `(org_id, id)`, så databasen selv nekter referanser på tvers av organisasjoner.

| Tabell | Nivå | Innhold |
|---|---|---|
| `organization` | | `slug` (unik, i adressen), navn |
| `plan` | organisasjon | `slug` (unik i organisasjonen), navn, **`weights`** (JSON: planleggerens vekter som avviker fra standard). Erstatter «periode/semester» |
| `person` | organisasjon | navn (unikt i organisasjonen), aktiv, **kan foreslås** (standard ja). Gjenbrukes i alle planer |
| `group`, `group_member` | organisasjon | **bygget som `grp`, `grp_member`**: navngitt sett personer (team, kor, par). Ingen egne regler. **Kan foreslås** (standard ja) |
| `group_task` | organisasjon | **bygget som `grp_task`**: oppgaver planleggeren kan foreslå gruppen til: plasser den fyller (`fills`) og nivå |
| `task` | organisasjon | navn, antall plasser, om planleggeren fordeler den (ellers bare for manuelle ruter og kombinasjoner) |
| `day_type`, `day_type_task` | organisasjon | dagtyper admin lager, hvilke oppgaver som gjelder for dem, og antall plasser |
| `task_combination` | organisasjon | **felles**: oppgave A × oppgave B: *kan ikke kombineres / upraktisk / praktisk / bør kombineres* |
| `person_task` | organisasjon | **bygget**: personen er med på oppgaven (eget valg; ingen rad = ikke med) og nivå (ikke / dårlig / nøytral / godt). Bare oppgaver planleggeren fordeler kan velges |
| `event` | plan | **hendelse**: dato, dagtype, merknad. Selvstendig enhet; flere på samme dag er lov |
| `person_event` | plan | **bygget**: nivå per hendelse (passer ikke / dårlig / godt; ingen rad = nøytral), felles for personens oppgaver. Sammensatt fremmednøkkel (org, plan, hendelse) |
| `person_task_event` | plan | *ikke bygget ennå*: settes på når en oppgave har «egen liste»; da brukes den i stedet for den felles |
| `pool_member` | plan | **bygget**: **utvalg** for én oppgave på én hendelse: de eneste personer og grupper som kan velges (finnes det rader, er det et utvalg). Sammensatt fremmednøkkel mot (org, plan, hendelse) |
| `manual_cell` | plan | **bygget**: manuell rute for en oppgave på en hendelse: én person, én gruppe eller en tekst (+ `fills` plasser for gruppe og tekst). Alltid låst. Forslag fra planleggeren lagres ikke her, men i `plan_run` |
| `plan_run` | plan | status (running/done/failed), frø, tidsgrense, hvem startet, hash av inndata, inndata, øyeblikksbilde av navn og plasser, svar fra planleggeren, feilmelding. De 20 nyeste per plan beholdes. Siste kjøring er planen (publisering er ikke bygget) |
| `change_log` | organisasjon (+ valgfri plan) | hvem (oppgitt navn), hva, når. Skrives i samme transaksjon som endringen |

**Optimalisering skjer bare innenfor én plan** (brukerens valg 2026-11-09). Tildelinger i tidligere planer teller ikke med: appen sender aldri `history` eller `previous_plan` til planleggeren (den kan fortsatt ta imot dem).
Alle oppslag er avgrenset av organisasjonen; en plan eller person fra en annen organisasjon gir 404.

**Hendelser:** en hendelse hører til én plan og har en dato og en dagtype (og en merknad). Hendelser er **selvstendige**: to kan ligge på
samme dag (f.eks. en gudstjeneste og en medarbeiderkveld, eller to vakter), og tilgjengelighet, kombinasjoner, plasser og gruppe-opptatthet gjelder
hendelsen, aldri kalenderdagen. API-et bruker id (`/events/{id}`). «Alle søndager i et tidsrom» legger til en hendelse per valgt ukedag
(ISO 1 = mandag … 7 = søndag, regnet ut i Go, ikke i nettleseren), høyst 366 dager om gangen; en hendelse med samme dato og dagtype hoppes over,
så samme kjøring to ganger ikke dupliserer. Rekkefølgen innen en dato er opprettelsesrekkefølgen (ingen klokkeslett ennå).
Sletting av en oppgave som er i en dagtype, eller en dagtype som har hendelser, nektes (409); kombinasjoner følger oppgaven.

### Samme skala overalt
Hendelse og oppgave bruker begge **ikke / dårlig / (nøytral) / godt**. Kombinasjoner bruker
**kan ikke / upraktisk / praktisk / bør**.

## Planleggeren (CP-SAT)

Én ja/nei-variabel per (person, oppgave, hendelse) som er mulig etter filtrering. Typisk 1–2 tusen variabler.

**Harde krav**
- «ikke» på hendelse eller oppgave fjerner variabelen. Det samme gjør «ikke med på oppgaven».
- Utvalg (`pool_member`): alle utenfor utvalget fjernes for den oppgaven og hendelsen.
- Antall plasser per oppgave og hendelse (låste ruter regnes med).
- Oppgavepar som *kan ikke kombineres*: høyst én av dem per person per dag.
- Gruppe i en manuell rute eller foreslått av planleggeren: hvert medlem er opptatt med gruppens oppgave. Paret (gruppens oppgave,
  medlemmets andre oppgave) følger kombinasjonsinnstillingen (forbud, straff eller bonus), og medlemmet kan ikke ta en ekstra
  plass på samme oppgave.

**Mål (poeng, justeres mot testdataene)**
- Pluss for «godt», minus for «dårlig», på hendelse og oppgave.
- Pluss/minus for *praktisk*, *upraktisk* og *bør kombineres*.
- **Jevn fordeling = lik total belastning (hovedleddet; avgjort av brukeren 2026-11-09).** Hver persons mål er sin andel av alle oppdrag (gruppearbeid
  teller ett oppdrag per medlem), regnet ut fra hvor mange hendelser personen kan stille. Målet er sjelden et helt tall, så det er et **bånd**
  [nedre, øvre hele tall] som er «jevnt nok», og bare det utenfor båndet koster (`w_fair`, 10). I tillegg straffes det verste avviket (`w_fair_range`, 10), så ikke én
  person tar hele avviket. Hvilke oppgaver oppdragene er i, spiller ingen rolle: en som bare tar Bilde skal ikke ende på halvparten av en som tar Lyd og Bilde.
  (Måltallet bygger på antall plasser; en foreslått gruppe med flere medlemmer enn plasser gir flere oppdrag enn plasser, og det regnes ikke inn i målet på forhånd.)
- **Fordeling per oppgave er av som standard** (`w_task_fair` og `w_task_range` er 0, og da bygges ingen variabler for den). Slås den på, får alle som kan en
  oppgave lik andel av akkurat den, og da får de med flere oppgaver mer totalt. Prisen for total-først: samme oppgave kan fordeles ujevnt mellom
  de som kan den (se *Resultater fra spiken*).
- **Historikk fra tidligere planer brukes ikke** (valg 2026-11-09). Planleggeren kan ta imot `history` per oppgave (teller halvt, `history_weight`),
  men appen sender den ikke.
- Ved ny kjøring kan planleggeren straffe endringer fra en forrige plan (`previous_plan`), men appen bruker det ikke. Det som skal bli stående, låses som manuell rute («Lås» på et forslag).
- Tom plass er tillatt, men koster så mye at planleggeren bare gjør det når det ikke går ellers.

**Forklaringer**: for hver tom plass eller brutt «dårlig» lages en kort tekst, f.eks. «Oppgave X 15.11: 4 aktuelle,
2 har *passer ikke*, 3 står utenfor utvalget, 1 er opptatt via en gruppe». Dette er et eget enkelt filter over inndata, ikke en uløselighetsanalyse.

## Arkitektur: ordentlig backend

- **Go-binær med innebygd SQLite** (`modernc.org/sqlite`, ren Go uten cgo), nummererte migreringer innebygd og kjørt i en
  transaksjon ved oppstart, vanilla JS-frontend innebygd. Kjører på kate (Flux, image til GHCR).
  Backend trengs for felles lagring mellom alle brukerne, planleggeren og historikk, selv uten innlogging.
- **API: vanlig JSON over `net/http`, ikke Connect** (valgt 2026-11-07, avviker fra `scoreboard/`). Det er under ti endepunkter,
  og Connect ville krevd `buf`, to kodegeneratorer og generert kode. Skiftet senere er lite siden grensesnittet er smalt.
  Feil er `{"error": kode, "message": tekst}` med riktig statuskode.
- **Organisasjonen ligger i adressen**, både i siden (`/o/<org>/p/<plan>`) og API-et (`/api/orgs/{org}/...`). Tjeneren har ingen
  «gjeldende organisasjon», så å bytte er bare å gå til en annen adresse. Organisasjonslisten kan leses av alle som når adressen.
- **Fremmednøkler** er av som standard i SQLite på hver tilkobling og slås på i DSN-en; en test sjekker at de håndheves.
- **Solveren er et Python-program med `ortools`** som Go starter som underprosess: JSON inn på stdin, JSON ut på stdout,
  tidsgrense. Samme container-image. Kontrakten er liten og kan testes uten resten av appen. **Python beholdes** (brukerens svar
  2026-11-08: «hvis Python er bedre egnet kan det beholdes»). Grunnen er at det er samme CP-SAT-løser, pip-pakken `ortools` er ferdigbygd
  med de native bibliotekene, og modellen har egne tester og mutasjoner for hver regel.
- **Go-alternativer, undersøkt 2026-11-08:**
  - *Offisielt Go-API for CP-SAT* (`github.com/google/or-tools/ortools/sat/go/cpmodel`): finnes, men er et cgo-skall rundt OR-Tools sitt
    C-API. Det har ingen versjonsmerking (pseudoversjon fra oktober 2026), ingen kjente brukere, og bygges med Bazel mot det native
    biblioteket, ikke som en vanlig Go-avhengighet. Appen ville mistet «én statisk binær» og fått en tung bygg-kjede, og modellen og alle
    testene måtte portes. Brukbart senere hvis Python blir et problem.
  - *Uoffisielt cgo-skall* (`irfansharif/solver`): bundet til OR-Tools 9.1, sist endret april 2023. Ikke aktuelt.
  - *Rene Go-biblioteker for begrensninger* (f.eks. `centipede`, `go-constraint-solver`): løser tilfredsstillbarhet med tilbakesporing, ikke
    optimering med jevn fordeling. Ikke likeverdige.
  - *MIP-løsere via cgo* (f.eks. HiGHS): krever også et native bibliotek, og er svakere enn CP-SAT på denne typen problem.
  - Jeg fant ingen ren Go-løser som kan det CP-SAT gjør her.

### Kjøring av planleggeren (bygget, fase 4 omgang 1)
- `POST /api/orgs/{org}/plans/{plan}/runs` (`{time_limit, seed}`, begge valgfrie) svarer 202 med id med en gang; kjøringen går i bakgrunnen og siden
  spør `GET .../runs/{id|latest}` hvert sekund. Bare **én kjøring om gangen** i hele tjeneren (409 `busy`), tidsgrense 5–120 s (standard 20).
  Uten planlegger konfigurert (`--solver` / `$TJENESTEPLAN_SOLVER`) svarer API-et 503 og `/api/health` har `"planner": false`; resten av appen virker.
- Go bygger inndata fra databasen (`internal/planner/input.go`): bare aktive personer, id-ene `t<id>/e<id>/d<id>/p<id>`, «kan foreslås»,
  oppgavevalg, tilgjengelighet per hendelse og kombinasjoner. Hashen av inndata (uten frø og tid) lagres, så en gammel plan kan merkes **utdatert**
  når svar eller oppsett er endret siden.
- Planleggeren startes med `--check`: den uavhengige sjekkeren går over svaret og `violations` lagres og vises. Svar som ikke er gyldig JSON med status
  OPTIMAL/FEASIBLE, tidsavbrudd og krasj blir en **feilet kjøring med årsak**, aldri en halv plan. Kjøringer som sto som «running» ved oppstart merkes feilet.
- Visningen (`internal/planner/view.go`) setter navn og oppgaver fra øyeblikksbildet og gir hver tom plass en forklaring på norsk
  (`explain.go`: passer ikke, ikke valgt oppgaven, kan ikke foreslås, opptatt med noe annet). «Prøv igjen» teller frøet opp.

### Vekting per plan (bygget, etter fase 4 omgang 2)
- Planleggerens vekter er innstillinger på planen (`GET|PUT /plans/{plan}/weights`, seksjonen «Vekting» på planen). De som kan endres, står i `store.WeightSpecs`
  med standard og grenser (fordeling samlet og per oppgave, poeng for «godt» og «dårlig» på oppgave og hendelse, poeng for kombinasjoner og hva et par uten innstilling regnes som).
  Ikke med: pris for tom plass (må komme etter alt annet), stabilitet og historikk (appen bruker dem ikke) og nøytral (null).
- Bare det som avviker fra standard lagres (`plan.weights`, JSON), så en endret standard når planer som aldri valgte noe. Verdier utenfor grensene og ukjente nøkler avvises;
  fortegn følger meningen (en «dårlig» kan ikke gi pluss). Settes en plan helt tilbake, er inndataene og hashen som før vektingen fantes.
- Vektene går til planleggeren som `params` (nøkler med punktum, som `w_task.godt`, nøstes) og er med i hashen: en annen vekting gir en annen plan, så en gammel kjøring merkes utdatert.
  Tid, frø og tråder er det ikke. Standardene i Go er kopier av planleggerens egne, og en test mot den ekte planleggeren (`--defaults`) holder dem like.
- Per plan, ikke per organisasjon: planen er enheten det optimaliseres innenfor. Skal nye planer arve fra forrige eller fra en organisasjonsstandard, legges det til da.

### Grupper, manuelle ruter og utvalg (bygget, fase 4 omgang 2)
- **Grupper** (`/orgs/{org}/groups`, seksjonen «Grupper» under Oppsett): navn, medlemmer, «kan foreslås» og oppgavene planleggeren kan foreslå gruppen til
  (plasser og nivå; bare oppgaver planleggeren fordeler). Slettes ikke mens en rute eller et utvalg nevner den (409).
- **Manuelle ruter** (`/plans/{plan}/cells`) og **utvalg** (`/plans/{plan}/pools`): settes for flere hendelser om gangen, alt eller ingenting, bare hendelser i planen.
  Ruter for personer må være aktive. En rute kan stå på en oppgave planleggeren ikke fordeler (f.eks. «Lovsang» for koret): kolonnen vises bare der det finnes ruter, og
  oppgavens plasser i dagtypen gir ingen tomme ruter. «Lås» på et forslag i planen lager en manuell rute for personen.
- **Aktive bare:** inaktive personer og grupper uten aktive medlemmer sendes aldri til planleggeren (den ville talt plassen uten å vite hvem). Det som nevner dem,
  utelates, og kjøringen sier fra («Ruten for Aslaug på Lyd … er ikke med fordi personen ikke er aktiv»). Et utvalg som blir tomt, beholdes (ingen kan velges) med advarsel.
- Planleggerens advarsler leses med navn; en rute for en person som «kan ikke foreslås» gir ingen advarsel (det er slik innstillingen er ment å brukes).
- **Publisering er ikke bygget.** Brukerens svar 2026-11-09 («Ja … optimaliser kun innenfor en plan») ble lest som at siste kjøring er planen. Skal et forslag
  kunne godtas eksplisitt (f.eks. for «mine oppdrag» i fase 5), legges det til da.

### Tilgang uten innlogging
Første versjon har ingen innlogging, ingen personlige lenker og ingen roller. Konsekvensene:
- **Alle med adressen kan se og endre alt**, også andres tilgjengelighet og planen. Preferansene er bare
  nivåer per hendelse og oppgave, uten fritekst, så det er lite å lekke, men de er ikke private.
- Det som gjør det forsvarlig: appen legges på en adresse som ikke er lenket noe sted og bes holdes utenfor
  søkemotorer (`noindex`), **`change_log`** viser hvem (oppgitt navn) som endret hva og når,
  og SQLite tas sikkerhetskopi av.
- Alle endringer sendes med hvem som handler, i `X-Actor` (prosentkodet UTF-8). Skrivinger må være `application/json`, og
  den egendefinerte headeren gjør at nettlesere nekter skriving fra andre nettsteder (ingen CORS). Mangler navnet, svarer API-et 400.
  Alle skrivinger går gjennom én hjelper som logger i samme transaksjon.
- Legger vi til lenker eller innlogging senere, er det et tillegg og krever ingen ny datamodell.

## Faser

0. **Spike (gjort 2026-11-03, oppdatert 11-04)**: `solver/`, se resultatene under. Ingen app ennå.
1. **Backend-skjelett (gjort 2026-11-07)**: Go, SQLite, migreringer, organisasjoner og planer, personer, navnevelger, endringslogg, nb-frontend.
2. **Oppsett (gjort 2026-11-08)**: oppgaver, dagtyper med plasser, kombinasjoner, hendelser (enkeltvis og for alle valgte ukedager i et tidsrom; selvstendige, flere samme dag er lov).
3. **Personsiden (gjort 2026-11-08)**: «Min side» per person og plan, mobil først: velg oppgaver og hvor gjerne, og svar *passer ikke / dårlig / godt* per hendelse med ett trykk (trykk igjen fjerner svaret; «Sett alle»). Svarene lagres med en gang. *Ikke bygget:* «egen liste for denne oppgaven». Smale skjermer er førstevalget; en test feiler om en side blir bredere enn en 390 px skjerm.
4. **Planlegging**: kjør, forhåndsvis, lås, utvalg, grupper, forklaringer. **Omgang 1 gjort (2026-11-08):** «Planlegg» / «Prøv igjen» på planen, kjøring i bakgrunnen, rutenett per hendelse med forklaringer, kompromisser og fordeling, kontroll og utdatert-merking (se «Kjøring av planleggeren»). **Omgang 2 gjort (2026-11-09):** grupper, manuelle ruter, utvalg og «Lås» (se «Grupper, manuelle ruter og utvalg»). Optimalisering skjer bare innenfor planen; ingen publisering og ingen historikk fra andre planer.
5. **Plan**: rutenett (hendelser bortover, oppgaver nedover) og «mine oppdrag». *Ingen kalenderfeed* (droppet av brukeren 2026-11-09: den ville vist utkast uten en godkjent plan, og hendelser har ikke klokkeslett).
6. **Utrulling på kate**: image, Flux, sikkerhetskopi av SQLite.

**Pilot**: først to-tre oppgaver med manuelle ruter, grupper og utvalg, så resten av oppgavene.

## Resultater fra spiken (fase 0)

Solveren alene, `solver/`: 78 tester (47 regeltester, testdataene fra høsten 2026, 30 tilfeldige instanser), alle grønne.

- **Reglene holder.** Hver regel har en test som feiler hvis regelen fjernes (prøvd ved å bryte modellen én regel av
  gangen). En uavhengig sjekker kontrollerer harde krav på alle planer. Utvalg, grupper i manuelle ruter (med kombinasjonsinnstillingen), foreslåtte team og par, den låste
  personen og tekstrutene virker på testdataene (18 personer, 17 hendelser, 7 oppgaver, 4 team/par, ca. 120 tildelinger).
- **Størrelse og tid.** Etter at hvile og dagstak ble tatt ut, kommer en gyldig og brukbar plan etter ca. 2 sekunder, og
  kvaliteten bedres til ca. 20 s. Solveren **beviser ikke** at planen er best mulig (øvre grense tettner ikke), så resultatet
  er et *forslag* innenfor tidsgrensen. Ulike frø gir litt ulike planer; «prøv en gang til» er derfor en nyttig knapp.
  Anbefalt tidsgrense i appen: 30 s. Gjentas med `mise run measure:time`.
- **Fordelingen måtte rettes to ganger.**
  1. Heltall skalert med total kapasitet og `abs` ga svak relaksasjon og dårlig konvergens. Byttet til bånd.
  2. **Avgjort 2026-11-09: total belastning er hovedleddet.** Spiken hadde gjort fordelingen *per oppgave* til hovedledd, fordi «samlet først» ga skjev fordeling
     innen én oppgave (i en kjøring tok én person 7 forbønner mens en annen fikk 0). Brukeren valgte likevel total: «en som har bare bilde bør ikke være halvparten
     av en som har lyd og bilde». Med lik andel av hver oppgave ender en som tar to oppgaver på dobbelt så mange oppdrag som en som tar én (6, 12 og 6 i
     stedet for 8, 8 og 8; testet). Målt med `mise run measure:fair` (8 s, tre frø, testdataene fra høsten 2026):

     | | verste avvik fra egen andel samlet | verste avvik per oppgave | største spredning per oppgave |
     |---|---|---|---|
     | per oppgave først (til 2026-11-09) | 4,3–4,5 oppdrag | 1,0 | 4 |
     | **total først (nå)** | **0,9** | 6,6 | 10 |
     | total først + svak per oppgave (`w_task_fair` 3) | 0,9 | 4,6–4,8 | 8 |

     **Prisen er at samme oppgave kan bli ujevnt fordelt mellom de som kan den**, siden det er totalen som holdes jevn. Vil du ha litt mer jevnhet per oppgave uten å
     tape på totalen, er `w_task_fair` 3 (med `w_task_range` 0) et alternativ: på testdataene holdt totalen like jevn.
- **Historikk er per oppgave** (`{person: {oppgave: antall}}`), og virker bare når fordeling per oppgave er på. Appen sender ikke historikk.
- **Vekter er ikke justert mot ekte ønsker.** De står i `DEFAULT_PARAMS` og er bare rimelige startverdier.
- **Oppgavepar uten innstilling** regnes som *upraktisk* (`default_combination`). I én kjøring delte 9–13 personer to
  oppgaver samme dag, de fleste par uten innstilling (altså upraktiske) og noen ønskede. Admin må derfor sette opp
  kombinasjonene med omhu.

## Testing

- Ett lite eksempel per regel (hvert krav har en test som feiler hvis det ikke holdes). `mise run mutations` bryter 95 regler i
  planleggeren (Python) og appen (Go, særlig organisasjonsavgrensningen) én om gangen og krever at en test feiler; stille når alt er fanget.
- Alt kjøres via mise (`mise.toml`: python og uv i eksakte versjoner, stille oppgaver); se `CLAUDE.md`.
- **Uavhengig sjekker** som kontrollerer at en ferdig plan ikke bryter noe hardt krav. Tilfeldige instanser
  kjøres gjennom solver + sjekker.
- Fast testplan basert på en kirkes høst 2026 med oppdiktede navn. Ekte navn og telefon- og e-postlister hører ikke inn i repoet.

## Valg jeg tar som standard hvis du ikke sier noe annet

1. Navn: «Tjenesteplan» (arbeidstittel). Adresse bestemmes ved utrulling.
2. Ingen innlogging i første versjon. Uoffentlig adresse, `noindex`, endringslogg.
3. Solver som Python-underprosess i samme image som Go-backenden.
4. Én plan per periode (f.eks. semester). Optimalisering skjer bare innenfor planen; ingen historikk fra tidligere planer.
5. Ingen regnearkstøtte (hverken import eller eksport).

## Risiko

- Vektene for poeng og jevn fordeling må finjusteres med ekte ønsker, ellers føles forslagene rare.
- Folk må faktisk fylle ut. Derfor bare én liste per person og skjult det som er uvedkommende.
- Uten innlogging kan en feilklikk eller en uvitende bruker endre andres svar. Endringsloggen og sikkerhetskopien er
  det som fanger det.
- Python i Go-imaget gjør bygget og imaget større. Besluttet: Python beholdes (se *Arkitektur*). Fase 4 må pakke Python og `ortools` inn i imaget.
