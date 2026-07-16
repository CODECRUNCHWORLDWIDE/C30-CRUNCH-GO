# Exercise 02 — Drive Load, Trace a Finding, Fix It, Re-Measure

> **Time:** ~90 minutes. **Prerequisites:** Lecture 1; the deployed capstone with Week 9 observability; `hey` or `k6`. The OTel docs at <https://opentelemetry.io/docs/> and the pprof blog at <https://go.dev/blog/pprof>.

## Goal

Produce the load-test-and-trace report: drive the deployed service under load, find one latency finding on the Grafana RED dashboard, localise it to a Jaeger span, fix it, and re-measure — proving the observability is usable, not decorative.

## Steps

1. **Drive load** against the deployed service (via `kubectl port-forward` or the public URL):
   ```bash
   hey -z 60s -c 50 http://localhost:8080/<a-read-endpoint>
   hey -z 60s -c 30 -m POST -D body.json http://localhost:8080/<a-write-endpoint>
   ```

2. **Read the RED dashboard.** Capture the before state: rate, errors, and the duration percentiles (p50/p95/p99). Identify a finding — a p99 spike, a percentile that climbs with concurrency, or an endpoint slower than its siblings.

3. **Localise to a span.** Open a slow trace in Jaeger. Read the span tree (handler → service → pgx). Identify *where* the time went — a slow query, an N+1, a serial step, lock contention.

4. **Fix it.** The fix depends on the finding — a missing index (a migration), an N+1 collapsed into one query, a serial loop parallelised, a lock narrowed. Make the change.

5. **(If the finding is in-process)** capture a `pprof` profile to confirm the hot function:
   ```bash
   go tool pprof http://localhost:2113/debug/pprof/profile?seconds=30
   ```

6. **Re-measure.** Re-run the same load; capture the after dashboard and trace. Tabulate before/after for the finding.

7. **Write `LOAD-AND-TRACE-REPORT.md`**: the load profile, the before dashboard, the slow trace, the localised finding (the span where the time went), the fix, and the after dashboard/trace.

## Acceptance criteria

- `LOAD-AND-TRACE-REPORT.md` documents one finding fully: where it showed on the dashboard, the span where the time went, the fix, and the measured improvement.
- The before/after is quantitative (a p99 number, a query count) — not "it felt faster."
- The trace screenshot shows the actual span tree with the slow span identified.
- The fix is a real code/schema change committed to the repo, not a config tweak.
- The report reads as "I found this with the tools," not "I knew where it was."

## Stretch

Find a *second* finding of a different kind (if the first was a query, find a CPU/alloc one with `pprof`; if the first was CPU, find an N+1). Two findings of different classes prove you can use all three observability signals — metrics to spot it, traces to localise a DB finding, pprof to localise an in-process one.
