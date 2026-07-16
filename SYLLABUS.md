# C30 · Crunch Go — Syllabus

> Cloud-Native Backend in Go
> 12 weeks · ~432 hours · 1 capstone · GPL-3.0
> Crunch Labs tier · sub-brand **Go** (`#00ADD8`)

This document covers the week-by-week plan, assessment matrix, capstone specification, and career engineering pack. For design rationale, see [`CHARTER.md`](./CHARTER.md). For audience and outcomes, see [`README.md`](./README.md).

Twelve weeks. Three phases. One capstone. The detail below is the contract: every week has a title, a topic list, a lecture spine, a named hands-on lab, and three stated skills the cohort should be able to demonstrate before the next week begins.

The language and its concurrency model come first, because every operational property of a cloud-native service is an expression of them. The service and its data layer come second, because you cannot operate what you have not built correctly. The cloud-native surface — observability, containers, Kubernetes, reliability — comes last, when the cohort has a service worth keeping alive.

The week titles below are the canonical curriculum folder slugs.

---

## Phases

| Phase | Weeks | Title | Theme |
| --- | --- | --- | --- |
| **I — The Language & the Runtime** | 1–4 | Go, idiom, and the concurrency model | Modules, generics, interfaces, goroutines, channels, `context`, the race detector |
| **II — Services & Data** | 5–8 | HTTP, Postgres, gRPC, and hard testing | `net/http` + `chi`, `pgx` + `sqlc`, protobuf + gRPC, benchmarks + fuzzing |
| **III — Cloud-Native & Capstone** | 9–12 | Observability, containers, Kubernetes, reliability | `slog` + OTel + Prometheus, distroless + 12-factor, K8s + probes, graceful shutdown, capstone |

---

## Phase I — The Language & the Runtime (Weeks 1–4)

The goal of Phase I is fluency in Go the language and, above all, in its concurrency model — before any service work begins. Everything in this phase runs from the command line on any laptop with the Go toolchain installed.

### Week 1 — The Go Tour & the Toolchain

- **Topics:** Installing Go; `go mod init` and the module system; package layout and visibility; `go build` / `go run` / `go install`; `gofmt`, `go vet`, `staticcheck`; the testing package and table-driven tests; `let`-free Go basics — declarations, slices, maps, structs, the zero value, `defer`.
- **Lecture:** "Go for the engineer who already knows a typed language" — what maps cleanly from Java/C#/TypeScript and what does not (no exceptions, no inheritance, the zero value, error values instead of throws, why the language is so deliberately small). Walk the `strings` package source to show idiomatic standard-library Go.
- **Hands-on:** **Lab 01 — `wordfreq` CLI.** Build a `go run . <file.txt>` tool that counts word frequencies, prints the top 20 as a Markdown table, reads from stdin when given no file, and ships with table-driven tests and a CI-clean `go vet`. Build a static binary and inspect its size.
- **Skills earned:**
  - Structure a Go module, build a static binary, and run the standard toolchain.
  - Write idiomatic Go using slices, maps, structs, and the zero value.
  - Author a table-driven test suite that runs clean under `go vet`.

### Week 2 — Idiomatic Go: Structs, Interfaces, Errors, Generics

- **Topics:** Methods and pointer vs value receivers; interfaces (small, consumer-defined; "accept interfaces, return structs"); embedding vs inheritance; error values, `errors.New`, `fmt.Errorf("%w", err)`, `errors.Is` / `errors.As`, sentinel vs typed errors; generics — type parameters, constraints, `comparable`, when generics earn their keep and when an interface is better.
- **Lecture:** "Idiomatic Go is a short list of strong opinions." The interface-at-the-consumer rule, the error-wrapping contract, why `panic` is not exception handling, and the generics decision matrix (container vs algorithm vs neither).
- **Hands-on:** **Lab 02 — generic `Cache[K comparable, V any]`.** Build a TTL cache with a pluggable eviction policy behind a small interface, typed errors for misses and expiry, and a `Store[K, V]` interface with an in-memory and a file-backed implementation. Full table tests, errors checked with `errors.Is`.
- **Skills earned:**
  - Design a small, consumer-defined interface and defend it in review.
  - Build a wrapped-error chain and inspect it with `errors.Is` / `errors.As`.
  - Decide between generics and interfaces deliberately, with reasons.

