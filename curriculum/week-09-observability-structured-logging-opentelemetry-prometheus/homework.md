# Week 9 — Homework

Six practice problems. Allocate roughly 1 hour per problem; the last two are longer and may need 90 minutes. Submit one `.zip` of code + a single `homework.md` write-up. Rubric at the bottom.

---

## Problem 1 — A slog JSON handler with `ReplaceAttr` redaction (60 min)

Build a `*slog.Logger` with a JSON handler whose `HandlerOptions.ReplaceAttr` **redacts a sensitive field by key**. Any attribute whose key is `password`, `token`, or `authorization` must be replaced with the string `"REDACTED"` *regardless of where it appears* (top level or inside a group).

**Required of your solution:**

- A `ReplaceAttr func(groups []string, a slog.Attr) slog.Attr` that matches the three keys case-insensitively and returns a redacted attr; passes everything else through unchanged.
- A demonstration that logs a record containing both a safe field (`user_id`) and a redacted one (`token`), at the top level and nested in a `slog.Group`.
- A short note on the difference between this approach (redact by *key*, in the handler) and the `LogValuer` approach (redact by *type*, at the value). When is each the right tool?
- Bonus: use `ReplaceAttr` to also rename slog's built-in `time` key to `ts` and reformat the level — both are common production tweaks done in `ReplaceAttr`.

**Deliverable:** `redact.go` plus a paragraph in `homework.md` defending the key-vs-type choice. Verify the real secret never appears in the output.

---

## Problem 2 — A context-aware logger that auto-adds `trace_id` (60 min)

Build the `ContextHandler` from Lecture 1, Section 8: a `slog.Handler` that wraps another handler and, on every record, reads the active OpenTelemetry span context from the `context.Context` and adds `trace_id` and `span_id` attributes when a valid span is present.

**Required of your solution:**

- A `ContextHandler` that embeds `slog.Handler` and overrides only `Handle`.
- A test (or a demo `main`) that logs once *inside* a span (`tracer.Start`) and once *outside* one, and shows the `trace_id` present in the first and absent in the second.
- Use a real (minimal) OTel SDK setup so the span context is genuinely populated — a no-op `TracerProvider` will produce an invalid span context and the `trace_id` will be all zeros; explain why.
- A note: why must callers use `InfoContext` (not `Info`) for this to work?

**Deliverable:** `context_handler.go` + a demo, and the answer to the `InfoContext` question in `homework.md`.

---

## Problem 3 — Instrument a function with a span, attributes, and `RecordError` (60 min)

Take any function that does a unit of work with a possible failure (a fake "fetch user," a file read, an HTTP call — your choice). Instrument it with OpenTelemetry:

- Start a span with `tracer.Start`, `defer span.End()`.
- Add at least two attributes describing the input.
- On the error path, call `span.RecordError(err)` and `span.SetStatus(codes.Error, ...)`.
- Add one `span.AddEvent` at a meaningful point.

Export to a local Jaeger (`docker run ... jaegertracing/all-in-one:1.57`) and read the span in the UI.

**Required of your solution:**

- A success run and a failure run, both visible in Jaeger.
- A description of what the failed span looks like in the UI (the red status, the error event).

**Deliverable:** `instrument.go` and a `homework.md` note describing the two traces in Jaeger and the trace IDs.

---

## Problem 4 — A RED histogram on an endpoint, with the p95 PromQL (60 min)

Add an `http_request_duration_seconds` `HistogramVec{method,route}` to a small two-route HTTP server (one fast route, one with an injected `time.Sleep`). Register it on a custom registry, expose `/metrics`, and generate enough load that both routes have observations across multiple buckets.

**Required of your solution:**

- Deliberate bucket choice (`DefBuckets`, or explicit buckets you justify in one sentence).
- A `curl /metrics` excerpt showing the bucket distribution differs between the two routes.
- The **p95** PromQL written out, correctly preserving the `le` label through the aggregation:
  `histogram_quantile(0.95, sum by (le, route) (rate(http_request_duration_seconds_bucket[5m])))`.
- A one-sentence explanation of what breaks if you drop `le` from the `by` clause.

**Deliverable:** `red_histogram.go`, the `/metrics` excerpt, and the p95 PromQL with the `le` explanation in `homework.md`.

---

## Problem 5 — Provision a Grafana panel from the metrics (90 min)

Bring up a local stack (`notes` or the Problem 4 server, plus Prometheus and Grafana via `docker compose`). Provision a Grafana dashboard from JSON — not a hand-clicked dashboard — with **one panel** showing p99 latency per route.

**Required of your solution:**

- A `docker-compose.yml` with Prometheus + Grafana, a `prometheus.yml` scraping your app, and Grafana provisioning (datasource + dashboards loader) so the panel exists the moment the stack is up.
- The dashboard JSON with one timeseries panel whose `expr` is the p99 `histogram_quantile` query and whose unit is `s` (seconds).
- A screenshot (or text description) of the panel showing the fast and slow routes at different p99 values.
- A note on why provisioning-from-JSON beats hand-clicking: reproducibility, version control, review.

**Deliverable:** the compose stack, `prometheus.yml`, the Grafana provisioning + dashboard JSON, and the panel evidence in `homework.md`.

---

## Problem 6 — Correlate one request end to end (90 min, stretch)

Take an instrumented request path (the mini-project's, or one you build from Problems 2–4 combined: a handler under `otelhttp`, a service span, a DB-ish span, RED metrics, and trace-correlated logs). For **one single request**, document the trace ID threading through all three signals.

**Required of your solution:**

- Drive one request and capture: (a) the `slog` "request started"/"request completed" log lines, (b) the trace in Jaeger, (c) the `/metrics` counter incrementing for that route.
- Show that the **same `trace_id`** appears in the log lines and in the Jaeger trace URL.
- Write the narrative: "the metric told me the route's rate ticked up; the trace `<id>` showed the request took N ms in the DB span; the log line for `<id>` showed it was fetching note X." Even though nothing is broken, walk the join as if it were.
- A diagram (ASCII is fine) showing the three signals joined by the one trace ID.

**Deliverable:** the captured artifacts (logs, trace ID, metrics excerpt) and the end-to-end narrative + diagram in `homework.md`.

---

## Rubric

For each problem (max 100 points):

| Tier | Points | Description |
|------|--------|-------------|
| Master | 90–100 | Compiles/runs. Every requirement met. The `homework.md` shows reasoning beyond the literal answer — at least one observation the spec did not ask for. |
| Solid | 75–89 | Compiles/runs. Every requirement met. The `homework.md` answers what was asked, no more. |
| Working | 60–74 | Compiles/runs. Most requirements met; one or two missed. |
| Partial | 40–59 | Runs in places but with significant gaps; the spec was not fully read. |
| Submitted | 0–39 | Submission exists; substantial parts are missing or broken. |

Total: **600 points** across the six problems. **480** is the C30-passing threshold for this week's homework. The mini-project is graded separately.

## Submission

Zip the six problem folders together as `week-09-homework-<your-name>.zip`. Include a top-level `homework.md` that links to each problem's notes and lists your self-assigned score in each tier.

Submit by Sunday 11:59 PM local time. Late submissions are accepted with a one-tier markdown per 24h past the deadline.
