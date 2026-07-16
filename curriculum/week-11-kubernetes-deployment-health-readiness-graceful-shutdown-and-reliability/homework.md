# Week 11 — Homework

Six practice problems consolidating the Kubernetes, graceful-shutdown, and reliability material. They are sized to ~45 minutes each. Do them after the lectures and the exercises; several feed directly into Lab 11 and the reliability drill. Cite the URLs you used while solving each one in the commit message of your homework branch.

## Problem 1 — Liveness vs readiness, reproduced

Wire `/healthz` (liveness, no dependencies) and `/readyz` (readiness, pings Postgres) into `notes`, then demonstrate the difference in three states on `kind` and document each:

1. **Healthy.** Both return 200; the pod is `1/1 Ready`.
2. **DB down.** Scale Postgres to 0. Show `/healthz` still returns 200 (process alive) and `/readyz` returns 503 (DB unreachable); show the pod goes `0/1` but `RESTARTS` stays 0. Explain why a liveness probe that checked the DB would cause a restart storm here.
3. **Recovery.** Scale Postgres back; readiness recovers with no restart.

Cite the probes doc at <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>.

Deliverable: `homework/01-liveness-vs-readiness.md`.

## Problem 2 — The graceful-shutdown budget

Document the shutdown budget arithmetic for `notes` and prove it holds:

1. State `terminationGracePeriodSeconds`, the `preStop`/readiness-fail delay, the shutdown budget, and the safety margin, and show they sum to less than the grace period.
2. Deliberately set the shutdown budget *higher* than the grace period, terminate a pod, and capture the `SIGKILL` (exit 137 in `kubectl describe`, the drain log cut off mid-way).
3. Restore the correct budget and show a clean drain (exit 0, `drain complete` logged).

Explain why the budget must be under the grace period and what `SIGKILL` does to an in-flight request.

Cite the pod-termination doc at <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination> and `http.Server.Shutdown` at <https://pkg.go.dev/net/http#Server.Shutdown>.

Deliverable: `homework/02-shutdown-budget.md`.

## Problem 3 — The close-order experiment

Demonstrate why the pgx pool must be closed *after* the servers drain, not before:

1. Build a variant that closes the pool *first*, then drains. Hold an in-flight request (`?slow=5`) and terminate the pod. Capture the in-flight handler's error (a "conn closed"/pool-closed error).
2. Build the correct variant (drain servers, close pool last). Repeat. The in-flight request completes with 200.

Explain the ordering rule (drain the things that *use* the resource before closing the resource) and how it generalizes to the trace exporter and any other dependency.

Cite the `pgxpool` doc at <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>.

Deliverable: `homework/03-close-order.md`.

## Problem 4 — Backoff and jitter, measured

Implement and compare three retry strategies against a flapping dependency (a test server that fails the first N requests in each window):

1. **No jitter** — fixed exponential backoff. Run many concurrent clients and show the synchronized retry spikes (the thundering herd).
2. **Full jitter** — random `[0, backoff)`. Show the smoothed retry load.
3. Tabulate total time-to-success and peak concurrent retries for each.

Explain why full jitter completes faster under contention and avoids the herd, referencing the AWS article's findings.

Cite <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>.

Deliverable: `homework/04-backoff-jitter.md` plus the code.

## Problem 5 — The circuit-breaker state machine

Drive a `gobreaker` breaker through all three states and document each transition:

1. **Closed → Open**: send failing requests until the failure-rate threshold trips it; show the next call returns `ErrOpenState` immediately (fail fast).
2. **Open → Half-open**: wait the cooldown; show one trial request is let through.
3. **Half-open → Closed** (trial succeeds) and **Half-open → Open** (trial fails): demonstrate both.

For each state, state what happens to a request and why. Explain how the breaker keeps the service responsive when a dependency is down (no piled-up goroutines waiting on timeouts).

Cite <https://github.com/sony/gobreaker> and the cascading-failures chapter at <https://sre.google/sre-book/addressing-cascading-failures/>.

Deliverable: `homework/05-circuit-breaker.md` plus the code.

## Problem 6 — Load-shedding under saturation

Wire the load-shedding middleware into `notes`, then drive it past capacity and document the behaviour:

1. Set `NOTES_MAX_INFLIGHT` to a small number; drive `hey -z 20s -c 200`.
2. Show the accepted requests stay fast (low p99) while the excess returns 503 *fast* (microseconds), and the service does not crash.
3. Build a control variant with an *unbounded* queue (a blocking semaphore acquire, no `default`) and show latency collapse under the same load (every request slow, the service unresponsive).
4. Explain why bounded-concurrency-with-shedding degrades gracefully and unbounded-queueing collapses.

Cite the "Handling Overload" chapter at <https://sre.google/sre-book/handling-overload/>.

Deliverable: `homework/06-load-shedding.md` plus the code.

## Submission

Push the six deliverables on a branch named `week11-homework/<your-handle>` and open a PR against the C30 curriculum repository. The PR description links each file and includes a 100-word summary of what you learned. The single most common review comment is "where is your citation for this claim" — preempt it by linking the Kubernetes doc, the AWS article, or the SRE book chapter for every non-trivial assertion.

Cited pages this homework draws from: <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>, <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination>, <https://pkg.go.dev/net/http#Server.Shutdown>, <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>, <https://github.com/sony/gobreaker>, and the Google SRE book at <https://sre.google/sre-book/>.