### Week 3 — Concurrency I: Goroutines, Channels, `select`

- **Topics:** Goroutines and the scheduler at a high level; channels (buffered vs unbuffered); `select`; the for-range-over-channel pattern; closing channels and the "who closes" rule; `sync.WaitGroup`; deadlocks and goroutine leaks; "share memory by communicating" — and the times you should *not*.
- **Lecture:** "Channels are a synchronisation primitive, not a queue." When a channel is right (pipelines, fan-out/fan-in, signalling) and when a `sync.Mutex` or a plain function call is the better, simpler answer. A live goroutine-leak reproduction with a stuck `select`.
- **Hands-on:** **Lab 03 — concurrent link-checker.** Build a CLI that reads a `sitemap.xml`, fans out to N concurrent HTTP HEAD requests through a channel-based pipeline (default 16), fans results back in, and prints a report. Use `WaitGroup` for completion and prove there are no leaked goroutines.
- **Skills earned:**
  - Build a fan-out / fan-in pipeline with channels and `select`.
  - Apply the "who closes the channel" rule and avoid goroutine leaks.
  - Choose between a channel and a mutex for a given coordination problem.

### Week 4 — Concurrency II: `context`, `sync`, Worker Pools, the Race Detector

- **Topics:** `context.Context` for cancellation, deadlines, and request-scoped values; threading `context` through a call tree; `sync.Mutex` / `RWMutex` / `Once` / `errgroup`; bounded worker pools (semaphore channel, `errgroup.SetLimit`); the data-race model and the memory model; `go test -race`; `go test -bench` and `pprof` first contact.
- **Lecture:** "`context` is the cancellation backbone of every Go service." Why it is threaded everywhere, what the race detector can and cannot prove, and a worked example of removing a real data race the detector caught.
- **Hands-on:** **Lab 04 — bounded worker pool.** Convert Lab 03's link-checker into a bounded worker pool with `errgroup`, full `context` cancellation (graceful Ctrl-C and a `--timeout`), and a deliberately-introduced data race that you then find and fix with `go test -race`. Benchmark throughput at three pool sizes.
- **Skills earned:**
  - Thread `context` through a workload for cancellation and deadlines.
  - Build a bounded worker pool that does not leak goroutines.
  - Find and eliminate a data race with `go test -race` and read a benchmark.

**Phase I gate.** Demo the bounded, race-free, `context`-cancellable worker pool from Lab 04 with a benchmark sweep and a clean `go vet` / `staticcheck` / `go test -race` run.

---

## Phase II — Services & Data (Weeks 5–8)

The cohort now builds a real HTTP and gRPC service backed by Postgres, and tests it the way a senior engineer would. Everything runs locally; Postgres runs in a container via `docker compose`.

### Week 5 — Building HTTP Services: `net/http`, `chi`, Middleware

- **Topics:** `net/http` server, `http.Handler` / `HandlerFunc`, the 1.22 routing patterns; `chi` router and sub-routers; middleware (request ID, structured logging, panic recovery, timeout); the handler → service → repository seam; request/response JSON with `encoding/json`; reading and validating input; sensible HTTP status and error responses.
- **Lecture:** "A grown-up HTTP service in Go, standard-library-first." From `http.ListenAndServe` to a layered service with composable middleware — and why we reach for `chi`, not a framework.
- **Hands-on:** **Lab 05 — `notes-api` (REST).** Build a `notes` service with `POST /notes`, `GET /notes`, `GET /notes/{id}`, `PATCH /notes/{id}`, `DELETE /notes/{id}`, an in-memory repository behind an interface, request-ID + logging + recovery middleware, and httptest-based handler tests. Returns correct status codes and JSON errors.
- **Skills earned:**
  - Build a layered HTTP service with `net/http` and `chi`.
  - Write composable middleware (request ID, logging, recovery, timeout).
  - Test handlers with `httptest` against a clean handler/service/repo seam.

### Week 6 — Databases: `pgx`, `sqlc`, Migrations, Transactions

