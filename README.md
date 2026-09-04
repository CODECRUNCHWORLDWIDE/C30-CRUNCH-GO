# C30 · Crunch Go — Cloud-Native Backend in Go

> Code Crunch Club · Crunch Labs tier · sub-brand **Go** (`#00ADD8`)
> 12 weeks · ~432 hours · GPL-3.0
> Track home: `C30-CRUNCH-GO/`

Twelve weeks to walk from "I can read a typed language" to "I ship the cloud-native service the platform team trusts in production." We start with Go the language — modules, the tour, the toolchain that makes the rest of the track cheap — and we end with a containerised microservice that serves both gRPC and REST, talks to Postgres through a typed query layer, emits traces and metrics that a human can act on at 3 AM, runs as a distroless image under a Kubernetes Deployment, answers liveness and readiness probes honestly, and drains its in-flight work on `SIGTERM` instead of dropping it on the floor. Along the way you will write a worker pool that respects `context` cancellation, prove a data race does not exist with the race detector, fuzz a parser until it stops crashing, and roll a schema migration forward and back without taking the service down.

This is not a "write a REST endpoint and call it a backend" course. It is the curriculum we wish existed for engineers who want to take server-side Go seriously — the concurrency model that actually scales, the observability that survives an incident, and the operational contract a service owes the cluster it runs in. Go is the language the cloud-native world is written in: Kubernetes, Docker, etcd, Prometheus, Terraform, and Vault are all Go. We teach you to build *with* that ecosystem and to read *into* it.

Go rewards a particular kind of engineer — one who values a small language they can hold entirely in their head, a standard library that ships batteries, a compiler that produces a single static binary, and a runtime whose concurrency model (goroutines, channels, `select`) makes the hard problems legible instead of hidden. We lean into that. We teach idiomatic Go, not Java-in-Go or Python-in-Go. We teach you to reach for the standard library first, a small well-chosen dependency second, and a framework almost never.

---

## Standards & equivalency

> C30 stands in for a university's concurrent-programming course and its server-side web programming course, taught in Go against a service that has to survive being deployed.

**University equivalent.** Two of them. **Concurrent and Parallel Programming** — `CDA 4102`, `CS 4823`, `CS 484`. **Server-Side Web Programming** — `COP 4813`, `CS 4485`, `SWE 432`.

Coverage is **partial** against both, and partial has a precise meaning in each case rather than "most of it".

Against **Concurrent and Parallel Programming**, partial is a matter of *which parallelism*. C30 teaches concurrency the way a service meets it — many things in flight at once, shared state that must stay correct, work that has to be cancelled, bounded and drained — down to the memory model, the happens-before rule, and a race detector run on real code. What it does not teach is the numerical half of a parallel-computing course: decomposing a numeric workload across cores or a cluster with OpenMP, MPI or GPU kernels, and the analytic speedup argument that goes with it. That is the row marked `lighter` below, it is the ledger's `stillToAdd`, and it is declared again at the end of this section.

Against **Server-Side Web Programming**, partial is a matter of *scope*. Every outcome below is taught and assessed, but the course is aimed squarely at the service tier: there is no browser, no template rendering, no client-side work, and no session, password-storage or OAuth material. Authentication appears as one middleware, one identity-on-the-context convention, and the status-code decision that goes with it — enough for the boundary, not a term of identity engineering. If you want the browser half, that is a different course; if you want the identity half, C30 will not give it to you.

C30 carries **no credit**, no transcript entry, no accreditation and no proctored exam. The equivalence is one of **content and skill**: the outcomes below are taught here at the same depth or deeper except where a row says otherwise, and every one of them is assessed by an exercise, a challenge, a homework problem, a quiz, a lab or the capstone.

