# Exercise 03 — The Full Local Stack with Compose

> **Time:** ~90 minutes. **Prerequisites:** Lectures 1–3; Exercises 1 and 2; `docker compose` v2. The Week 9 Prometheus scrape config and Grafana dashboard.

## Goal

Write a `compose.yaml` that brings up the whole stack — `notesd` + Postgres + Jaeger + Prometheus + Grafana — with one command, wired entirely by environment variables, gated on Postgres's healthcheck. The local topology mirrors the cloud one.

## Steps

1. **Create `compose.yaml`** at the repo root with five services: `postgres` (with a `pg_isready` healthcheck and a named volume), `jaeger` (`all-in-one`, OTLP enabled, UI on 16686, OTLP/HTTP on 4318), `prometheus` (mounting the Week 9 scrape config), `grafana` (mounting the Week 9 dashboard provisioning, anonymous admin), and `notes` (built from the Dockerfile).

2. **Configure `notes` entirely through `environment:`** — `NOTES_DATABASE_URL` pointing at the `postgres` service by name, `NOTES_OTLP_ENDPOINT: "jaeger:4318"`, the three listen addresses, and the log level.

3. **Gate `notes` on the DB** with `depends_on: postgres: condition: service_healthy`.

4. **Map ports** carefully: `notes` REST 8080, gRPC 9090, metrics 2112; Jaeger UI 16686; Grafana 3000; Prometheus on host 9091 (so it does not collide with the gRPC 9090).

5. **Create `deploy/observability/prometheus.yml`** scraping `notes:2112` (the in-network service name and metrics port).

6. **Bring it up** and verify end-to-end:
   ```bash
   docker compose up --build -d
   curl -s http://localhost:8080/healthz   # ok
   curl -s http://localhost:8080/readyz    # ok (DB reachable)
   curl -s -XPOST http://localhost:8080/notes -d '{"title":"t","body":"b"}'
   curl -s http://localhost:8080/notes      # the note
   # Jaeger: open http://localhost:16686, search service "notes"
   # Grafana: open http://localhost:3000, the RED dashboard shows the request
   ```

## Acceptance criteria

- `docker compose up --build` brings all five services up; `notes` waits for Postgres healthy.
- `/healthz` and `/readyz` both return `ok` once the stack settles.
- A `POST /notes` then `GET /notes` round-trips through Postgres.
- The request's trace appears in Jaeger with the handler → service → Postgres span tree.
- Prometheus scrapes `notes:2112` (check `http://localhost:9091/targets` — the `notes` target is `UP`) and the Grafana RED dashboard reflects the traffic.
- `docker compose down` tears it all down; `docker compose down -v` also drops the Postgres volume.

## Stretch

Add a `compose.override.yaml` that mounts the local source and runs `notes` with `go run` (a fast inner-loop dev mode) while the base `compose.yaml` builds the real image. Document when you would use each.
