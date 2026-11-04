# Tjenesteplan: plan

Arbeidstittel. Status 2026-11-04: **plan + en fungerende planlegger alene (`solver/`). Ingen app ennå. Lokalt git-repo, ikke på GitHub.**
Første bruksområde og testdata: en kirkes tjenesteplan for høsten 2026 (oppdiktede navn i repoet). Verktøyet er generelt; se *Generelt*.

## Mål

Fordele oppgaver (lyd, bilde, møteleder, vakter, … hva som helst) på datoer i en periode slik at

1. **harde krav aldri brytes** (passer ikke, kan ikke kombineres, manuelt utvalg),
2. **preferanser følges så godt det går**, og
3. **fordelingen blir mest mulig lik**, også på tvers av semestre.

Folk legger selv inn når det passer. Planleggeren gir et *forslag*; det som er
lagt inn for hånd (f.eks. at et kor synger en bestemt dato) er *låst* og flyttes aldri.

## Generelt

Bruksområdet er generelt. **Alt er data som admin lager selv**: oppgaver, personer, grupper (team), datoer og dagtyper,
og innstillingene mellom dem. Ingenting i planleggeren, datamodellen eller språket i appen er bundet til kirke.
Navn som Lyd, Kirkekoret, Gudstjeneste og Alpha finnes bare i testdataene og som eksempler her. Appen får norsk
bokmål først, men tekstene skal kunne byttes ut.

## Avgrensning i første versjon

- **Ingen innlogging, ingen lenker med hemmelighet, ingen roller.** Se *Tilgang uten innlogging*.
- **Grupper er generelle.** En gruppe er et navngitt sett personer (et team, et kor, et par, et lag). Ingen gruppe har
  særregler. En gruppe kan stå i en manuell rute og fylle én eller flere plasser. Medlemmene er da opptatt med den
  oppgaven, og den felles kombinasjonsinnstillingen avgjør hva de kan ha samme dag (se *Manuelle ruter og utvalg*).
- **Planleggeren velger ikke grupper selv ennå.** Grupper settes inn for hånd. Å la planleggeren velge hele team som
  kandidater kan komme senere hvis det trengs.
- Ingen kompetanser og ingen betingede krav («når X, så Y»). Utvalg dekker det.
- **Arbeid via en gruppe i en manuell rute teller foreløpig ikke i fordelingen.** Verken medlemmenes totale antall,
  per-oppgave-målene eller historikken for neste periode regner det med. Et team som fyller en kafédato for hånd, får
  altså fortsatt full andel av de andre kafédatoene. Regelen for hvordan det skal telle (ett oppdrag per medlem, eller
  plassene delt på medlemmene) er ikke valgt; det er brukerens avgjørelse.
- Ingen øvre grense for antall oppgaver per person per dag, og ingen hvilekrav mellom datoer. Det er kombinasjonsinnstillingen
  som styrer hva som kan, bør og ikke bør samme dag. (Begge ble prøvd og tatt ut etter ønske 2026-11-04.)

## Hvordan det brukes

**Hvem som helst med adressen**
- Åpner siden og velger navnet sitt.
- **Velger selv hvilke oppgaver hen er med på.** Alle oppgaver er skjult til å begynne med; «vis alle» lar
  personen krysse av for flere. Det som ikke er avkrysset, vises ikke og blir aldri tildelt.
- Velger preferanse per oppgave: *ikke / dårlig / nøytral / godt*. Folk er ulike, og det er opp til dem.
- Velger for hver dato: *passer ikke / dårlig / godt* (ikke markert = nøytral). **Én kalender gjelder
  alle oppgavene personen er med på.** Lyd og Bilde har samme preferanse, så det er ett valg per dato.
- Har en oppgave et annet mønster, slår personen på **«egen kalender for denne oppgaven»** (av som standard).
  Da får oppgaven sin egen kalender, som starter som en kopi av den felles.
- Ser planen og egne oppdrag.

**Oppsett og planlegging** (en del av samme app, ikke et eget område)
- Oppgaver, personer, grupper, datoer og dagtyper, **felles innstilling for hvilke oppgaver som kan kombineres**,
  manuelle ruter og utvalg. Kjøre planleggeren, se forklaringer, publisere.

