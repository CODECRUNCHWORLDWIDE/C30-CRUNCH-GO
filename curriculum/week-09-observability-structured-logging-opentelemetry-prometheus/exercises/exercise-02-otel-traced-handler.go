// Exercise 2 — An OpenTelemetry-traced HTTP handler exporting to local Jaeger.
//
// TASK
// ----
// Wire the OpenTelemetry SDK end to end and prove the trace reaches Jaeger:
//
//  1. Initialize the SDK: a resource with semconv.ServiceName, a
//     TracerProvider with a BatchSpanProcessor and an OTLP/gRPC exporter
//     pointed at a local Jaeger (localhost:4317), and the W3C TraceContext
//     propagator. Return a Shutdown closure.
//  2. Wrap the HTTP handler with otelhttp so the inbound traceparent is
//     extracted and a server span is started per request.
//  3. In a service function, start a CHILD span using the context the handler
//     passed down — not a fresh background context — so it attaches correctly.
//  4. Simulate a DB call in its own child span, set an attribute on it, and
//     occasionally record an error + Error status to show a red span.
//  5. Emit slog log lines that carry the live trace_id, extracted with
//     trace.SpanContextFromContext, so logs and traces share a join key.
//  6. Defer Shutdown with a BOUNDED context so the BatchSpanProcessor flushes
//     buffered spans before exit (and a dead collector cannot hang the process).
//
// Run it (Jaeger must be up — see the mini-project compose, or:
//
//	docker run --rm -p16686:16686 -p4317:4317 \
//	  -e COLLECTOR_OTLP_ENABLED=true jaegertracing/all-in-one:1.57
//
// then):
//
//	go mod init ex02
//	go get go.opentelemetry.io/otel \
//	       go.opentelemetry.io/otel/sdk \
//	       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc \
//	       go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
//	go run exercise-02-otel-traced-handler.go
//	# in another shell:
//	for i in 1 2 3 4 5; do curl -s localhost:8080/notes/$i >/dev/null; done
//
// Open Jaeger at http://localhost:16686, select service "notes-ex02", and
// inspect a trace: server span -> service.GetNote span -> db.query span. Copy
// the trace_id printed in the logs and confirm it matches the trace in Jaeger.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	serviceName  = "notes-ex02"
	otlpEndpoint = "localhost:4317"
)

// tracer is the instrumentation scope for this package.
var tracer = otel.Tracer("ex02/notes")

// initTracer wires the SDK and returns a shutdown function that flushes spans.
func initTracer(ctx context.Context) (func(context.Context) error, error) {
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
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("build OTLP exporter: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter, sdktrace.WithBatchTimeout(2*time.Second)),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.AlwaysSample())),
	)

	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	return tp.Shutdown, nil
}

// --- the handler -> service -> "db" path ------------------------------------

func handleGetNote(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := strings.TrimPrefix(r.URL.Path, "/notes/")

	// Log with the trace_id so this line joins to the trace in Jaeger.
	sc := trace.SpanContextFromContext(ctx)
	slog.InfoContext(ctx, "handling get note",
		slog.String("note_id", id),
		slog.String("trace_id", sc.TraceID().String()),
	)

	note, err := getNote(ctx, id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"id":%q,"body":%q}`, id, note)
}

// getNote is the service layer. It starts a child span off the handler's ctx.
func getNote(ctx context.Context, id string) (string, error) {
	ctx, span := tracer.Start(ctx, "service.GetNote",
		trace.WithAttributes(attribute.String("note.id", id)),
	)
	defer span.End()

	body, err := queryNote(ctx, id) // passes the span's ctx down
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "fetch note failed")
		return "", err
	}
	span.SetAttributes(attribute.Int("note.body_len", len(body)))
	return body, nil
}

// queryNote simulates a Postgres call inside its own span.
func queryNote(ctx context.Context, id string) (string, error) {
	ctx, span := tracer.Start(ctx, "db.query.GetNote",
		trace.WithAttributes(
			attribute.String("db.system", "postgresql"),
			attribute.String("db.operation", "SELECT"),
			attribute.String("db.statement", "SELECT body FROM notes WHERE id = $1"),
		),
	)
	defer span.End()

	// Realistic jittered latency, occasionally slow, occasionally failing.
	delay := time.Duration(10+rand.Intn(40)) * time.Millisecond
	if rand.Intn(5) == 0 {
		delay = 300 * time.Millisecond // a slow query, to show in the waterfall
		span.AddEvent("slow query path taken")
	}
	select {
	case <-time.After(delay):
	case <-ctx.Done():
		span.RecordError(ctx.Err())
		span.SetStatus(codes.Error, "query cancelled")
		return "", ctx.Err()
	}

	if id == "0" {
		err := errors.New("note not found")
		span.RecordError(err)
		span.SetStatus(codes.Error, "not found")
		return "", err
	}

	slog.InfoContext(ctx, "query complete",
		slog.String("note_id", id),
		slog.Duration("db_duration", delay),
	)
	return "note body for " + id, nil
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	ctx := context.Background()
	shutdown, err := initTracer(ctx)
	if err != nil {
		logger.Error("init tracer failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer func() {
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(flushCtx); err != nil {
			logger.Error("tracer shutdown failed", slog.String("error", err.Error()))
		}
	}()

	// otelhttp is the OUTERMOST wrapper: it extracts traceparent and starts the
	// server span before any inner middleware or handler runs.
	mux := http.NewServeMux()
	mux.HandleFunc("/notes/", handleGetNote)
	handler := otelhttp.NewHandler(mux, "notes-api",
		otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
			return r.Method + " /notes/{id}"
		}),
	)

	srv := &http.Server{Addr: ":8080", Handler: handler}

	go func() {
		logger.Info("listening", slog.String("addr", ":8080"))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", slog.String("error", err.Error()))
		}
	}()

	// Block until interrupted, then return so the deferred Shutdown runs (and
	// flushes spans). os.Exit would skip the defer and drop buffered spans.
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop

	shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutCtx)
	logger.Info("shutting down")
}
