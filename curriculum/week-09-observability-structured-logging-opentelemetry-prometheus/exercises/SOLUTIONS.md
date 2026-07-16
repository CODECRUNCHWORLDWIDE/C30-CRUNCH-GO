# Exercise Solutions — Week 9

These annotated solutions assume you have made a serious attempt at each exercise. Read your own attempt against the explanations below; do not copy without trying first. The three exercise files (`exercise-01-slog-request-logger.go`, `exercise-02-otel-traced-handler.go`, `exercise-03-prometheus-red-metrics.go`) are the reference solutions — these notes call out the correctness properties, the output you should reproduce, and the mistakes that cost the most.

---

## Exercise 1 — slog request logger

### Key correctness properties

- **The context key is an unexported struct type** (`loggerCtxKey{}`, `requestIDCtxKey{}`), never a string. A string key can collide with another package's context value; an unexported type cannot, because no other package can name it. This is the standard `context.Value` discipline.
- **The logger is derived once per request with `slog.With`** and stashed in the context. Deeper layers retrieve it with `LoggerFrom(ctx)` and inherit `request_id`, `method`, and `path` without those values appearing in any function signature below the middleware.
- **The middleware uses the `*Context` methods** (`InfoContext`, `ErrorContext`), not the plain ones. The plain methods pass `context.Background()` to the handler, which would defeat any context-aware handler (the `ContextHandler` from the lecture, or the trace-id read).
- **`statusRecorder` defaults to 200.** `net/http` treats a body write with no explicit `WriteHeader` as a 200, so the recorder must too, or every happy-path request logs the wrong status.
- **The route comes from `chi.RouteContext().RoutePattern()`**, the template `/notes/{id}`, not `r.URL.Path`. This keeps the field bounded and makes it usable as a metric label in Exercise 3.

### Expected output

Two requests, each with its own `request_id` threading through the started/completed pair:

```json
{"time":"2026-06-19T14:32:07.001Z","level":"INFO","msg":"request started","request_id":"a1b2c3d4e5f60718","method":"GET","path":"/notes/4f2"}
{"time":"2026-06-19T14:32:07.001Z","level":"INFO","msg":"fetching note","request_id":"a1b2c3d4e5f60718","method":"GET","path":"/notes/4f2","note_id":"4f2"}
{"time":"2026-06-19T14:32:07.002Z","level":"INFO","msg":"request completed","request_id":"a1b2c3d4e5f60718","method":"GET","path":"/notes/4f2","status":200,"route":"/notes/{id}","duration":612000}
{"time":"2026-06-19T14:32:09.114Z","level":"INFO","msg":"request started","request_id":"99fe10ab77c30421","method":"GET","path":"/boom"}
{"time":"2026-06-19T14:32:09.114Z","level":"ERROR","msg":"deliberate failure","request_id":"99fe10ab77c30421","method":"GET","path":"/boom","reason":"demo"}
{"time":"2026-06-19T14:32:09.114Z","level":"INFO","msg":"request completed","request_id":"99fe10ab77c30421","method":"GET","path":"/boom","status":500,"route":"/boom","duration":98000}
```

Note that `duration` is rendered as nanoseconds by the JSON handler (`slog.Duration` serializes as `time.Duration`, an int64 of nanoseconds). The text handler would render it as `612µs`. If you want milliseconds in JSON, use `slog.Float64("duration_ms", float64(d)/1e6)` or a `ReplaceAttr` that reformats the duration — a reasonable extension.

### Reflection answers

1. **Why bind the logger once per request rather than passing `request_id` to every function?** Because the alternative — adding `requestID string` to every signature from handler to repository — pollutes every function with a cross-cutting concern and breaks the moment you add a second field (now every signature needs `traceID` too). The request-scoped logger in the context binds all per-request fields once; any layer that has the `ctx` gets them all, and adding a field is a one-line change in the middleware, not a signature change across the codebase.

2. **Why `InfoContext` and not `Info`?** The `*Context` variants pass the context to the handler's `Handle(ctx, record)`. The custom `ContextHandler` (lecture, Section 8) reads the trace ID out of that context. A plain `Info` hands the handler `context.Background()`, the trace lookup finds no span, and the `trace_id` silently never appears — a bug with no error message.

3. **What did the `trace.SpanContextFromContext` line do when you ran the exercise standalone (no tracing)?** Nothing visible: `sc.IsValid()` returned false because no span was in the context, so no `trace_id` was bound. When you compose this middleware under Exercise 2's `otelhttp` wrapper, the span context *is* present, and the same line lights up every log with the trace ID. The middleware is written to work in both worlds.

---

