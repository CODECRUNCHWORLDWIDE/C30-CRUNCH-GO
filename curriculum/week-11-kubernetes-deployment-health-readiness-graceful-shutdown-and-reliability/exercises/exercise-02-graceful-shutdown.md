# Exercise 02 — Graceful Shutdown on `SIGTERM`

> **Time:** ~90 minutes. **Prerequisites:** Lecture 2; Exercise 1; the Week 4 `context`/`errgroup` material.

## Goal

Implement graceful shutdown in `notes` so that on `SIGTERM` it stops accepting new connections, drains in-flight HTTP and gRPC requests, flushes the trace exporter, and closes the pgx pool last — all within a budget under the termination grace period. Prove an in-flight request finishes during a pod termination.

## Steps

1. **Rewire `cmd/notesd/main.go`** into a `run` function using:
   - `signal.NotifyContext(ctx, syscall.SIGTERM, syscall.SIGINT)` for the root context.
   - An `errgroup` running the HTTP server, the gRPC server, and a shutdown watcher.
   - Treating `http.ErrServerClosed` and `grpc.ErrServerStopped` as clean exits, not errors.

2. **Write the shutdown watcher** that, on root-context cancellation:
   - Flips readiness to "not ready" (so the Service stops routing) and sleeps `PreStopDelay` (~5s) to let the endpoint removal propagate.
   - Creates a `context.WithTimeout(context.Background(), cfg.ShutdownTimeout)` budget.
   - Calls `httpSrv.Shutdown(ctx)` (drain HTTP), then `grpcSrv.GracefulStop()` wrapped in a `select` with the budget (force `Stop()` if it blows), then flushes the trace exporter.
   - Lets the deferred `pool.Close()` run *after* the servers stop.

3. **Add a `health.SetNotReady()`/`SetReady()`** toggle so the shutdown watcher can fail readiness on `SIGTERM`.

4. **Size the budget**: `terminationGracePeriodSeconds: 30`, `NOTES_PRESTOP_DELAY: 5s`, `NOTES_SHUTDOWN_TIMEOUT: 20s` — confirm `5 + 20 < 30`.

5. **Add a deliberately-slow test endpoint** (a handler that sleeps `?slow=N` seconds) so you can hold a request in flight during termination. Gate it behind `NOTES_ENV != prod`.

6. **Prove the drain** on `kind`:
   ```bash
   kubectl port-forward -n notes svc/notes 8080:80 &
   curl -s "http://localhost:8080/notes?slow=8" &         # in-flight, 8s
   kubectl delete pod -n notes <the-serving-pod>           # terminate it
   # The curl COMPLETES with 200; the pod logs draining -> drain complete.
   kubectl logs -n notes <that-pod> --previous | grep drain
   ```

## Acceptance criteria

- `go test -race ./...` is green; the shutdown path has no data race.
- A clean `SIGTERM` (e.g. `kubectl delete pod`) drains in-flight requests: a request started before termination completes with 200, not a connection error.
- The pod logs `"shutdown signal received, draining"` then `"drain complete"` and exits 0 — it is not `SIGKILL`ed (no `137` exit, no "OOMKilled"/"Error" in `kubectl describe`).
- The drain completes well within the grace period (the `previous` pod's logs show it finished before 30s).
- Closing the pool happens *after* the servers stop (no "conn closed" error in an in-flight handler during drain).

## Stretch

Run a small `hey` load (`hey -z 30s -c 5 http://localhost:8080/notes`) while you `kubectl rollout restart deploy/notes`, and show zero non-2xx — a preview of challenge-01. Capture the before/after pod set and the load summary.
