# C30 · Crunch Go — Charter

> The design rationale for the track. Why Go for cloud-native backends, why 12 weeks, why we teach the language before the cluster, why our open-source defaults are what they are, and how this track sits among its DevOps, backend, and cloud siblings.

This document is the source of truth for *why* C30 is shaped the way it is. The `SYLLABUS.md` is the *what* and *when*. When the two disagree, this charter wins — and the syllabus is the one we change.

---

## Why Go for cloud-native backends

The cloud-native world is, quite literally, written in Go. Kubernetes is Go. Docker is Go. etcd, Prometheus, Grafana's backend, Terraform, Vault, Consul, Helm, containerd, Istio's control plane, the CNCF graduated-project shelf — Go. When you operate a modern platform, you are operating Go binaries; when you extend it, you write Go. A backend engineer who cannot read and write Go in 2026 is locked out of the source code of the infrastructure they depend on. That alone would justify the track. But it is not the main reason.

The main reason is that Go is *the right shape* for a cloud-native service. A cloud-native service must start fast (the orchestrator will reschedule it constantly), ship small (it lives in an image that gets pulled a thousand times), handle concurrency without ceremony (one request is one of thousands in flight), and expose an honest operational contract (probes, signals, structured telemetry) to the platform around it. Go was designed by people who had operated services at scale and were tired of the alternatives:

- **A single static binary.** No interpreter, no runtime to install, no `node_modules`, no virtualenv. The build produces one file. The container can be `FROM scratch` or distroless. The image is tens of megabytes, not hundreds, and it starts in milliseconds.
- **Goroutines and channels.** Concurrency is a first-class, cheap, legible primitive. Ten thousand concurrent connections is a non-event. The hard part of concurrent backend code — cancellation, deadlines, back-pressure — is expressible directly with `context` and `select` rather than hidden inside an event loop.
- **A standard library that ships batteries.** `net/http` is a production-grade server. `encoding/json`, `database/sql`, `crypto/tls`, `log/slog`, `testing`, and now `testing/fuzz` are all in the box. The dependency graph of a serious Go service is shallow, which is an operational and a security property, not just an aesthetic one.
- **A compiler and toolchain that make discipline cheap.** `gofmt` ends formatting debates. `go vet` and `staticcheck` catch real bugs. `go test -race` proves the absence of a data race the compiler cannot. `go test -bench` and `pprof` make performance work routine, not heroic.
- **A small language you can hold in your head.** Go is deliberately, almost aggressively, small. There is one loop keyword. There are no exceptions, no inheritance, no operator overloading. New engineers are productive in days, and a senior engineer can review any file in the codebase without surprise. For a *team* shipping services, that smallness is the feature.

We teach Go because it is the language a backend engineer ships cloud-native services in, contributes to the platform with, and gets hired into the cloud-native job market on. We do not teach it because it is fashionable. We teach it because it is correct for the job this track is about.

---

## Why 12 weeks

Go is a small language with a deep operational surface. That combination sets the length precisely.

A 12-week track is enough — and only just enough — to do all three of the following honestly:

1. **Teach the language to fluency, not familiarity.** Four weeks: the tour and tooling, idiom (structs, interfaces, errors, generics), and the two concurrency weeks that separate a Go engineer from a tourist. Go's *syntax* can be learned in a weekend; its *idiom* and its *concurrency model* cannot, and the concurrency model is the whole point.
2. **Build a real service with real data.** Four weeks: HTTP services, Postgres with a typed query layer and migrations, gRPC and protobuf, and a full testing week (table tests, integration tests, benchmarks, fuzzing). This is where most "learn Go" material stops — at a service that runs on a laptop and is never operated.
3. **Make the service cloud-native and ship it.** Four weeks: observability, containers and 12-factor config, Kubernetes deployment with honest probes, reliability patterns and graceful shutdown, and the capstone. This is the half the job actually grades you on and the half most curricula skip.

Twelve weeks is the smallest window that covers all three without dropping one. We considered fifteen (the length of C18/C19) and rejected it: Go's language surface is genuinely smaller than a cloud platform's, so the language phase compresses cleanly into four weeks where C18 needs more. We considered eight and rejected it harder: eight weeks forces you to drop either the data layer or the cloud-native layer, and a backend course that skips either is a tutorial, not a track. Twelve is the honest number.

The 12-week length matches C15 (DevOps), C16/C17 (Pro tier), and the other focused Crunch Labs language tracks — by design, so a learner can stack two of them into a semester.

---

## Topic ordering — why the language comes before the cluster

The track moves: language → service → cloud-native. We teach goroutines before we teach Postgres, Postgres before gRPC, and everything before Kubernetes. This is deliberate, and it is the opposite of how a lot of "build a microservice in Go" material is sequenced.

