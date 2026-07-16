# Challenge 1 — Trace Across gRPC: One Trace ID Spans Two Services

> **Estimated time:** 3 hours. **Prerequisite:** Exercise 2 complete (you can wire the OTel SDK and read a trace in Jaeger) and the `notes` gRPC surface from Week 7. **Citations:** the `otelgrpc` godoc at <https://pkg.go.dev/go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc>, the OpenTelemetry-Go instrumentation docs at <https://opentelemetry.io/docs/languages/go/instrumentation/>, and the W3C Trace Context spec at <https://www.w3.org/TR/trace-context/>.

## The premise

A trace is only useful if it stays whole. Inside one process, the context propagates the span automatically because you pass `ctx` down the call stack. Across a *network* boundary the context does not travel on its own — it has to be serialized into the request's headers on the way out and deserialized on the way in. For HTTP, `otelhttp` does this. For gRPC, the `otelgrpc` stats handler does it. This challenge proves the gRPC half: a request enters a REST front-end service, which makes a gRPC call to a second service, and you confirm in Jaeger that **one trace ID spans both services** — the REST span, the gRPC client span, the gRPC server span, and the second service's work span all hang off the same trace.

If propagation works, you see one connected waterfall across two service names. If it is broken, you see two disconnected traces and an all-zeros parent on the second service's root span. The difference is a few lines of wiring, and learning to spot which one you have is the skill.

## What you will build

- **Service A — `notes-gateway`** (REST front door): a chi server wrapped with `otelhttp`, exposing `GET /notes/{id}`. On each request it acts as a gRPC *client* and calls Service B's `GetNote` RPC.
- **Service B — `notes-core`** (gRPC backend): a gRPC server implementing `GetNote`, with the `otelgrpc` server handler installed, that opens a span for its work and a child span for a simulated DB query.
- Both services initialize the OTel SDK (resource with a distinct `service.name` each, the OTLP exporter to the same local Jaeger, the `TraceContext` propagator).
- A short `RESULTS.md` documenting the single trace spanning both services, with the trace ID and a description of the waterfall.

## Setup

### 1. Bring up Jaeger

```bash
docker run --rm -p 16686:16686 -p 4317:4317 \
  -e COLLECTOR_OTLP_ENABLED=true \
  jaegertracing/all-in-one:1.57
```

Or use the mini-project's `docker compose up jaeger`. Confirm the UI loads at <http://localhost:16686>.

### 2. The shared init (both services)

Reuse `initTracer` from Exercise 2 verbatim, parameterizing the service name:

```go
func initTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	res, _ := resource.New(ctx, resource.WithAttributes(semconv.ServiceName(serviceName)))
	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint("localhost:4317"),
		otlptracegrpc.WithInsecure())
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})
	return tp.Shutdown, nil
}
```

Service A calls `initTracer(ctx, "notes-gateway")`; Service B calls `initTracer(ctx, "notes-core")`. Both export to the same Jaeger. The two distinct `service.name`s are what let Jaeger draw the cross-service waterfall with the boundary labeled.

### 3. Service B — the gRPC server (the extract side)

The load-bearing line is `grpc.StatsHandler(otelgrpc.NewServerHandler())`. It extracts the propagated `traceparent` from the incoming gRPC metadata and continues the caller's trace.

```go
import "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"

srv := grpc.NewServer(
	grpc.StatsHandler(otelgrpc.NewServerHandler()),
)
notespb.RegisterNotesServer(srv, &coreServer{})
```

The server method opens its own spans off the *incoming* context (which now carries the propagated parent), so its work attaches to Service A's trace:

```go
var tracer = otel.Tracer("notes-core")

func (s *coreServer) GetNote(ctx context.Context, req *notespb.GetNoteRequest) (*notespb.GetNoteResponse, error) {
	ctx, span := tracer.Start(ctx, "core.GetNote",
		trace.WithAttributes(attribute.String("note.id", req.GetId())))
	defer span.End()

	body := s.query(ctx, req.GetId()) // a child span for the simulated DB call
	return &notespb.GetNoteResponse{Body: body}, nil
}

func (s *coreServer) query(ctx context.Context, id string) string {
	_, span := tracer.Start(ctx, "db.query.GetNote",
		trace.WithAttributes(attribute.String("db.system", "postgresql")))
	defer span.End()
	time.Sleep(time.Duration(10+rand.Intn(30)) * time.Millisecond)
	return "body for " + id
}
```

### 4. Service A — the gRPC client (the inject side)

The load-bearing line is `grpc.WithStatsHandler(otelgrpc.NewClientHandler())` on the dial. It injects the current span context into the outbound gRPC metadata.

