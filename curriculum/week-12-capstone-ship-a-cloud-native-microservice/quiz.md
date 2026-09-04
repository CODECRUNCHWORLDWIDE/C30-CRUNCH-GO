# Week 12 — Quiz

Ten multiple-choice questions revisiting the whole track at interview depth — the senior backend-Go loop in quiz form. There is no Week 12 quiz in the assessment matrix (the capstone replaces it), so treat this as defense rehearsal. The answer key with reasoning is at the bottom.

## Question 1 — Liveness vs readiness

Your liveness probe should check the database:

- (A) Yes — a liveness probe should verify the pod can serve.
- (B) No — liveness depends on nothing external; a DB-checking liveness probe turns a transient DB blip into a restart storm. Readiness checks the database.
- (C) Only in production.
- (D) Only if there is no readiness probe.

<details>
<summary>Answer</summary>

**(B).** Liveness depends on nothing external; a DB-checking liveness probe restarts every healthy pod on a DB blip — a self-inflicted outage. Readiness checks the database. The question that separates operators from authors. Citation: <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>.

</details>

## Question 2 — Graceful shutdown order

In your graceful shutdown, the pgx pool is closed:

- (A) First, to free connections fast.
- (B) Last — after the HTTP and gRPC servers have drained — so no in-flight handler loses its connection mid-query.
- (C) It does not matter.
- (D) By a `defer` in each handler.

<details>
<summary>Answer</summary>

**(B).** Drain the servers first, close the pool last, so no in-flight handler loses its connection mid-query. Drain the things that use the resource before closing the resource. Citation: <https://pkg.go.dev/net/http#Server.Shutdown>.

</details>

## Question 3 — Why both gRPC and REST

The capstone serves both gRPC and REST because:

- (A) gRPC is faster and REST is slower.
- (B) Cloud-native services speak gRPC to each other and REST to the world; serving both over one shared service layer keeps the surfaces from diverging while reaching both audiences.
- (C) REST is deprecated.
- (D) gRPC cannot do JSON.

<details>
<summary>Answer</summary>

**(B).** gRPC between services, REST to the world; one shared service layer keeps the two surfaces consistent. Citation: the SYLLABUS Week 7 framing.

</details>

## Question 4 — `sqlc` vs ORM

Choosing `sqlc` over an ORM trades:

- (A) Type safety for convenience.
- (B) The ORM's convenience and lazy-loading for legible, reviewable SQL, compile-time-checked queries, visible N+1s, and explicit transaction control.
- (C) Nothing — they are equivalent.
- (D) Speed for safety.

<details>
<summary>Answer</summary>

**(B).** `sqlc` trades the ORM's convenience for legible, reviewable, compile-time-checked SQL and explicit transaction control. Citation: <https://docs.sqlc.dev/>.

</details>

## Question 5 — Retries and jitter

Adding jitter to exponential-backoff retries:

- (A) Makes retries slower for no benefit.
- (B) Randomizes the wait so clients that failed together do not retry in lockstep, avoiding a synchronized thundering herd on the recovering dependency.
- (C) Is required by the `context` package.
- (D) Caps the number of attempts.

<details>
<summary>Answer</summary>

**(B).** Jitter randomizes the retry wait so clients do not synchronize into a thundering herd on the recovering dependency. Citation: <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>.

</details>

## Question 6 — The circuit breaker, open

When your circuit breaker is open and a request arrives:

- (A) The request is queued until the dependency recovers.
- (B) The breaker fails fast — returns an error immediately without calling the dependency — keeping the service responsive and giving the dependency a rest, then half-opens to test recovery.
- (C) The request is sent anyway, to test the dependency.
- (D) The pod restarts.

<details>
<summary>Answer</summary>

**(B).** An open breaker fails fast (no call), keeping the service responsive and resting the dependency, then half-opens to test recovery. Citation: <https://github.com/sony/gobreaker>.

</details>

## Question 7 — The race detector

`go test -race` passing on your suite proves:

- (A) Your code has no data races, ever.
- (B) No data race *occurred during the tested runs* — it cannot prove none *can* occur on an untested interleaving; it is a detector of races that happen, not a proof of their absence.
- (C) Your code is thread-safe by construction.
- (D) The code compiles.

<details>
<summary>Answer</summary>

**(B).** `-race` detects races that *occur* in the tested runs; it cannot prove none can occur on an untested interleaving. It is a detector, not a proof of absence. Citation: <https://go.dev/doc/articles/race_detector>.

</details>

## Question 8 — Expand-then-contract migrations

Migrations being expand-only (additive) matters because:

- (A) It makes them faster.
- (B) It keeps the previous version's code able to read the current schema, so a rollback to the previous deployment is schema-safe — the rollback target does not hit a column it does not understand.
- (C) It reduces the migration count.
- (D) It is required by golang-migrate.

<details>
<summary>Answer</summary>

**(B).** Expand-only migrations keep the previous version's code valid against the current schema, making a rollback to the previous deployment schema-safe. Citation: the migration strategy from Week 6 and the runbook.

</details>

## Question 9 — The 3am rule

A deploy you just shipped is throwing 500s. Your first move per the runbook is:

- (A) SSH into a pod and debug it under load.
- (B) Roll back first (move traffic to the known-good version), then diagnose — mitigate before you investigate.
- (C) Scale up to absorb the errors.
- (D) Delete and recreate the Deployment.

<details>
<summary>Answer</summary>

**(B).** Roll back first (mitigate), then diagnose. An incident is contained by moving traffic to the known-good version, not by debugging the broken one under load. Citation: <https://sre.google/workbook/incident-response/>.

</details>

## Question 10 — What the capstone is graded on

The capstone is graded primarily on:

- (A) Visual polish and a slick UI.
- (B) Engineering and operability — code quality, end-to-end correctness, concurrency/reliability, cloud-native posture, observability, and the reliability-drill outcome — with visual polish earning zero points.
- (C) Lines of code.
- (D) The number of features.

<details>
<summary>Answer</summary>

**(B).** Engineering and operability across the seven grading axes; visual polish earns zero. A non-functional capstone does not pass regardless of every other score. Citation: the SYLLABUS assessment matrix.

</details>

---

## Self-assessment

- 9-10: you are ready for the defense and the senior backend-Go interview loop.
- 7-8: re-read the homework drill for the area you missed; the cloud-native-ops and concurrency questions are the ones the defense leans on.
- 5-6: re-read the Week 9–11 lectures and your own labs; the defense assembles every pattern this quiz tests.
- 0-4: the defense will not go well without the conceptual foundation — rewind to the relevant week's lectures. The capstone is 30% of the grade and a non-functional one does not pass.
