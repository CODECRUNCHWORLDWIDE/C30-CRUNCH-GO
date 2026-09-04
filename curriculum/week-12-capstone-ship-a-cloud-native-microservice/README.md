# Week 12 — Capstone: Ship a Cloud-Native Microservice — Integrating Everything Into One Deployed, Observable, Gracefully-Shutting-Down Go Service, and Defending It

Welcome to **C30 · Crunch Go**, Week 12 — the final week. You started at the Go tour in Week 1 and the concurrency model in Weeks 3–4; you built an HTTP service in Week 5, put it on Postgres in Week 6, gave it gRPC in Week 7, hardened it with tests and fuzzing in Week 8, instrumented it in Week 9, containerized it in Week 10, and deployed it to Kubernetes with reliability patterns in Week 11. This week you do the last thing a real system needs and the thing learner projects almost always skip: **you integrate it, deploy it, drill it, document it, and defend it.** The same service — gRPC + REST, Postgres through `pgx`+`sqlc`, `slog`/OTel/Prometheus observability, a distroless container, a Kubernetes Deployment with honest probes and graceful shutdown — becomes one coherent capstone you would send a hiring manager. You do not invent a new project. You ship the one you built across eleven weeks.

The first thing to internalize is that **the capstone is one substantial service taken all the way, not a parade of toys**. The technical bar is fixed (gRPC + REST, Postgres with reversible migrations and a transaction, observability to Jaeger and Grafana, a distroless 12-factor container, a Kubernetes deployment with honest probes and graceful shutdown, a reliability drill); the product domain is open — a notes service, a URL shortener, a feature-flag service, an inventory service, a short-order job queue, anything you can defend in a scope review. The grading rewards a *working, operable* service over a feature-rich broken one. The full specification is the SYLLABUS capstone section, restated and made actionable in `mini-project/README.md`.

The second thing to internalize is that **architecture decision records turn "I built it this way" into "I chose this way, and here is what I traded"**. A senior reviewer does not want a tour of your code; they want the reasons. Why a channel and not a mutex in the worker path? Why `sqlc` and not an ORM? Why distroless and not alpine? Why `maxUnavailable: 0`? An ADR is a short, dated record of one decision, its context, the alternatives, and the consequences — and the defense Q&A is, in effect, a live ADR review. Lecture 1 covers writing them; the defense grades whether you can defend them. The reference is Michael Nygard's original ADR pattern at <https://github.com/joelparkerhenderson/architecture-decision-record> and the ADR community at <https://adr.github.io/>.

The third thing to internalize is that **the load-test-and-trace report is the artifact that proves the observability earns its keep**. Anyone can add OpenTelemetry; the report proves you can *use* it. You drive the service under load, capture the Grafana RED dashboard and a Jaeger trace, find one latency finding (a slow query, an N+1, a lock-contention spike), and document the fix — the same find-it-on-the-dashboard-localise-it-to-the-span loop from Week 9, now on the deployed service under real load. Lecture 1 specifies the report; the defense grades it. The reference is the OpenTelemetry docs at <https://opentelemetry.io/docs/> and the `pprof` blog post at <https://go.dev/blog/pprof>.

The fourth thing to internalize is that **the reliability-drill postmortem is where you prove the service survives failure, on yourself, before the demo**. You run one drill — a rolling deploy under load (Week 11 challenge-01), a dependency outage (challenge-02), or a saturation/load-shedding event — and you write a ~3–5-page postmortem: what you did, what happened, what the mechanisms (readiness, graceful drain, timeouts, retries, breaker, shedding) did, and what you would change. A postmortem that says "and then it all worked" is not a postmortem; the discipline is documenting the failure honestly. Lecture 2 covers postmortem writing; the drill is graded. The reference is the SRE workbook's postmortem-culture chapter at <https://sre.google/workbook/postmortem-culture/>.

The fifth thing to internalize is that **the production runbook is the deliverable your future self thanks you for, and it is graded by being used**. `production-runbook.md` answers, in concrete commands, the questions you will be asked at 3am: how do I deploy, how do I roll back a bad rollout *and* a bad migration, the probe semantics for *this* service, the five most likely outages with the first three diagnostics each, and who to page (you are paging yourself — the discipline is the point). A runbook a grader cannot follow cold is not a runbook. Lecture 2 specifies its shape; the defense walks one section of it without you narrating. The reference is the SRE workbook's playbooks chapter at <https://sre.google/workbook/playbooks/>.