**Reason one — the concurrency model is load-bearing.** Almost every operational property a cloud-native service needs — cancellation on client disconnect, request deadlines, bounded concurrency, graceful drain — is an expression of Go's concurrency model. A learner who reaches Kubernetes without having internalised `context`, channels, `select`, and the race detector will write a service that *runs* and cannot be *operated*. We spend two full weeks on concurrency in Phase I precisely so that the graceful-shutdown and reliability weeks in Phase III are applications of a model the cohort already owns, not new magic.

**Reason two — you cannot operate what you cannot build correctly.** Observability, containers, and Kubernetes are *wrappers* around a service. If the service inside is wrong — leaks goroutines, ignores `context`, has a data race, mishandles a transaction — no amount of YAML saves it. We build the correct service first (Phases I–II) and make it cloud-native second (Phase III). Reverse the order and you get engineers who can write a Deployment manifest for a service they cannot debug.

**Reason three — the data layer is where the bugs live.** We place Postgres, transactions, and migrations in Phase II, before the cloud-native phase, because data correctness under concurrency is the hardest part of a real backend and the part interviews probe hardest. A learner needs the concurrency foundation from Phase I to reason about it, and needs it solid before piling deployment concerns on top.

**Reason four — production concerns reward maturity.** Graceful shutdown, readiness that tells the truth, retries with jitter, circuit breaking, load-shedding — these only make sense once you have a service worth keeping alive and have felt the failure modes. A week-two learner has not. A week-eleven learner has. The track gets harder as the cohort gets stronger.

---

## How C30 complements the DevOps, backend, and cloud tracks

C30 is a focused language-and-runtime track that sits deliberately between the platform tracks. The boundaries are clean:

| Track | Owns | Does not own |
| --- | --- | --- |
| **C30 — Crunch Go (this)** | Go the language; idiomatic backend Go; the concurrency model; a single cloud-native service end-to-end; its observability, container, and graceful-shutdown contract | Cluster administration; multi-service platform design; cloud-provider specifics; Python backends |
| **C15 — Crunch DevOps** | Docker, Kubernetes, Terraform, CI/CD, monitoring, incident response — the *operational platform* | Application code; the language a service is written in; service-internal design |
| **C16 — Crunch Pro · Web Backend** | Production Python backends with Django and FastAPI | Go; the static-binary / goroutine runtime model |
| **C18 / C19 — Crunch GCP / AWS** | Running workloads on a managed cloud — multi-region, autoscaled, provider-managed data and identity | The application inside the workload; the language it is written in |

The seams are intentional and the track is designed to plug into them:

- **C30 ↔ C15 (DevOps).** C15 builds the Docker and Kubernetes muscle; C30 *uses* it from the application side. C15's learner learns to operate a cluster; C30's learner learns to author a service that is *operable* — one that ships a sensible image, answers probes honestly, and drains on `SIGTERM`. A learner who takes both owns the whole picture: the service and the platform it runs in. C30 teaches only the slice of Kubernetes a service author must own (Deployment, Service, ConfigMap, probes, rollout) and explicitly defers cluster internals, networking, and operators to C15.
- **C30 ↔ C16 (Python Web Backend).** Both ship production HTTP backends with persistence, migrations, and observability. C16 does it in Python (Django/FastAPI, WSGI/ASGI, an ORM); C30 does it in Go (`net/http`/`chi`, `sqlc`, a single static binary). The *shape* of the work — handlers, middleware, a typed data layer, schema migrations, structured telemetry — transfers directly; the runtime model does not. The two together are the polyglot backend profile most platform teams actually want.
- **C30 ↔ C18 / C19 (GCP / AWS).** C30's capstone runs on local `kind`. C18 and C19 take exactly that kind of containerised Go workload and run it multi-region, autoscaled, and observed on a managed cloud. The Go image you ship here is the input to those tracks; nothing about it changes when it moves to GKE or EKS, which is the whole point of building it 12-factor and distroless.
- **C30 ↔ C22 (Mesh).** C30 builds one service correctly. C22 designs the *system* of services — mesh, event streaming, multi-region data, chaos engineering. C30's gRPC, observability, and reliability weeks are the per-service prerequisites C22 assumes you already have.

A graduate who wants the **full cloud-native backend profile** takes C15 (or arrives with it), then C30, then C18 or C19, optionally C22 for the staff-track. A graduate who wants **Go specifically** takes C30 alone. Both are correct paths.

---

## Open-source-first stance

The Crunch Labs Charter states that Crunch Labs teaches open-source paths first and vendor lock-in paths second. Go makes this easy: the language, its toolchain, and nearly the entire cloud-native ecosystem are open-source, and most of it is itself written in Go.

We commit to:

