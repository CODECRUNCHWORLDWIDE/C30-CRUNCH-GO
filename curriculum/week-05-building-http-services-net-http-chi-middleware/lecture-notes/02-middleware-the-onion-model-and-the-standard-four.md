# Lecture 2 — Middleware as `func(http.Handler) http.Handler`: the Onion Model and the Standard Four

> **Time:** 2 hours. Take the middleware-shape material in one sitting and the four standard middlewares in a second. **Prerequisites:** Lecture 1 (the handler/service/repository seam) and Week 4 (`context`, `signal`, `context.WithValue`). **Citations:** the `net/http` docs at <https://pkg.go.dev/net/http>, the `chi/middleware` package at <https://pkg.go.dev/github.com/go-chi/chi/v5/middleware>, and the `log/slog` docs at <https://pkg.go.dev/log/slog>.

## 1. The one shape that makes web middleware composable

A *middleware* is a function that wraps a handler in another handler. In Go the shape is uniform and unmagical:

```go
type Middleware func(next http.Handler) http.Handler
```

A middleware takes the *next* handler in the chain and returns a *new* handler that does something before and/or after calling next. Because every middleware has the same signature, middlewares *compose*: you can stack any number of them in any order, and each one wraps the next. The skeleton:

```go
func Example(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// ... do something BEFORE the next handler (inbound) ...
		next.ServeHTTP(w, r)
		// ... do something AFTER the next handler (outbound) ...
	})
}
```

Read it as: "return a handler that, when called, does some inbound work, calls the wrapped handler, then does some outbound work." Everything cross-cutting in a web service — request IDs, logging, recovery, timeouts, auth, rate limiting, metrics, tracing — is one instance of this shape. Master the shape once and you can write any of them. Citation: the `http.Handler` interface at <https://pkg.go.dev/net/http#Handler>.

## 2. The onion model — why order matters

Stack three middlewares around a handler and you get an onion. The request travels *inward* through the layers to the handler; the response travels *outward* back through them:

```
        ┌─────────────── RequestID (outermost) ───────────────┐
        │  ┌──────────────── Logger ────────────────────────┐ │
        │  │   ┌───────────── Recoverer ─────────────────┐   │ │
   req ─┼──┼───┼──────────────> Handler ─────────────────┼───┼─┼─> resp
        │  │   └──────────────────────────────────────────┘   │ │
        │  └──────────────────────────────────────────────────┘ │
        └────────────────────────────────────────────────────────┘
```

You compose them with a helper that wraps in order:

```go
func Chain(h http.Handler, mws ...Middleware) http.Handler {
	// Apply in reverse so the FIRST middleware listed is the OUTERMOST layer
	// (the first to see the request, the last to see the response).
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// usage:
handler := Chain(mux,
	RequestID,  // outermost: assigns an ID first
	Logger,     // logs with that ID
	Recoverer,  // catches panics from the handler
	Timeout(5*time.Second),
)
```

Order is a *decision*, not an accident. Three rules:

1. **Request ID goes outermost** so every inner layer — including the logger — can read the ID. If the logger ran before the request-ID injector, it would have no ID to log.
2. **Recovery goes close to the handler** (inner) so it catches panics from the handler *and* from any middleware inside it, but logging and request-ID survive a handler panic (they are outside the recover, so they still run their outbound half).
3. **Timeout placement depends on what you want to time** — outside the logger times the whole chain including logging; inside times just the handler. Usually you want it just outside the handler.

`chi` applies middleware in *registration* order (first-registered is outermost), which is the intuitive order; our `Chain` above matches that. Citation: the middleware ordering note in <https://pkg.go.dev/github.com/go-chi/chi/v5#Mux.Use>.

## 3. Middleware 1 — Request ID