The sixth thing to internalize is that **the five-minute demo video and the live defense are where the work becomes legible — and visual polish earns nothing**. The video (voice-over required, no marketing edits) shows the service end to end: gRPC + REST returning the same domain data, a trace in Jaeger, the Grafana dashboard, a rolling deploy under load, and a clean graceful drain. The defense is a reviewer-panel Q&A that probes the concurrency model, the reliability decisions, and the architecture — the staff-engineer questions your drill answers. Lecture 3 covers demo discipline and the interview loop; the defense is 30% of the grade and a non-functional capstone does not pass regardless of every other score. The SYLLABUS assessment matrix is the contract.

The seventh thing to internalize is that **this is the senior backend-Go interview loop, dressed as a capstone**. The career pack — the Go language deep-dive, the concurrency questions, the services-and-data trade-offs, the cloud-native-operations questions (liveness vs readiness, graceful shutdown, distroless/12-factor, retries-with-jitter), the system-design prompts, and the behavioural drills — is exactly what a cloud-native shop's loop covers, and your capstone is the worked example you bring to it. Week 12 ends C30 by turning twelve weeks of building into a portfolio artifact and an interview you are ready for.

By the end of this week you will be the engineer who can take a Go service from `go mod init` to a deployed, observable, gracefully-shutting-down Kubernetes microservice, defend every architecture and reliability decision to a senior reviewer, and hand a hiring manager a repo, a runbook, a trace report, and a postmortem that prove you can ship *and* operate. That is the job. C30 ends here; you finish it operating.

## Learning objectives

By the end of this week, you will be able to:

- **Integrate** everything from Phases I–III into one deployed cloud-native service: gRPC + REST over shared logic, Postgres via `pgx`+`sqlc` with reversible migrations and a transaction, `slog`/OTel/Prometheus observability, a distroless 12-factor container, and a Kubernetes deployment with honest probes and graceful shutdown. Cite the SYLLABUS capstone spec.
- **Write** architecture decision records for the load-bearing choices (channel vs mutex, `sqlc` vs ORM, gRPC + REST, distroless, `maxUnavailable: 0`) and defend each in review. Cite <https://adr.github.io/>.
- **Produce** a load-test-and-trace report: drive the deployed service under load, capture the Grafana dashboard and a Jaeger trace, localise one latency finding, and document the fix. Cite <https://opentelemetry.io/docs/> and <https://go.dev/blog/pprof>.
- **Run** one reliability drill on yourself and write a ~3–5-page postmortem of it (rolling deploy under load, dependency outage, or saturation/load-shedding). Cite <https://sre.google/workbook/postmortem-culture/>.
- **Author** a `production-runbook.md`: every deploy/rollback command, the probe semantics, the five most likely outages and the first three diagnostics for each, the rollback for a bad rollout and a bad migration, and who to page. Cite <https://sre.google/workbook/playbooks/>.
- **Record** a five-minute demo video (voice-over, no marketing edits) showing gRPC + REST, a Jaeger trace, the Grafana dashboard, a rolling deploy under load, and a clean graceful drain.
- **Defend** the capstone in a reviewer-panel Q&A: the concurrency model, the reliability decisions, the architecture, and the senior backend-Go interview questions. Cite the SYLLABUS career pack.
- **Answer** the senior backend-Go interview loop — pointer vs value receivers, channel vs mutex, `context` cancellation, liveness vs readiness, graceful shutdown, distroless/12-factor, retries-with-jitter — pointing at your own code as the worked example.
- **Cite** the SYLLABUS capstone spec and assessment matrix, the ADR community, opentelemetry.io, the Go pprof blog, and the Google SRE workbook (postmortems, playbooks) for each deliverable.

## Standards this week meets

| Bar | What this week is measured against |
| --- | --- |
| University | `COP 4813` — Deploy the service so a second client can reach it, and defend the load-bearing design decisions behind it: the concurrency model, the data layer, the transports and the reliability behaviour. |
| Industry | Ship it and then answer for it: a running service, decision records for the choices that were arguable, a load-and-trace report, a drill postmortem, a production runbook, and a defence of all of it in front of reviewers who did not write it. |
| Beyond the bar | One pull request landed in a real open-source Go project as assessed coursework — a doc fix that clears up something genuinely confusing, a small bug fix with a test, or a scoped feature with the maintainer's buy-in; a whitespace-only change does not count — `challenges/challenge-01-merge-an-open-source-go-pr.md` |