- **Topics:** Postgres in a container; `pgx` connection pools; `sqlc` — writing SQL, generating type-safe Go; `golang-migrate` for forward-and-back schema migrations; transactions and the `BeginTx` pattern; isolation levels and concurrent-write hazards; `context` deadlines on every query; the repository interface backed by Postgres.
- **Lecture:** "Type-safe SQL without an ORM." Why `sqlc` over `GORM` for a service that values legible queries, how a transaction boundary maps to a use case, and the concurrency hazards (lost update, write skew) the cohort must reason about.
- **Hands-on:** **Lab 06 — `notes-api` on Postgres.** Replace the in-memory repository with a Postgres-backed one. Write the schema as `golang-migrate` migrations (up and down), generate the query layer with `sqlc`, wrap a multi-step update in a transaction, and write integration tests that run against a real Postgres container. Demonstrate a clean `migrate down` and re-up.
- **Skills earned:**
  - Build a `pgx` + `sqlc` data layer with compile-time-checked queries.
  - Author reversible schema migrations with `golang-migrate`.
  - Use transactions correctly and reason about concurrent-write hazards.

### Week 7 — gRPC & Protocol Buffers

- **Topics:** Protocol Buffers (`.proto` syntax, messages, services, field evolution); `buf` for linting and generation; `protoc-gen-go` / `protoc-gen-go-grpc`; the gRPC server and client; unary vs streaming RPCs; interceptors (the gRPC analogue of middleware); status codes and error details; serving REST and gRPC over the same business logic.
- **Lecture:** "One domain, two transports." Why cloud-native services speak gRPC to each other and REST to the world, how to share a service layer behind both, and how protobuf field evolution keeps a contract backward-compatible.
- **Hands-on:** **Lab 07 — `notes` over gRPC + REST.** Define the `notes` service in a `.proto`, generate Go with `buf`, implement the gRPC server against the *same* service/repository layer from Lab 06, add a logging + recovery interceptor, and stand up a gRPC client. Keep the REST surface from Lab 05 live against shared logic. Demonstrate both calling the same Postgres-backed service.
- **Skills earned:**
  - Define a service in Protocol Buffers and generate Go with `buf`.
  - Implement a gRPC server and client with interceptors and proper status codes.
  - Serve the same domain over both gRPC and REST from shared logic.

### Week 8 — Testing, Benchmarking & Fuzzing

- **Topics:** Table-driven tests at scale; test doubles via small interfaces; integration tests against a real Postgres (`testcontainers` or a compose harness); golden-file tests; `go test -bench` and `testing.B`; `pprof` (CPU, heap, goroutine, block, mutex profiles); `go test -fuzz` and writing fuzz targets; coverage as a signal, not a goal.
- **Lecture:** "Test like a senior — and let the machine find the inputs you didn't think of." The testing pyramid for a Go service, when an integration test earns its slowness, reading a `pprof` flame graph, and a live fuzz run that finds a parser crash.
- **Hands-on:** **Lab 08 — harden `notes`.** Bring the `notes` service to a strong test posture: table-driven unit tests, integration tests against a Postgres container, a benchmark for the hot path, a `pprof` capture with one measured optimisation, and a fuzz target against the input-parsing/validation code that surfaces and fixes at least one crash.
- **Skills earned:**
  - Build a layered test suite — unit, integration, benchmark — for a real service.
  - Capture and read a `pprof` profile and ship a measured optimisation.
  - Write a fuzz target that finds a real input-handling bug and fix it.

**Phase II gate.** Demo the `notes` service serving both gRPC and REST against Postgres, with reversible migrations, an integration-test suite green against a real database, a benchmark, and a fuzz target that found and fixed a bug.

---

## Phase III — Cloud-Native & Capstone (Weeks 9–12)

The service becomes a cloud-native citizen: observable, containerised, configured for the environment, deployed to Kubernetes, and reliable under failure. Then the capstone integrates all of it.

### Week 9 — Observability: Structured Logging, OpenTelemetry, Prometheus