## Exercise 2 — OTel traced handler

### Key correctness properties

- **`tracer.Start` returns a new context, and the code uses it.** `getNote` calls `queryNote(ctx, id)` with the context from its own `Start`, so the DB span is a child of the service span, which is a child of the `otelhttp` server span. Pass the *original* request context instead and the DB span attaches to the wrong parent — the most common span bug.
- **`otelhttp.NewHandler` is the outermost wrapper.** It extracts the inbound `traceparent` and starts the server span before any handler runs. If you put it *inside* another wrapper that needs the span context, that wrapper sees no span.
- **The propagator is set.** `otel.SetTextMapPropagator(propagation.TraceContext{})` — without it, `otelhttp` has nothing to extract, and the server starts a fresh root trace per request instead of continuing a caller's. Standalone (curl with no `traceparent`) you would not notice; across a service boundary the trace would break.
- **`Shutdown` is deferred with a bounded context, and `main` returns on a signal.** The `BatchSpanProcessor` buffers spans; `Shutdown` flushes them. The bounded context stops a dead Jaeger from hanging exit. `os.Exit` would skip the defer and drop the buffer — so the code blocks on `os.Interrupt` and returns.
- **The error path sets `RecordError` + `SetStatus(codes.Error, ...)`.** This turns the span red in Jaeger and attaches the error as a span event.

### Expected output

Logs (one request, trace-correlated):

```json
{"time":"...","level":"INFO","msg":"handling get note","note_id":"3","trace_id":"4bf92f3577b34da6a3ce929d0e0e4736"}
{"time":"...","level":"INFO","msg":"query complete","note_id":"3","db_duration":18000000}
```

