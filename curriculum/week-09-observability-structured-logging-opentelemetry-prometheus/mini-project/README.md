# Mini-Project (Lab 09) — Instrument `notes`: slog, OpenTelemetry, Prometheus RED, and a Grafana Dashboard

> Take the `notes` service you built across Weeks 5–8 — chi REST handler → service → Postgres repository, with a gRPC surface — and make it observable. Add `log/slog` structured logging correlated with trace IDs, OpenTelemetry tracing across the HTTP handler → service → Postgres path exported to a local Jaeger, and Prometheus RED metrics behind a provisioned Grafana dashboard. Bring the whole stack up with one `docker compose up`. Then inject an artificial slow query and localize it, from the dashboard down to the trace span, in under five minutes. By the end you have a `notes` service a stranger could debug at 3am.

This is the canonical "make a real service observable" exercise. The shape is production-shaped: structured logs that carry a trace ID, a trace per request from the front door to the database, RED metrics on every route, a dashboard, and a trace backend — all wired so the three signals point at the same request through one identifier. Every senior engineer who has been on call has built or wished for exactly this. The lab is that experience in microcosm, and the deliverable is the discipline of localizing a regression with the tools rather than by guessing.

**Estimated time:** ~13 hours (split across the weekend in the suggested schedule). If your `notes` service from Weeks 5–8 runs, you are instrumenting, not building, and you will be at the lower end.

---

## What you will build

You start from the `notes` service (Weeks 5–8) and add an `internal/observability` package plus a `compose/` stack. Concretely:

- **Structured logging with slog, correlated with trace IDs.** A JSON `slog.Handler` in production (text in dev), a `*slog.LevelVar` so on-call can raise verbosity at runtime, a request-scoped logger bound with `request_id` per request, and a custom `ContextHandler` that stamps the live `trace_id` onto every log record. Every request emits a "request started" and a "request completed" line, both carrying the trace ID.
- **OpenTelemetry tracing across HTTP → service → Postgres.** The OTel SDK initialized once (resource with `service.name=notes`, a `BatchSpanProcessor`, an OTLP/gRPC exporter to a local Jaeger, the W3C `TraceContext` propagator). The HTTP entry wrapped with `otelhttp`. A span per layer: the server span, a `service.*` span, and a `db.query.*` span (via `otelpgx` or manual). Clean shutdown that flushes buffered spans.
- **Prometheus RED metrics on every route.** An `http_requests_total{method,route,code}` counter and an `http_request_duration_seconds{method,route}` histogram, registered on an explicit registry, exposed at `/metrics`, recorded in a middleware that labels by the chi route *template* (never the concrete path).
- **A docker-compose stack** bringing up `notes` + Postgres + Jaeger + Prometheus + Grafana, with Grafana provisioned to load a RED dashboard (Rate, Errors, Duration panels) from JSON on startup.
- **An injected slow query and its localization.** A code path (behind `NOTES_INJECT_SLOW`) that adds `pg_sleep` to one route's query, plus a `LOCALIZATION.md` documenting the dashboard → route → trace → span walk.

You ship one repository: the instrumented `notes` plus the `compose/`, `prometheus.yml`, and `grafana/` provisioning.

---

## Rules

- **You may** read the `log/slog`, OpenTelemetry-Go, and `client_golang` documentation, the Week 9 lecture notes and exercises, the Jaeger/Prometheus/Grafana docs, and any free Go documentation.
- **Allowed dependencies** (and only these, beyond what `notes` already used in Weeks 5–8):
  - `log/slog` (standard library — no third-party logger).
  - `go.opentelemetry.io/otel`, `.../otel/sdk`, `.../otel/exporters/otlp/otlptrace/otlptracegrpc`, and the contrib packages `.../contrib/instrumentation/net/http/otelhttp` and `.../contrib/instrumentation/google.golang.org/grpc/otelgrpc`.
  - `github.com/prometheus/client_golang`.
  - `github.com/go-chi/chi/v5` (already in use from Week 5).
  - For the DB span, `github.com/exaring/otelpgx` is allowed; or instrument the query with a manual span and no new dependency.
- Target Go **1.22 or later**.
- **Context is propagated everywhere.** Every function that does I/O takes `ctx context.Context` first, and every span/DB/gRPC call runs under it. A dropped context is an automatic deduction.
- **`trace_id` appears in every request log line.** Verified by reading the logs.
- **RED metrics on every route**, labeled by bounded dimensions only — no unbounded labels (no note id, no user id, no concrete path).
- **Clean shutdown flushes traces.** `TracerProvider.Shutdown` deferred with a bounded context; `main` returns on a signal, never `os.Exit` on the happy path.
- **`EnableDetailedErrors`-style debug surfaces are off in the production branch.** This is a production-shaped service.

---

## Project structure

