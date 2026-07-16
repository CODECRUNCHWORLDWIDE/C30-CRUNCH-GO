# Week 12 — Homework

Six problems that drill the senior backend-Go interview loop, using your capstone as the worked example. They are the interview-prep pack made concrete; do them as defense rehearsal. Each is sized to ~45 minutes. Where a problem asks for an answer, write it as you would *say* it in an interview — concrete, specific, pointing at your code.

## Problem 1 — The Go language deep-dive

Answer these as you would in an interview, each in a short paragraph, pointing at your capstone code where relevant:

1. Pointer vs value receivers — when do you use each, and what is the consistency rule? Show one of each from your code and justify it.
2. "Accept interfaces, return structs" and consumer-defined interfaces — show a small interface defined at its consumer in your service.
3. Error wrapping — show an `fmt.Errorf("%w", err)` chain and an `errors.Is`/`errors.As` inspection from your code.
4. Generics vs interfaces — name one place you chose one over the other and why.
5. `defer`/`panic`/`recover` — why is `panic` not exception handling, and where (if anywhere) do you `recover` (the middleware)?

Cite the Go spec at <https://go.dev/ref/spec> and Effective Go at <https://go.dev/doc/effective_go>.

Deliverable: `homework/01-go-deep-dive.md`.

## Problem 2 — Concurrency at interview depth

1. Goroutine leaks — show a goroutine in your code that could leak and how `context` cancellation prevents it. State the "who closes the channel" rule.
2. Channel vs mutex — name one place you used each and why each was the simpler answer there.
3. `context` cancellation and deadlines — trace one `context` from the HTTP handler down to the `pgx` query and explain what cancels when.
4. The race detector — what does `go test -race` *prove*, and what can it *not* prove? (It proves a race occurred in this run; it cannot prove none can.)
5. The Go memory model at interview depth — what does a `sync.Mutex`/channel guarantee about happens-before?

Cite the memory model at <https://go.dev/ref/mem> and the race-detector doc at <https://go.dev/doc/articles/race_detector>.

Deliverable: `homework/02-concurrency.md`.

## Problem 3 — Services and data trade-offs

1. HTTP server design — explain your handler/service/repository seam and why the layering matters.
2. gRPC vs REST — when each, and how do you serve both over shared logic without divergence?
3. Transaction isolation and concurrent-write hazards — explain lost update and write skew, and how your one multi-step transaction avoids them.
4. Migration strategy — explain expand-then-contract and why it makes a rollback safe.
5. Typed-query vs ORM — your `sqlc` ADR, spoken aloud.

Cite the Postgres transaction-isolation doc at <https://www.postgresql.org/docs/current/transaction-iso.html> and the sqlc docs at <https://docs.sqlc.dev/>.

Deliverable: `homework/03-services-and-data.md`.

## Problem 4 — Cloud-native operations

The questions a cloud-native shop's loop *always* asks. Answer each, pointing at your Weeks 9–11 work:

1. **Liveness vs readiness** — the difference, and why your liveness probe does not check the database (the restart-storm answer). This is the question that separates operators from authors.
2. **Graceful shutdown on `SIGTERM`** — walk the order; why is the pool closed last?
3. **Distroless / 12-factor** — why distroless, and what makes config 12-factor?
4. **Retries-with-jitter and circuit breaking** — why jitter, and what does an open breaker do?
5. **Reading a trace and a flame graph** — describe localising a latency finding from the dashboard to a span (your load-and-trace report).

Cite the probes doc at <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/> and the SRE book at <https://sre.google/sre-book/>.

Deliverable: `homework/04-cloud-native-ops.md`.

## Problem 5 — A system-design prompt, cold

Pick one SYLLABUS prompt you did *not* write a dossier for (challenge-02) and design it in 45 minutes, naming the Go primitives:

- design a URL shortener / a rate limiter / a job queue / a feature-flag service / a zero-drop deploy / a DB-failover-surviving service.

Cover the data model (named indexes), the scaling story (a bottleneck and a load number), the failure modes (detection + mitigation), and the Go primitives (worker pool, `context`, `pgx`, breaker, readiness probe). Push it one step past your capstone.

Cite the SRE book and the Go primitives from across C30.

Deliverable: `homework/05-system-design.md`.

## Problem 6 — The behavioural drills

Write framework answers (situation / what you did / what you learned) for five backend-specific prompts, using the track as your material:

1. "Walk me through a service you owned in production." (Your capstone.)
2. "The worst data race you've shipped." (The Week 4 race you found with `-race`.)
3. "A deploy that went wrong and what you changed." (Your reliability-drill finding.)
4. "A time you read code you didn't write." (Your open-source PR, challenge-01.)
5. "A decision you would make differently now." (Your least-confident ADR.)

Keep each to a short paragraph; concrete and specific, pointing at the artifact.

Cite none — these are about your work.

Deliverable: `homework/06-behavioural.md`.

## Submission

Push the six deliverables on a branch named `week12-homework/<your-handle>` and open a PR against the C30 curriculum repository. These are your defense rehearsal — the PR description should note which two questions you are *least* comfortable answering, so you can drill them before the defense. The single most common defense failure is "it works, trust me" instead of a specific answer pointing at the code — this homework is how you avoid it.

Cited pages this homework draws from: <https://go.dev/ref/spec>, <https://go.dev/ref/mem>, <https://go.dev/doc/articles/race_detector>, <https://www.postgresql.org/docs/current/transaction-iso.html>, <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>, and the Google SRE book at <https://sre.google/sre-book/>.
