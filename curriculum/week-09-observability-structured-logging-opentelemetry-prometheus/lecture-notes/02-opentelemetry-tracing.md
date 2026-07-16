# Lecture 2 — OpenTelemetry Tracing: the Data Model, the SDK, Context Propagation, and Jaeger

> **Time:** 2 hours. Read the data model first (it is short and everything else depends on it), the SDK setup second, and the propagation material last — propagation is where the bugs live. **Prerequisites:** Lecture 1 (the `ContextHandler` that reads the trace ID) and fluency with `context.Context`. **Citations:** the OpenTelemetry-Go documentation at <https://opentelemetry.io/docs/languages/go/>, the getting-started guide at <https://opentelemetry.io/docs/languages/go/getting-started/>, the `go.opentelemetry.io/otel` godoc at <https://pkg.go.dev/go.opentelemetry.io/otel>, the SDK trace godoc at <https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace>, and the W3C Trace Context spec at <https://www.w3.org/TR/trace-context/>.

## 1. What a trace is, and why metrics and logs are not enough

A metric tells you the service got slow. A log line tells you what one piece of code did. Neither tells you *where the time went inside a single request that crossed three layers and a database*. That is the question a **trace** answers.

A trace is the causal tree of one request. Its nodes are **spans**, and each span is a timed operation with a name, a start, an end, a parent, attributes, and a status. The root span is the request as a whole; its children are the operations it triggered — the service-layer call, the database query, the outbound call to another service. Drawn on a timeline, a trace looks like a waterfall: each span is a horizontal bar whose length is its duration, nested under its parent. A glance at that waterfall tells you the thing no metric can: of the 400ms this request took, 380ms was the `db.query` span. That is *localization at the request level*, and it is exactly the resolution metrics throw away.

