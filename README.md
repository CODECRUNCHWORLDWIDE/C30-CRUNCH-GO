# C30 · Crunch Go — Cloud-Native Backend in Go

> Code Crunch Club · Crunch Labs tier · sub-brand **Go** (`#00ADD8`)
> 12 weeks · ~432 hours · GPL-3.0
> Track home: `C30-CRUNCH-GO/`

Twelve weeks to walk from "I can read a typed language" to "I ship the cloud-native service the platform team trusts in production." We start with Go the language — modules, the tour, the toolchain that makes the rest of the track cheap — and we end with a containerised microservice that serves both gRPC and REST, talks to Postgres through a typed query layer, emits traces and metrics that a human can act on at 3 AM, runs as a distroless image under a Kubernetes Deployment, answers liveness and readiness probes honestly, and drains its in-flight work on `SIGTERM` instead of dropping it on the floor. Along the way you will write a worker pool that respects `context` cancellation, prove a data race does not exist with the race detector, fuzz a parser until it stops crashing, and roll a schema migration forward and back without taking the service down.

This is not a "write a REST endpoint and call it a backend" course. It is the curriculum we wish existed for engineers who want to take server-side Go seriously — the concurrency model that actually scales, the observability that survives an incident, and the operational contract a service owes the cluster it runs in. Go is the language the cloud-native world is written in: Kubernetes, Docker, etcd, Prometheus, Terraform, and Vault are all Go. We teach you to build *with* that ecosystem and to read *into* it.

Go rewards a particular kind of engineer — one who values a small language they can hold entirely in their head, a standard library that ships batteries, a compiler that produces a single static binary, and a runtime whose concurrency model (goroutines, channels, `select`) makes the hard problems legible instead of hidden. We lean into that. We teach idiomatic Go, not Java-in-Go or Python-in-Go. We teach you to reach for the standard library first, a small well-chosen dependency second, and a framework almost never.

---

## Who this is for

Four personas, all welcome, all stretched:

1. **The Python or Node backend engineer going cloud-native.** You ship Django, FastAPI, Rails, or Express today. You can build features, but your services are hard to operate at scale, and the platform team keeps asking you for things you do not yet know how to give them (clean shutdown, real readiness, bounded concurrency, structured traces). Go is how a backend becomes operable. We start you on the language and end you on Kubernetes.
2. **The platform / DevOps engineer who reads Go but does not write it.** You finished C15 (DevOps) and you operate Go services every day — Kubernetes controllers, Prometheus exporters, custom operators — without being able to author them. You want to cross the line from operator to author, contribute to the open-source tooling you depend on, and own the services, not just the YAML around them.
3. **The new-grad or self-taught engineer aiming at cloud-native shops.** You know one typed language and you want a backend job at a company whose stack is Go (which, in the cloud-native world, is most of them). You need idiomatic Go, the concurrency model interviewers actually probe, and a deployed microservice in your portfolio that proves you can ship.
4. **The polyglot senior wanting Go depth.** You ship Java, C#, Rust, or C++. You can already reason about systems. You want the specific fluency to hold your own in a senior Go review — when a channel is the wrong answer, why `context` is threaded everywhere, what the race detector can and cannot prove, and how a distroless image and a readiness probe change the operational story. This track gives you that.

