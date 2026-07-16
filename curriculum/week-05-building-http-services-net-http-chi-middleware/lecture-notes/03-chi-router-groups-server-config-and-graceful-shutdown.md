# Lecture 3 — The `chi` Router, Route Groups, `http.Server` Configuration, and Graceful Shutdown

> **Time:** 2 hours. Take the `chi` material in one sitting and the server-config-and-shutdown material in a second. **Prerequisites:** Lectures 1 and 2, and Week 4 (`signal.NotifyContext`, `context` cancellation). **Citations:** the `chi` docs at <https://pkg.go.dev/github.com/go-chi/chi/v5>, the `http.Server` docs at <https://pkg.go.dev/net/http#Server>, and `Server.Shutdown` at <https://pkg.go.dev/net/http#Server.Shutdown>.

## 1. What `chi` adds, and what it deliberately does not

You can build a real service on the Go 1.22 `ServeMux` alone — and for a small surface you should. `chi` earns its place when the routing surface grows: nested resource groups, per-group middleware, sub-routers you can test in isolation, and a stable URL-parameter API. Crucially, `chi` is `net/http` to its bones:

- every `chi` handler is an `http.HandlerFunc`,
- every `chi` middleware is the `func(http.Handler) http.Handler` from Lecture 2,
- a `chi.Router` *is* an `http.Handler`, so you can mount it anywhere a handler goes.

What `chi` deliberately does *not* add: no reflection-based handler signatures, no struct-tag routing, no dependency-injection container, no "magic." It is a router and a middleware stack, nothing more. That restraint is why a senior Go engineer trusts it — there is no hidden request lifecycle to learn. Citation: the project philosophy at <https://github.com/go-chi/chi#chi-router> and the package docs at <https://pkg.go.dev/github.com/go-chi/chi/v5>.

## 2. A `chi` router with groups and sub-routers

```go
package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
)

func newRouter(h *Handler) http.Handler {
	r := chi.NewRouter()

	// Global middleware: applied to every route. Order = onion order.
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(Logger)            // our slog logger from Lecture 2
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(15 * time.Second))

	// A health endpoint, outside the versioned API.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// A versioned API sub-router with its own group.
	r.Route("/v1", func(r chi.Router) {
		r.Route("/notes", func(r chi.Router) {
			r.Get("/", h.List)            // GET    /v1/notes
			r.Post("/", h.Create)         // POST   /v1/notes
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", h.Get)         // GET    /v1/notes/{id}
				r.Patch("/", h.Update)    // PATCH  /v1/notes/{id}
				r.Delete("/", h.Delete)   // DELETE /v1/notes/{id}
			})
		})
	})

	return r
}
```

Five `chi` features on display:

