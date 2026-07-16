# Week 9 — Quiz

Ten multiple-choice questions. Take it with your lecture notes closed. Aim for 9/10 before moving to Week 10. Answer key at the bottom — do not peek.

---

**Q1.** In `log/slog`, what is the division of responsibility between a `*slog.Logger` and a `slog.Handler`?

- A) The `Logger` formats the output and writes it; the `Handler` is just a configuration struct.
- B) The `Logger` is the front end your code calls (`Info`, `With`); the `Handler` is the back end that decides the output format, destination, level, and any attribute transformation. The same call site works with any handler.
- C) They are interchangeable — `Logger` and `Handler` are two names for the same interface.
- D) The `Handler` calls the `Logger`; the `Logger` is the lowest level that touches the file descriptor.

---

**Q2.** You wrap the same attributes with `slog.NewTextHandler` and `slog.NewJSONHandler`. What is the difference in output, and what is the doctrine for choosing?

- A) JSON is faster to write; use it everywhere.
- B) Text emits `key=value` pairs for a human at a terminal; JSON emits one JSON object per line for a log aggregator to parse. Doctrine: text in development, JSON in production.
- C) Text drops attributes that JSON keeps; never use text.
- D) They are identical except JSON adds a trailing comma.

---

**Q3.** Why is structured logging (`logger.Info("note created", "duration_ms", 38)`) preferred over `fmt`-style logging (`log.Printf("note created in %dms", 38)`)?

- A) Structured logging is shorter to type.
- B) Because the values are emitted as named, typed fields, so the log backend can query them (`duration_ms > 100`) without a regex, and a format change cannot silently break the query. `fmt`-style logs are strings a machine can only grep.
- C) `fmt.Printf` cannot log integers.
- D) Structured logging is required by Go 1.22; `log.Printf` was removed.

---

**Q4.** The W3C `traceparent` header looks like `00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01`. What do those four hyphen-separated fields carry?

- A) Timestamp, hostname, port, checksum.
- B) Version, the 16-byte trace ID, the parent span ID, and trace flags (e.g. the sampled bit).
- C) Service name, method, route, status code.
- D) A nonce, the request body length, the user ID, and a signature.

---

**Q5.** Two services are correctly instrumented with `otelhttp`/`otelgrpc`, but a trace still breaks at the boundary — the second service starts a fresh root trace. What is the most likely cause?

- A) The exporter is using OTLP instead of the Jaeger-native protocol.
- B) `otel.SetTextMapPropagator` was never called (no propagator is set), so the instrumentation libraries have nothing to inject/extract — or the downstream call was made with `context.Background()` instead of the request context, so there was no span context to propagate.
- C) The two services use different `service.name` values.
- D) The `BatchSpanProcessor` batch timeout is too short.

---

**Q6.** What is the difference in role between `span.End()` and `TracerProvider.Shutdown()`?

- A) They are the same call under two names.
- B) `span.End()` marks one span complete (fixing its duration) and hands it to the processor; `TracerProvider.Shutdown()` flushes the `BatchSpanProcessor`'s buffer of completed-but-not-yet-exported spans before the process exits. Skipping `End` leaves a span open forever; skipping `Shutdown` drops the buffered spans.
- C) `Shutdown` ends all open spans automatically, so `span.End()` is optional.
- D) `span.End()` exports the span synchronously; `Shutdown` does nothing in production.

---

**Q7.** You need to record the latency of an HTTP endpoint so you can later compute p99 across ten replicas. Which Prometheus metric type, and why?

- A) A `Gauge`, because latency is a current value.
- B) A `Summary`, because it computes the p99 in-process and is exact.
- C) A `Histogram`, because its buckets can be summed across replicas and the quantile computed after — summary quantiles cannot be aggregated across instances.
- D) A `Counter`, because you read its rate.

---

**Q8.** Why is it dangerous to add a label like `note_id` to a Prometheus metric?

- A) Prometheus does not allow more than three labels per metric.
- B) Because every distinct label-value combination is a separate time series; an unbounded value like a note id creates unbounded time series — a cardinality explosion that exhausts memory in your process and in Prometheus. Per-request detail belongs on a span or a log line, not a metric label.
- C) `note_id` is a reserved label name.
- D) Labels with underscores are silently dropped.

---

**Q9.** What does `histogram_quantile(0.99, sum by (le, route) (rate(http_request_duration_seconds_bucket[5m])))` compute, and why must `le` appear in the `by` clause?

