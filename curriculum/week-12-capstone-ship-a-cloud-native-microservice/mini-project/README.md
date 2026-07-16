# Capstone Defense Brief — Ship a Cloud-Native Microservice: Deploy, Drill, Document, and Defend

> **Time:** ~23.5 hours across the week (the bulk of Week 12). **Prerequisites:** Labs 9, 10, and 11 complete (observability, container, Kubernetes + reliability); this week's Lectures 1–3 and Exercises 1–3; ideally both challenges. **Citations:** the C30 SYLLABUS capstone spec and assessment matrix, the ADR community at <https://adr.github.io/>, opentelemetry.io, the Go pprof blog, and the Google SRE workbook (postmortems, playbooks).

This is the culmination of **C30 · Crunch Go**. It is not another lab — it is the **capstone defense**. You take the service you built across eleven weeks, make it coherent, deploy it, drive load through it, break it on purpose, document it, and defend it. The capstone is **30% of the C30 grade and cannot be carried by other components** (SYLLABUS assessment matrix: "a weak quiz week is forgivable; a non-functional capstone is not"). It is graded on engineering and operability — code quality, end-to-end correctness, concurrency/reliability, cloud-native posture, observability, communication, and the reliability-drill outcome — **not on visual polish.**

## What the capstone is

**One substantial cloud-native service, taken all the way to a running, observable, gracefully-shutting-down Kubernetes deployment.** The product domain is yours to choose — a notes service, a URL shortener, a feature-flag service, an inventory service, a short-order job queue, or any backend you can defend in scope review. The **technical bar is fixed**. The architecture (from the SYLLABUS):

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

This is the service Weeks 5–11 built. Week 12 changes none of the architecture; it integrates, deploys, drills, documents, and defends it.

## The ten required deliverables

All ten must be present and consistent with each other (the architecture diagram must describe the system you actually demo):

1. **Source code** — public GitHub repository, GPL-3.0, clean commit history (squash-merged feature branches). Clean under `go vet`, `staticcheck`, and `go test -race`.
2. **Dual transport** — the same domain served over **both gRPC and REST**, sharing one service/repository layer. `.proto` checked in and generated with `buf`. (Weeks 5, 7.)
3. **Postgres data layer** — `pgx` + `sqlc` typed queries, reversible `golang-migrate` migrations (up *and* down), at least one multi-step transaction, integration tests against a real Postgres container. (Weeks 6, 8.)
4. **Observability** — `slog` structured logs correlated with trace IDs, OpenTelemetry traces exported to Jaeger, Prometheus RED metrics, and a Grafana dashboard checked into the repo. (Week 9.)
5. **Container** — a multi-stage, **distroless, non-root** image, configured entirely through the environment (**12-factor**), with a `docker compose` that brings up the full local stack. (Week 10.)
6. **Kubernetes deployment** — Deployment + Service + ConfigMap manifests, **honest liveness and readiness** probes, and a **graceful shutdown** that drains in-flight work on `SIGTERM` within the termination grace period. Runs on `kind`. (Week 11.)
7. **A load-test-and-trace report** (`LOAD-AND-TRACE-REPORT.md`) — drive the service under load, capture a Grafana dashboard and a Jaeger trace, document one latency finding and its fix. (Exercise 2.)
8. **A reliability-drill postmortem** (~3–5 pages) of **one** drill you ran on yourself — rolling deploy under load, dependency outage, or saturation/load-shedding. (Lecture 2; Week 11 challenges.)
9. **A production runbook** (`production-runbook.md`) — every build/deploy command, the probe semantics, the five most likely outages and the first three diagnostics each, the rollback for a bad rollout and a bad migration, and who to page. (Exercise 3.)
10. **A five-minute demo video** — voice-over required, no marketing edits — showing gRPC + REST, a trace in Jaeger, the Grafana dashboard, a rolling deploy under load, and a clean graceful drain. (Lecture 3.)

