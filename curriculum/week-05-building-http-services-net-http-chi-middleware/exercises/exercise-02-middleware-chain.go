// Exercise 2 — Middleware Chain: the Onion Model, from Scratch
//
// GOAL
//   Write the four standard middlewares (request ID, slog logger, panic
//   recovery, timeout) as func(http.Handler) http.Handler, compose them with a
//   Chain helper, and prove the onion order in a test: the logger must see the
//   request ID, and a panic in the handler must become a logged 500 while the
//   outer layers still run.
//
// HOW TO RUN
//   mkdir ex02 && cd ex02 && go mod init ex02
//   # save this file as main.go and the test below as main_test.go
//   go test ./... && go vet ./...
//   go run .   # then: curl -i localhost:8080/panic  -> 500; curl -i localhost:8080/ok
//
// TASKS
//   1. Read RequestID: it mints/honours an ID, puts it on the context (unexported
//      key), and echoes X-Request-Id. RequestIDFrom reads it back.
//   2. Read Logger: it wraps the ResponseWriter to capture the status, then logs
//      one slog line with the request_id, status, and latency.
//   3. Read Recoverer: defer/recover turns a panic into a logged 500.
//   4. Read Chain: it applies middlewares so the FIRST listed is OUTERMOST.
//   5. Confirm the test that reorders RequestID after Logger would break the
//      "logger sees the id" assertion.

package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

type Middleware func(http.Handler) http.Handler

func Chain(h http.Handler, mws ...Middleware) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// ---- request id ----

type ctxKey int

const requestIDKey ctxKey = 0

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			b := make([]byte, 8)
			_, _ = rand.Read(b)
			id = hex.EncodeToString(b)
		}
		w.Header().Set("X-Request-Id", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// ---- logger ----

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		level := slog.LevelInfo
		if rec.status >= 500 {
			level = slog.LevelError
		}
		slog.LogAttrs(r.Context(), level, "http request",
			slog.String("request_id", RequestIDFrom(r.Context())),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
			slog.Int("status", rec.status),
			slog.Duration("latency", time.Since(start)),
		)
	})
}

// ---- recoverer ----

func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rv := recover(); rv != nil {
				if rv == http.ErrAbortHandler {
					panic(rv)
				}
				slog.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.Any("panic", rv),
					slog.String("stack", string(debug.Stack())),
				)
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"internal server error"}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// ---- timeout ----

func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newHandler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/ok", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok " + RequestIDFrom(r.Context())))
	})
	mux.HandleFunc("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})
	return Chain(mux, RequestID, Logger, Recoverer, Timeout(5*time.Second))
}

func main() {
	srv := &http.Server{Addr: ":8080", Handler: newHandler(), ReadHeaderTimeout: 5 * time.Second}
	_ = srv.ListenAndServe()
}

/*
main_test.go — save alongside this file:

package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOnionOrderAndRecovery(t *testing.T) {
	h := newHandler()

	// /ok: the handler should see a request id (proves RequestID is outer of it),
	// and the response header should echo the same id.
	req := httptest.NewRequest("GET", "/ok", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/ok: got %d", rec.Code)
	}
	id := rec.Header().Get("X-Request-Id")
	if id == "" {
		t.Fatal("/ok: missing X-Request-Id")
	}
	if got := rec.Body.String(); got != "ok "+id {
		t.Errorf("/ok body = %q, want %q (handler did not see the id)", got, "ok "+id)
	}

	// /panic: Recoverer turns the panic into a 500; the chain does not crash.
	req = httptest.NewRequest("GET", "/panic", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Errorf("/panic: got %d want 500", rec.Code)
	}
}
*/
