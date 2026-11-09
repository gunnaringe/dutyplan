---
name: run-app
description: Start og prøv Tjenesteplan-appen (Go-tjener med innebygd frontend) lokalt, eller se at den fungerer uten å lese kode. Bruk når noen vil se appen, kalle API-et, eller sjekke organisasjoner og planer.
---

Alt via mise. Ingen innlogging; organisasjonen ligger i adressen.

- **Se at alt virker uten å åpne noe:** `mise run app:smoke` (ca. 2 s; starter binæren på en midlertidig database, to organisasjoner,
  isolasjon, aktørkrav, en ekte planlegging, omstart; stopper den igjen; én linje) og `mise run app:ui` (headless nettleser, ca. 10 s).
- **Kjøre den selv:** `mise run app:run -- --addr 127.0.0.1:8080 --db /tmp/tp.db` i bakgrunnen (skriver `tjenesteplan: http://...`). Stopp den
  på PID-en du startet, ikke med `pkill`. Standarddatabasen `tjenesteplan.db` ligger i arbeidsmappen; bruk `--db /tmp/...` til prøving.
- **Testdata:** `mise run app:seed -- <adresse>` legger organisasjonen «Test» (`/o/test`) med oppgavene Lovsang, Lyd og Bilde inn i en kjørende app, med Lyd+Bilde upraktisk og begge «kan ikke» med Lovsang, dagtypen Gudstjeneste, åtte personer som har valgt oppgaver, og planen Høst 2026 med 21 søndager; etterpå gir «Planlegg» en plan. Trygt å kjøre flere ganger.
  Standarddatabasen `tjenesteplan.db` (i repoets rot, ikke i git) har dem allerede.
- **API (JSON):** `GET /api/orgs`, `POST /api/orgs {"name"}`, `GET|POST /api/orgs/{org}/plans`, `/people`, `PATCH /people/{id}`,
  `/tasks`, `/day-types` (+ `PUT /{id}/tasks`), `/combinations`, og per plan `/plans/{plan}/events` (+ `/bulk`, `PATCH|DELETE /{id}`),
  Grupper: `GET|POST /groups`, `PATCH|DELETE /groups/{id}`, `PUT /groups/{id}/members` (`{people:[id]}`) og `/tasks` (`{tasks:[{task_id,fills,level}]}`). Vekting per plan: `GET|PUT /plans/{plan}/weights` (`{values:{w_fair: 12, "w_task.godt": 3, default_combination: "praktisk"}}`; erstatter alt, `{}` er standard; svaret lister alle innstillinger med standard og grenser). Per plan: manuelle ruter `GET|POST /plans/{plan}/cells` (`{events:[id], task, person|group|label, fills}`), `DELETE /cells/{id}`, utvalg `GET|PUT /plans/{plan}/pools` (`{events:[id], task, people:[id], groups:[id]}`; tomt fjerner det).
  `GET /api/orgs/{org}/log?plan=&limit=`. Personens svar: `GET /plans/{plan}/people/{id}/me`, `PUT|DELETE /people/{id}/tasks/{task}` (`{level}`), `PUT /plans/{plan}/people/{id}/availability` (`{levels: {eventId: ikke|darlig|godt|""}}`). Skrivinger trenger `Content-Type: application/json` og `X-Actor: <prosentkodet navn>`.
  Eksempel: `curl -s -H 'Content-Type: application/json' -H 'X-Actor: Ola' -d '{"name":"Menigheten"}' $URL/api/orgs`.
- **Planlegge:** `mise run app:run` har planleggeren via `$TJENESTEPLAN_SOLVER`; uten den svarer `POST /api/orgs/{org}/plans/{plan}/runs` 503. Start: `POST` (`{"time_limit":10}`, valgfritt) gir `{id}` med en gang; hent `GET .../runs/{id|latest}` til `run.status` er `done` eller `failed` (liste: `GET .../runs`). `run.violations` skal være tom; `run.stale` betyr at svar eller oppsett er endret siden.
- Bruk bare oppdiktede navn i data du legger inn og committer.