- **The Go toolchain itself.** `go build`, `go test`, `go vet`, `gofmt`, `go tool pprof`, the module system, and the race detector are the spine of the track. They are open, free, and on every platform.
- **`chi` over a heavy framework.** We teach `net/http` first and `chi` as a thin, idiomatic router on top of it — explicitly *not* a framework. `gin` and `echo` are named as alternatives; we teach the composable path so the cohort owns the request lifecycle.
- **`pgx` + `sqlc` over a heavy ORM.** `sqlc` generates type-safe Go from plain SQL — the cohort writes SQL, gets compile-time-checked Go, and never loses sight of the query. `GORM` is named as the ORM alternative and its trade-offs are taught honestly; we default to the typed-query path.
- **`golang-migrate` over ad-hoc migration scripts.** Versioned, forward-and-back migrations as a first-class artifact.
- **`buf` + `protoc-gen-go` / `protoc-gen-go-grpc` over hand-rolled RPC.** Protocol Buffers and gRPC are the open, language-neutral contract; `buf` is the open toolchain that makes them pleasant.
- **OpenTelemetry over proprietary APM.** We instrument with OpenTelemetry SDKs and export to **Jaeger** (traces) and **Prometheus** + **Grafana** (metrics) — all open, all self-hostable. Honeycomb, Datadog, and the cloud APMs are named as OTel-compatible destinations, never as the only path.
- **`slog` (standard-library structured logging) over a third-party logger.** Since Go 1.21 the standard library ships structured logging; we teach it as the default and name `zap` / `zerolog` where their performance characteristics earn the dependency.
- **Docker / Podman, `kind` / minikube, and upstream Kubernetes.** The whole deployment story runs on the learner's laptop with open tooling. No managed cloud is required to complete the track.

Where a vendor service is genuinely the better operational answer at scale — a managed Postgres, a hosted tracing backend, a managed Kubernetes control plane — we name it, explain when it earns its cost, and teach the open self-hosted equivalent first so the learner's own work is portable. The point is not purity; it is that a C30 graduate's service runs on any cluster and exports to any OTel collector, because nothing in it is bolted to a vendor.

---

## Why these specific tools

A short defense of choices a reviewer might question:

- **Go 1.22+ as the floor.** Generics (1.18), `slog` (1.21), and the enhanced `net/http` routing (1.22) are all things we teach as current idiom, not workarounds. Pinning the floor at 1.22 keeps the syllabus honest about what "modern Go" means.
- **`chi` as the router.** Standard-library-compatible (`http.Handler` all the way down), tiny, no framework lock-in, idiomatic middleware. It teaches the request lifecycle instead of hiding it.
- **`pgx` as the Postgres driver and `sqlc` for queries.** `pgx` is the modern, performant, well-maintained Postgres driver; `sqlc` gives compile-time-checked queries from plain SQL without an ORM's hidden behaviour. Together they are the current best-practice data layer for a serious Go service.
- **Postgres as the database.** Open-source, ubiquitous, the default relational store in the cloud-native world, and rich enough (transactions, JSONB, `LISTEN/NOTIFY`) to teach real data discipline.
- **gRPC + Protocol Buffers with `buf`.** gRPC is the dominant service-to-service RPC in cloud-native systems; `buf` is the open toolchain that makes protobuf workflows reproducible and lintable.
- **OpenTelemetry + Jaeger + Prometheus + Grafana.** The CNCF-standard, vendor-neutral observability stack. Self-hostable in a `docker compose` for the labs; portable to any managed backend later.
- **`kind` for the Kubernetes weeks.** A real Kubernetes cluster, on a laptop, in Docker, for free. Identical API surface to a managed cluster for everything this track teaches.

---

## What happens after each Go release

Go ships a major release roughly every six months (February and August). The syllabus is therefore a *living document* with a stated revision policy:

1. After each February and August Go release, the curriculum council reviews every week against the release notes.
2. The syllabus is revved on a branch in the weeks after the release.
3. The new version freezes for the next academic cohort before the term begins.
4. The previous version is archived under `OLD/SYLLABUS-YYYY.md` and remains available to running cohorts.

Generics entered the idiom phase the year after 1.18. `slog` replaced the third-party logging recommendation the year after 1.21. The enhanced `net/http` router entered the HTTP week after 1.22. The track ages forward with the language.

---

## Status

This charter is live as of **2026-06-19**. It is the first edition of the C30 track, drafted under the Crunch Labs Charter.

Signed by the Code Crunch Club curriculum council. Open an issue on the master curriculum repository to propose amendments. Changes that affect more than wording (week count, capstone shape, tool defaults, open-source posture) require a charter revision and a PR review, not a silent edit.

Licensed **GPL-3.0** along with the rest of the academy.
