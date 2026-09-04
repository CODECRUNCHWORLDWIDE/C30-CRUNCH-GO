# Week 11 — Quiz

Ten multiple-choice questions covering Kubernetes Deployments, liveness/readiness probes, graceful shutdown, rolling updates, timeouts, retries with jitter, circuit breaking, and load-shedding. Treat it as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — Liveness vs readiness

A Postgres outage makes every `notes` pod restart in a storm. The most likely bug is:

- (A) The Deployment has too few replicas.
- (B) The **liveness** probe checks the database, so a DB blip fails liveness and the kubelet restarts every (healthy) process — a self-inflicted outage. Liveness must depend on nothing external.
- (C) The readiness probe is too slow.
- (D) `maxUnavailable` is set wrong.

<details>
<summary>Answer</summary>

**(B).** A liveness probe that checks the database turns a transient DB blip into a restart storm of healthy processes. Liveness depends on nothing external; only readiness checks dependencies. This is the question that separates operators from authors. Citation: <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>.

</details>

## Question 2 — What readiness must check

The readiness probe (`/readyz`) for `notes`:

- (A) Must check nothing, like liveness.
- (B) Must check Postgres (and any hard dependency), so a pod that cannot serve is pulled from the Service endpoints — without being restarted.
- (C) Should restart the pod when it fails.
- (D) Should be the same handler as liveness.

<details>
<summary>Answer</summary>

**(B).** Readiness answers "should this pod get traffic?" — so it checks the dependencies the pod needs to serve. A failing readiness pulls the pod from the Service (no restart); when the dependency returns, readiness recovers. Citation: the same probes doc.

</details>

## Question 3 — The termination sequence

When Kubernetes terminates a pod, two things happen concurrently:

- (A) The pod is deleted and recreated.
- (B) The pod is removed from the Service endpoints AND the container receives `SIGTERM`; these propagate via different components, so `SIGTERM` often arrives before the endpoint removal has propagated everywhere.
- (C) The container is `SIGKILL`ed immediately.
- (D) The readiness probe is disabled.

<details>
<summary>Answer</summary>

**(B).** Endpoint removal and `SIGTERM` propagate via different components and asynchronously; `SIGTERM` often arrives first, which is why the readiness-fail-then-wait (or `preStop` sleep) pattern exists. Citation: <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination>.

</details>

## Question 4 — The close-order rule

In a graceful shutdown, the pgx pool should be closed:

- (A) First, before draining the servers.
- (B) After the HTTP and gRPC servers have drained, so no in-flight handler loses its connection mid-query.
- (C) It does not matter.
- (D) Never — let the process exit close it.

<details>
<summary>Answer</summary>

**(B).** Drain the servers first, close the pool last — otherwise an in-flight handler loses its connection mid-query. Drain the things that use the resource before closing the resource. Citation: <https://pkg.go.dev/net/http#Server.Shutdown>.

</details>

## Question 5 — The shutdown budget

The shutdown budget (`NOTES_SHUTDOWN_TIMEOUT`) must be:

- (A) Equal to `terminationGracePeriodSeconds`.
- (B) Less than `terminationGracePeriodSeconds` (with a margin), so the process finishes draining and exits before Kubernetes sends `SIGKILL`.
- (C) Greater than the grace period, for safety.
- (D) Unset — Go handles it automatically.

<details>
<summary>Answer</summary>

**(B).** The budget must be under the grace period with a margin, or Kubernetes `SIGKILL`s the process mid-drain — the exact thing graceful shutdown prevents. Citation: the pod-termination doc.

</details>

## Question 6 — Why a rolling deploy drops zero requests

A rolling deploy with `maxUnavailable: 0` drops zero requests because:

- (A) Kubernetes pauses all traffic during the rollout.
- (B) A new pod takes traffic only after its readiness probe passes (honest readiness), and an old pod drains its in-flight work on `SIGTERM` before dying (graceful drain) — both mechanisms present make the rollout a handoff, not a swap.
- (C) The Service buffers requests.
- (D) Single-replica deployments cannot drop requests.

<details>
<summary>Answer</summary>

**(B).** Honest readiness (new pod takes traffic only when ready) and graceful drain (old pod finishes in-flight work) are the two independent mechanisms; both present make the rollout a handoff. Remove either and requests drop. Citation: <https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment>.

</details>

## Question 7 — Timeouts

A `notes` service with no `context` deadline on its database calls, when Postgres gets slow:

- (A) Returns errors quickly.
- (B) Wedges — each request blocks forever waiting on the slow DB, goroutines and connections pile up, the pool exhausts, and a slow *dependency* makes the *service* unhealthy. A timeout converts the hang into a fast, handleable failure.
- (C) Automatically retries.
- (D) Restarts the pod.

<details>
<summary>Answer</summary>

**(B).** No timeout means each request blocks forever on a slow dependency; goroutines and connections pile up, the pool exhausts, and the service wedges. A `context` deadline converts the hang into a fast failure you can handle. Citation: <https://go.dev/blog/context>.

</details>

## Question 8 — Retries and jitter

Retrying a failed downstream call with exponential backoff but **no jitter** risks:

- (A) Nothing — jitter is cosmetic.
- (B) A thundering herd: every client that failed at the same instant retries at the same instant, synchronizing the load and keeping the recovering dependency on its knees. Jitter randomizes the wait to spread retries out.
- (C) Retrying too slowly.
- (D) Infinite retries.

<details>
<summary>Answer</summary>

**(B).** Backoff without jitter synchronizes retries into a thundering herd that keeps the dependency down. Full jitter randomizes the wait and spreads the load. Citation: <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>.

</details>

## Question 9 — The circuit breaker

When a circuit breaker is **open**:

- (A) It sends every request to the dependency to test it.
- (B) It fails fast — returns an error immediately without sending the request — for a cooldown, so the service stays responsive (no piled-up goroutines waiting on timeouts) and the dead dependency gets a rest; then it half-opens to test recovery.
- (C) It restarts the dependency.
- (D) It queues requests until the dependency recovers.

<details>
<summary>Answer</summary>

**(B).** An open breaker fails fast (no request sent) for a cooldown, keeping the service responsive and giving the dependency a rest, then half-opens to test recovery. Citation: <https://github.com/sony/gobreaker> and <https://sre.google/sre-book/addressing-cascading-failures/>.

</details>

## Question 10 — Load-shedding

A load-shedding middleware that bounds inbound concurrency should, when full:

- (A) Queue the excess requests until a slot frees.
- (B) Return 503 (with `Retry-After`) immediately for excess requests, so the accepted requests stay healthy and the service degrades gracefully instead of collapsing — bounded concurrency, not an unbounded queue.
- (C) Crash to force a restart.
- (D) Increase the concurrency limit automatically.

<details>
<summary>Answer</summary>

**(B).** Shedding returns a fast 503 for excess so the accepted requests stay healthy — bounded concurrency, not an unbounded queue that turns saturation into a latency collapse. Citation: <https://sre.google/sre-book/handling-overload/>.

</details>

---

## Self-assessment

- 9-10: you can deploy a Go service to Kubernetes, drain it gracefully, and defend the reliability patterns without further reading.
- 7-8: re-read the lecture notes on the questions you missed; the liveness-vs-readiness and the close-order questions are the two that bite in the capstone defense.
- 5-6: re-read all three lecture notes and redo the exercises, paying particular attention to the probe distinction and the graceful-shutdown ordering.
- 0-4: rewind to Lecture 1. Lab 11 and the reliability drill assemble every pattern this quiz tests; the operational contract is 35% of the capstone (concurrency/reliability + cloud-native posture).