1. **`r.Use(mw)`** registers global middleware in onion order — first registered is outermost (the intuitive order, matching Lecture 2's `Chain`).
2. **`r.Route(pattern, fn)`** creates a *sub-router* mounted at `pattern`. Routes registered inside `fn` are relative to it (`r.Get("/")` inside `/notes` is `GET /notes`). Sub-routers nest cleanly.
3. **`r.Get` / `r.Post` / `r.Patch` / `r.Delete`** are method-specific registrars — clearer than the `ServeMux`'s `"METHOD /path"` string.
4. **Per-group middleware**: a `r.Use(...)` *inside* a `r.Route` block applies only to that group. This is how you put auth on `/v1/admin` but not on `/healthz` — the single biggest ergonomic win over the bare `ServeMux`.
5. **URL parameters via `chi.URLParam(r, "id")`**, the `chi` equivalent of `r.PathValue("id")`:

```go
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	n, err := h.svc.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toResponse(n))
}
```

Citation: <https://pkg.go.dev/github.com/go-chi/chi/v5#Mux.Route> and <https://pkg.go.dev/github.com/go-chi/chi/v5#URLParam>.

## 3. Route groups and per-group middleware

The pattern for "these routes need auth, these do not" uses `r.Group` (an inline group with no path prefix) or a sub-router with its own `Use`:

```go
r.Route("/v1", func(r chi.Router) {
	// Public: no auth.
	r.Get("/health", h.Health)

	// Authenticated group: everything inside requires a valid token.
	r.Group(func(r chi.Router) {
		r.Use(Authenticate) // your auth middleware, applied only to this group
		r.Route("/notes", func(r chi.Router) {
			r.Get("/", h.List)
			r.Post("/", h.Create)
		})
	})
})
```

`r.Group(fn)` creates an inline middleware-stack boundary without a URL prefix — the routes inside share the group's middleware but keep their paths. This is the composition that makes a real API's authorization surface legible: you can read the router and see exactly which routes are public and which are gated. Citation: <https://pkg.go.dev/github.com/go-chi/chi/v5#Mux.Group>.

## 4. `http.Server` configuration — the timeouts that matter

`http.ListenAndServe(addr, handler)` constructs a `http.Server` with *no timeouts*, which means a single slow client can hold a connection open forever. A real service constructs the server explicitly:

```go
srv := &http.Server{
	Addr:              ":8080",
	Handler:           router,
	ReadHeaderTimeout: 5 * time.Second,   // time to read request headers
	ReadTimeout:       15 * time.Second,  // time to read the whole request
	WriteTimeout:      15 * time.Second,  // time to write the response
	IdleTimeout:       60 * time.Second,  // keep-alive idle time before closing
	MaxHeaderBytes:    1 << 20,           // cap header size (1 MiB)
}
```

Each timeout defends against a real attack or failure mode:

1. **`ReadHeaderTimeout`** is the single most important one — without it, a *slow-loris* attacker opens a connection and dribbles header bytes one per second, tying up a server goroutine indefinitely. Set it always. (If you set nothing else, set this.)
2. **`ReadTimeout`** caps the total time to read the request (headers + body). A client that sends a huge body slowly hits this.
3. **`WriteTimeout`** caps the time to write the response — a client that reads the response one byte per second hits this.
4. **`IdleTimeout`** caps how long a kept-alive connection sits idle before the server closes it, bounding idle-connection resource use.
5. **`MaxHeaderBytes`** caps header size, the header-level analogue of `http.MaxBytesReader` for the body.

The trade with `ReadTimeout`/`WriteTimeout` is that they are *absolute* per-request budgets, awkward for legitimately long requests (a large upload, a streaming response). For those, prefer `ReadHeaderTimeout` plus per-handler `context.WithTimeout` (Lecture 2), which times the *work* rather than the connection. Citation: <https://pkg.go.dev/net/http#Server> and the blog post "The complete guide to Go net/http timeouts" referenced in resources.

## 5. Graceful shutdown — cancellation at the top of the server

`srv.Shutdown(ctx)` is the graceful counterpart to `ListenAndServe`. It (1) stops accepting new connections, (2) closes idle keep-alive connections, and (3) waits for in-flight requests to finish — up to the deadline on the `ctx` you pass. This is Week 4's cancellation model applied to the whole server, driven by `signal.NotifyContext`:

```go
func main() {
	// 1. The signal context: cancelled on Ctrl-C or SIGTERM (what a container
	//    runtime sends to ask a pod to stop).
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{Addr: ":8080", Handler: router, ReadHeaderTimeout: 5 * time.Second}

	// 2. Run the server in a goroutine so main can wait on the signal.
	serverErr := make(chan error, 1)
	go func() {
		// ListenAndServe returns http.ErrServerClosed on a clean Shutdown.
		serverErr <- srv.ListenAndServe()
	}()
	slog.Info("server started", "addr", srv.Addr)

	// 3. Wait for either a fatal server error or a shutdown signal.
	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "err", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		slog.Info("shutdown signal received; draining")

		// 4. Give in-flight requests a bounded grace period to finish.
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			// Deadline hit before all requests drained: force-close.
			slog.Error("graceful shutdown timed out; forcing close", "err", err)
			_ = srv.Close()
		} else {
			slog.Info("graceful shutdown complete")
		}
	}
}
```

Six load-bearing details:

1. **`signal.NotifyContext`** (Week 4) turns `SIGTERM`/`SIGINT` into a context cancellation. In Kubernetes, the kubelet sends `SIGTERM` to ask a pod to stop; this is exactly how you respond to it.
2. **The server runs in a goroutine** so `main` can `select` on both the server-error channel and the signal context.
3. **`ListenAndServe` returns `http.ErrServerClosed`** after a clean `Shutdown` — that is *not* an error, it is the success sentinel. Any *other* error is a real failure. Distinguish with `errors.Is`.
4. **A fresh `shutdownCtx` with its own deadline** bounds the drain. We do *not* reuse the cancelled signal context — that is already done; the drain needs its own budget (here 10s).
5. **`Shutdown` returning an error means the deadline passed before the drain finished** — some requests were still in flight. The fallback is `srv.Close()`, which force-closes everything. The choice of grace period (10s) should match your slowest legitimate request plus a margin; in Kubernetes it must be shorter than the pod's `terminationGracePeriodSeconds`.
6. **In-flight requests that thread `ctx`** see their context cancelled when the *server* shuts down, so a request stuck on a slow downstream is cancelled rather than blocking the drain forever. This is why threading `r.Context()` (Lecture 1) all the way down matters for shutdown, not just for client disconnects.

This is the seed of Week 11's full Kubernetes graceful shutdown — the drain-on-`SIGTERM` contract a service owes the cluster. We plant the whole pattern here. Citation: <https://pkg.go.dev/net/http#Server.Shutdown> and <https://pkg.go.dev/net/http#pkg-variables> for `ErrServerClosed`.

## 6. Testing the whole router with `httptest.NewServer`

Lecture 1 tested handlers with `NewRequest`+`NewRecorder` (no network). To test the *whole router* — middleware, routing, the lot — `httptest.NewServer` stands up a real server on a random port:

```go
func TestNotesAPI(t *testing.T) {
	svc := notes.NewService(notes.NewMemRepo())
	srv := httptest.NewServer(newRouter(&Handler{svc: svc}))
	defer srv.Close()

	// Create a note.
	body := strings.NewReader(`{"title":"first","body":"hello"}`)
	resp, err := http.Post(srv.URL+"/v1/notes", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc == "" {
		t.Error("create: missing Location header")
	}

	// A missing note is a 404.
	resp2, _ := http.Get(srv.URL + "/v1/notes/does-not-exist")
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusNotFound {
		t.Errorf("get missing: got %d, want 404", resp2.StatusCode)
	}
}
```

`httptest.NewServer` exercises the real stack — TLS optional via `NewTLSServer`, real `http.Client`, real middleware chain — at the cost of a real (loopback) socket. Use `NewRequest`+`NewRecorder` for fast unit tests of one handler; use `NewServer` for an integration test of the assembled router. Both are in the same package. Citation: <https://pkg.go.dev/net/http/httptest#NewServer>.

## 7. Exercise pointer

Now do **Exercise 3 — `chi` Service and Shutdown**. Wire a `chi` router with a versioned group and a fake service behind an interface, configure `http.Server` timeouts, and implement graceful shutdown driven by `signal.NotifyContext`. The acceptance criterion is that a `SIGTERM` during an in-flight request lets that request finish before the process exits, and `ListenAndServe`'s `ErrServerClosed` is treated as success.

## 8. Summary

- `chi` is `net/http`-native: handlers are `HandlerFunc`, middleware is `func(http.Handler) http.Handler`, a router is an `http.Handler`. No magic.
- `r.Route` nests sub-routers; `r.Group` creates a middleware boundary without a path; `r.Use` registers middleware in onion order; `chi.URLParam(r, "id")` reads path params.
- Per-group middleware (auth on one group, not another) is `chi`'s biggest win over the bare `ServeMux`.
- Construct `http.Server` explicitly for timeouts. `ReadHeaderTimeout` always (slow-loris defence); `ReadTimeout`/`WriteTimeout`/`IdleTimeout`/`MaxHeaderBytes` as budgets.
- Graceful shutdown: `signal.NotifyContext` → on signal, `srv.Shutdown(freshCtxWithDeadline)`; `ListenAndServe` returns `http.ErrServerClosed` on a clean shutdown (success, not error).
- The grace period must exceed your slowest legitimate request and (in K8s) be shorter than `terminationGracePeriodSeconds`. Threading `r.Context()` down lets stuck requests be cancelled during the drain.
- Test handlers with `NewRequest`+`NewRecorder`; test the assembled router with `httptest.NewServer`.

Cited references this lecture pulled from: <https://pkg.go.dev/github.com/go-chi/chi/v5>, <https://pkg.go.dev/net/http#Server>, <https://pkg.go.dev/net/http#Server.Shutdown>, <https://pkg.go.dev/net/http#pkg-variables>, <https://pkg.go.dev/net/http/httptest#NewServer>, <https://pkg.go.dev/os/signal#NotifyContext>.
