# Challenge 2 — Localize a Regression: Dashboard → Route → Trace → Slow Span

> **Estimated time:** 3 hours. **Prerequisite:** Exercise 3 complete (you can expose RED metrics), Exercise 2 complete (you can read a trace in Jaeger), and a `notes` service with Postgres (Weeks 5–6). **Citations:** the Prometheus histograms practice doc at <https://prometheus.io/docs/practices/histograms/>, the PromQL `histogram_quantile` reference at <https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantile>, and the Grafana dashboards docs at <https://grafana.com/docs/grafana/latest/dashboards/>.

## The premise

This is the drill the whole week builds toward. You will deliberately break `notes` — inject an artificial slow query on one route — and then play the role of the on-call engineer who knows *nothing* except "the service feels slow." Using only the Grafana RED dashboard and the Jaeger trace view, you will localize the regression to a route and a time, then pinpoint it to a single span, then confirm it in the logs. The point is the *method*, not the bug: the bug is obvious because you planted it. In production the bug is not obvious, and the only thing that saves you is having instrumented the service so the two-step — **localize on the dashboard, pinpoint in the trace** — actually works.

You will write up the localization as if it were an incident postmortem: what each tool told you, in what order, and how long each step took.

## What you build / use

- The instrumented `notes` service (from the mini-project, or a stripped version with the RED middleware and OTel tracing on `GET /notes/{id}`).
- An **artificial slow query**: a code path that, for a specific note id (or behind a feature flag / env var), runs `SELECT pg_sleep(0.4), body FROM notes WHERE id = $1` instead of the fast query — adding 400ms inside the DB span. This simulates a missing index, a lock, or a bad query plan.
- The `docker compose` stack up: `notes`, Postgres, Jaeger, Prometheus, Grafana with the provisioned RED dashboard.
- A `LOCALIZATION.md` write-up.

The slow path, in the repository layer:

```go
func (r *Repo) GetNote(ctx context.Context, id string) (Note, error) {
	ctx, span := tracer.Start(ctx, "db.query.GetNote")
	defer span.End()

	const fast = `SELECT id, body FROM notes WHERE id = $1`
	const slow = `SELECT pg_sleep(0.4)::text, id, body FROM notes WHERE id = $1` // injected regression

	query := fast
	if r.injectSlow { // flipped via env var NOTES_INJECT_SLOW=true
		query = slow
		span.AddEvent("slow query path injected")
	}
	span.SetAttributes(attribute.String("db.statement", query))
	// ... scan and return
}
```

## Steps

### 1. Establish a baseline

Bring the stack up. Drive steady traffic at the healthy service:

```bash
docker compose up -d
# steady load for a minute:
while true; do curl -s localhost:8080/notes/$((RANDOM % 100)) >/dev/null; sleep 0.05; done
```