```
notes/
├── go.mod
├── cmd/
│   └── notes/
│       └── main.go                  # wiring: logger, tracer init+shutdown, metrics, router
├── internal/
│   ├── observability/
│   │   ├── logger.go                # slog setup, LevelVar, ContextHandler, LoggerFrom
│   │   ├── tracer.go                # InitTracer (resource, TP, OTLP exporter, propagator, Shutdown)
│   │   ├── metrics.go               # Metrics struct, RED middleware, registry
│   │   └── middleware.go            # RequestLogger middleware (request_id, trace_id, start/end)
│   ├── handler/                     # chi handlers (Week 5) — now under spans + scoped logger
│   ├── service/                     # service layer (Week 5) — opens a service.* span
│   └── repository/                  # pgx/sqlc repo (Week 6) — opens a db.query.* span; slow path
├── compose/
│   └── docker-compose.yml           # notes + postgres + jaeger + prometheus + grafana
├── prometheus.yml                   # scrape config for the notes /metrics endpoint
└── grafana/
    ├── provisioning/
    │   ├── datasources/prometheus.yml   # auto-add the Prometheus datasource
    │   └── dashboards/dashboards.yml    # tell Grafana to load dashboards/ from disk
    └── dashboards/
        └── notes-red.json               # the RED dashboard: Rate, Errors, Duration panels
```

---

## Acceptance criteria

### Logging

- [ ] Production emits JSON via `slog.NewJSONHandler`; development emits text. The choice is one branch at startup.
- [ ] The log level is driven by a `*slog.LevelVar`, changeable at runtime (an admin endpoint or a signal handler that flips it).
- [ ] A request-scoped logger is derived per request with `slog.With(request_id, method, path)` and stashed in the request context; deeper layers retrieve it with `LoggerFrom(ctx)`.
- [ ] A custom `ContextHandler` (or the middleware) injects the live `trace_id` onto every log record.
- [ ] Every request produces a "request started" and a "request completed" line, the latter with status, route template, and duration — both carrying the `trace_id`.
- [ ] No sensitive field is logged raw (redact via `LogValuer` or `ReplaceAttr` if any exist).

### Tracing

- [ ] `InitTracer` builds a resource with `semconv.ServiceName("notes")` — Jaeger shows `notes`, not `unknown_service`.
- [ ] The `TracerProvider` uses `WithBatcher`, exports OTLP to the local Jaeger (port 4317), and sets `propagation.TraceContext{}`.
- [ ] `otelhttp.NewHandler` is the outermost HTTP wrapper.
- [ ] One request produces one connected trace: server span → `service.*` span → `db.query.*` span, visible in Jaeger.
- [ ] The DB span carries `db.system`, `db.operation`, and `db.statement` attributes.
- [ ] Failure paths call `span.RecordError` and `span.SetStatus(codes.Error, ...)`; failed spans are red in Jaeger.
- [ ] `TracerProvider.Shutdown` is deferred with a bounded context and flushes on a clean exit.
- [ ] (If the gRPC surface is wired) `otelgrpc` handlers are installed on both gRPC ends.

### Metrics

- [ ] `http_requests_total{method,route,code}` counter and `http_request_duration_seconds{method,route}` histogram, registered on an explicit `prometheus.Registry`.
- [ ] `/metrics` is served by `promhttp.HandlerFor` and returns valid exposition text.
- [ ] The route label is the chi route template; no metric label is unbounded.
- [ ] The RED middleware runs inside the `otelhttp` wrapper so the route pattern resolves.
- [ ] The histogram uses deliberate buckets (`DefBuckets` or explicit) and is not labeled by `code`.

### Dashboard and localization