- **Topics:** `log/slog` structured logging (handlers, levels, request-scoped attributes, log/trace correlation); OpenTelemetry SDK — tracer, spans, context propagation across the request and across gRPC; exporting traces to **Jaeger**; Prometheus metrics (`promhttp`, counters, histograms, the RED method — rate, errors, duration); a Grafana dashboard over the metrics.
- **Lecture:** "Observability is the operational contract of a cloud-native service." The three signals (logs, traces, metrics), how a trace ID ties them together, and how to localise a latency regression from a Grafana dashboard down to a span.
- **Hands-on:** **Lab 09 — instrument `notes`.** Add `slog` structured logging correlated with trace IDs, OpenTelemetry tracing across the HTTP handler → service → Postgres path exported to a local Jaeger, and Prometheus RED metrics behind a Grafana dashboard. Inject an artificial slow query and localise it from the dashboard to the trace span.
- **Skills earned:**
  - Emit structured, trace-correlated logs with `slog`.
  - Instrument a request path with OpenTelemetry and read the trace in Jaeger.
  - Expose Prometheus RED metrics and build a Grafana dashboard that localises a regression.

### Week 10 — Containerizing Go: Multi-Stage, Distroless & 12-Factor Config

- **Topics:** Multi-stage Dockerfiles for Go; `CGO_ENABLED=0` static builds; distroless and `scratch` base images; non-root containers; reproducible builds and image size; `.dockerignore`; layer caching; the 12-factor app — config strictly from the environment, no secrets in the image, logs to stdout, stateless processes; healthcheck and signal handling readiness.
- **Lecture:** "A Go container should be small, static, non-root, and start in milliseconds." Why distroless over a full base image, how to keep config out of the binary, and the 12-factor checklist a cloud-native service must pass.
- **Hands-on:** **Lab 10 — containerize `notes`.** Write a multi-stage Dockerfile producing a distroless, non-root image; configure the service entirely through environment variables (12-factor); add a `docker compose` that brings up the service, Postgres, Jaeger, Prometheus, and Grafana together. Measure and shrink the final image; confirm it runs as non-root and reads all config from the environment.
- **Skills earned:**
  - Build a multi-stage, distroless, non-root Go container image.
  - Configure a service for 12-factor deployment (env-only config, stdout logs).
  - Compose a full local stack (service + Postgres + observability) with one command.

### Week 11 — Kubernetes Deployment, Health/Readiness, Graceful Shutdown & Reliability

- **Topics:** `kind`/minikube; Deployment, Service, ConfigMap, Secret; liveness vs readiness probes (and why the difference matters); rolling updates and `maxUnavailable`/`maxSurge`; `SIGTERM`, the termination grace period, and graceful shutdown (stop accepting, drain in-flight, close pools); reliability patterns — timeouts everywhere, retries with exponential backoff + jitter, circuit breaking, load-shedding under saturation.
- **Lecture:** "The contract a service owes the cluster." Why readiness must tell the truth, what happens to an in-flight request on a rolling deploy, how graceful shutdown is just `context` cancellation applied to the whole process, and the reliability patterns that keep a rollout clean under load.
- **Hands-on:** **Lab 11 — deploy `notes` to Kubernetes.** Write Deployment + Service + ConfigMap manifests; wire honest liveness and readiness probes; implement graceful shutdown that drains in-flight requests on `SIGTERM` within the grace period; add request timeouts and a retry-with-jitter client for a downstream dependency. Run a rolling deploy under a small load generator and prove zero dropped requests.
- **Skills earned:**
  - Deploy a Go service to Kubernetes with truthful liveness/readiness probes.
  - Implement graceful shutdown that drains in-flight work on `SIGTERM`.
  - Roll out a new version under load with zero dropped requests using reliability patterns.

### Week 12 — Capstone: Ship a Cloud-Native Microservice

- **Topics:** Capstone integration; architecture decision records; the load-test-and-trace report; the reliability-drill postmortem; the `production-runbook.md`; the senior backend-Go interview loop.
- **Lecture:** "What a hiring manager at a cloud-native shop looks for in a Go service — and how your capstone covers each axis." Demo discipline, postmortem writing, and the staff-engineer questions your reliability drill answers.
- **Hands-on:** **Capstone defense** — see full spec below. Live demo of the deployed service (gRPC + REST, Postgres, Jaeger traces, Grafana dashboard, rolling deploy under load, graceful drain). Reviewer-panel Q&A. Public postmortem of one reliability drill.
- **Skills earned:**
  - Integrate everything from Phases I–II into one deployed cloud-native service.
  - Defend architecture, concurrency, and reliability decisions to a senior reviewer.
  - Ship a capstone artifact you would send to a hiring manager.

