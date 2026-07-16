# Week 9 — Resources

Every resource on this page is **free**. The Go package documentation, the OpenTelemetry docs, the Prometheus docs, the Jaeger docs, and the GitHub source repositories are all public and require no account. No paywalled material is linked.

## Required reading (work it into your week)

### Structured logging — `log/slog`

- **`log/slog` package documentation** — the canonical reference for `Logger`, `Handler`, `HandlerOptions`, levels, and attributes:
  <https://pkg.go.dev/log/slog>
- **Go blog — "Structured Logging with slog"** — the design rationale, the performance notes, the `LogValuer` interface. Required; plan for 30 minutes:
  <https://go.dev/blog/slog>
- **The structured-logging proposal / design** — the wiki page tracking the slog proposal and its discussion:
  <https://go.dev/wiki/Proposal-for-structured-logging>
- **`slog.Handler` interface godoc** — the four-method back end you implement for the `ContextHandler`:
  <https://pkg.go.dev/log/slog#Handler>

### Tracing — OpenTelemetry-Go

- **OpenTelemetry-Go docs (top)** — the API/SDK split, the signal overview:
  <https://opentelemetry.io/docs/languages/go/>
- **OpenTelemetry-Go getting started** — wire the SDK, create spans, export. Required before Friday; plan for 45 minutes:
  <https://opentelemetry.io/docs/languages/go/getting-started/>
- **OpenTelemetry-Go instrumentation guide** — manual and library instrumentation, attributes, events, status:
  <https://opentelemetry.io/docs/languages/go/instrumentation/>
- **`go.opentelemetry.io/otel` godoc** — the global API: `Tracer`, `SetTracerProvider`, `SetTextMapPropagator`:
  <https://pkg.go.dev/go.opentelemetry.io/otel>
- **`go.opentelemetry.io/otel/sdk/trace` godoc** — `TracerProvider`, `BatchSpanProcessor`, samplers, `Shutdown`:
  <https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace>

### Metrics — Prometheus `client_golang`

- **`client_golang/prometheus` godoc** — `Counter`, `Gauge`, `Histogram`, `Summary`, the vectors, the registry:
  <https://pkg.go.dev/github.com/prometheus/client_golang/prometheus>
- **`client_golang/prometheus/promhttp` godoc** — `HandlerFor`, exposing `/metrics`:
  <https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp>
- **Prometheus — metric types** — Counter vs Gauge vs Histogram vs Summary, and when each fits. Required; plan for 20 minutes:
  <https://prometheus.io/docs/concepts/metric_types/>
- **Prometheus — histograms and quantiles** — buckets, `histogram_quantile`, why summaries do not aggregate. Required before the mini-project; plan for 30 minutes:
  <https://prometheus.io/docs/practices/histograms/>

## Authoritative deep dives

### The contrib instrumentation packages

- **`otelhttp` godoc** — `NewHandler`/`NewTransport` for HTTP context propagation:
  <https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/otelhttp>
- **`otelgrpc` godoc** — `NewServerHandler`/`NewClientHandler` stats handlers for gRPC propagation:
  <https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc>
- **`opentelemetry-go` repository** — the API and SDK source; read it when the godoc is not enough:
  <https://github.com/open-telemetry/opentelemetry-go>
- **`opentelemetry-go-contrib` repository** — the instrumentation packages (`otelhttp`, `otelgrpc`) and examples:
  <https://github.com/open-telemetry/opentelemetry-go-contrib>

### Prometheus practices and PromQL

- **Prometheus — metric and label naming** — the conventions for `_total`, `_seconds`, base units, label discipline:
  <https://prometheus.io/docs/practices/naming/>
- **Prometheus — instrumentation best practices** — what to instrument, how, and the cardinality rules:
  <https://prometheus.io/docs/practices/instrumentation/>
- **Prometheus — `histogram_quantile` function reference**:
  <https://prometheus.io/docs/prometheus/latest/querying/functions/#histogram_quantile>
