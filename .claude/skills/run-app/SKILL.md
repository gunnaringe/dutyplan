---
name: run-app
description: Start og prøv Tjenesteplan-appen (Go-tjener med innebygd frontend) lokalt, eller se at den fungerer uten å lese kode. Bruk når noen vil se appen, kalle API-et, eller sjekke organisasjoner og planer.
---

Alt via mise. Ingen innlogging; organisasjonen ligger i adressen.

- **Se at alt virker uten å åpne noe:** `mise run app:smoke` (ca. 2 s; starter binæren på en midlertidig database, to organisasjoner,
  isolasjon, aktørkrav, omstart; stopper den igjen; én linje) og `mise run app:ui` (headless nettleser, ca. 10 s).
- **Kjøre den selv:** `mise run app:run -- --addr 127.0.0.1:8080 --db /tmp/tp.db` i bakgrunnen (skriver `tjenesteplan: http://...`). Stopp den
  på PID-en du startet, ikke med `pkill`. Standarddatabasen `tjenesteplan.db` ligger i arbeidsmappen; bruk `--db /tmp/...` til prøving.
- **API (JSON):** `GET /api/orgs`, `POST /api/orgs {"name"}`, `GET|POST /api/orgs/{org}/plans`, `/people`, `PATCH /people/{id}`,
  `/tasks`, `/day-types` (+ `PUT /{id}/tasks`), `/combinations`, og per plan `/plans/{plan}/dates` (+ `/bulk`, `PATCH|DELETE /{id}`),
  `GET /api/orgs/{org}/log?plan=&limit=`. Skrivinger trenger `Content-Type: application/json` og `X-Actor: <prosentkodet navn>`.
  Eksempel: `curl -s -H 'Content-Type: application/json' -H 'X-Actor: Ola' -d '{"name":"Menigheten"}' $URL/api/orgs`.
- Bruk bare oppdiktede navn i data du legger inn og committer.