In Jaeger (<http://localhost:16686>, service `notes-ex02`), one trace contains three spans:

```
GET /notes/{id}                400.0ms   (otelhttp server span)
  service.GetNote              398.2ms   note.id=3  note.body_len=13
    db.query.GetNote           395.7ms   db.system=postgresql  db.statement=SELECT ...
                                          (event: "slow query path taken")
```

The `trace_id` printed in the "handling get note" log (`4bf92f...`) is identical to the trace ID in the Jaeger URL for that trace. That match *is* the log/trace join key working. A request to `/notes/0` produces a trace whose `db.query.GetNote` span is red, with the "note not found" error recorded on it.

### Reflection answers

1. **Why does the DB span attach correctly only if you pass the right context?** Because the parent/child relationship is carried *in the context*, not inferred from the call stack. `tracer.Start(ctx, ...)` reads the current span out of `ctx` to set the new span's parent. If `queryNote` is called with the handler's original `ctx` instead of `getNote`'s post-`Start` `ctx`, the DB span's parent is the *server* span, not the service span — the waterfall flattens and the layering is lost.

2. **What breaks if you use `WithSyncer` instead of `WithBatcher`?** Functionally nothing in this toy — but each span End would block on a synchronous OTLP export, so a request that creates three spans makes three blocking network calls inline on the request path. Under load that serializes your throughput on the exporter. `WithBatcher` enqueues cheaply and exports in the background. Syncer is for tests; batcher is for everything else.

3. **What does an all-zeros `trace_id` in the log mean?** That the span context did not propagate to that code point — almost always because `otelhttp` is missing, is not the outermost wrapper, or the propagator is not set. The all-zeros trace ID is the canonical symptom of a broken propagation chain; learn to recognize it.

---

## Exercise 3 — Prometheus RED metrics

### Key correctness properties

- **Two instruments, three signals.** `http_requests_total{method,route,code}` (Rate via `rate()`, Errors via the 5xx ratio) and `http_request_duration_seconds{method,route}` (Duration via `histogram_quantile`). You do not need a third instrument for RED.
- **The histogram is NOT labeled by `code`.** Duration cares about how long, not the status. Adding `code` would multiply the bucket series by the number of status codes for no analytical gain.
- **The route label is a fixed template.** `/notes/{id}`, never `/notes/4f2`. Labeling by the concrete id is a cardinality explosion: one new time series per note, growing without bound. This is the rule the lecture hammered and the rubric checks.
- **A custom registry, not the global default.** `prometheus.NewRegistry()` + `MustRegister` keeps the exposed surface explicit and lets tests run with a fresh registry — no global state leaking between tests.
- **`statusRecorder` defaults to 200**, same reason as Exercise 1, so the counter labels the happy path correctly.

### Expected `/metrics` scrape (excerpt, after 50 `/fast`, 10 `/slow`, 1 `/notes/4f2`)

```
# HELP http_requests_total Total HTTP requests by method, route, and status code.
# TYPE http_requests_total counter
http_requests_total{code="200",method="GET",route="/fast"} 50
http_requests_total{code="200",method="GET",route="/slow"} 10
http_requests_total{code="200",method="GET",route="/notes/{id}"} 1
# HELP http_request_duration_seconds HTTP request latency in seconds by method and route.
# TYPE http_request_duration_seconds histogram
http_request_duration_seconds_bucket{method="GET",route="/fast",le="0.005"} 50
http_request_duration_seconds_bucket{method="GET",route="/fast",le="+Inf"} 50
http_request_duration_seconds_sum{method="GET",route="/fast"} 0.0021
http_request_duration_seconds_count{method="GET",route="/fast"} 50
http_request_duration_seconds_bucket{method="GET",route="/slow",le="0.1"} 0
http_request_duration_seconds_bucket{method="GET",route="/slow",le="0.25"} 0
http_request_duration_seconds_bucket{method="GET",route="/slow",le="0.5"} 10
http_request_duration_seconds_bucket{method="GET",route="/slow",le="+Inf"} 10
http_request_duration_seconds_sum{method="GET",route="/slow"} 2.51
http_request_duration_seconds_count{method="GET",route="/slow"} 10
```

The shape to verify: every `/fast` observation lands in the `le="0.005"` bucket (sub-5ms), and every `/slow` observation (250ms sleep) lands at `le="0.5"` but not `le="0.25"`. The histogram has located the slow route in its bucket distribution, exactly what `histogram_quantile` will read later.

### Reflection answers

1. **Why is the average latency a bad Duration signal?** Because it hides the tail. If 99 of 100 requests are 5ms and one is 2s, the average is ~25ms — which looks healthy and tells you nothing about the request that is timing out a user. The p99 from the histogram (`histogram_quantile(0.99, ...)`) surfaces that 2s request; the average buries it. The tail is what pages you, so you measure the tail.

2. **What is the cardinality of `http_requests_total` as written, and what would labeling by note id make it?** As written: (methods) × (routes) × (codes) ≈ 2 × 3 × 5 = ~30 time series, bounded forever. Labeled by note id instead of the route template: one new series per distinct note id ever requested — unbounded, growing with your data, eventually exhausting Prometheus memory. That detail belongs on a span or a log line, never a metric label.

3. **Why a histogram and not a summary here?** Because `notes` will run multiple replicas, and summary quantiles cannot be aggregated across instances — you cannot combine ten replicas' p99 summaries into a fleet p99. Histogram buckets *can* be summed across replicas and the quantile computed after (`sum by (le, route) (...)` then `histogram_quantile`). Histogram is the multi-replica default; summary is a single-instance niche.

---

## Common mistakes across the three exercises

- **Logging in a hot loop without a level guard.** A `Debug` line inside a per-row loop that runs a million times still pays the cost of building the record's attributes even when `Debug` is off — unless you use `LogValuer` (lazy) or check `logger.Enabled(ctx, slog.LevelDebug)` first. In a genuinely hot path, guard or use lazy values; do not put an unguarded `Debug` inside a million-iteration loop.
- **High-cardinality metric labels.** Labeling a metric by note id, user id, request id, or the raw URL path. Every distinct value is a new time series; unbounded values mean unbounded series and an out-of-memory Prometheus. Label by bounded dimensions (method, route template, status code class) only.
- **Forgetting `TracerProvider.Shutdown`.** The `BatchSpanProcessor` buffers spans; if `main` exits without `Shutdown`, the buffer — the last few seconds of traces, often the most interesting ones around a crash — is silently dropped. Defer `Shutdown` with a bounded context and return from `main` on a signal rather than `os.Exit`.
- **Not propagating context, so the trace breaks at the boundary.** Either failing to set `otel.SetTextMapPropagator`, or putting `otelhttp` in the wrong place, or — most subtly — calling a downstream function with a fresh `context.Background()` instead of the request context. The symptom is disconnected one-span traces and an all-zeros `trace_id` in the logs.
- **Using a `Summary` when you need cross-instance aggregation.** Summaries compute quantiles in-process and cannot be combined across replicas. Reach for a histogram for any service that scales horizontally — which is essentially all of them.
- **`trace_id` not correlated into logs.** Logging through the default logger (or the plain `Info` instead of `InfoContext`) inside a span, so the log line has no `trace_id`. The whole point of the week is the join key; a log line without the `trace_id` cannot be joined to its trace, and the 3am workflow collapses back to correlating by timestamp and hoping.

Next: the challenges. Challenge 1 propagates one trace across a gRPC boundary into a second service; Challenge 2 injects a slow query and makes you localize it dashboard → route → trace → span.