The vocabulary you must hold (citation: <https://opentelemetry.io/docs/concepts/signals/traces/>):

- **Trace** — the whole tree for one request, identified by a 16-byte **trace ID**.
- **Span** — one operation, identified by an 8-byte **span ID** and carrying the trace ID of its trace plus the span ID of its parent.
- **Span context** — the immutable identity of a span (trace ID, span ID, trace flags) that gets *propagated*; this is what `trace.SpanContextFromContext(ctx)` returned in Lecture 1.
- **Attributes** — typed key/value pairs on a span (`http.method = "GET"`, `db.statement = "SELECT ..."`).
- **Events** — timestamped annotations within a span ("cache miss," "retry").
- **Status** — `Unset`, `Ok`, or `Error`; you set `Error` on a span that failed.

## 2. The API/SDK split, and the four packages you import

OpenTelemetry-Go, like slog, separates a **front-end API** from a **back-end SDK** (citation: <https://opentelemetry.io/docs/languages/go/>):

- The **API** (`go.opentelemetry.io/otel` and `.../otel/trace`) is what your instrumentation code calls: `otel.Tracer("name")`, `tracer.Start(ctx, "span")`, `span.End()`. It is a stable, no-op-by-default surface. Code instrumented against the API does nothing until an SDK is installed.
- The **SDK** (`go.opentelemetry.io/otel/sdk/trace`) is the implementation you wire up once at startup: it samples spans, batches them, and exports them. Your business code never imports the SDK; only `main` (or an `observability` setup package) does.

The four packages you will import this week:

| Package | Role |
|---------|------|
| `go.opentelemetry.io/otel` | The global API: `otel.Tracer`, `otel.SetTracerProvider`, `otel.SetTextMapPropagator`. |
| `go.opentelemetry.io/otel/trace` | The span/tracer types and `SpanContextFromContext`. Imported by instrumentation. |
| `go.opentelemetry.io/otel/sdk/trace` | The `TracerProvider`, `BatchSpanProcessor`, samplers. Imported by setup only. |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | The OTLP/gRPC exporter that ships spans to Jaeger. |

Plus two helpers: `go.opentelemetry.io/otel/sdk/resource` and `go.opentelemetry.io/otel/semconv/v1.26.0` for the resource, and `go.opentelemetry.io/otel/propagation` for the propagator.

## 3. SDK setup: resource, TracerProvider, exporter, propagator, shutdown

Here is the canonical initialization, written as a function that returns a shutdown closure. Read it once top to bottom; the subsections dissect each piece.

```go
package observability

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// InitTracer wires the OpenTelemetry SDK to export spans over OTLP/gRPC to the
// collector at otlpEndpoint (e.g. "localhost:4317", which a local Jaeger
// listens on). It returns a shutdown function the caller must defer; that
// shutdown flushes any buffered spans before the process exits.
func InitTracer(ctx context.Context, serviceName, otlpEndpoint string) (func(context.Context) error, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("build resource: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(otlpEndpoint),
		otlptracegrpc.WithInsecure(), // local stack: no TLS
	)
	if err != nil {
		return nil, fmt.Errorf("build OTLP exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
		),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, // W3C traceparent / tracestate
		propagation.Baggage{},      // W3C baggage
	))

	return tp.Shutdown, nil
}
```

### 3.1 The resource — who is emitting these spans

A **resource** identifies the *entity* producing telemetry: the service. The single most important attribute is `service.name`, because that is how every trace backend groups spans into services. Use the `semconv` helpers — they encode the OpenTelemetry semantic conventions so your attribute keys match what backends expect (citation: <https://opentelemetry.io/docs/specs/semconv/>):

```go
semconv.ServiceName("notes")        // -> service.name = "notes"
semconv.ServiceVersion("1.0.0")     // -> service.version = "1.0.0"
```

A trace whose resource has no `service.name` shows up in Jaeger as `unknown_service:<binary>` — a sign you forgot the resource. Set it.

### 3.2 The TracerProvider and the BatchSpanProcessor

The `TracerProvider` is the SDK's heart: it owns the resource, the sampler, and the span processors, and it manufactures `Tracer`s. The crucial choice is the **span processor**, which decides what happens to a span when it ends. Two implementations matter:

- `WithSyncer` (a `SimpleSpanProcessor`) exports each span *synchronously* the moment it ends — one network call per span. Fine for tests and examples; catastrophic for throughput in production because it blocks the request path on the exporter.
- `WithBatcher` (a `BatchSpanProcessor`) buffers ended spans and exports them in batches on a timer or when the buffer fills. **This is the production choice.** It decouples the request path from the exporter: ending a span is a cheap enqueue, and a background goroutine ships batches. (Citation: <https://pkg.go.dev/go.opentelemetry.io/otel/sdk/trace#NewBatchSpanProcessor>.)

The batcher's existence is *the* reason `Shutdown` matters (Section 7): a buffer of un-exported spans lives in memory, and if the process exits without flushing, those spans are lost.

### 3.3 The exporter — OTLP to a local Jaeger

The **OTLP exporter** speaks the OpenTelemetry Protocol, the vendor-neutral wire format for telemetry. We use `otlptracegrpc`, which ships spans over gRPC to an OTLP receiver on port 4317. Modern Jaeger (v1.35+) ingests OTLP directly — you no longer need the old Jaeger-specific exporter; you point the OTLP exporter at Jaeger's OTLP port and it Just Works (citation: <https://www.jaegertracing.io/docs/latest/apis/#opentelemetry-protocol>). This is the vendor-neutrality payoff: the same exporter that talks to Jaeger today talks to any OTLP-compatible backend tomorrow with a config change and no code change. `WithInsecure()` disables TLS, appropriate for a local docker-compose stack where everything is on a private network.

### 3.4 The propagator — the part everyone forgets

`otel.SetTextMapPropagator` installs the global propagator: the thing that *serializes* the current span context into outbound request headers and *deserializes* it from inbound ones. We install the **W3C Trace Context** propagator (`propagation.TraceContext{}`), which reads and writes the standard `traceparent` header (citation: <https://www.w3.org/TR/trace-context/>):

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
             ^  ^                                ^                ^
          version  trace-id (16 bytes hex)    parent span-id    flags
```

Without a propagator set, the instrumentation libraries (`otelhttp`, `otelgrpc`) have nothing to inject or extract, and **the trace silently breaks at every service boundary** — each service starts a fresh root trace instead of continuing the caller's. This is the single most common OTel bug, and it produces no error: you just get disconnected one-service traces in Jaeger and wonder why nothing joins up. Set the propagator. We wrap it in a `CompositeTextMapPropagator` so it also carries baggage; if you do not use baggage, `propagation.TraceContext{}` alone is enough.

## 4. Creating spans

With the SDK installed, instrumenting code is three lines: get a tracer, start a span, defer the end.

```go
package service

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// A package-level tracer is conventional: name it after the instrumentation
// scope, usually the import path of the package doing the instrumenting.
var tracer = otel.Tracer("github.com/crunch/notes/internal/service")

func (s *Service) GetNote(ctx context.Context, id string) (Note, error) {
	ctx, span := tracer.Start(ctx, "service.GetNote",
		trace.WithAttributes(attribute.String("note.id", id)),
	)
	defer span.End()

	note, err := s.repo.GetNote(ctx, id) // passes the span's ctx down
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch note failed")
		return Note{}, err
	}

	span.SetAttributes(attribute.Int("note.body_len", len(note.Body)))
	return note, nil
}
```

Four rules, each load-bearing:

1. **`tracer.Start` returns a new context *and* the span.** You must use the returned `ctx` for everything downstream — `s.repo.GetNote(ctx, id)` — because that context carries the new span. Pass the *original* `ctx` instead and the repository's span attaches to the wrong parent (or starts a new root). This is the most common span bug: shadowing the wrong context.
2. **`defer span.End()` immediately.** The span's duration is `End` time minus `Start` time; forgetting `End` leaves the span open forever and it never exports.
3. **`span.RecordError(err)` and `span.SetStatus(codes.Error, ...)` on failure.** `RecordError` adds the error as a span event; `SetStatus` marks the span red in Jaeger so a failed request is visually obvious in the waterfall. Do both on the error path.
4. **Attributes describe the span.** `attribute.String("note.id", id)` lets you filter traces in Jaeger ("show me traces where `note.id = 4f2`"). Keep attribute *keys* bounded (a fixed vocabulary); attribute *values* may be high-cardinality on spans — unlike metric labels (Lecture 3), a high-cardinality attribute on a span is fine, because a span is one request, not an aggregate.

## 5. Context propagation across the HTTP boundary

A trace begins at the front door. When a request arrives, the HTTP server must **extract** the inbound `traceparent` header into the request context, so spans started during the request attach to the caller's trace. The contrib package `otelhttp` does this for you (citation: <https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp/otelhttp>):

```go
import "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

// Wrap the whole router. otelhttp extracts the inbound traceparent, starts a
// server span, and puts it in the request context that flows to your handlers.
handler := otelhttp.NewHandler(router, "notes-api",
	otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return r.Method + " " + r.URL.Path
	}),
)
http.ListenAndServe(":8080", handler)
```

`otelhttp.NewHandler` does three things per request: extracts the propagated context from the inbound headers, starts a server span named for the operation, and injects that span's context into `r.Context()`. Every handler downstream now runs under a live span, and `tracer.Start(ctx, ...)` in the service layer produces a child of it. **Order matters with the Lecture 1 middleware:** `otelhttp` must be the *outermost* wrapper so the span context is set before the request-logging middleware reads the trace ID. The chain is `otelhttp.NewHandler(RequestLogger(router))`.

## 6. Context propagation across gRPC

The `notes` service has a gRPC surface (Week 7). To keep a trace whole when one service calls another over gRPC, both ends use the `otelgrpc` stats handler (citation: <https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc>). The stats-handler approach is the current recommended integration; the older interceptor API (`otelgrpc.UnaryClientInterceptor`) is deprecated.

Server side:

```go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

srv := grpc.NewServer(
	grpc.StatsHandler(otelgrpc.NewServerHandler()),
)
```

Client side:

```go
conn, err := grpc.NewClient(target,
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
```

The client handler **injects** the `traceparent` into the outbound gRPC metadata; the server handler **extracts** it into the incoming context. As long as the *outbound call runs under the context that carries the current span* (you pass `ctx` to the gRPC call, as you always should), the second service's spans attach to the first service's trace, and Jaeger draws them as one waterfall. Challenge 1 proves this end to end.

The same principle applies to Postgres. There is no standard "stats handler" for `pgx`, so you either use `otelpgx` (a contrib tracer that hooks `pgx`'s `QueryTracer` and emits a span per query with the SQL as an attribute, citation: <https://github.com/exaring/otelpgx>) or you wrap the query in a manual span:

```go
func (r *Repo) GetNote(ctx context.Context, id string) (Note, error) {
	ctx, span := tracer.Start(ctx, "db.query.GetNote",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
		),
	)
	defer span.End()
	// ... r.pool.QueryRow(ctx, ...) -- the query runs under the span's ctx
}
```

`otelpgx` is the production choice (it instruments every query automatically and follows the `db.*` semantic conventions); the manual span is what you reach for when you want one specific query traced or when you cannot add the dependency. Either way, the DB span is the leaf of the waterfall — and in the mini-project, it is the leaf that turns red when you inject the slow query.

## 7. Clean shutdown — flushing the batch

The `BatchSpanProcessor` buffers spans. If `main` returns while the buffer holds un-exported spans, those spans are dropped — and the trace for the last few requests before shutdown silently vanishes, which is precisely the worst time to lose telemetry (a service shutting down is often a service that just crashed). The fix is to call `TracerProvider.Shutdown`, which flushes the buffer and waits for the export to complete:

```go
func main() {
	ctx := context.Background()
	shutdown, err := observability.InitTracer(ctx, "notes", "localhost:4317")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		// Give the flush a bounded window so a dead collector cannot hang exit.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			slog.Error("tracer shutdown failed", "error", err)
		}
	}()

	// ... run the server, block until a signal, then return so the defer runs
}
```

Two details. First, give `Shutdown` a *bounded* context — if the collector is down, the flush would block forever, and you do not want a dead Jaeger to wedge your process exit. Second, the `defer` only runs if `main` returns normally, so block on a signal (`os.Interrupt`) and return, rather than calling `os.Exit` (which skips defers). This is the same graceful-shutdown discipline you applied to the HTTP server in Week 5; the tracer joins the shutdown sequence.

## 8. Correlating the trace ID back into logs

Lecture 1's `ContextHandler` already does this: `trace.SpanContextFromContext(ctx)` reads the active span out of the context, and we add its `trace_id` to every log record. Now we can see the full loop. A request arrives; `otelhttp` starts a span and puts it in the context; the request-logging middleware logs `"request started"`, and the `ContextHandler` stamps that line with the span's `trace_id`; the service layer starts a child span and logs `"fetching note"`, also stamped with the *same* `trace_id` (same trace, the span ID differs); the DB span runs. Every log line for this request, across every layer, carries one `trace_id`. In Jaeger you find the slow trace; you copy its trace ID; you search your logs for that ID; you read the narrative. That is the join, and it only works because the context propagated from the front door down through every layer.

A useful sanity check while developing: log the trace ID explicitly at the handler so you can eyeball it:

```go
func handleGetNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	sc := trace.SpanContextFromContext(ctx)
	lg := observability.LoggerFrom(ctx)
	lg.InfoContext(ctx, "handling get note", "trace_id", sc.TraceID().String())
	// ...
}
```

If `sc.TraceID().String()` is all zeros, the span context did not propagate — your `otelhttp` wrapper is missing or in the wrong order. That all-zeros trace ID is the symptom you will learn to recognize.

## 9. Wrap-up — the tracing checklist

When you instrument a service with OpenTelemetry this week:

- [ ] `InitTracer` builds a `resource` with `semconv.ServiceName` set — no `unknown_service` in Jaeger.
- [ ] The `TracerProvider` uses `WithBatcher` (the `BatchSpanProcessor`), not `WithSyncer`, outside tests.
- [ ] The OTLP exporter points at the local Jaeger's OTLP port (4317) with `WithInsecure` on the local stack.
- [ ] `otel.SetTextMapPropagator(propagation.TraceContext{})` is set — the trace survives boundaries.
- [ ] `otelhttp.NewHandler` is the outermost HTTP wrapper, so the span context is set before the logging middleware.
- [ ] `otelgrpc.NewServerHandler` / `NewClientHandler` are installed on both gRPC ends.
- [ ] Every layer's span uses the context returned by `tracer.Start`, passed down to children.
- [ ] Failure paths call `span.RecordError` and `span.SetStatus(codes.Error, ...)`.
- [ ] `TracerProvider.Shutdown` is deferred with a bounded context, and `main` returns on a signal rather than `os.Exit`.
- [ ] The `ContextHandler` from Lecture 1 stamps `trace_id` on every log line.

Read the OpenTelemetry-Go getting-started guide before Friday — <https://opentelemetry.io/docs/languages/go/getting-started/>. Exercise 2 (`exercise-02-otel-traced-handler.go`) wires the SDK, an `otelhttp` handler, a service span, a simulated DB span, and trace-correlated `slog` lines, all exporting to a local Jaeger.

Next lecture: Prometheus metrics, the RED method, and the Grafana dashboard that localizes a regression down to the route — which you then pinpoint with the trace you just learned to read.