**Phase III gate.** The capstone is deployed and healthy on `kind`, the reliability-drill postmortem is signed off, the load-test-and-trace report is published, and the cohort completes the senior backend-Go mock interview.

---

## Assessment matrix

| Component | Weight | Cadence | Format |
| --- | --- | --- | --- |
| Weekly quiz | 10% | Weeks 1–11 | Auto-graded, ~30 min, spec- and standard-library-heavy |
| Weekly lab | 30% | Weeks 1–11 | Reviewed by peers + a TA; must be clean under `go vet` / `staticcheck` / `go test -race` |
| Phase gates (×2, end of Phases I–II) | 15% | Weeks 4, 8 | Code review + live demo |
| Capstone | 30% | Weeks 9–12 | Deployed service, 5-min video, load-and-trace report, postmortem |
| Reliability drill | 5% | Week 11 | Timed rolling-deploy-under-load + dependency-outage incident |
| Mock interview | 10% | Week 12 | 60-min senior backend-Go loop with an external reviewer |

Passing bar: **70% overall**, AND a passing capstone, AND a passing reliability drill. A weak quiz week is forgivable; a non-functional capstone is not. No phase gate may fall below 60%.

---

## Capstone — Ship a Cloud-Native Microservice

The capstone is **one** substantial service taken all the way to a running, observable, gracefully-shutting-down Kubernetes deployment — not a parade of toy programs. You will build, deploy, and operate:

```text
                         +-----------------------------+
                         |        Kubernetes (kind)    |
                         |                             |
   REST (clients) -----> |  +-----------------------+  |
                         |  | Deployment: svc       |  | ---> OTel traces ---> Jaeger
   gRPC (services) ----> |  |  - liveness probe     |  |
                         |  |  - readiness probe    |  | ---> /metrics ---> Prometheus ---> Grafana
                         |  |  - graceful SIGTERM   |  |
                         |  |  - 12-factor config   |  | ---> stdout logs (slog, trace-correlated)
                         |  +-----------+-----------+  |
                         |              |              |
                         |        ConfigMap/Secret     |
                         +--------------|--------------+
                                        |
                                  pgx (TLS, pool)
                                        v
                              +---------------------+
                              |  Postgres           |
                              |  sqlc queries       |
                              |  migrate up/down    |
                              +---------------------+
```

The product domain is intentionally open — a notes service, a URL shortener, a feature-flag service, an inventory service, a short-order job queue, or any backend surface you can defend in scope review. **The technical bar is fixed.**

### Required deliverables

1. **Source code** — public GitHub repository, GPL-3.0, clean commit history (squash-merged feature branches). Clean under `go vet`, `staticcheck`, and `go test -race`.
2. **Dual transport** — the same domain served over **both gRPC and REST**, sharing one service/repository layer. `.proto` checked in and generated with `buf`.
3. **Postgres data layer** — `pgx` + `sqlc` typed queries, reversible `golang-migrate` migrations (up *and* down), at least one multi-step transaction, integration tests against a real Postgres container.
4. **Observability** — `slog` structured logs correlated with trace IDs, OpenTelemetry traces exported to Jaeger, Prometheus RED metrics, and a Grafana dashboard checked into the repo.
5. **Container** — a multi-stage, **distroless, non-root** image, configured entirely through the environment (**12-factor**), with a `docker compose` that brings up the full local stack.
6. **Kubernetes deployment** — Deployment + Service + ConfigMap manifests, **honest liveness and readiness** probes, and a **graceful shutdown** that drains in-flight work on `SIGTERM` within the termination grace period. Runs on `kind` (a managed cluster is optional, for a public URL).
7. **A load-test-and-trace report** — drive the service under load, capture a Grafana dashboard and a Jaeger trace, and document one latency finding and its fix.
8. **A reliability-drill postmortem** (~3–5 pages) of **one** of the following drills you must run on yourself before the demo:
   - **Rolling deploy under load** — deploy a new version while a load generator runs; prove zero dropped requests and document how readiness + graceful drain achieved it.
   - **Dependency outage** — kill Postgres (or a downstream) mid-traffic; document how timeouts, retries-with-jitter, and circuit breaking contained the blast radius and how the service recovered.
   - **Saturation / load-shedding** — drive the service past capacity; document the load-shedding behaviour, what stayed healthy, and how readiness reflected reality.