| University outcome | Where this course teaches it | Depth |
| --- | --- | --- |
| **Concurrent and Parallel Programming** — create concurrent units of execution, and account for what one costs against an operating-system thread | [Week 03](curriculum/week-03-concurrency-i-goroutines-channels-select/) | deeper |
| **Concurrent and Parallel Programming** — coordinate concurrent units by message passing, and state the blocking semantics of a send and a receive | [Week 03](curriculum/week-03-concurrency-i-goroutines-channels-select/) | deeper |
| **Concurrent and Parallel Programming** — multiplex several concurrent sources at one control point, with timeouts and a non-blocking path | [Week 03](curriculum/week-03-concurrency-i-goroutines-channels-select/) | deeper |
| **Concurrent and Parallel Programming** — protect shared state with mutual exclusion, reader/writer locks and one-time initialisation | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) | same |
| **Concurrent and Parallel Programming** — use atomic operations, and say when an atomic is the right answer instead of a lock | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) | same |
| **Concurrent and Parallel Programming** — reason from a memory model: happens-before, visibility, and why an unsynchronised access is undefined rather than merely flaky | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) | same |
| **Concurrent and Parallel Programming** — define a data race, find one with a tool rather than by inspection, and eliminate it | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) | deeper |
| **Concurrent and Parallel Programming** — diagnose deadlock and the stranded worker, from the runtime's own output | [Week 03](curriculum/week-03-concurrency-i-goroutines-channels-select/) | deeper |
| **Concurrent and Parallel Programming** — decompose work into the standard parallel structures — pipeline, fan-out/fan-in, bounded worker pool — and choose the bound deliberately | [Week 03](curriculum/week-03-concurrency-i-goroutines-channels-select/) and [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) | deeper |
| **Concurrent and Parallel Programming** — cancel concurrent work, propagate a deadline through it, and shut it down without losing what finished | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) | deeper |
| **Concurrent and Parallel Programming** — measure concurrent throughput and find the point at which adding workers stops helping | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) and [Week 08](curriculum/week-08-testing-benchmarking-and-fuzzing/) | same |
| **Concurrent and Parallel Programming** — keep a server correct while it serves many requests at once | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) and [Week 11](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/) | deeper |
| **Concurrent and Parallel Programming** — recognise the concurrency hazards of shared persistent state: the lost update, write skew, and what an isolation level buys | [Week 06](curriculum/week-06-databases-pgx-sqlc-migrations-transactions/) | deeper |
| **Concurrent and Parallel Programming** — distribute work beyond one process: replicas across machines, coordinated over a defined wire protocol | [Week 07](curriculum/week-07-grpc-and-protocol-buffers/) and [Week 11](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/) | same |
| **Concurrent and Parallel Programming** — decompose a numerical workload across cores or accelerators and argue its speedup analytically: OpenMP, MPI, GPU kernels | [Week 04](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/) — the parallelism here is goroutines over IO-bound work and a measured throughput sweep; no OpenMP, MPI or GPU work appears anywhere in the track | lighter |
| **Server-Side Web Programming** — build an HTTP server from the platform's own primitives and account for the whole request lifecycle | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) | deeper |
| **Server-Side Web Programming** — route a request by method and path, including path parameters and grouped routes | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) | same |
| **Server-Side Web Programming** — factor cross-cutting concerns into reusable layers: request identity, logging, failure recovery, per-request timeout | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) | deeper |
| **Server-Side Web Programming** — layer an application so the transport holds no business logic and the domain holds no transport | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) and [Week 07](curriculum/week-07-grpc-and-protocol-buffers/) | deeper |
| **Server-Side Web Programming** — parse and validate untrusted request input, and reject it safely | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) | deeper |
| **Server-Side Web Programming** — design a resource-oriented API and choose response status codes correctly against the HTTP specification | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) | same |
| **Server-Side Web Programming** — serialise and deserialise a wire format, and evolve its schema without breaking a deployed client | [Week 07](curriculum/week-07-grpc-and-protocol-buffers/) | deeper |
| **Server-Side Web Programming** — persist application state to a relational database through a connection pool sized deliberately | [Week 06](curriculum/week-06-databases-pgx-sqlc-migrations-transactions/) | same |
| **Server-Side Web Programming** — author and apply schema migrations, forward and back | [Week 06](curriculum/week-06-databases-pgx-sqlc-migrations-transactions/) | deeper |
| **Server-Side Web Programming** — keep a multi-step write atomic with a transaction, and choose an isolation level with reasons | [Week 06](curriculum/week-06-databases-pgx-sqlc-migrations-transactions/) | deeper |
| **Server-Side Web Programming** — authenticate a caller at the service boundary, carry the identity through the request, and return the right refusal | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) — the API-key middleware in `homework.md`, with 401/403 and identity on the context — and [Week 07](curriculum/week-07-grpc-and-protocol-buffers/) for the `Unauthenticated` versus `PermissionDenied` decision | same |
| **Server-Side Web Programming** — return failure to a client in one consistent shape, mapped from domain errors | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) and [Week 07](curriculum/week-07-grpc-and-protocol-buffers/) | deeper |
| **Server-Side Web Programming** — test a service at its boundary, and against a real database rather than a stand-in | [Week 05](curriculum/week-05-building-http-services-net-http-chi-middleware/) and [Week 08](curriculum/week-08-testing-benchmarking-and-fuzzing/) | deeper |
| **Server-Side Web Programming** — offer the same domain over a second protocol without duplicating the logic behind it | [Week 07](curriculum/week-07-grpc-and-protocol-buffers/) | deeper |
| **Server-Side Web Programming** — configure an application for deployment rather than for the machine it was written on | [Week 10](curriculum/week-10-containerizing-go-multi-stage-distroless-and-12-factor-config/) | deeper |
| **Server-Side Web Programming** — operate the service you wrote: health, logs, metrics, traces, and a shutdown that does not drop work | [Week 09](curriculum/week-09-observability-structured-logging-opentelemetry-prometheus/) and [Week 11](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/) | deeper |
| **Server-Side Web Programming** — deploy the service so a second client can reach it | [Week 11](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/) and [Week 12](curriculum/week-12-capstone-ship-a-cloud-native-microservice/) | deeper |

