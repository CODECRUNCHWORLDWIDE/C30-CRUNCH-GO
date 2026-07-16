# Week 12 Resources — Capstone: Ship a Cloud-Native Microservice

The canonical reading list for the capstone week. Every URL has been opened and every technique referenced by the lectures, exercises, challenges, or the defense brief. The capstone reuses the whole track's stack, so this list adds the *integration, documentation, and defense* references on top of the Weeks 9–11 lists — re-open those for the technique-level docs.

Grouped by the role the document plays in the capstone — the spec, ADRs, the load-and-trace report, the postmortem and runbook, the demo and interview loop, and the career pack. The "adjacent" section is the most valuable for the engineer who wants to outgrow the lectures.

## The capstone spec

- **The C30 SYLLABUS — Capstone section** — `../../SYLLABUS.md` (§ Capstone). The fixed technical bar, the ten deliverables, and the grading axes. The contract; re-read it before you start.
- **The C30 SYLLABUS — Assessment matrix** — `../../SYLLABUS.md` (§ Assessment matrix). The capstone is 30%, cannot be carried, and a non-functional one does not pass.
- **The C30 README — Capstone preview** — `../../README.md` (§ Capstone preview). The one-paragraph statement of what the service is.

## Architecture decision records

- **The ADR community** — <https://adr.github.io/>. What an ADR is, the format, the lifecycle.
- **architecture-decision-record (templates)** — <https://github.com/joelparkerhenderson/architecture-decision-record>. Nygard's original format and many template variants.
- **Effective Go** — <https://go.dev/doc/effective_go>. The idiom your code-quality ADRs reference.
- **The Go spec** — <https://go.dev/ref/spec>. The language reference for the Go-deep-dive defense questions.

## The load-test-and-trace report

- **OpenTelemetry docs** — <https://opentelemetry.io/docs/>. Reading the trace; the span model; context propagation (carried from Week 9).
- **OpenTelemetry Go** — <https://github.com/open-telemetry/opentelemetry-go>. The SDK the capstone uses.
- **The Go pprof blog** — <https://go.dev/blog/pprof>. Capturing and reading a CPU/heap/goroutine profile for an in-process finding.
- **`net/http/pprof`** — <https://pkg.go.dev/net/http/pprof>. Exposing the profiling endpoints (gated to non-prod).
- **`hey`** — <https://github.com/rakyll/hey> and **`k6`** — <https://k6.io/docs/>. The load generators.

## The postmortem and the runbook

- **SRE workbook — postmortem culture** — <https://sre.google/workbook/postmortem-culture/>. The blameless postmortem; the structure (summary, timeline, what happened, action items).
- **SRE workbook — playbooks** — <https://sre.google/workbook/playbooks/>. The shape of a runbook someone else can follow cold.
- **SRE workbook — incident response** — <https://sre.google/workbook/incident-response/>. Mitigate before you diagnose; the 3am discipline behind the runbook.
- **SRE book — table of contents** — <https://sre.google/sre-book/table-of-contents/>. The cascading-failures and overload chapters behind the reliability drill.

## The demo and the interview loop

- **The C30 SYLLABUS — career engineering pack** — `../../SYLLABUS.md` (§ Career engineering pack). The interview-prep topics, the system-design prompts, the behavioural drills, the portfolio recommendations.
- **The Go memory model** — <https://go.dev/ref/mem>. The happens-before guarantees behind the concurrency defense questions.
- **The race detector article** — <https://go.dev/doc/articles/race_detector>. What `-race` proves and cannot prove.
- **Postgres transaction isolation** — <https://www.postgresql.org/docs/current/transaction-iso.html>. Lost update, write skew — the services-and-data defense questions.
- **The Kubernetes probes doc** — <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>. The liveness-vs-readiness question, the most-probed cloud-native-ops topic.

## The career pack

- **Go contribution guide** — <https://go.dev/doc/contribute>. Contributing to Go itself; the conventions most Go projects mirror.
- **The ecosystem repos** — `chi` (<https://github.com/go-chi/chi>), `pgx` (<https://github.com/jackc/pgx>), `sqlc` (<https://github.com/sqlc-dev/sqlc>), `opentelemetry-go` (<https://github.com/open-telemetry/opentelemetry-go>), `gobreaker` (<https://github.com/sony/gobreaker>), `kind` (<https://github.com/kubernetes-sigs/kind>) — the projects to contribute the challenge-01 PR to.
- **CNCF landscape** — <https://landscape.cncf.io/>. The Go-heavy cloud-native ecosystem to find a contribution target in.
- **The GPL-3.0 license** — <https://www.gnu.org/licenses/gpl-3.0.en.html>. The capstone repo's license.

## Adjacent reading — strongly recommended

- **"How to write a good postmortem" (SRE community)** — the worked examples of blameless postmortems; cross-reference with the SRE workbook chapter.
- **The Twelve-Factor App** — <https://12factor.net/>. The principles the whole capstone rests on (carried from Week 10); a defense reviewer may ask which factors your service satisfies.
- **Kubernetes "Production-Grade" docs** — <https://kubernetes.io/docs/concepts/overview/>. The mental model behind the operational defense answers.
- **"Designing Data-Intensive Applications" (Kleppmann), ch. on consistency/failover** — the depth behind the system-design dossiers (the DB-failover prompt in particular).

## Bookmarks worth saving past C30

- The ADR community and templates.
- The Google SRE workbook (postmortems, playbooks).
- The OpenTelemetry docs and the Go pprof blog.
- The Kubernetes probes doc.
- The career-pack section of the SYLLABUS — your interview-prep map.

By the end of this week these are the references you reach for not as a learner but as an engineer: writing the next ADR, running the next postmortem, drafting the next runbook, and preparing for the next interview loop. The capstone is the worked example you carry into all of them.
