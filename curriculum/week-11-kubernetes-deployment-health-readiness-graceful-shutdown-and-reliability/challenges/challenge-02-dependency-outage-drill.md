# Challenge 2 — Kill Postgres Mid-Traffic and Prove Timeouts, Jittered Retries, and the Circuit Breaker Contained the Blast Radius and the Service Recovered

> **Time:** 2 hours. **Prerequisites:** Lecture 3; Exercise 3; a deployed `notes` on `kind` with the reliability patterns wired. **Citations:** the SRE book's "Addressing Cascading Failures" at <https://sre.google/sre-book/addressing-cascading-failures/>, "Handling Overload" at <https://sre.google/sre-book/handling-overload/>, the AWS backoff-and-jitter article at <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>, the Go context blog at <https://go.dev/blog/context>, and `gobreaker` at <https://github.com/sony/gobreaker>.

## The premise

A dependency *will* fail. Postgres will fail over, get slow, or go away. The question this drill answers is not "can you prevent it" (you cannot) but "does your service contain the failure or amplify it." A service with no timeouts wedges when Postgres is slow; a service with naive retries turns a blip into a thundering herd; a service with no circuit breaker keeps sending doomed requests to a dead database and exhausts its own goroutines waiting for them. This drill kills Postgres mid-traffic and makes you prove the blast radius stayed contained and the service recovered cleanly when Postgres came back — the dependency-outage reliability drill the capstone requires.

The thing you are proving: when Postgres dies, `notes` should **fail fast, not wedge**. Read requests should return an error (a 503) in *milliseconds* — the timeout firing, then the breaker opening so subsequent requests do not even wait for the timeout — and the service should stay responsive (its liveness stays green, its goroutine count stays bounded, its readiness honestly reports 503). When Postgres returns, the breaker should half-open, find it healthy, close, and the service should recover *without a restart*.

## Setup

Confirm the reliability patterns from Exercise 3 are wired: a `context` deadline on every `pgx` call, a circuit breaker on the database-backed path (or on a downstream you can kill), and the readiness probe that pings the pool. Add the observability to measure the blast radius:

- A Prometheus counter for breaker state transitions (`notes_breaker_state{state="open|half_open|closed"}`).
- The Week 9 RED dashboard (rate, errors, duration) and the goroutine-count metric (`go_goroutines`).

A load generator running a steady read load, classifying responses by latency *and* code so you can see "fail fast" vs "wedge":

```bash
#!/usr/bin/env bash
# outage-load.sh — steady reads, logging code AND latency.
URL="http://localhost:8080/notes"
end=$(( $(date +%s) + 180 ))
while [ "$(date +%s)" -lt "$end" ]; do
  out=$(curl -s -o /dev/null -w '%{http_code} %{time_total}' --max-time 6 "$URL")
  echo "$(date -u +%H:%M:%S) $out"
  sleep 0.05
done
```

The key column is `time_total`: when Postgres dies, a *contained* service returns its 503s **fast** (sub-second — the timeout, then sub-millisecond once the breaker opens); a *wedged* service returns slowly (every request waiting the full timeout) or stops responding entirely (`000`, connection refused, the pool exhausted).

## The drill

1. `kubectl port-forward -n notes svc/notes 8080:80`; start `outage-load.sh`.
2. **Steady state** (~20s): all 200s, fast. Capture the dashboard (rate up, errors zero, duration low).
3. **Kill Postgres**: `kubectl scale -n notes deploy/postgres --replicas=0`. Note the time.
4. **Observe the containment**:
   - Reads start failing — but **fast**: first a burst of 503s at ~the query timeout (the `context` deadline firing), then, once ≥5 failures cross the breaker threshold, the breaker **opens** and subsequent 503s return in sub-milliseconds (fail-fast, no query attempted).
   - `go_goroutines` stays **bounded** — it does not climb, because requests are not piling up waiting on a dead database (that is the timeout + breaker working; without them, goroutines would climb as each request blocks forever).
   - Liveness stays 200 (the process is healthy); readiness honestly returns 503 (the pod cannot serve); the pods go `0/1` but are **not restarted**.
5. **Recover**: `kubectl scale -n notes deploy/postgres --replicas=1`. Note the time.
6. **Observe the recovery**:
   - After the breaker cooldown, it **half-opens**, lets a trial request through, finds Postgres healthy, and **closes**.
   - Readiness returns to 200, the pods go `1/1` Ready, reads return 200 again — all **without a pod restart**.