Every row above points at a week that **assigns work** on that outcome — an exercise, a challenge, a homework problem, a quiz item, the week's lab, or the capstone — not merely a week that mentions it.

**The industry bar.** What an employer expects of somebody paid to write and run a backend service, and where this course makes the learner do it. Two rows below say what the course does *not* have; they are there because a false row is worse than a missing one.

| What the job expects | Where this course does it |
| --- | --- |
| Work lands as a commit in a repository you own, not a file on your desktop | The capstone submission is a public repository with a clean history of squash-merged feature branches — [`curriculum/week-12-capstone-ship-a-cloud-native-microservice/mini-project/README.md`](curriculum/week-12-capstone-ship-a-cloud-native-microservice/mini-project/README.md). C30 ships no Git tutorial of its own; it inherits the workflow from C1 and states the standard the capstone is held to |
| You read code you did not write and form a judgement on it | Four times, escalating: read `strings.Fields` and `bufio.Scanner` from the standard library and write up why they are idiomatic — [`curriculum/week-01-the-go-tour-and-the-toolchain/challenges/challenge-02-stdlib-source-walk.md`](curriculum/week-01-the-go-tour-and-the-toolchain/challenges/challenge-02-stdlib-source-walk.md); find and fix a planted data race in a program handed to you — [`curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/exercises/exercise-03-find-and-fix-the-race.go`](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/exercises/exercise-03-find-and-fix-the-race.go); find the bug a fuzzer finds in a parser you were given — [`curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/exercise-03-fuzz-target.go`](curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/exercise-03-fuzz-target.go); and land a change in somebody else's codebase — [`curriculum/week-12-capstone-ship-a-cloud-native-microservice/challenges/challenge-01-merge-an-open-source-go-pr.md`](curriculum/week-12-capstone-ship-a-cloud-native-microservice/challenges/challenge-01-merge-an-open-source-go-pr.md) |
| Tests exist, and the command to run them is written down | `go test ./...` and `go test -race ./...` are a stated contract from Week 1 forward, and Week 8 ships runnable test files beside the code they test — [`curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/exercise-01-table-driven-and-golden_test.go`](curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/exercise-01-table-driven-and-golden_test.go). Every lab from Week 5 on is graded in part on its test suite |
| Integration tests run against the real dependency, not a stand-in for it | A Postgres container brought up in `TestMain`, migrations applied, the suite gated behind a build tag — [`curriculum/week-08-testing-benchmarking-and-fuzzing/challenges/challenge-01-integration-tests-against-postgres.md`](curriculum/week-08-testing-benchmarking-and-fuzzing/challenges/challenge-01-integration-tests-against-postgres.md) |
| You read a real failure instead of guessing at one | C30 heads this differently from the framework's `Common bugs to catch`: the failure material lives in each week's `SOLUTIONS.md`, under a per-exercise **Common pitfalls** heading and a closing **Common mistakes across the three exercises** section, quoting the output the learner should reproduce — the race detector's two-stack report in [`curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/exercises/SOLUTIONS.md`](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/exercises/SOLUTIONS.md), the fuzzer's crasher in [`curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/SOLUTIONS.md`](curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/SOLUTIONS.md) |
| Dependencies are isolated per project | Go modules from Week 1 — `go.mod`, `go.sum`, `go mod tidy` — and every external service the course needs (Postgres, Jaeger, Prometheus, Grafana) runs in a container rather than being installed machine-wide — [`curriculum/week-10-containerizing-go-multi-stage-distroless-and-12-factor-config/exercises/exercise-03-compose-stack.md`](curriculum/week-10-containerizing-go-multi-stage-distroless-and-12-factor-config/exercises/exercise-03-compose-stack.md) |
| A formatter, a linter and a schema gate, run the way a team runs them | `gofmt`, `go vet` and `staticcheck` are a contract from Week 1 — "a warning is a bug you have not fixed yet" — restated at the head of every later week; `buf lint` and `buf breaking` gate the schema in [`curriculum/week-07-grpc-and-protocol-buffers/lecture-notes/01-protobuf-and-buf.md`](curriculum/week-07-grpc-and-protocol-buffers/lecture-notes/01-protobuf-and-buf.md) |
| One unit puts the work through a continuous-integration pipeline | **Not here.** C30 ships no CI unit and no pipeline configuration; the toolchain and the tests are run locally by the learner and checked at the two phase-gate code reviews and the capstone defense. The cross-compilation that makes a CI build trivial is taught in Week 1, but the pipeline itself is C15's material, not this track's |
| Failure is rehearsed before it happens in production | Two drills, both assessed: a rolling deploy under load — [`curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-01-zero-dropped-requests-rolling-deploy.md`](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-01-zero-dropped-requests-rolling-deploy.md) and a dependency outage — [`curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-02-dependency-outage-drill.md`](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-02-dependency-outage-drill.md) |
| The output is portfolio-grade: a stranger can clone it, run it, and know what you can do | The capstone runs from a clean clone with `docker compose` locally and `kind` for the cluster, and ships a README, an architecture diagram, the decision records, the load-and-trace report, the drill postmortem and a production runbook — [`curriculum/week-12-capstone-ship-a-cloud-native-microservice/mini-project/README.md`](curriculum/week-12-capstone-ship-a-cloud-native-microservice/mini-project/README.md) |
| The practice is named, not implied | Every week carries a `## Standards this week meets` block naming the professional task it builds — [`curriculum/week-09-observability-structured-logging-opentelemetry-prometheus/README.md`](curriculum/week-09-observability-structured-logging-opentelemetry-prometheus/README.md) |