- [ ] `docker compose up` brings up `notes` + Postgres + Jaeger + Prometheus + Grafana, all healthy.
- [ ] Prometheus scrapes the `notes` `/metrics` endpoint (the `notes` target is `up` at <http://localhost:9090/targets>).
- [ ] Grafana auto-provisions the Prometheus datasource and loads the RED dashboard with three panels: Rate (`rate`), Errors (5xx ratio), Duration (p99 via `histogram_quantile`).
- [ ] Setting `NOTES_INJECT_SLOW=true` makes one route's `db.query.*` span ~400ms slower.
- [ ] The Duration panel's p99 for the affected route spikes within two scrape intervals while other routes stay flat.
- [ ] `LOCALIZATION.md` documents the dashboard → route → trace → span walk with the actual trace ID, and the recovery when the injection is removed.

---

## The local stack and ports

The `compose/docker-compose.yml` brings up five services. The ports you will use:

| Service    | Port  | URL / purpose                                   |
|------------|-------|-------------------------------------------------|
| `notes`    | 8080  | the app; `/metrics` and the REST API            |
| Postgres   | 5432  | the database                                    |
| Jaeger     | 16686 | the trace UI — <http://localhost:16686>         |
| Jaeger     | 4317  | the OTLP/gRPC receiver the app exports to        |
| Prometheus | 9090  | the metrics UI — <http://localhost:9090>        |
| Grafana    | 3000  | the dashboard — <http://localhost:3000>         |

Jaeger's all-in-one image ingests OTLP directly (`COLLECTOR_OTLP_ENABLED=true`); you do not need a separate collector. Grafana provisioning auto-adds the Prometheus datasource and loads `grafana/dashboards/notes-red.json` on startup, so the dashboard exists the moment the stack is up — no manual import. If any of these ports is taken on your machine, remap it in the compose file before you start.

---

## Day-by-day plan

### Thursday (3h) — slog and the request logger

1. Create `internal/observability/logger.go`: the JSON/text branch, the `*slog.LevelVar`, `slog.SetDefault`, and the `ContextHandler` that injects `trace_id`.
2. Create `middleware.go`: the `RequestLogger` middleware (request_id, scoped logger, start/end lines). Add `LoggerFrom`/`ContextWithLogger` helpers.
3. Thread `LoggerFrom(ctx)` into the handler and service layers so they log through the scoped logger.
4. Run `notes` locally and confirm structured JSON with a consistent `request_id` per request.

### Friday (3h) — OpenTelemetry tracing

1. Create `tracer.go`: `InitTracer` (resource, `TracerProvider` with `WithBatcher`, OTLP exporter, propagator) returning a `Shutdown` closure.
2. In `main.go`, call `InitTracer`, defer `Shutdown` with a bounded context, and wrap the router with `otelhttp.NewHandler` as the outermost layer (the logging middleware inside it).
3. Open a `service.*` span in the service layer and a `db.query.*` span in the repository (via `otelpgx` or manual). Pass the right context down at every layer.
4. Bring up Jaeger (`docker compose up -d jaeger`), drive a few requests, and confirm a three-span trace in Jaeger with the `trace_id` matching the logs.

### Saturday (4h) — Prometheus RED + the full stack

1. Create `metrics.go`: the `Metrics` struct, the RED counter+histogram on an explicit registry, the middleware, and `/metrics` via `promhttp`.
2. Write `prometheus.yml` (scrape `notes:/metrics`) and the Grafana provisioning (datasource + dashboards loader) and `notes-red.json` (three RED panels).
3. Complete `compose/docker-compose.yml`: `notes`, Postgres, Jaeger, Prometheus, Grafana. `docker compose up`.
4. Confirm: Prometheus `notes` target is `up`; the Grafana RED dashboard renders with live data under load.

### Sunday (3h) — Localize the regression and write it up

1. Establish a baseline under steady load; record the healthy p99/rate/error numbers.
2. Set `NOTES_INJECT_SLOW=true`, restart `notes`, keep the load running.
3. Localize: watch the Duration panel spike for the affected route; confirm with the PromQL; open the slowest trace in Jaeger; find the slow `db.query.*` span; confirm in the logs by trace ID.
4. Remove the injection; confirm recovery. Write `LOCALIZATION.md`.
5. Final pass: `go build ./...` clean, `go vet ./...` clean, the stack comes up from a cold `docker compose up`.

---

## What you will be graded on

| Area                                                                       | Weight |
|----------------------------------------------------------------------------|-------:|
| Structured logging (scoped logger, JSON/text, LevelVar, no raw secrets)    |  15% |
| Log/trace correlation (`trace_id` on every request log line)               |  10% |
| Tracing wiring (resource, batcher, OTLP, propagator, clean shutdown)       |  20% |
| Span coverage (handler → service → Postgres, attributes, error status)     |  15% |
| RED metrics (correct instruments, cardinality discipline, `/metrics`)      |  15% |
| The stack (compose brings up all five; Prometheus scrapes; Grafana panels) |  15% |
| Localization write-up (dashboard → route → trace → span, with numbers)     |  10% |
| **Total**                                                                  | **100%** |

The passing bar is **80**. The "a stranger could debug this at 3am" bar is **90** — which means the localization actually works end to end, not just that the pieces compile.

---

## A note on the local stack

Some learners will run this in an environment without Docker installed. Installing Docker Desktop is your responsibility — <https://www.docker.com/products/docker-desktop/> is free for individuals and small teams. The starter files assume `docker compose version` works and that ports 8080, 5432, 16686, 4317, 9090, and 3000 are free. The Go side needs no special tooling beyond `go 1.22+`; the OTLP exporter, `client_golang`, and the contrib instrumentation packages are ordinary `go get` dependencies. If a port is taken, remap it in the compose file before you start — a half-up stack is the most confusing way to fail this lab.

---

## Submission

Commit the instrumented `notes` plus the `compose/`, `prometheus.yml`, and `grafana/` provisioning (excluding `bin/` and any local data volumes). Open a PR against `main` with the commit message:

```
mini-project (lab 09): instrument notes with slog, opentelemetry, and prometheus RED metrics
```

The PR description should include:

1. A `slog` JSON log excerpt showing a "request started"/"request completed" pair with the same `request_id` and `trace_id`.
2. A screenshot or text description of one Jaeger trace: server → service → DB span, with the trace ID.
3. A `curl localhost:8080/metrics | grep http_request` excerpt showing the RED instruments labeled by route template.
4. The `LOCALIZATION.md` walk: baseline numbers, the post-injection p99 spike on the dashboard, the trace ID of the slow request, and the recovery.

If the stack does not come up from a cold `docker compose up`, the PR is not reviewable — fix that first.