9. **A production runbook** — `production-runbook.md`: every build/deploy command, the probe semantics, the five most likely outages and the first three diagnostics for each, the rollback procedure, and who to page (you are paging yourself; the discipline is the point).
10. **A five-minute demo video** — voice-over required, no marketing edits — showing the service end-to-end: gRPC + REST, a trace in Jaeger, the Grafana dashboard, a rolling deploy under load, and a clean graceful drain.

### Capstone grading axes

| Axis | Weight |
| --- | --- |
| Code quality (idiomatic, review-ready Go; clean under vet/staticcheck/-race) | 20% |
| Correctness end-to-end (gRPC + REST + Postgres + migrations actually work) | 20% |
| Concurrency & reliability (context, graceful shutdown, timeouts, retries, no leaks) | 20% |
| Cloud-native posture (distroless image, 12-factor, K8s manifests, honest probes) | 15% |
| Observability (structured logs, OTel traces, Prometheus + Grafana) | 10% |
| Communication (architecture doc + video + runbook) | 5% |
| Reliability-drill outcome + postmortem | 10% |

Minimum to pass: **70 / 100**.

---

## Career engineering pack

Delivered alongside the capstone, archived to the `interview-prep/` and `portfolio/` directories of the track.

### Interview prep topics (covered in Weeks 11–12)

- **Go language deep-dive.** Pointer vs value receivers; interfaces at the consumer; error wrapping and `errors.Is` / `errors.As`; generics vs interfaces; the zero value; `defer`/`panic`/`recover` semantics.
- **Concurrency.** Goroutine lifecycle and leaks; channel vs mutex; `select` patterns; `context` cancellation and deadlines; the race detector — what it proves and what it cannot; the Go memory model at interview depth.
- **Services & data.** HTTP server design; gRPC vs REST trade-offs; transaction isolation and concurrent-write hazards; migration strategy; the typed-query vs ORM argument.
- **Cloud-native operations.** Liveness vs readiness (the question that separates people who have operated a service from people who have not); graceful shutdown on `SIGTERM`; the distroless / 12-factor rationale; retries-with-jitter and circuit breaking; reading a trace and a flame graph.
- **System design with a Go lens.** Six prompts — "design a URL shortener", "design a rate limiter", "design a job queue", "design a feature-flag service", "design a deploy that drops zero requests", "design a service that survives a database failover" — each with a reference solution that names the actual Go primitives and cloud-native pieces involved.
- **Behavioural drills.** Five backend-specific prompts ("walk me through a service you owned in production", "the worst data race you've shipped", "a deploy that went wrong and what you changed") with framework answers.

### Production runbook contents (template provided)

- Build & deploy steps — every command, no hand-waving.
- The probe semantics — what liveness vs readiness mean for *this* service.
- The graceful-shutdown contract — what drains, in what order, within what grace period.
- The five most likely outages and the first three diagnostics for each.
- The rollback procedure for a bad Kubernetes rollout and a bad migration.
- The observability entry points — which dashboard, which trace, which log query.
- Who to call at 3 AM.

### Portfolio recommendations

- The capstone repo — public, GPL-3.0, with a real README, an architecture diagram, and the load-and-trace report.
- One PR merged into an open-source Go project (a CNCF project, a `chi`/`pgx`/`sqlc` ecosystem library, an OpenTelemetry Go component, or a Kubernetes tool).
- A short technical blog post explaining one bug from the track — the data race from Week 4, the fuzz crash from Week 8, or the latency finding from the capstone.
- A LinkedIn / website page that links to all of the above and does not contain the word "passionate."

---

## License

This curriculum is licensed under **GPL-3.0**. See [`LICENSE`](./LICENSE).

Course identity, accent colour, and brand position are governed by the [`CRUNCH-LABS-CHARTER.md`](../CRUNCH-LABS-CHARTER.md). The track's design rationale lives in [`CHARTER.md`](./CHARTER.md). Fork, teach, remix; PR improvements back to <https://github.com/CODE-CRUNCH-CLUB>.
