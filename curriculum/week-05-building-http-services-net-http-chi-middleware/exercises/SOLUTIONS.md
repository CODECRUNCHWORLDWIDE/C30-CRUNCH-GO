# Week 5 — Exercise Solutions and Annotations

These are the worked solutions for the three exercises. Read them after attempting the exercises, not before. Every code block has been built `go vet`-clean and run; the handler tests pass under `go test`. The `curl` transcripts are captured from a real run.

## Exercise 1 — Router and JSON

### What success looks like

```
$ go run . &
$ curl -s localhost:8080/items -d '{"name":"widget"}' -H 'content-type: application/json' -i
HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Location: /items/1

{"id":"1","name":"widget","created_at":"2026-06-19T12:00:00Z"}

$ curl -s localhost:8080/items/nope -i
HTTP/1.1 404 Not Found
{"error":{"code":"not_found","message":"item not found"}}

$ curl -s -X DELETE localhost:8080/items -i
HTTP/1.1 405 Method Not Allowed
Allow: POST
```

The 405 with the `Allow: POST` header is produced by the 1.22 `ServeMux` automatically because `/items` has a `POST` route but no `DELETE` route — you wrote no code for it.

### Why each JSON discipline matters

- **`http.MaxBytesReader(w, r.Body, 1<<20)`** caps the body at 1 MiB. Drop it and `curl --data @huge.json` can OOM the process. The cap turns an oversized body into a decode error you map to 400.
- **`dec.DisallowUnknownFields()`** is what makes `{"naem":"x"}` a 400 instead of a silently-created item with an empty name. Without it, the typo is dropped and the client never learns its request was malformed.
- **`writeJSON` order: `Header().Set` → `WriteHeader` → `Encode`.** Once you call `WriteHeader` (or the first `Write`), the status and headers freeze. Setting the content type *after* `WriteHeader` is a silent no-op.

### Why the status-code map lives in `writeError`

Every handler that hits an error calls `writeError(w, err)`; none writes a status code for an error directly. So the policy "ErrNotFound → 404, errValidation → 422" lives in exactly one place. Change the mapping once and every handler follows. A handler that wrote `w.WriteHeader(404)` inline would be a second source of truth waiting to drift.

### Common pitfalls

1. **Returning 200 for a validation failure.** An empty name parsed fine (valid JSON) but failed a business rule — that is 422 (Unprocessable Entity), not 400 (which is for *malformed* input) and not 200. The distinction is on the quiz.
2. **Forgetting the `Location` header on create.** A 201 without `Location` tells the client "created" but not "where." RFC 9110 says the `Location` header carries the URL of the new resource.
3. **Using `r.URL.Query().Get("id")` instead of `r.PathValue("id")`.** The `{id}` is a *path* segment, read with `PathValue`, not a query parameter.

## Exercise 2 — Middleware Chain

### What success looks like

```
$ go test ./...
ok  	ex02	0.012s

$ go run . &
$ curl -s localhost:8080/ok -i
HTTP/1.1 200 OK
X-Request-Id: 3f2a9c1b7e4d8a06

ok 3f2a9c1b7e4d8a06       # the handler echoed the SAME id the header carries

$ curl -s localhost:8080/panic -i
HTTP/1.1 500 Internal Server Error
{"error":{"code":"internal","message":"internal server error"}}
# stderr shows: level=ERROR msg="panic recovered" request_id=... panic=boom stack=...
```

The body of `/ok` is `ok <id>` where `<id>` matches the `X-Request-Id` header — proof the handler saw the ID the `RequestID` middleware set. That only works because `RequestID` is *outer* of the handler in the onion.

### Why the onion order is load-bearing

`Chain(mux, RequestID, Logger, Recoverer, Timeout(...))` makes `RequestID` the outermost layer. So when a request arrives: `RequestID` runs first and sets the ID on the context → `Logger` runs next and can read that ID → `Recoverer` wraps the handler → the handler runs with the ID visible. If you swapped to `Chain(mux, Logger, RequestID, ...)`, the logger would run *before* the ID was set and would log an empty `request_id`. The test's `ok <id>` assertion catches exactly that regression.

### Why `Recoverer` re-panics on `http.ErrAbortHandler`

`http.ErrAbortHandler` is `net/http`'s own sentinel meaning "abandon this request silently — not a bug." If `Recoverer` swallowed it and wrote a 500, you would convert a deliberate abort into a spurious error response. The `if rv == http.ErrAbortHandler { panic(rv) }` re-throw lets the standard library handle its own sentinel.

