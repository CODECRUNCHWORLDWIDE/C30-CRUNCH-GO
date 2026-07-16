# Challenge 2 — Prove Graceful Shutdown Drains In-Flight Requests Under Load

> **Time:** 90 minutes. **Prerequisites:** Exercise 3 (the shutdown wiring). **Deliverable:** a service with graceful shutdown, a load generator that drives it, a measured demonstration that a `SIGTERM` mid-load drops zero in-flight requests within the grace period, and a one-page write-up.

## Statement of the problem

"Graceful shutdown" is easy to *claim* and easy to get subtly wrong. The honest test is: drive real traffic at the service, send it `SIGTERM` while requests are in flight, and prove that *every request that had already started gets a response* — none is dropped on the floor — and that *no new request is accepted* once the drain begins. You will build the demonstration and measure it. This is a dry run of the Week 11 reliability drill, on a single process instead of a Kubernetes rollout.

## What you will build

A small service with a deliberately-slow handler, a load generator, and a harness that orchestrates the SIGTERM.

```
src/drain/
  server.go     // the service: a handler that sleeps 200ms, full shutdown wiring
  load.go       // a load generator: N concurrent clients firing requests in a loop
  drain_test.go // the orchestrated test: start, load, SIGTERM, assert zero drops
  DRAIN.md      // the write-up
```

The slow handler — long enough that requests are reliably in flight when the signal lands:

```go
func slowHandler(w http.ResponseWriter, r *http.Request) {
	select {
	case <-time.After(200 * time.Millisecond):
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("done"))
	case <-r.Context().Done():
		// The request's context is cancelled when the server shuts down past
		// its grace period. Report it so we can count it.
		w.WriteHeader(http.StatusServiceUnavailable)
	}
}
```

The shutdown wiring is Exercise 3's `runServer`, with a grace period of, say, 5 seconds — comfortably longer than the 200ms handler.

## The measurement plan

### M1 — baseline: no shutdown

Drive 50 concurrent clients firing requests in a loop for 3 seconds with the server running normally. Record: total requests, total 200s, total errors. There should be zero errors. This establishes the load generator works.

### M2 — SIGTERM mid-load, within grace

Start the server, start the same 50-client load, and after ~1 second send the process `SIGTERM` (from the test, `srv.Shutdown` directly stands in for the signal path; or use `syscall.Kill(os.Getpid(), syscall.SIGTERM)` to exercise the real signal). Record:

- **Requests that were in flight at the moment of SIGTERM** and whether each got a 200 (drained) or was dropped.
- **Requests attempted *after* SIGTERM** — these should fail to *connect* (the listener is closed), which is correct: a draining server refuses new work.
- **The wall-clock drain time** — from SIGTERM to process exit. It should be ~200ms (one slow handler's worth), well under the 5s grace.

The assertion: **zero in-flight requests dropped.** Every request that received a connection before the drain began got a response.

### M3 — SIGTERM with the grace period too short

Set the grace period to 50ms (shorter than the 200ms handler). Repeat M2. Now some in-flight requests *do not finish* in time; `Shutdown` returns a deadline error and the fallback `srv.Close()` force-closes them. Record how many were dropped and confirm `Shutdown` returned `context.DeadlineExceeded`. The lesson: **the grace period must exceed your slowest legitimate request.**

### M4 — new connections refused during drain

While the drain is in progress (within the 5s window of M2), attempt a brand-new connection. Confirm it is refused (connection error), not served. A draining server accepts no new work — that is what readiness probes will key off in Week 11.

## Acceptance criteria

1. M2 demonstrates **zero dropped in-flight requests** with an adequate grace period; `DRAIN.md` reports the count (drained vs dropped) and the drain time.
2. M3 demonstrates that an *inadequate* grace period drops requests and that `Shutdown` returns `context.DeadlineExceeded`; the write-up states the rule "grace ≥ slowest request."
3. M4 confirms new connections are refused once the drain begins.
4. `runServer` treats `http.ErrServerClosed` as success (clean exit 0 on SIGTERM).
5. `go test -race ./...` is green — the load generator and the counters must be race-free (use `atomic.Int64` for the tallies).

## A trap to watch for

Counting "in flight at SIGTERM" requires care: a request is in flight if its handler has *started* but not *returned*. Instrument with two `atomic.Int64`s — `started` incremented at the top of the handler, `finished` at the bottom — and snapshot `started - finished` at the moment you send the signal. Do not count requests that had not yet been accepted; those are the M4 "refused" set, a different category.

## A second trap: the load generator must respect the closed listener

After SIGTERM, the listener is closed and new `http.Client` requests get a `connection refused`. Your load generator must *expect* those and count them as "refused after drain," not as "dropped" — conflating the two will make a correct shutdown look broken. A dropped request is one that *connected and started* but got no response; a refused request never connected.

## Submission

Submit the `drain` package (runnable with `go test -race ./...`) and `DRAIN.md` with the four measurements, the drain-time numbers, and the "grace ≥ slowest request" conclusion. A comment block in `server.go` links to `Server.Shutdown` and the `ErrServerClosed` docs.

The rubric:

- (35%) M2 correctly demonstrates zero dropped in-flight requests with the counting done right (started/finished atomics).
- (25%) M3 demonstrates the too-short-grace failure and identifies `context.DeadlineExceeded`.
- (20%) M4 confirms new connections are refused during drain; the dropped-vs-refused distinction is correct.
- (10%) `http.ErrServerClosed` handled as success; clean exit 0.
- (10%) `go test -race` green; citations present.

Cited references: <https://pkg.go.dev/net/http#Server.Shutdown>, <https://pkg.go.dev/net/http#pkg-variables>, <https://pkg.go.dev/os/signal#NotifyContext>, <https://pkg.go.dev/sync/atomic>.