Plus the supporting artifacts that make the deliverables defensible: the **ADRs** (`docs/adr/`, Exercise 1), the **architecture diagram**, and the **career pack** (the open-source PR and the system-design dossiers, this week's challenges).

## The deployed topology (what runs during the demo)

```text
   developer build + kind load
            |
            v
   +--------------------+      docker build -> kind load docker-image
   |  notesd image      |---- distroless, non-root, 12-factor (Week 10)
   |  (the ONE image)   |
   +---------+----------+
             | loaded into the cluster
             v
   +----------------------------------------------------------------------+
   |                       kind cluster (notes namespace)                 |
   |   +------------------+   pgx (pool, ctx deadline)   +-------------+   |
   |   |  Deployment      |----------------------------->| Postgres    |   |
   |   |  notes (3 reps)  |   sqlc queries, migrate up   | (in-cluster)|   |
   |   |  - /healthz live |                              +-------------+   |
   |   |  - /readyz ready |          +-----------+                         |
   |   |  - SIGTERM drain |--------->| Jaeger    |  OTel traces            |
   |   |  - 12-factor cfg |          +-----------+                         |
   |   +--------+---------+--------->| Prometheus|->| Grafana | RED dash   |
   |            | Service (ClusterIP) +-----------+  +---------+            |
   |     ConfigMap + Secret (NOTES_* env)                                  |
   +------------|---------------------------------------------------------+
                |  port-forward (or public URL on a managed cluster)
        REST    |                       gRPC
                v                         v
          +------------+          +----------------+
          | curl /     |          | grpcurl        |
          | hey load   |          | (internal      |
          +------------+          |  clients)      |
                                  +----------------+
```

gRPC and REST hit the same service layer on the same deployed backend. That is the whole point of "dual transport over shared logic."

## The defense

The defense is a live session (in person or recorded + Q&A). It has two parts.

### The live demo script

Staged, not improvised — have the cluster up, the port-forwards open, the load generator and the Jaeger/Grafana tabs ready, and a fresh terminal before you begin:

1. **Say the scope sentence.** "This is `<domain>`, a Go microservice serving gRPC and REST over a shared service layer, backed by Postgres, deployed to Kubernetes with honest probes and graceful shutdown." This is the thesis; lead with it.
2. **Dual transport.** `curl` the REST endpoint and `grpcurl` the gRPC endpoint; show they return the **same** domain data — the shared service layer. Point out the `.proto` is the gRPC contract.
3. **Postgres + transaction.** Do a write that goes through the multi-step transaction; show it persisted. Mention the reversible migrations (`migrate up`/`down`).
4. **The trace ties to the log.** Open the request's trace in Jaeger (handler → service → pgx span tree); show the same `trace_id` in the `slog` JSON log line. The three signals, correlated.
5. **The dashboard.** The Grafana RED dashboard reflecting your traffic — rate, errors, duration.
6. **Zero-drop rolling deploy.** Start the load generator; `kubectl rollout restart`; show `drop=0` while pods hand off (new pods take traffic only when ready; old pods drain). This is the reliability drill, live.
7. **Clean graceful drain.** Hold an in-flight request; delete its pod; show the request completes 200 and the pod logged `drain complete`.
8. **Walk the runbook** — one sentence per section: "here is how the next operator runs this without me." Have the file open; do not recite from memory.

Rehearse the choreography end to end at least once with everything live; the demo is staged precisely so a flaky moment does not derail the defense. Have the previous-version SHA and a fresh terminal ready before the rollout step.

### Q&A — what reviewers probe

Expect questions that go behind the demo (the senior backend-Go interview loop, Lecture 3):

- **Concurrency.** "Show me a goroutine that could leak and how `context` cancellation prevents it. What does `go test -race` prove — and what can it not prove?"
- **Reliability decisions.** "Walk me through the graceful-shutdown order — why is the pool closed *last*? Why a circuit breaker and not just retries? What happens to an in-flight request during your rolling deploy?"
- **Architecture.** "Why `sqlc` and not an ORM? Why both gRPC and REST? Why distroless and not alpine?" (Your ADRs, spoken aloud.)
- **Operations.** "It's 3am, the deploy you just shipped is throwing 500s. Walk me through it." (Roll back first, then diagnose — runbook section 2.) "Where do the logs live? How do you pivot to the trace?"
- **The drill.** "What did your reliability drill prove? What surprised you?" (Your postmortem.)
- **Liveness vs readiness.** "Why does your liveness probe not check the database?" (The restart-storm answer — the question that separates operators from authors.)

A confident, specific answer pointing at code, an ADR, the runbook, or the postmortem is what the defense rewards. "It works, trust me" does not.

## Grading rubric

The capstone is **30% of the C30 grade and cannot be carried by other components.** Points sum to 100, mirroring the SYLLABUS capstone grading axes — graded on engineering, not polish:

- **20 points — code quality.** Idiomatic, review-ready Go; clean under `go vet`, `staticcheck`, and `go test -race`. (Weeks 1–8.)
- **20 points — correctness end-to-end.** gRPC + REST + Postgres + reversible migrations actually work; the transaction is correct; integration tests are green against a real Postgres. (Weeks 5–8.)
- **20 points — concurrency & reliability.** `context` threaded for cancellation, graceful shutdown that drains, timeouts everywhere, retries with jitter, a circuit breaker, no goroutine leaks. (Weeks 3–4, 11.)
- **15 points — cloud-native posture.** Distroless non-root image, 12-factor config, Kubernetes manifests, honest probes. (Weeks 10–11.)
- **10 points — observability.** Structured trace-correlated logs, OTel traces to Jaeger, Prometheus + Grafana, demonstrated by localising the load-and-trace finding. (Week 9.)
- **5 points — communication.** The architecture diagram + the video + the runbook, each clear and consistent with the system.
- **10 points — reliability-drill outcome + postmortem.** A drill run on yourself, with an honest postmortem of what the mechanisms did and what surprised you. (Week 11 challenges; Lecture 2.)

Minimum to pass: **70 / 100**, AND a passing capstone, AND a passing reliability drill (SYLLABUS). **Visual polish earns zero points.** A beautiful UI on a service that cannot deploy, has no tests, lies about readiness, or has no runbook fails the capstone. A plain service that deploys with honest probes, drains on `SIGTERM`, traces a request end to end, survives a drill, and hands the next operator a followable runbook passes it.

## Portfolio packaging (the career pack)

Delivered alongside the capstone (SYLLABUS career engineering pack):

- **The capstone repo** — public, GPL-3.0, with a real `README.md` (the architecture diagram, how to run locally with `docker compose`, how to deploy to `kind`), the `docs/adr/` ADRs, the `LOAD-AND-TRACE-REPORT.md`, the reliability-drill postmortem, and the `production-runbook.md`.
- **The five-minute demo video** linked from the README.
- **The open-source PR** (challenge-01) — one merged or review-ready PR in a Go ecosystem project, with the portfolio writeup.
- **The system-design dossiers** (challenge-02) — two designs from the SYLLABUS prompts, naming the Go primitives and cloud-native pieces.
- **A technical blog post** (optional but recommended) — explaining one bug from the track: the data race from Week 4, the fuzz crash from Week 8, or the latency finding from the capstone.
- **A LinkedIn / website page** linking all of the above — and, per the SYLLABUS, not containing the word "passionate."

## Submission

This is the final submission of C30. Push the deployed, documented capstone to its public repo on `main`. The submission must include:

- The repo URL, GPL-3.0, clean under `go vet`/`staticcheck`/`go test -race`.
- The architecture diagram, the ADRs (`docs/adr/`), the `LOAD-AND-TRACE-REPORT.md`, the reliability-drill postmortem, and `production-runbook.md`.
- The five-minute demo video link.
- A short `CAPSTONE.md` mapping each rubric line and each of the ten deliverables to where in the repo it is satisfied (the `.proto`, the migrations, the dashboard JSON, the manifests, the shutdown code, the reports), so the reviewer can verify each claim.
- The career-pack artifacts: the open-source PR and the two system-design dossiers.

The teaching staff schedules the live defense within the final week. The defense is where the rubric is scored: reviewers run the demo with you, probe the concurrency model, the reliability decisions, the architecture, and the runbook, and ask the senior backend-Go interview questions. Bring the system, not the slides.

This is the end of C30 · Crunch Go. You started at the Go tour in Week 1; you finish operating a deployed, tested, observable, gracefully-shutting-down Go microservice, with a runbook your future self will thank you for and an interview loop rehearsed against your own work. That is the job.