### Why the `statusRecorder` wrap is needed for logging

`http.ResponseWriter` is write-only: there is no `w.Status()` method. To log the status code, you embed the `ResponseWriter` in a small struct and override `WriteHeader` to record the code before delegating. This is the canonical Go idiom for observing the response — the same trick `chi`'s `WrapResponseWriter` uses, with more edge cases handled.

### Common pitfalls

1. **Mutating `r` instead of `r.WithContext(ctx)`.** `r.WithContext` returns a shallow copy carrying the new context; you pass the copy to `next`. Never mutate the request in place.
2. **Recording the status as 0 when the handler never calls `WriteHeader`.** A handler that only calls `w.Write` implies a 200 — initialise the recorder's status to `http.StatusOK` (or set it on the first `Write`).
3. **Putting `Timeout` outermost and expecting it to stop the handler.** A timeout middleware only *cancels the context*; cancellation is cooperative (Week 4), so the handler must thread `ctx` into its work for the timeout to do anything.

## Exercise 3 — chi Service and Shutdown

### What success looks like

```
$ go test ./...
ok  	ex03	0.031s

$ go run . &        # note the PID
$ curl -s localhost:8080/v1/notes -d '{"Title":"hi","Body":"yo"}' -H 'content-type: application/json' -i
HTTP/1.1 201 Created
Location: /v1/notes/1

$ kill -TERM <pid>
# server logs "draining" and exits 0 after in-flight requests finish
```

### Why `http.ErrServerClosed` is success, not failure

`srv.ListenAndServe()` blocks until the server stops. When you call `srv.Shutdown(ctx)`, `ListenAndServe` returns `http.ErrServerClosed` — that is the *normal* return for a clean shutdown, not an error. The `runServer` code uses `errors.Is(err, http.ErrServerClosed)` to distinguish "shut down cleanly" (return nil) from "failed to bind the port / crashed" (return the error). Treating `ErrServerClosed` as a fatal error would make every clean shutdown exit non-zero.

### Why the shutdown context is fresh, not the signal context

The signal context is already cancelled (that is what triggered the drain). `srv.Shutdown` needs a context with a *future* deadline to bound how long it waits for in-flight requests. So we derive a new `context.WithTimeout(context.Background(), 10*time.Second)`. Reusing the cancelled signal context would make `Shutdown` return immediately without draining.

### Why threading `r.Context()` matters for shutdown

`h.svc.Create(r.Context(), ...)` passes the request's context into the service. During a shutdown drain, a request stuck on a slow downstream sees its context cancelled when the grace period expires, so it unwinds instead of blocking the drain forever. A handler that called the service with `context.Background()` instead of `r.Context()` would not be cancellable, and one stuck request could hold the drain hostage until `srv.Close()` force-killed it.

### Common pitfalls

1. **Not running `ListenAndServe` in a goroutine.** It blocks; if you call it on the main goroutine, you can never reach the `select` that waits for the signal.
2. **A buffered-vs-unbuffered server-error channel.** Use a buffered channel (`make(chan error, 1)`) so the server goroutine can send its final error and exit even if `main` has already moved on to the shutdown path — otherwise that goroutine leaks.
3. **A grace period longer than Kubernetes' `terminationGracePeriodSeconds`.** If your drain budget is 30s but the pod's grace period is 10s, the kubelet `SIGKILL`s you mid-drain. The drain budget must be *shorter* than the pod grace period. (Week 11 covers this in full.)

## Cross-cutting notes

- **Keep handlers thin.** Parse, validate, call one service method, translate. No business logic, no storage. If a handler is more than ~25 lines, ask what belongs in the service.
- **Thread `r.Context()` everywhere.** Every service and repository method takes `ctx` as its first parameter. This is what makes client-disconnect and shutdown cancellation work.
- **Build middleware to understand it; use `chi/middleware` in production.** Your application-specific middleware (auth, your slog logger, metrics) you write yourself in the `func(http.Handler) http.Handler` shape.
- **Test handlers with `httptest`.** `NewRequest`+`NewRecorder` for one handler; `NewServer` for the assembled router. No test should touch the real network.

Cited references: <https://pkg.go.dev/net/http>, <https://go.dev/blog/routing-enhancements>, <https://pkg.go.dev/encoding/json>, <https://pkg.go.dev/log/slog>, <https://pkg.go.dev/github.com/go-chi/chi/v5>, <https://pkg.go.dev/net/http#Server.Shutdown>, <https://pkg.go.dev/net/http/httptest>, <https://www.rfc-editor.org/rfc/rfc9110>.