7. Stop the load. Tabulate the phases.

```text
  phase            postgres   reads          latency        goroutines   pods       breaker
  steady state     up         200            low            bounded      3/3 Ready  closed
  outage (early)   down       503            ~timeout       bounded      0/3 Ready  closed->open
  outage (settled) down       503            sub-ms         bounded      0/3 Ready  OPEN (fail fast)
  recovery         up         200            low            bounded      3/3 Ready  half-open->closed
```

The story the table tells: the failure was **contained** (fast failures, bounded goroutines, no restart storm, liveness stayed green) and the recovery was **automatic** (the breaker found the dependency healthy and closed; no human, no restart).

## The broken control run

Prove the patterns matter by removing them. Deploy a build of `notes` with **no timeout on the database calls** (use `context.Background()` in the query path) and **no circuit breaker**. Re-run the drill:

- When Postgres dies, reads **hang** (each waits forever for a dead database). `time_total` climbs to the curl `--max-time`. `go_goroutines` **climbs** as requests pile up. The pool exhausts; eventually the service stops responding at all (`000`). Liveness may even start failing if the wedge starves the health endpoint.
- This is the **wedge**: a slow/dead *dependency* made the *service* unhealthy, which is exactly the cascading failure the patterns prevent.

```text
  build                 postgres down -> reads         goroutines   service
  with timeout+breaker  503 FAST (ms)                  bounded      stays responsive
  no timeout, no breaker hang (waits forever) -> 000    CLIMB        wedges, pool exhausted
```

Capture both and contrast them: the difference between "fail fast and stay up" and "hang and fall over" is the timeout and the breaker, and the control run is the proof.

## Acceptance criteria

1. A `OUTAGE-DRILL.md` with the four-phase table (steady / outage-early / outage-settled / recovery), each phase backed by the dashboard capture and the load-log excerpt.
2. Evidence the failures were **fast**: the `time_total` distribution during the outage shows sub-second (and, post-breaker-open, sub-ms) failures, not timeout-length hangs.
3. Evidence the goroutine count stayed **bounded** during the outage (the `go_goroutines` graph is flat, not climbing).
4. Evidence of **no restart**: the pod restart count stays 0 through the whole drill; recovery is via the breaker closing, not a kubelet restart.
5. The broken control run (no timeout, no breaker) captured, showing the wedge — climbing goroutines, hanging requests, eventual `000` — with the one-paragraph contrast.
6. A 200-word section explaining which pattern handled which part of the failure: the timeout (fail fast instead of hang), the breaker (stop sending doomed requests, fail in sub-ms), jittered retries (absorb a *brief* blip without a herd), and honest readiness (tell the cluster the truth so it stops routing).

## Stretch goals

1. **The retry-herd, demonstrated.** Add naive retries (no jitter, no cap) and show, under the outage, the retry-amplified load on Postgres as it tries to recover (the recovery is *slower* because the herd hammers it). Then restore jitter+cap and show the smoother recovery. Cite the AWS article.
2. **Partial outage / slow dependency.** Instead of killing Postgres, make it *slow* (a `pg_sleep` injected via a slow query, or `tc` latency on the pod). Show the timeout fires on the slow path while fast queries still succeed — graceful degradation, not all-or-nothing.
3. **Load-shed under the outage.** Combine with saturation: drive load *while* Postgres is slow and show load-shedding keeps the accepted requests' latency bounded. Cite <https://sre.google/sre-book/handling-overload/>.

## Deliverable

`OUTAGE-DRILL.md` in the `notes` repo: the four-phase table with dashboard and load-log evidence, the fast-failure and bounded-goroutine proof, the no-restart proof, the broken control run, and the 200-word pattern explanation. This *is* the capstone's **reliability-drill postmortem** (the "dependency outage" option) in draft form — write it once, well, and reuse it. The line this challenge defends, in one sentence: *when Postgres died, `notes` failed fast instead of wedging — timeouts turned the hang into a sub-second error, the breaker turned subsequent errors into sub-millisecond fail-fasts, goroutines stayed bounded, liveness stayed green, and when Postgres returned the breaker closed and the service recovered with no restart — the blast radius of a dependency failure stayed at "honest 503s," not "self-inflicted outage."*
