# Lab 10 — Containerize `notes`: A Distroless, Non-Root, 12-Factor Image and the Full Local Stack

> **Time:** ~5 hours (Friday studio). **Prerequisites:** Lectures 1–3; Exercises 1–3; ideally both challenges. A working Week 9 instrumented `notes` service. **Citations:** the multi-stage guide at <https://docs.docker.com/build/building/multi-stage/>, the distroless repo at <https://github.com/GoogleContainerTools/distroless>, 12factor.net (config/logs/processes/disposability), and the Compose spec at <https://docs.docker.com/compose/>.

This is the Week 10 lab and the first deliverable of the capstone's container axis. You take the same `notes` service you built across Weeks 5–9 — the `chi` REST surface, the gRPC server, the `pgx`+`sqlc` data layer, the `slog`/OTel/Prometheus instrumentation — and you package it as a **multi-stage, distroless, non-root image**, configure it **entirely through the environment**, and bring up the **full local stack** with one command. You do not build a new project. You containerize the one you have.

## What you ship

1. **The Dockerfile.** Multi-stage: `golang:1.22` build stage with the module-download-before-source cache ordering and BuildKit cache mounts; static `CGO_ENABLED=0` compile with `-trimpath -ldflags="-s -w"`; `gcr.io/distroless/static-debian12:nonroot` runtime stage; `USER 65532:65532`; high ports exposed. Under 25 MB.
2. **The `.dockerignore`.** Keeps `.git/`, `*.env`, `testdata`, the manifests, and the compose files out of the build context.
3. **The env-only config.** `internal/config` loads every setting from `NOTES_*` env vars once at startup, validates, fails fast on a missing required value, and never logs a secret. No flags, no checked-in config files.
4. **The `compose.yaml`.** Brings up `notesd` + Postgres + Jaeger + Prometheus + Grafana, wired by environment variables, gated on Postgres's healthcheck.
5. **The measurement.** A `MEASUREMENTS.md` (from challenge-01 if you did it, else produced here) with the four-base size table and the CVE comparison, explaining why the production image is the size it is.

## The build, end to end

```bash
# 1. Build the image and confirm it is small, static, non-root.
docker build -t notesd:local .
docker images notesd:local --format "{{.Size}}"            # ~18 MB
docker inspect notesd:local --format '{{.Config.User}}'    # 65532:65532

# 2. Bring up the full stack.
docker compose up --build -d

# 3. The service answers, configured entirely from the environment.
curl -s http://localhost:8080/healthz       # ok       (liveness)
curl -s http://localhost:8080/readyz        # ok       (readiness — DB reachable)

# 4. A round-trip through Postgres, observable end-to-end.
curl -s -XPOST http://localhost:8080/notes -d '{"title":"lab10","body":"shipped in a box"}'
curl -s http://localhost:8080/notes          # the note, from Postgres

# 5. The trace in Jaeger, the metrics in Grafana.
open http://localhost:16686                  # service "notes": handler -> service -> pgx spans
open http://localhost:3000                   # the RED dashboard reflects the request

# 6. Tear down.
docker compose down -v
```

## Acceptance criteria

A passing Lab 10 satisfies all of the following, demonstrably:

- **The image is small, static, non-root.** Under 25 MB; `ldd` on the binary says "not a dynamic executable"; `docker inspect` shows `65532:65532`; there is no shell (`docker run --rm --entrypoint /bin/sh notesd:local` fails — there is no `/bin/sh`).
- **Config is entirely environmental.** `grep -rn "os.Getenv" cmd/ internal/` shows reads only in `internal/config`; no checked-in config files; a missing `NOTES_DATABASE_URL` fails startup with a clear stderr message and exit 1; no secret appears in the logs.
- **The stack composes.** `docker compose up --build` brings up all five services; `notes` waits for Postgres healthy; `/healthz`, `/readyz`, a `POST`+`GET` round-trip, the Jaeger trace, and the Grafana dashboard all work from the composed stack.
- **The image is portable.** The *same image* runs with `docker run` (pointing `NOTES_DATABASE_URL` at a standalone Postgres) and in the compose stack (pointing it at the `postgres` service) with no rebuild — only the env value changes.
- **It is measured.** `MEASUREMENTS.md` documents the size across bases and the CVE comparison, with each number explained.
- **Clean under the C30 bar.** `go vet ./...`, `staticcheck ./...`, and `go test -race ./...` are green (the config struct and its test included).

## How this feeds the capstone

This lab is the capstone's **deliverable 5 (Container)** in miniature: "a multi-stage, distroless, non-root image, configured entirely through the environment (12-factor), with a `docker compose` that brings up the full local stack." The image you produce here is the *exact artifact* Week 11 deploys to Kubernetes — the `kind` Deployment runs this image, the ConfigMap and Secret supply the same `NOTES_*` environment variables the compose stack supplies, and the readiness probe hits the same `/readyz` that answers here. Nothing about the image changes between this lab and the cluster; only where its environment comes from.

Keep the `compose.yaml` too — it is the local mirror you will reach for whenever a cluster behaviour is confusing and you want to reproduce it on your laptop with the same dependency shape. "Does it work in a box, with its dependencies, configured from the environment?" is the question this lab answers, and it is the question that has to be "yes" before the cluster question is worth asking.

## Submission

Push the Dockerfile, `.dockerignore`, `internal/config`, the `compose.yaml`, the observability config under `deploy/observability/`, and `MEASUREMENTS.md` to the `notes` repo on a `week10-lab` branch and open a PR. The PR description states the final image size, links the `MEASUREMENTS.md`, and includes the `docker compose up` → `curl /readyz` → Jaeger-trace screenshot sequence that proves the stack works end to end. The single most common review comment is "your image runs as root" or "config is in a file" — preempt both by pasting the `docker inspect ... .Config.User` output and the `grep -rn os.Getenv` output in the PR description.