If you have shipped one non-trivial product in a typed language (Java, C#, TypeScript, Rust, modern C++, Swift, or Go itself), and you are comfortable on a Linux shell, you are ready. If you have not, take C1 (Convos) and then C14 (Linux) first. If you have never touched containers or Kubernetes, C15 (DevOps) before this track will make Weeks 10–11 land far harder.

---

## What you will be able to do at the end

Twelve concrete capabilities you should have on day 84:

1. Write idiomatic, modern Go — modules, generics, `errors.Is` / `errors.As` / wrapped errors, table-driven tests, `gofmt`, `go vet`, `staticcheck` — and explain why Go's smallness is a feature, not a limitation.
2. Model a domain with structs and interfaces the Go way — accept interfaces, return structs; keep interfaces small; define them at the consumer — and defend each choice in review.
3. Reach for goroutines, channels, and `select` deliberately, and articulate the times a mutex or a plain function call is the better answer than a channel.
4. Thread `context.Context` through a request's whole lifetime for cancellation and deadlines, build a bounded worker pool that does not leak goroutines, and prove the absence of a data race with `go test -race`.
5. Build an HTTP service on `net/http` and `chi` with composable middleware, request-scoped logging, panic recovery, and a clean handler/service/repository seam.
6. Talk to Postgres through `pgx` and `sqlc` with compile-time-checked queries, run forward-and-back schema migrations, and use transactions correctly under concurrency.
7. Define a service in Protocol Buffers, generate Go from it, and serve the same domain over **both gRPC and REST** with shared business logic.
8. Test like a senior — table-driven unit tests, integration tests against a real Postgres in a container, benchmarks that catch regressions, and fuzz targets that find the inputs you did not think of.
9. Instrument a service with structured logging (`slog`), distributed tracing (OpenTelemetry), and Prometheus metrics — and read the resulting traces and dashboards to localise a latency regression.
10. Containerise a Go service as a multi-stage, **distroless**, non-root image that builds reproducibly and starts in milliseconds, configured entirely through the environment (12-factor).
11. Deploy that service to Kubernetes with a Deployment, Service, and ConfigMap; wire **liveness and readiness** probes that tell the truth; and roll out a new version with zero dropped requests.
12. Implement **graceful shutdown** on `SIGTERM`, plus the reliability patterns a cloud-native service needs — timeouts everywhere, retries with backoff and jitter, circuit breaking, and load-shedding under pressure.

---

## Prerequisites

| Required | Helpful | Not required |
| --- | --- | --- |
| **C1 — Code Crunch Convos** (or equivalent typed-language fluency) | **C15 — Crunch DevOps** (Docker + Kubernetes mental model) | A four-year CS degree |
| Comfort with one typed language and basic data structures | **C14 — Crunch Linux** (shell, processes, signals) | A previous Go job |
| A laptop running Linux, macOS, or Windows + WSL2 | Some prior backend or API experience | A cloud account |

**Hardware and software reality.** Everything in this track runs on a single laptop. You need the **Go toolchain** (1.22 or newer — generics, `slog`, and the modern `net/http` routing all assume a current release), **Docker** (or Podman) for the database and the container labs, and **`kind`** (Kubernetes-in-Docker) or **minikube** for the deployment weeks. No paid cloud account is required: `kind` runs a real Kubernetes cluster on your laptop, and Postgres runs in a container. 16 GB of RAM is comfortable; 8 GB works if you are disciplined about what you leave running. The capstone can optionally deploy to a managed cluster (GKE, EKS, or a cheap VPS) if you want a public URL, but local `kind` satisfies every grading requirement.

**No proprietary tools required.** The entire stack is open-source: the Go toolchain, `chi`, `pgx`, `sqlc`, `golang-migrate`, `protobuf` + `buf`, OpenTelemetry, Prometheus, Grafana, Jaeger, Docker, and Kubernetes. Vendor cloud services appear as named examples, never as the only path.

---

## Program at a glance — three phases

| Phase | Weeks | Title | Focus | Capstone milestone |
| --- | --- | --- | --- | --- |
| I | 1–4 | The Language & the Runtime | Go, tooling, idiom, generics, concurrency, the race detector | A correct, race-free concurrent CLI tool, fully tested |
| II | 5–8 | Services & Data | HTTP, Postgres, gRPC + protobuf, testing & fuzzing | A REST + gRPC service backed by Postgres, hard-tested |
| III | 9–12 | Cloud-Native & Capstone | Observability, containers, 12-factor, Kubernetes, reliability, capstone | A deployed, observable, gracefully-shutting-down microservice |

Week-by-week detail lives in [`SYLLABUS.md`](./SYLLABUS.md). Design rationale (why Go, why this ordering, how it complements the DevOps, backend, and cloud tracks) lives in [`CHARTER.md`](./CHARTER.md).

---

## Weekly cadence

The track runs at **36 hours per week** for full-time cohorts and compresses to **12 hours per week** for self-paced cohorts. Each week ships one named lab, one quiz, and one logged build-and-profile entry.

| Day | Block | Typical content |
| --- | --- | --- |
| Mon | Lecture (2h) | Topic intro, standard-library source walkthrough, reference reading |
| Mon | Lab (3h) | Guided exercise — write the code, run the tests, run the race detector |
| Wed | Lecture (2h) | Deeper dive, code review of last week's lab, architecture decision record |
| Wed | Lab (3h) | Open-ended mini-project sprint |
| Fri | Studio (4h) | `pprof` clinic, trace-reading session, Kubernetes debugging, code-review office hours |
| Sun | Quiz (~30m) + reading | Auto-graded; covers the standard library docs, the Go spec, and the week's reading list |

The remaining hours are unstructured project time — building, breaking, and shipping.

---

## Recommended pre/post tracks

```text
C1 (Code Crunch Convos · Python)
        |
        v
C14 (Crunch Linux)  -->  C15 (Crunch DevOps)   <-- both strongly recommended before C30
        |
        v
*** C30 (Crunch Go — Cloud-Native Backend in Go) ***
        |
        +--> C18 / C19  (Crunch GCP / AWS)
        |       to run the service on a managed cloud at fleet scale
        |
        +--> C22 (Crunch Mesh — Microservices & Distributed Systems)
        |       to design the platform the service lives in
        |
        +--> C26 (Crunch Rust)
                for the systems-programming sibling, when GC is not acceptable
```

- **C30 vs C16 (Python Web Backend).** C16 owns Python backends with Django and FastAPI. C30 owns Go backends for cloud-native deployment. They overlap on the shape of an HTTP service (handlers, middleware, ORMs/query layers, migrations, observability) and diverge on everything runtime: Go's static binary, goroutine concurrency, and single-process deploy model are a different operational world from a WSGI/ASGI Python service behind Gunicorn. Many graduates take both for a polyglot backend profile.
- **C30 to C15.** C30 *consumes* the Docker and Kubernetes muscle that C15 builds. If you arrive without it, you will still finish — Weeks 10–11 teach the Go-relevant slice from scratch — but C15 first makes those weeks a victory lap instead of a climb.
- **C30 to C18 / C19.** The capstone runs on local `kind`. C18 (GCP) and C19 (AWS) teach you to take the same image and run it multi-region, autoscaled, and observed on a managed cloud. The Go service you ship here is exactly the kind of workload those tracks operate.
- **C30 to C22 (Mesh).** If building one service makes you want to design the *platform* of services — service mesh, event streaming, multi-region data — C22 is the staff-track continuation. C30 ships a service; C22 designs the system of them.

---

## What this course will NOT do

Honest expectations, set up front:

- **It will not make you a distributed-systems architect in twelve weeks.** We teach you to build *one* cloud-native service correctly and operate it under load. Designing a fleet of them — consensus, sharding, multi-region data, service mesh — is C22's job, and it is a 24-week job for a reason.
- **It will not teach you Kubernetes from zero.** We teach the slice a Go service author must own: a Deployment, a Service, a ConfigMap, probes, and a graceful rollout. We do not teach cluster administration, networking internals, CNI, or operators. C15 owns that; we name it and move on.
- **It will not turn Go into your favourite language by force.** Go makes trade-offs some engineers dislike — verbose error handling, no exceptions, a deliberately small feature set, a garbage collector. We teach you to work *with* those choices and to articulate, honestly, where another language (Rust for no-GC systems work, Python for data and glue) is the better tool.
- **It will not ship you a framework.** There is no Go equivalent of Rails here, on purpose. We teach the standard library, a small router (`chi`), and a typed query layer (`sqlc`) — composable pieces you understand top to bottom — not a monolith that hides the request lifecycle from you.
- **It will not give you a vendor certification.** There is no "certified Go engineer." We give you a deployed microservice, a benchmark-and-trace portfolio, a runbook, and an interview-prep pack — the closest equivalent the actual job market recognises.

---

## Capstone preview

The Phase III capstone is **one substantial cloud-native service**, not a parade of toy programs:

> **Cloud-Native Microservice.** A single Go service that exposes its domain over **both gRPC and REST**, persists to **Postgres** through `pgx` + `sqlc` with forward-and-back migrations, is fully **instrumented** (structured `slog` logs, OpenTelemetry traces to Jaeger, Prometheus metrics behind a Grafana dashboard), ships as a **multi-stage distroless non-root container**, is configured entirely through the environment (**12-factor**), and is **deployed to Kubernetes** with a Deployment, Service, and ConfigMap. It answers **liveness and readiness** probes honestly, performs a **graceful shutdown** that drains in-flight work on `SIGTERM`, and survives a reliability drill — a rolling deploy under load with zero dropped requests, a dependency outage handled by timeout-retry-circuit-break, and a load-shedding event under saturation.

Full specification in [`SYLLABUS.md` § Capstone](./SYLLABUS.md#capstone). Deliverables include an architecture diagram, a five-minute video walkthrough, a load-test-and-trace report, a reliability-drill postmortem, and a production runbook.

---

## License & maintainers

Licensed **GPL-3.0**. See [`LICENSE`](./LICENSE).

Maintained by the Code Crunch Club curriculum council. Open an issue on the master curriculum repository to propose curriculum changes or contribute lecture notes. Contributions follow the repository-wide `CONTRIBUTING.md`.

This is a living document. Go ships a major release roughly every six months. We rev the syllabus after each February and August Go release and freeze it for the following academic cohort.