**Manuelle ruter, grupper og utvalg** (to generelle verktøy som dekker «kor synger», «egen lydtekniker» og alt liknende)
- Admin kan legge inn en **manuell rute** for en oppgave og dato: en person, en **gruppe** eller en tekst
  (f.eks. «Gjest utenfra»). Den er låst og teller som en fylt plass. En gruppe eller tekst kan fylle flere plasser (`fills`).
- En **gruppe** i en manuell rute gjør medlemmene opptatt med den oppgaven den dagen. Hva de kan ha i tillegg, styres av den
  felles kombinasjonsinnstillingen, uten særregel. Eksempel: gruppen «Kirkekoret» står på oppgaven «Lovsang» 20.9, og
  *Lovsang × Lyd* og *Lovsang × Bilde* er satt til «kan ikke kombineres». Da er medlemmene ute av Lyd og Bilde den dagen.
  Står paret på «upraktisk» eller «praktisk» i stedet, blir det en straff eller en bonus, ikke et forbud.
- Admin kan sette et **utvalg** for en oppgave og dato: bare disse kan velges. Planleggeren velger blant dem.
  «Egen lydtekniker når koret synger» er et utvalg for Lyd på to personer som er med på oppgaven og ikke synger.
  Er antallet i utvalget like stort som antall plasser, er det i praksis en manuell tildeling.
- Utvalget er et vanlig hardt krav. Står noen i utvalget som har «passer ikke» den dagen eller ikke er med på
  oppgaven, kommer det som forklaring (og en tom plass hvis ingen i utvalget går).
- Står et gruppemedlem på «passer ikke» den dagen gruppen er satt opp, beholdes det manuelle valget, med en advarsel.

## Datamodell (SQLite)

| Tabell | Innhold |
|---|---|
| `person` | navn, aktiv |
| `group`, `group_member` | navngitt sett personer (team, kor, par). Ingen egne regler |
| `task` | navn, antall plasser, om planleggeren fordeler den (ellers bare for manuelle ruter og kombinasjoner). F.eks. én oppgave per aldersgruppe |
| `period` | f.eks. et semester |
| `slot_date` | dato, dagtype (fritt navn admin lager selv), merknad |
| `day_type_task` | hvilke oppgaver som gjelder for dagtypen, og antall plasser |
| `person_task` | om personen er med på oppgaven (eget valg, skjult som standard) og nivå |
| `person_date` | nivå per dato, felles for personens oppgaver |
| `person_task_calendar` | settes på når en oppgave har «egen kalender»; da brukes den i stedet for den felles for den oppgaven |
| `task_combination` | **felles**: oppgave A × oppgave B: *kan ikke kombineres / upraktisk / praktisk / bør kombineres* |
| `selection` | **utvalg** for én oppgave på én dato: de eneste som kan velges |
| `assignment` | dato, oppgave, plass, person, gruppe eller fritekst (+ `fills`), `locked`, kilde (manuell / plan) |
| `plan_run` | parametere, resultat, mål, forklaringer, publisert ja/nei |
| `change_log` | hvem (oppgitt navn), hva, når. Se *Tilgang uten innlogging* |

Rader i et eksisterende ark som bare er informasjon (tema, kollekt), er ikke oppgaver og hører ikke inn i planleggeren.

### Samme skala overalt
Dato og oppgave bruker begge **ikke / dårlig / (nøytral) / godt**. Kombinasjoner bruker
**kan ikke / upraktisk / praktisk / bør**.

## Planleggeren (CP-SAT)

Én ja/nei-variabel per (person, oppgave, dato) som er mulig etter filtrering. Typisk 1–2 tusen variabler.

**Harde krav**
- «ikke» på dato eller oppgave fjerner variabelen. Det samme gjør «ikke med på oppgaven».
- Utvalg (`selection`): alle utenfor utvalget fjernes for den oppgaven og datoen.
- Antall plasser per oppgave og dato (låste ruter regnes med).
- Oppgavepar som *kan ikke kombineres*: høyst én av dem per person per dag.
- Gruppe i en manuell rute: hvert medlem er opptatt med gruppens oppgave. Paret (gruppens oppgave, medlemmets andre oppgave)
  følger kombinasjonsinnstillingen (forbud, straff eller bonus), og medlemmet kan ikke ta en ekstra plass på samme oppgave.