## Prerequisites

- **Weeks 1–11 of C30 complete, Weeks 9–11 in particular.** This week integrates and defends the service those weeks built, instrumented, containerized, and deployed. If Lab 9 (observability), Lab 10 (container), or Lab 11 (Kubernetes + reliability) is not done, finish it first — there is nothing to integrate otherwise.
- **A working `kind` cluster and the deployed service from Week 11.** The capstone is the Week 11 deployment, made coherent and documented. The cluster, the manifests, the graceful shutdown, and the reliability patterns all carry forward.
- **The observability stack from Weeks 9–10.** Jaeger, Prometheus, and Grafana — in the compose stack for local work and as in-cluster (or port-forwarded) services for the deployed demo.
- **A public GitHub account.** The capstone repo is public, GPL-3.0, with a real README, the architecture diagram, and the reports.
- **A way to record a five-minute screen capture with voice-over.** Any screen recorder; the requirement is voice-over and no marketing edits.
- **The eleven weeks of muscle.** The defense probes the whole track — the concurrency model (Weeks 3–4), the service design (Weeks 5–7), the testing posture (Week 8), and the operational contract (Weeks 9–11). Skim your own labs before the defense; the reviewer will ask about them.

## Topics covered

- **Capstone integration.** Bringing the eleven weeks' work into one coherent, deployed service; the scope review; the architecture diagram that matches what you demo.
- **Architecture decision records.** The ADR format (context, decision, alternatives, consequences); the load-bearing decisions to record; the defense as a live ADR review.
- **The load-test-and-trace report.** Driving the deployed service under load (`hey`/`k6`); capturing the Grafana RED dashboard and a Jaeger trace; localising a latency finding from dashboard to span; the `pprof` capture and the measured fix.
- **The reliability-drill postmortem.** Running one drill on yourself; the postmortem structure (summary, timeline, what happened, what the mechanisms did, action items); blameless postmortem culture.
- **The production runbook.** Deploy/rollback commands; bad-rollout and bad-migration rollback; the probe semantics; the five-most-likely-outages-with-diagnostics; the observability entry points; who to page.
- **Demo discipline and the five-minute video.** The staged demo choreography; voice-over; no marketing edits; what to show and in what order.
- **The senior backend-Go interview loop.** The Go language deep-dive; concurrency; services and data; cloud-native operations; the system-design prompts; the behavioural drills.
- **Portfolio packaging.** The public repo; the README and architecture diagram; the reports; the open-source PR; the technical blog post; the profile that does not say "passionate."

## Weekly schedule

The schedule adds up to approximately **36 hours**, weighted heavily toward the capstone. Treat it as a target, not a contract. The capstone punishes leaving the deploy, the drill, or the runbook to the last day — they each surface problems that take a day to fix.

| Day       | Focus                                                                       | Lectures | Integration | Drill | Capstone | Self-Study | Daily Total |
|-----------|-----------------------------------------------------------------------------|---------:|------------:|------:|---------:|-----------:|------------:|
| Monday    | Capstone integration, ADRs, the load-test-and-trace report                   |   2h     |    3h       |  0h   |   0.5h   |    0.5h    |    6h       |
| Tuesday   | The reliability drill, the postmortem, the production runbook                |   2h     |    1h       |  2h   |   0.5h   |    0.5h    |    6h       |
| Wednesday | Demo discipline, the five-minute video, the interview loop                   |   2h     |    0h       |  0h   |   3h     |    1h      |    6h       |
| Thursday  | Challenges — the open-source PR, the system-design dossier                   |   0.5h   |    0h       |  0h   |   4h     |    1.5h    |    6h       |
| Friday    | Capstone integration day — deploy, report, runbook, rehearse                 |   0h     |    0h       |  0h   |   5.5h   |    0.5h    |    6h       |
| Saturday  | Record the demo, package the portfolio                                       |   0h     |    0h       |  0h   |   4h     |    0h      |    4h       |
| Sunday    | Defense dry-run, Q&A prep, interview-loop review                             |   0h     |    0h       |  0h   |   1.5h   |    0.5h    |    2h       |
| **Total** |                                                                             | **8.5h** | **7h**      | **2h**| **23.5h**|  **4.5h**  |   **38h**   |

## How to navigate this week

