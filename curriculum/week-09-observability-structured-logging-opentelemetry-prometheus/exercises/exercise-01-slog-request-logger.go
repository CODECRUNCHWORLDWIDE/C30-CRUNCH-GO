// Exercise 1 — A request-scoped slog logger middleware.
//
// TASK
// ----
// Build a chi middleware that, for every HTTP request:
//
//  1. Mints a short request_id (8 random bytes, hex-encoded).
//  2. Derives a request-scoped *slog.Logger via slog.With, binding request_id,
//     method, and path so every log line for this request shares those fields.
//  3. Stashes the bound logger AND the request_id in the request context, with
//     helpers (LoggerFrom, RequestIDFrom) so any downstream layer can retrieve
//     them without threading them through every function signature.
//  4. Logs "request started" on entry and "request completed" on exit, the
//     latter carrying the status code, the chi route template, and the
//     duration.
//  5. Reads the active OpenTelemetry trace_id from the context if one is
//     present (so this middleware composes with Exercise 2's tracing) and
//     binds it onto the logger.
//
// Run it:
//
//	go mod init ex01 && go get github.com/go-chi/chi/v5 go.opentelemetry.io/otel/trace
//	go run exercise-01-slog-request-logger.go
//	# in another shell:
//	curl -s localhost:8080/notes/4f2 ; curl -s localhost:8080/boom
//
// The server logs JSON to stdout. Confirm both requests share their own
// request_id across the "started" and "completed" lines, and that the status
// and duration are recorded.
//
// EXTEND IT (do these after the base works):
//   - Add a LevelVar so an admin endpoint (POST /admin/loglevel?level=debug)
//     raises verbosity at runtime without a restart.
//   - Switch the production branch to LogAttrs with typed slog.Attr on the
//     hot "request completed" line and explain why in a comment.
//   - Add a panic-recovery wrapper that logs the panic at Error with the
//     request_id and re-panics or returns 500, your call — defend it.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"go.opentelemetry.io/otel/trace"
)

// --- context plumbing -------------------------------------------------------

type loggerCtxKey struct{}
type requestIDCtxKey struct{}

// ContextWithLogger returns a child context carrying lg.
func ContextWithLogger(ctx context.Context, lg *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerCtxKey{}, lg)
}

// LoggerFrom returns the request-scoped logger, or slog.Default if none is set.
func LoggerFrom(ctx context.Context) *slog.Logger {
	if lg, ok := ctx.Value(loggerCtxKey{}).(*slog.Logger); ok {
		return lg
	}
	return slog.Default()
}

// RequestIDFrom returns the request_id, or "" if none is set.
func RequestIDFrom(ctx context.Context) string {
	if id, ok := ctx.Value(requestIDCtxKey{}).(string); ok {
		return id
	}
	return ""
}

// --- the middleware ---------------------------------------------------------

// statusRecorder captures the status code the handler writes, because
// http.ResponseWriter does not expose it after the fact. It defaults to 200,
// which is what net/http assumes when a handler writes a body without an
// explicit WriteHeader.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (sr *statusRecorder) WriteHeader(code int) {
	sr.status = code
	sr.ResponseWriter.WriteHeader(code)
}

// RequestLogger builds a request-scoped logger per request and logs the request
// lifecycle. Mount it inside the otelhttp wrapper (Exercise 2) so the trace
// context is already populated when this middleware reads the trace_id.
func RequestLogger(base *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			reqID := newRequestID()

			lg := base.With(
				slog.String("request_id", reqID),
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
			)

			// Correlate with a trace if one propagated this far.
			if sc := trace.SpanContextFromContext(r.Context()); sc.IsValid() {
				lg = lg.With(slog.String("trace_id", sc.TraceID().String()))
			}

			ctx := context.WithValue(r.Context(), requestIDCtxKey{}, reqID)
			ctx = ContextWithLogger(ctx, lg)
			r = r.WithContext(ctx)

			lg.InfoContext(ctx, "request started")

			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)

			lg.InfoContext(ctx, "request completed",
				slog.Int("status", rec.status),
				slog.String("route", routePattern(r)),
				slog.Duration("duration", time.Since(start)),
			)
		})
	}
}

// routePattern returns the chi route template ("/notes/{id}"), never the
// concrete path — this keeps log fields and (in Exercise 3) metric labels
// stable and low-cardinality.
func routePattern(r *http.Request) string {
	if rc := chi.RouteContext(r.Context()); rc != nil {
		if p := rc.RoutePattern(); p != "" {
			return p
		}
	}
	return "unmatched"
}

func newRequestID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns a short read on these platforms
	return hex.EncodeToString(b[:])
}

// --- a handler that uses the request-scoped logger --------------------------

func handleGetNote(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")

	// The service layer would retrieve the same logger and inherit request_id.
	lg := LoggerFrom(r.Context())
	lg.InfoContext(r.Context(), "fetching note", slog.String("note_id", id))

	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"` + id + `","body":"a note"}`))
}

func handleBoom(w http.ResponseWriter, r *http.Request) {
	lg := LoggerFrom(r.Context())
	lg.ErrorContext(r.Context(), "deliberate failure", slog.String("reason", "demo"))
	http.Error(w, `{"error":"boom"}`, http.StatusInternalServerError)
}

func main() {
	// Text in development, JSON in production. Here we emit JSON so the
	// structured output is obvious.
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	r := chi.NewRouter()
	r.Use(RequestLogger(logger))
	r.Get("/notes/{id}", handleGetNote)
	r.Get("/boom", handleBoom)

	logger.Info("listening", slog.String("addr", ":8080"))
	if err := http.ListenAndServe(":8080", r); err != nil {
		logger.Error("server stopped", slog.String("error", err.Error()))
		os.Exit(1)
	}
}