Assign every request a unique ID, stash it on the context (Week 4's `context.WithValue` with an unexported key), and echo it in a response header so a client can quote it in a bug report:

```go
package middleware

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

type ctxKey int

const requestIDKey ctxKey = 0

func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Honour an upstream ID (from a proxy / gateway) if present; else mint one.
		id := r.Header.Get("X-Request-Id")
		if id == "" {
			id = newID()
		}
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		w.Header().Set("X-Request-Id", id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestIDFrom reads the request ID set by the RequestID middleware.
func RequestIDFrom(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
```

Three points: **(1)** the key is an unexported `ctxKey` type — Week 4's collision-avoidance rule. **(2)** `r.WithContext(ctx)` returns a *shallow copy* of the request carrying the new context; you pass that copy to `next`. You never mutate `r` in place. **(3)** honouring an inbound `X-Request-Id` lets a request keep its ID across service hops — the seed of distributed tracing in Week 9. Citation: <https://pkg.go.dev/context#WithValue>.

## 4. Middleware 2 — Structured request logging with `slog`

Log one structured line per request, correlated with the request ID, at the right level, with the status and latency. The wrinkle: `http.ResponseWriter` does not expose the status code you wrote, so you wrap it to capture it:

```go
import (
	"log/slog"
	"net/http"
	"time"
)

// statusRecorder wraps a ResponseWriter to remember the status code and bytes.
type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (rec *statusRecorder) WriteHeader(code int) {
	rec.status = code
	rec.ResponseWriter.WriteHeader(code)
}

func (rec *statusRecorder) Write(b []byte) (int, error) {
	if rec.status == 0 {
		rec.status = http.StatusOK // a Write without WriteHeader implies 200
	}
	n, err := rec.ResponseWriter.Write(b)
	rec.bytes += n
	return n, err
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 0}

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
			slog.Int("bytes", rec.bytes),
			slog.Duration("latency", time.Since(start)),
		)
	})
}
```

Four points:

1. **`log/slog`** is the standard library's structured logger (Go 1.21+). It emits key/value attributes that a log aggregator can index — the foundation of Week 9's observability. `slog.LogAttrs` is the allocation-light form.
2. **The `statusRecorder` wrap** is the canonical trick to observe the status code. `ResponseWriter` is write-only by design; embedding it and overriding `WriteHeader`/`Write` lets you record what flew past.
3. **The level is `Error` for 5xx, `Info` otherwise** — so an alert on `level=error` fires on server bugs but not on a client's 404.
4. **The request ID ties this log line to the trace and the metrics** later in the track. One ID, three signals — the observability story Week 9 completes.

Citation: <https://pkg.go.dev/log/slog> and the slog guide at <https://go.dev/blog/slog>.

## 5. Middleware 3 — Panic recovery

A panic in a handler, unrecovered, crashes the goroutine serving that request — and while `net/http` recovers it enough not to kill the whole process, the client gets a dropped connection with no response. A recovery middleware turns a panic into a logged `500`:

```go
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				// http.ErrAbortHandler is a sentinel meaning "drop this; not a bug."
				if rec == http.ErrAbortHandler {
					panic(rec) // re-panic; net/http handles this one specially
				}
				slog.LogAttrs(r.Context(), slog.LevelError, "panic recovered",
					slog.String("request_id", RequestIDFrom(r.Context())),
					slog.Any("panic", rec),
					slog.String("stack", string(debug.Stack())),
				)
				// Only write a 500 if nothing has been written yet.
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = w.Write([]byte(`{"error":{"code":"internal","message":"internal server error"}}`))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

Four points:

1. **`defer` + `recover()`** is the only way to catch a panic. The deferred func runs as the stack unwinds; `recover()` returns the panic value (or nil if there was no panic).
2. **Re-panic on `http.ErrAbortHandler`** — that sentinel is `net/http`'s own "abort this connection silently" signal, not a bug; let it through.
3. **Log the stack** with `runtime/debug.Stack()` so you can find the panicking line. This is your incident trail.
4. **Write the 500 only once.** If the handler already wrote a status before panicking, `WriteHeader` is a no-op with a warning; that is acceptable — you cannot un-send bytes. A more careful version tracks whether anything was written (the `statusRecorder` from §4 can tell you).

Citation: the `recover` builtin at <https://pkg.go.dev/builtin#recover> and `http.ErrAbortHandler` at <https://pkg.go.dev/net/http#pkg-variables>.

## 6. Middleware 4 — Per-request timeout

Bound how long a handler may run by deriving a `context.WithTimeout` from the request's context — Week 4's deadline, applied per request:

```go
func Timeout(d time.Duration) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

The key insight: this is a *parameterised* middleware — `Timeout(5*time.Second)` returns a `Middleware`. It does not stop the handler by force (cancellation is cooperative, Week 4); it gives the handler a context that will be cancelled at the deadline, and a handler that threads `ctx` into its service calls (which it must) will see those calls return `context.DeadlineExceeded`. The standard library also ships `http.TimeoutHandler`, which additionally writes a `503` if the deadline passes before the handler responds — useful, but it does not cancel the handler's work, only races it. Prefer the context-based timeout so the *work itself* stops, and reach for `http.TimeoutHandler` when you also need to guarantee a response within the deadline. Citation: <https://pkg.go.dev/net/http#TimeoutHandler> and <https://pkg.go.dev/context#WithTimeout>.

## 7. The `chi/middleware` library — do not reinvent in production

You just built the four from scratch — which is the point: you must understand them. In production you reach for the well-tested versions in `github.com/go-chi/chi/v5/middleware`, which handle the edge cases (the `statusRecorder` wrap, the `ErrAbortHandler` re-panic, the WebSocket-flush hijack) that a from-scratch version gets wrong on the first try:

```go
import "github.com/go-chi/chi/v5/middleware"

r := chi.NewRouter()
r.Use(middleware.RequestID)               // X-Request-Id, on the context
r.Use(middleware.RealIP)                  // honour X-Forwarded-For from a trusted proxy
r.Use(middleware.Recoverer)               // panic -> 500, with a stack
r.Use(middleware.Timeout(60 * time.Second))
// then your own slog logger, since chi's Logger is a reference impl, not slog-based
```

The pattern is "understand it by building it, then use the library so you do not maintain it." Your *application-specific* middleware (auth, your slog logger, your metrics) you still write yourself, in the shape this lecture taught. Citation: <https://pkg.go.dev/github.com/go-chi/chi/v5/middleware>.

## 8. Testing the chain

A middleware is an `http.Handler` wrapper, so you test it with `httptest` exactly like a handler — assert that the wrapped behaviour happened:

```go
func TestRequestIDEchoed(t *testing.T) {
	var seen string
	h := RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFrom(r.Context()) // the handler can read the ID
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen == "" {
		t.Fatal("handler did not see a request ID on the context")
	}
	if got := rec.Header().Get("X-Request-Id"); got != seen {
		t.Errorf("response header %q != context ID %q", got, seen)
	}
}
```

The same harness tests recovery (panic in the inner handler, assert a 500 and a log line), timeout (a slow inner handler, assert the context was cancelled), and the onion order (a chain that appends to a slice, assert the order). Citation: <https://pkg.go.dev/net/http/httptest>.

## 9. Exercise pointer

Now do **Exercise 2 — Middleware Chain**. Write the four middlewares from scratch, stack them with a `Chain` helper, and write a test that proves the onion order (the request-ID is visible to the logger) and that a panic in the handler becomes a logged 500 while the outer layers still run. The acceptance criterion is a test that fails if you reorder request-ID after the logger.

## 10. Summary

- A middleware is `func(next http.Handler) http.Handler`: do work before and/or after calling `next`. The uniform shape is what makes them compose.
- The onion model: the request travels inward to the handler, the response outward. Order is a decision — request-ID outermost, recovery near the handler.
- Request ID: mint or honour an ID, put it on the context (unexported key), echo it in a header.
- Logging: wrap the `ResponseWriter` to capture the status; log one structured `slog` line per request, correlated with the request ID, error-level on 5xx.
- Recovery: `defer`/`recover`, log the stack, write a 500 once, re-panic on `http.ErrAbortHandler`.
- Timeout: derive `context.WithTimeout` from `r.Context()`; cancellation is cooperative, so the handler must thread `ctx` into its work.
- Build the four to understand them; use `chi/middleware` in production for the edge cases; write your application-specific middleware yourself.
- Test middleware with `httptest` like any handler; assert the wrapped behaviour and the onion order.

Cited references this lecture pulled from: <https://pkg.go.dev/net/http>, <https://pkg.go.dev/log/slog>, <https://go.dev/blog/slog>, <https://pkg.go.dev/github.com/go-chi/chi/v5/middleware>, <https://pkg.go.dev/net/http#TimeoutHandler>, <https://pkg.go.dev/net/http/httptest>.