**Mål (poeng, justeres mot testdataene)**
- Pluss for «godt», minus for «dårlig», på dato og oppgave.
- Pluss/minus for *praktisk*, *upraktisk* og *bør kombineres*.
- **Jevn fordeling, per oppgave (hovedleddet)**: De som kan en oppgave, deler den likt. Hver persons mål er sin andel av
  oppgavens plasser, regnet ut fra hvor mange datoer personen kan stille. Målet er sjelden et helt tall, så det er et
  **bånd** [nedre, øvre hele tall] som er «jevnt nok», og bare det utenfor båndet koster. I tillegg straffes det verste
  avviket per oppgave, så ikke én person tar hele avviket.
- **Samlet antall oppdrag (vern mot overbelastning)**: samme type bånd over alle oppgaver, med lavere vekt. Oppgavene har
  ulik etterspørsel (f.eks. 33 plasser på 7 personer for én oppgave, 17 på 5 for en annen), så samlet antall kan ikke bli likt uten å gjøre
  fordelingen per oppgave skjev. Vektene bestemmer avveiningen: se *Resultater fra spiken*.
- **Historikk fra tidligere semestre, per oppgave**: den som har gjort mest av en oppgave, får litt mindre av akkurat
  den. Teller halvt som standard (`history_weight`). Regnes ut fra tidligere semestres tildelinger.
- Ved ny kjøring: straff endringer fra forrige plan, så ikke alt stokkes om.
- Tom plass er tillatt, men koster så mye at planleggeren bare gjør det når det ikke går ellers.

**Forklaringer**: for hver tom plass eller brutt «dårlig» lages en kort tekst, f.eks. «Oppgave X 15.11: 4 aktuelle,
2 har *passer ikke*, 3 står utenfor utvalget, 1 er opptatt via en gruppe». Dette er et eget enkelt filter over inndata, ikke en uløselighetsanalyse.

## Arkitektur: ordentlig backend

- **Go-binær med innebygd SQLite**, migreringer ved oppstart, Connect-API, vanilla JS-frontend innebygd,
  samme oppsett som `scoreboard/` og `chores/`. Kjører på kate (Flux, image til GHCR).
  Backend trengs for felles lagring mellom alle brukerne, planleggeren og historikk, selv uten innlogging.
- **Solveren er et Python-program med `ortools`** som Go starter som underprosess:
  JSON inn på stdin, JSON ut på stdout, tidsgrense. Samme container-image. Kontrakten er liten og kan testes
  uten resten av appen. Jeg valgte dette fordi OR-Tools ikke har en støttet Go-binding, og fordi Python-eksemplene
  for turnusplanlegging ligner mest på dette problemet.
- Alternativ om det blir tungvint: ren Python-backend, eller CP-SAT i nettleseren via et av
  WebAssembly-byggene (uprøvd og ikke offisielt).

### Tilgang uten innlogging
Første versjon har ingen innlogging, ingen personlige lenker og ingen roller. Konsekvensene:
- **Alle med adressen kan se og endre alt**, også andres tilgjengelighet og planen. Preferansene er bare
  nivåer per dato og oppgave, uten fritekst, så det er lite å lekke, men de er ikke private.
- Det som gjør det forsvarlig: appen legges på en adresse som ikke er lenket noe sted og bes holdes utenfor
  søkemotorer (`noindex`), **`change_log`** viser hvem (oppgitt navn) som endret hva og når,
  og SQLite tas sikkerhetskopi av.
- Alle endringer sendes med hvem som handler. Legger vi til lenker eller innlogging senere, er det
  et tillegg og krever ingen ny datamodell.

## Faser

0. **Spike (gjort 2026-11-03, oppdatert 11-04)**: `solver/`, se resultatene under. Ingen app ennå.
1. **Backend-skjelett**: Go, SQLite, migreringer, navnevelger, endringslogg.
2. **Oppsett og import**: oppgaver, dagtyper, datoer. Lim inn/CSV fra arket.
3. **Personsiden**: oppgavepreferanser og dato-kalender med tegning/trykk (som i `whenever/`).
4. **Planlegging**: kjør, forhåndsvis, lås, utvalg, forklaringer, godta og publiser.
5. **Plan og eksport**: rutenett som i arket, «mine oppdrag», kopier som tabell/CSV, kalenderfeed per person.
6. **Utrulling på kate**: image, Flux, sikkerhetskopi av SQLite.