**Beyond both bars.** Clearing the two floors is entry, not success. Open any of these and check it in under a minute.

| What we add | Which bar it beats | Where it lives |
| --- | --- | --- |
| Every exercise publishes its worked answer in the open, beside the code, with the exact toolchain output the learner should reproduce — including the race detector's report and the fuzzer's crasher. No key withheld until a deadline | both | [`curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/exercises/SOLUTIONS.md`](curriculum/week-04-concurrency-ii-context-sync-worker-pools-and-the-race-detector/exercises/SOLUTIONS.md) |
| The learner ends holding a running, publicly reachable service with a runbook, a postmortem and a recorded demo — not a grade only a registrar can read | both | [`curriculum/week-12-capstone-ship-a-cloud-native-microservice/mini-project/README.md`](curriculum/week-12-capstone-ship-a-cloud-native-microservice/mini-project/README.md) |
| A reliability drill run on the learner's own service: kill Postgres mid-traffic and prove the timeouts, the jittered retries and the circuit breaker contained the blast radius — then write the blameless postmortem of it | industry | [`curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-02-dependency-outage-drill.md`](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-02-dependency-outage-drill.md) |
| A rolling deploy under a load generator with a deliberately broken control run beside it, so the learner *sees* the dropped requests that honest probes and a drained shutdown are what prevent | industry | [`curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-01-zero-dropped-requests-rolling-deploy.md`](curriculum/week-11-kubernetes-deployment-health-readiness-graceful-shutdown-and-reliability/challenges/challenge-01-zero-dropped-requests-rolling-deploy.md) |
| Native fuzzing against a parser with a real, findable bug in it — the learner runs the engine, reads the crasher it writes, and fixes the input class rather than the one input | university | [`curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/exercise-03-fuzz-target_test.go`](curriculum/week-08-testing-benchmarking-and-fuzzing/exercises/exercise-03-fuzz-target_test.go) |
| A performance claim is only accepted with evidence: ten runs each side, a `benchstat` delta and a p-value, and a profile naming the frame that changed | both | [`curriculum/week-08-testing-benchmarking-and-fuzzing/challenges/challenge-02-profile-and-optimize.md`](curriculum/week-08-testing-benchmarking-and-fuzzing/challenges/challenge-02-profile-and-optimize.md) |
| A schema evolved across a four-cell old-client / new-client by old-server / new-server compatibility matrix, gated by a breaking-change check — the version-skew problem a deployed API actually has | both | [`curriculum/week-07-grpc-and-protocol-buffers/challenges/challenge-02-protobuf-schema-evolution.md`](curriculum/week-07-grpc-and-protocol-buffers/challenges/challenge-02-protobuf-schema-evolution.md) |
| Landing a pull request in a real open-source Go project as assessed coursework, in the ecosystem the course teaches | both | [`curriculum/week-12-capstone-ship-a-cloud-native-microservice/challenges/challenge-01-merge-an-open-source-go-pr.md`](curriculum/week-12-capstone-ship-a-cloud-native-microservice/challenges/challenge-01-merge-an-open-source-go-pr.md) |

**Gaps we declare.** Four, stated plainly. First, the ledger's own gap: C30 does not teach numerical parallelism — no OpenMP, no MPI, no GPU kernels, and no analytic speedup argument; its parallelism is goroutines over IO-bound work, measured empirically. Second, the Server-Side Web Programming claim is scoped to the service tier — no browser, no templates, no client-side work — and its authentication content is one middleware, one identity convention and the status-code decision, not sessions, password storage or OAuth. Third, C30 ships no continuous-integration unit and no pipeline configuration; the linters and the tests are run locally and checked at the phase gates. Fourth, C30 carries no collapsible `Under the hood` blocks: the internals a syllabus would stop at are taught inline in the lecture notes and the week overviews instead, which means the depth is present but a learner cannot skip past it as cleanly as the framework intends.

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
