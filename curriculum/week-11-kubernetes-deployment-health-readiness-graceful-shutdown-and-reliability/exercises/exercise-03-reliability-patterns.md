# Exercise 03 — Timeouts, Retries with Jitter, a Circuit Breaker, and Load-Shedding

> **Time:** ~90 minutes. **Prerequisites:** Lecture 3; the Week 4 `context`/semaphore material.

## Goal

Add the four reliability patterns to `notes`: a `context` deadline on every outbound call and server timeouts; a retry-with-backoff-and-jitter client; a circuit breaker on a downstream call; and a load-shedding middleware on the inbound path.

## Steps

1. **Timeouts.**
   - Set `ReadTimeout`, `WriteTimeout`, `IdleTimeout` on the `http.Server`.
   - Add a per-request `context.WithTimeout` in handlers (or a timeout middleware).
   - Confirm every `pgx` call passes the request `context` so the deadline propagates to the query.

2. **Retry client** (`internal/retry/retry.go`): implement `Do(ctx, maxAttempts, base, maxDelay, shouldRetry, fn)` with exponential backoff, **full jitter** (`rand.Int63n([0, backoff))`), a `select` on `ctx.Done()` between attempts, and an `IsTransient` predicate that does *not* retry `context.DeadlineExceeded`/`Canceled` or a 4xx.
   - Write a table-driven test: success on first try, retry-then-succeed, exhaust attempts, permanent error not retried, context-cancel beats the retry loop.

3. **Circuit breaker** (`internal/downstream`): wrap a downstream HTTP call in `sony/gobreaker` with a `ReadyToTrip` of ">50% of ≥5 requests failed," a cooldown `Timeout`, and `MaxRequests: 1` for half-open. Make the retry predicate treat `gobreaker.ErrOpenState` as **not retryable**.

4. **Load-shedding middleware** (`internal/middleware/shed.go`): a semaphore bounding inbound concurrency to `limit`, returning `503` + `Retry-After` immediately when full (the `default` case of a `select` on the semaphore). Wire it into the `chi` middleware chain. Make `limit` configurable via `NOTES_MAX_INFLIGHT`.

5. **Reflect shedding in readiness** (stretch-adjacent): under sustained shedding, flip readiness to 503 so the Service routes to a less-loaded pod.

## Acceptance criteria

- `go test -race ./internal/retry/...` is green; all five retry subtests pass.
- A unit test proves the breaker opens after the failure threshold, fails fast (`ErrOpenState`) during cooldown, and half-opens after the timeout.
- A `hey -z 20s -c 200 http://localhost:8080/notes` against a service with `NOTES_MAX_INFLIGHT=50` shows: the accepted requests stay fast (low p99), the excess returns 503 fast (not slow), and the service does not crash or wedge.
- `grep` confirms every `pgx` query call site passes a `context` with a deadline (no `context.Background()` in a query path).
- Retries never outlive the request: a request with a 3s deadline does not produce a 30s retry marathon (the `select` on `ctx.Done()`).

## Stretch

Compose all four on one downstream-backed endpoint and write a 200-word note on the order (shed → deadline → breaker → retry → call) and which failure each layer handles. Add a Prometheus counter for shed requests and breaker state transitions so the Week 9 dashboard shows them.