| File | What's inside |
|------|---------------|
| [README.md](./README.md) | This overview (you are here) |
| [resources.md](./resources.md) | The SYLLABUS capstone spec, the ADR community, opentelemetry.io, the Go pprof blog, the SRE workbook (postmortems, playbooks), and the interview-prep references — every URL opened and verified |
| [lecture-notes/01-integration-adrs-and-the-load-test-and-trace-report.md](./lecture-notes/01-integration-adrs-and-the-load-test-and-trace-report.md) | Bringing the eleven weeks into one service; writing ADRs for the load-bearing decisions; the load-test-and-trace report — driving load, capturing the dashboard and the trace, localising and fixing a finding |
| [lecture-notes/02-reliability-drill-postmortem-and-the-production-runbook.md](./lecture-notes/02-reliability-drill-postmortem-and-the-production-runbook.md) | Running one drill on yourself; the blameless postmortem; the `production-runbook.md` spec — deploy, rollback (rollout + migration), probe semantics, the five outages, who to page |
| [lecture-notes/03-demo-discipline-and-the-senior-backend-go-interview-loop.md](./lecture-notes/03-demo-discipline-and-the-senior-backend-go-interview-loop.md) | The five-minute demo choreography; the defense Q&A; the senior backend-Go interview loop — Go, concurrency, services/data, cloud-native operations, system design, behavioural |
| [exercises/exercise-01-write-the-adrs.md](./exercises/exercise-01-write-the-adrs.md) | Write five ADRs for the load-bearing decisions in your capstone |
| [exercises/exercise-02-load-test-and-trace.md](./exercises/exercise-02-load-test-and-trace.md) | Drive the deployed service under load, capture the dashboard and a trace, localise and fix one finding |
| [exercises/exercise-03-draft-the-runbook.md](./exercises/exercise-03-draft-the-runbook.md) | Draft `production-runbook.md` and have a peer follow one section cold |
| [exercises/SOLUTIONS.md](./exercises/SOLUTIONS.md) | Worked examples — a model ADR, a model load-finding writeup, a model runbook section — with what graders look for and common stumbles |
| [challenges/challenge-01-merge-an-open-source-go-pr.md](./challenges/challenge-01-merge-an-open-source-go-pr.md) | Land one PR in an open-source Go project (chi/pgx/sqlc/OTel-Go/a CNCF tool) |
| [challenges/challenge-02-the-system-design-dossier.md](./challenges/challenge-02-the-system-design-dossier.md) | Write two system-design dossiers from the SYLLABUS prompts, with the Go primitives and cloud-native pieces named |
| [mini-project/README.md](./mini-project/README.md) | **The capstone defense brief** — the full deliverable spec, the deployed topology, the live demo script, the Q&A, the grading rubric, and the final submission |
| [homework.md](./homework.md) | Six consolidation problems — the interview-loop drills (Go, concurrency, services/data, cloud-native ops, system design, behavioural) |
| [quiz.md](./quiz.md) | 10 multiple-choice questions revisiting the whole track at interview depth — the senior backend-Go loop in quiz form |

## The capstone contract

C30 has accumulated three contracts: the clean-code contract (`go vet`/`staticcheck`/`go test -race`), the container contract (one distroless image from the environment, Week 10), and the operational contract (honest readiness, graceful drain, contained dependency failure, Week 11). The capstone is where all three are demonstrated at once, under load, in front of a reviewer: **a single Go service, deployed to Kubernetes, that serves gRPC and REST over shared logic against Postgres, is observable enough to localise a latency regression, drains its work on `SIGTERM`, survives a reliability drill, and can be operated by someone reading your runbook.** A capstone that is feature-rich but cannot deploy, has no tests, or has no runbook fails. A capstone that does one domain correctly, deploys with honest probes, traces a request end to end, and hands the next operator a followable runbook passes. Visual polish earns nothing; engineering and operability earn everything.

> **Note on tooling.** The capstone reuses the whole track's stack: Go 1.22+, `chi`, `pgx`+`sqlc`, `golang-migrate`, `buf`+`protoc-gen-go`/`-go-grpc`, the OpenTelemetry Go SDK + Jaeger, the Prometheus client + Grafana, the Week 10 distroless image, and the Week 11 `kind` + manifests + `gobreaker` + the hand-written retry/shed code. Load: `hey` or `k6`. Profiling: `net/http/pprof` and `go tool pprof`. Docs: ADRs in `docs/adr/`, the reports and the runbook in the repo root. All free and open source; the whole capstone is defensible on local `kind` with no cloud account (a managed cluster is optional, for a public URL).