Open Grafana (<http://localhost:3000>), find the RED dashboard, and note the baseline: Rate steady, Errors at zero, Duration p99 for `GET /notes/{id}` flat at ~20–30ms. **Write the baseline numbers down.** You cannot recognize a regression without knowing normal.

### 2. Inject the regression

Restart `notes` with the slow path on (`NOTES_INJECT_SLOW=true`), keep the same load running. Wait ~30 seconds — two scrape intervals.

### 3. Localize on the dashboard

Look at the dashboard, not the code. You should observe, on the Duration panel:

- The p99 line for `GET /notes/{id}` lifts from ~25ms to ~410ms.
- Every *other* route's p99 line stays flat.

That is localization: you now know **which route** (`GET /notes/{id}`) and **what time** (the step in the line) without having read any code. Run the PromQL by hand in Prometheus (<http://localhost:9090>) to confirm:

```promql
histogram_quantile(0.99,
  sum by (le, route) (rate(http_request_duration_seconds_bucket[1m]))
)
```

and compare the per-route Rate and Errors:

```promql
sum by (route) (rate(http_requests_total[1m]))
sum by (route) (rate(http_requests_total{code=~"5.."}[1m]))
  / sum by (route) (rate(http_requests_total[1m]))
```

Note what the dashboard *cannot* tell you: *why* the route is slow. Metrics are aggregates; there is no per-request detail. That is the trace's job.

### 4. Pinpoint in the trace

Switch to Jaeger (<http://localhost:16686>). Filter: service `notes`, operation `GET /notes/{id}`, and sort by duration descending. Open the slowest trace. The waterfall shows:

```
GET /notes/{id}              412ms
  service.GetNote            410ms
    db.query.GetNote         405ms   <- the slow span
        (event: "slow query path injected")
        db.statement = SELECT pg_sleep(0.4)::text, id, body FROM notes WHERE id = $1
```

You have pinpointed: 405 of the 412ms is the `db.query.GetNote` span, and the `db.statement` attribute shows the offending query. The dashboard narrowed the search from "the whole service" to "this route"; the trace narrowed it from "this route" to "this query."

### 5. Confirm in the logs

Copy the trace ID from the Jaeger URL. Search your logs (or `docker compose logs notes | grep <trace_id>`). You find the request-scoped log lines for that exact request, all carrying the `trace_id`, including any "slow query" warning the repository logged. The narrative is now complete and triangulated across all three signals.

### 6. Remove the regression

Restart without the slow path. Confirm the p99 line returns to baseline within two scrape intervals. The recovery is as visible as the regression — which is how you confirm a fix in production.

## Acceptance criteria

- [ ] The stack comes up and the RED dashboard renders Rate, Errors, and Duration panels with live data.
- [ ] You recorded a baseline (healthy p99, rate, error ratio) before injecting.
- [ ] After injection, the dashboard's Duration p99 for `GET /notes/{id}` rises while other routes stay flat — within two scrape intervals.
- [ ] You ran the three RED PromQL queries by hand in Prometheus and they agree with the dashboard.
- [ ] In Jaeger, the slowest `GET /notes/{id}` trace shows the `db.query.GetNote` span consuming the bulk of the time, with the slow `db.statement` attribute visible.
- [ ] You found the matching log lines by the trace ID.
- [ ] Removing the regression returns the p99 to baseline.
- [ ] `LOCALIZATION.md` documents the full path with the actual numbers and the trace ID.

## Reflection (write into LOCALIZATION.md)

1. **Which tool told you *what*, and in what order?** Write one sentence each for the dashboard, the trace, and the log. Be precise about the boundary: the dashboard told you the *route and time*; the trace told you the *span*; the log told you the *application context*. State exactly what each could NOT have told you.

2. **Why could you not have localized this in Jaeger alone?** You had thousands of traces. Without the dashboard pointing you at `GET /notes/{id}`, how would you have known which traces to open? Estimate how long a trace-only investigation would have taken versus the dashboard-first path.

3. **Why could you not have pinpointed this in the metrics alone?** Suppose someone "fixed" the visibility problem by adding a `note_id` label to the duration histogram so you could see which note was slow. What happens to your Prometheus, and why is the trace the correct place for per-request detail? Tie this back to the cardinality rule.

4. **The scrape-interval lag.** The dashboard took up to two scrape intervals (~30s) to show the regression. What is the trade-off between a shorter scrape interval (faster detection) and a longer one (less load, less storage)? What detection latency is acceptable for a user-facing service?

5. **A regression the dashboard would miss.** Construct a failure mode that RED metrics would *not* surface (hint: think about a slow path that only one specific user hits, or an error that returns 200). How would you catch it? (This is the honest limit of RED: it is aggregate, so a regression invisible in aggregate is invisible to it.)

## Stretch goals (optional)

- **Add a Grafana alert.** Configure an alert rule that fires when `GET /notes/{id}` p99 exceeds 100ms for 1 minute. Inject the regression and confirm the alert fires; remove it and confirm it resolves.
- **Add an exemplar.** Wire OpenTelemetry exemplars so the Grafana histogram panel links directly to an example trace at the spiking p99 — one click from dashboard to trace, no manual trace ID copy. (Citation: <https://grafana.com/docs/grafana/latest/fundamentals/exemplars/>.)
- **Localize a different signal.** Inject an *error* regression (return 500 for a fraction of requests) instead of a latency one, and localize via the Errors panel. Confirm the failed spans are red in Jaeger.

## Submission

Place under `challenges/challenge-02/`:

- The instrumented service with the injectable slow path (or a pointer to the mini-project code and the env var that flips it).
- `LOCALIZATION.md` with the baseline numbers, the post-injection numbers, the trace ID, screenshots or text of the dashboard panels and the Jaeger waterfall, and the five reflection answers.

Commit with the message `challenge-02: localized injected slow query from dashboard to span`.