```go
conn, err := grpc.NewClient("localhost:9090",
	grpc.WithTransportCredentials(insecure.NewCredentials()),
	grpc.WithStatsHandler(otelgrpc.NewClientHandler()),
)
client := notespb.NewNotesClient(conn)
```

The REST handler runs under the `otelhttp` server span; it calls the gRPC client *with the request context*, so the client span is a child of the server span and the injected `traceparent` carries that lineage to Service B:

```go
func (g *gateway) handleGetNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context() // carries the otelhttp server span
	id := chi.URLParam(r, "id")
	resp, err := g.client.GetNote(ctx, &notespb.GetNoteRequest{Id: id}) // ctx is mandatory
	// ... write resp.Body
}

// in main:
handler := otelhttp.NewHandler(router, "notes-gateway")
```

### 5. Run and exercise

```bash
go run ./cmd/notes-core    # Service B on :9090
go run ./cmd/notes-gateway # Service A on :8080
curl -s localhost:8080/notes/4f2
```

## Acceptance criteria

- [ ] Both services start, register their OTel SDKs, and export to the same Jaeger.
- [ ] Service A's HTTP entry is wrapped with `otelhttp.NewHandler`.
- [ ] Service A's gRPC client is dialed with `grpc.WithStatsHandler(otelgrpc.NewClientHandler())`.
- [ ] Service B's gRPC server is created with `grpc.StatsHandler(otelgrpc.NewServerHandler())`.
- [ ] `otel.SetTextMapPropagator(propagation.TraceContext{})` is set in **both** services.
- [ ] One `curl` produces **one** trace in Jaeger spanning **two** service names (`notes-gateway` and `notes-core`).
- [ ] The waterfall shows: `GET /notes/{id}` (gateway server span) → gRPC client span → `notes.Notes/GetNote` (core server span) → `core.GetNote` → `db.query.GetNote`.
- [ ] The single trace ID is identical across all spans; the core service's root span has the gateway's client span as its parent, not an all-zeros parent.
- [ ] `RESULTS.md` records the trace ID and describes the waterfall.

## Reflection (write into RESULTS.md)

1. **Delete the propagator from Service B only.** Re-run. What do you see in Jaeger now? Explain why the gateway's trace and the core's trace become disconnected, and which span shows the all-zeros parent. (This is the canonical "no propagator" failure — you should be able to recognize it on sight after this.)

2. **Pass `context.Background()` to the gRPC call instead of `r.Context()`.** Re-run with the propagator restored. The wiring is correct but the trace still breaks — why? What does this tell you about the relative importance of the stats handler versus passing the right context? (Hint: the stats handler can only inject what is *in* the context it is given.)

3. **The `traceparent` on the wire.** The W3C Trace Context spec defines `traceparent` as `version-traceid-spanid-flags`. When Service A calls Service B, which field carries the *gateway's* span ID, and how does Service B use it to set the parent of its root span? Read <https://www.w3.org/TR/trace-context/#traceparent-header> and quote the relevant field.

4. **gRPC vs HTTP propagation.** For HTTP, `traceparent` is an HTTP header. For gRPC, where does it live on the wire? (Hint: gRPC carries metadata as HTTP/2 headers too.) Why can the same `propagation.TraceContext{}` propagator serve both transports?

5. **Sampling across the boundary.** Both services use `AlwaysSample`. If Service A used a probabilistic sampler that *dropped* this trace, what should Service B do — sample independently, or honor the parent's decision? Read about `ParentBased` samplers and explain why honoring the parent is the correct default. (This is why the exercise's production `initTracer` uses `ParentBased(AlwaysSample())`.)

## Stretch goals (optional)

- **Add a third hop.** Have `notes-core` call a `notes-audit` service over gRPC. Confirm the single trace now spans three services.
- **Inject a failure.** Make `notes-core`'s `GetNote` return a gRPC `codes.NotFound` for `id == "0"`. Confirm the error status propagates onto the span (the `otelgrpc` handler maps the gRPC code to the span status) and the failed span is red in Jaeger.
- **Correlate logs across both services.** Wire the Lecture 1 `ContextHandler` into both services' loggers and confirm a single `trace_id` appears in *both* services' log output for one request — the cross-service log/trace join.

## Submission

Place under `challenges/challenge-01/`:

- `cmd/notes-gateway/` and `cmd/notes-core/` (or a single module with both).
- The shared `.proto` (reuse the Week 7 `notes.proto`) and generated stubs.
- `RESULTS.md` with the trace ID, the waterfall description, and the five reflection answers.

Commit with the message `challenge-01: trace propagated across grpc, one trace id spans two services`.