**Pilot**: først to-tre oppgaver med manuelle ruter, grupper og utvalg, så resten av oppgavene.

## Resultater fra spiken (fase 0)

Solveren alene, `solver/`: 60 tester (29 regeltester, testdataene fra høsten 2026, 30 tilfeldige instanser), alle grønne.

- **Reglene holder.** Hver regel har en test som feiler hvis regelen fjernes (prøvd ved å bryte modellen én regel av
  gangen). En uavhengig sjekker kontrollerer harde krav på alle planer. Utvalg, gruppe i manuell rute (med kombinasjonsinnstillingen), den låste
  personen og tekstrutene virker på testdataene (18 personer, 17 datoer, 7 oppgaver, ca. 120 tildelinger).
- **Størrelse og tid.** Etter at hvile og dagstak ble tatt ut, kommer en gyldig og brukbar plan etter ca. 2 sekunder, og
  kvaliteten bedres til ca. 20 s. Solveren **beviser ikke** at planen er best mulig (øvre grense tettner ikke), så resultatet
  er et *forslag* innenfor tidsgrensen. Ulike frø gir litt ulike planer; «prøv en gang til» er derfor en nyttig knapp.
  Anbefalt tidsgrense i appen: 30 s. Gjentas med `solver/scripts/timing.py`.
- **Fordelingen måtte rettes to ganger.**
  1. Heltall skalert med total kapasitet og `abs` ga svak relaksasjon og dårlig konvergens. Byttet til bånd.
  2. Med samlet antall som hovedledd ble fordelingen *per oppgave* skjev: i en kjøring tok én person 7 forbønner mens en
     annen fikk 0. Nå er oppgaveleddet hovedleddet. Målt med `solver/scripts/fairness_experiment.py` (8 s, fire frø):

     | | verste avvik fra egen andel per oppgave | største spredning per oppgave | verste samlet avvik |
     |---|---|---|---|
     | gammelt vektsett (samlet først) | 4,1–5,1 oppdrag | 7–8 | 0,9–1,9 |
     | nåværende (oppgave først) | 1,0 | 4 | 4,4–5,4 |

     **Prisen er at samlet antall oppdrag per person blir mer ulikt.** Er det feil avveining, endres det med vektene
     `w_fair` og `w_task_fair`. Det er en beslutning for deg, ikke en teknisk detalj.
- **Historikk måtte være per oppgave** for å virke når oppgaveleddet er hovedleddet. Støttet som `{person: {oppgave: antall}}`.
- **Vekter er ikke justert mot ekte ønsker.** De står i `DEFAULT_PARAMS` og er bare rimelige startverdier.
- **Oppgavepar uten innstilling** regnes som *upraktisk* (`default_combination`). I én kjøring delte 9–13 personer to
  oppgaver samme dag, de fleste par uten innstilling (altså upraktiske) og noen ønskede. Admin må derfor sette opp
  kombinasjonene med omhu.

## Testing

- Ett lite eksempel per regel (hvert krav har en test som feiler hvis det ikke holdes).
- **Uavhengig sjekker** som kontrollerer at en ferdig plan ikke bryter noe hardt krav. Tilfeldige instanser
  kjøres gjennom solver + sjekker.
- Fast testplan basert på en kirkes høst 2026 med oppdiktede navn. Ekte navn og telefon- og e-postlister hører ikke inn i repoet.

## Valg jeg tar som standard hvis du ikke sier noe annet

1. Navn: «Tjenesteplan» (arbeidstittel). Adresse bestemmes ved utrulling.
2. Ingen innlogging i første versjon. Uoffentlig adresse, `noindex`, endringslogg.
3. Solver som Python-underprosess i samme image som Go-backenden.
4. Én plan per periode (f.eks. semester), med historikk som telles med i neste.
5. Eksport via kopier/CSV. Ingen direkte skriving i Google Sheets.

## Risiko

- Vektene for poeng og jevn fordeling må finjusteres med ekte ønsker, ellers føles forslagene rare.
- Folk må faktisk fylle ut. Derfor bare én dato-kalender per person og skjult det som er uvedkommende.
- Uten innlogging kan en feilklikk eller en uvitende bruker endre andres svar. Endringsloggen og sikkerhetskopien er
  det som fanger det.
- Python i Go-imaget gjør bygget og imaget større. Avgjøres i fase 1.