- **`client_golang` repository** — the Go client source and examples:
  <https://github.com/prometheus/client_golang>

### The RED method and dashboards

- **Tom Wilkie — "The RED Method: key metrics for microservices architecture"** — the original article that named Rate/Errors/Duration. Required; plan for 20 minutes:
  <https://www.weave.works/blog/the-red-method-key-metrics-for-microservices-architecture/>
- **Grafana — dashboards documentation** — panels, queries, provisioning dashboards from JSON:
  <https://grafana.com/docs/grafana/latest/dashboards/>

### Tracing backends and the wire standard

- **Jaeger — getting started** — running the all-in-one image, the UI, the ports:
  <https://www.jaegertracing.io/docs/latest/getting-started/>
- **Jaeger — APIs, including OTLP ingestion** — Jaeger ingests OTLP directly (`COLLECTOR_OTLP_ENABLED`), so the OTLP exporter points straight at it; no separate collector needed:
  <https://www.jaegertracing.io/docs/latest/apis/>
- **W3C Trace Context specification** — the `traceparent`/`tracestate` headers that carry the trace across boundaries:
  <https://www.w3.org/TR/trace-context/>

## Source you should read

The Go standard library, OpenTelemetry, and `client_golang` are all open source; source-link works. When a lecture says "the `Handler` interface is four methods, go read it," it means literally that — open the link, scroll, return.

- **`log/slog` source** — the handler implementations are small and worth skimming:
  <https://cs.opensource.google/go/go/+/refs/tags/go1.22.0:src/log/slog/>
- **OpenTelemetry signals — traces (concepts)** — the data model in prose:
  <https://opentelemetry.io/docs/concepts/signals/traces/>
- **OpenTelemetry semantic conventions** — the canonical attribute keys (`service.name`, `http.*`, `db.*`) the `semconv` package encodes:
  <https://opentelemetry.io/docs/specs/semconv/>
- **`otelpgx`** — the `pgx` query tracer that emits a span per query following the `db.*` conventions (the production way to trace the Postgres layer):
  <https://github.com/exaring/otelpgx>

## Tools (all free)

- **Jaeger all-in-one** — traces UI + OTLP receiver in one container; the trace backend for this week:
  <https://www.jaegertracing.io/docs/latest/getting-started/>
- **Prometheus** — the metrics server that scrapes `/metrics`:
  <https://prometheus.io/docs/prometheus/latest/getting_started/>
- **Grafana** — the dashboard layer over Prometheus:
  <https://grafana.com/docs/grafana/latest/setup-grafana/>
- **`promtool`** — ships with Prometheus; validates `prometheus.yml` and checks PromQL:
  <https://prometheus.io/docs/prometheus/latest/command-line/promtool/>
- **OpenTelemetry Collector** — the vendor-neutral telemetry pipeline; not required this week (Jaeger ingests OTLP directly) but the production path when you outgrow a single backend:
  <https://opentelemetry.io/docs/collector/>

## How to use this resource list

The lectures cite specific URLs from this page at decision points. The links you should read end-to-end this week, in order:

1. **Go blog — "Structured Logging with slog"** — the logging model. ~30 minutes, before Monday's exercise.
2. **OpenTelemetry-Go getting started** — the tracing model. ~45 minutes, before Wednesday's exercise.
3. **Prometheus — metric types** and **histograms & quantiles** — the metrics model. ~50 minutes total, before Friday's exercise.
4. **Tom Wilkie — the RED method** — the opinionated answer to "what to measure." ~20 minutes, before the mini-project.
5. **W3C Trace Context** — skim the `traceparent` section so the propagation reflection questions in Challenge 1 make sense. ~15 minutes.

The rest are reference material. Bookmark and return when a specific question arises.

---

*Bookmarks decay. If a link rots, search the title — these are all canonical pieces and they reappear on the same projects' new homes.*