- A) The average latency over 5 minutes; `le` is the legend label.
- B) The estimated 99th-percentile (p99) latency per route, interpolated from the cumulative histogram buckets. `le` (the bucket upper-bound label) must survive the aggregation because `histogram_quantile` reads the per-bucket counts from it — drop `le` and the function has no buckets to work with.
- C) The total number of requests in the slowest 1%.
- D) The error ratio for the 99th request.

---

**Q10.** How does a single trace ID let you correlate a log line with a span in two different backends (a log store and a trace store)?

- A) It does not; correlation is always by timestamp.
- B) The trace ID is a shared join key: OpenTelemetry assigns it to the request, every span in the trace carries it, and if you also attach it as a `trace_id` attribute to every log line (via a context-aware handler reading `trace.SpanContextFromContext`), the log store and the trace store share that identifier — you find a slow trace in Jaeger, copy its trace ID, and search the log store for the same value.
- C) Both backends must be the same product for correlation to work.
- D) The span ID, not the trace ID, is the join key; the trace ID is internal.

---

## Answer key (no peeking until you have answered all ten)

1. **B.** The `Logger`/`Handler` split is the structural heart of slog. The logger is a cheap front end that builds a `Record` and delegates; the handler is the interface that decides format, destination, level, and transformation. Because the split is clean, the same `logger.Info(...)` call works whether the handler is text or JSON — the format is a deployment decision, not a code decision.

2. **B.** Text is `key=value` for human eyes at a terminal; JSON is newline-delimited objects for a log aggregator to parse. The doctrine — text in dev, JSON in prod — is a one-branch decision at startup, and every call site downstream is identical.

3. **B.** The decisive reason is that machines consume production logs. Named typed fields are queryable (`duration_ms > 100`) by the backend and immune to format reordering; a formatted string is only greppable and breaks the day someone changes the format. The brevity in A is incidental; B is the real reason.

4. **B.** `version-traceid-parentid-flags`. The 16-byte trace ID is what ties all spans of a request together; the parent span ID is how the next service sets its root span's parent; the flags carry the sampled bit. Citation: <https://www.w3.org/TR/trace-context/>.

5. **B.** Two failure modes, both common: no propagator set (so `otelhttp`/`otelgrpc` have nothing to inject or extract), or the downstream call was made with a fresh context that carried no span. The stats handler can only propagate what is in the context it is handed. The symptom is a fresh root trace and an all-zeros parent on the second service.

6. **B.** `End` completes one span and enqueues it; `Shutdown` flushes the batch processor's buffer of completed spans before exit. They operate at different scopes — one span versus the whole provider — and skipping either loses telemetry in a different way (an open span never exports; a buffered span is dropped at exit).

7. **C.** Histogram. The defining reason for a multi-replica service is aggregability: histogram buckets sum across instances and you compute the quantile after; summary quantiles are computed in-process and cannot be combined (averaging quantiles is meaningless). When in doubt, histogram.

8. **B.** Cardinality. Each label-value combination is a time series; an unbounded label value means unbounded series, which exhausts memory and can take down Prometheus. The fix is discipline: label by bounded dimensions (method, route template, status class) and push per-request detail to spans and logs.

9. **B.** It estimates the p99 latency per route from the cumulative buckets. `le` is the bucket-boundary label that `histogram_quantile` consumes; it must survive the `sum by (...)` aggregation or the function has no bucket structure to interpolate within. Dropping `le` is the canonical histogram-PromQL mistake. Citation: <https://prometheus.io/docs/practices/histograms/>.

10. **B.** The trace ID is the shared join key. OTel assigns it, every span carries it, and a context-aware log handler stamps it onto every log record. The two backends then share an identifier: find the slow trace in Jaeger, copy the trace ID, search the log store for it, read the narrative. This is the entire 3am workflow, and it only works if the trace ID is correlated into the logs.

---

## Scoring

- **10/10**: You can teach this material. Move to Week 10 with confidence.
- **8–9**: Solid. Re-read the lecture sections corresponding to the questions you missed, then move on.
- **6–7**: Re-read all three lectures and retake. The three-signals model is dense; do not skim it.
- **≤5**: Slow down. Spend an extra evening on the lectures and the SOLUTIONS.md before attempting the mini-project — the localization drill assumes you have internalized all three signals.
