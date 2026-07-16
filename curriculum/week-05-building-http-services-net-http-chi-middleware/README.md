# Week 5 — Building HTTP Services: `net/http`, the 1.22 Routing Patterns, the `chi` Router, and Composable Middleware

Welcome to **C30 · Crunch Go**, Week 5, and the start of Phase II. Phase I was the language and the runtime — the concurrency model, `context`, the race detector. This week the work product changes shape: instead of a CLI that runs once and exits, we build a **service** that stays up, accepts requests over the network, and is expected to behave correctly while many requests are in flight at once. Everything you learned about `context` last week now has a home: every request carries a `Context`, and when the client hangs up or the deadline passes, that context cancels the work — the exact machinery from Week 4, applied to a server. The technology is the standard library's `net/http`, the Go 1.22 routing enhancements, and the `chi` router (`github.com/go-chi/chi/v5`) for the ergonomics the standard library still leaves to you. The question carrying the week: *what is the request's path through your process — from the socket, through the middleware chain, into a handler, down to a service and a repository, and back out as a status code and a JSON body — and can you point at each layer?*

The first thing to internalize is that **`net/http` is a real HTTP server, not a toy**. The standard library ships a production-grade HTTP/1.1 and HTTP/2 server in `net/http`: connection handling, TLS, keep-alives, request parsing, the whole stack. The entire surface you build on is two interfaces and a function. `http.Handler` is `interface { ServeHTTP(ResponseWriter, *Request) }`. `http.HandlerFunc` adapts an ordinary `func(w, r)` to that interface. `http.ListenAndServe(addr, handler)` runs the server. That is the whole foundation. A "framework" in Go is not a runtime that owns your process — it is, at most, a router and some middleware helpers sitting on top of the standard library's server. We lean into that: standard library first, a small router second, a framework almost never. The canonical reference is the `net/http` package documentation at <https://pkg.go.dev/net/http>.

The second thing to internalize is the **Go 1.22 routing patterns**. Before Go 1.22, the standard `http.ServeMux` could not route on method or path parameters — `/notes/{id}` and "only POST" required either a third-party router or a pile of manual string parsing. Go 1.22 fixed this: `mux.HandleFunc("POST /notes/{id}", handler)` now routes on method *and* extracts `{id}` via `r.PathValue("id")`. For a large class of services, the standard library's `ServeMux` is now enough on its own. We teach it first, on its own terms, so you understand what a router *does* before reaching for one. Citation: the routing-enhancements section of the Go 1.22 release notes at <https://go.dev/blog/routing-enhancements>.

The third thing to internalize is **why we still reach for `chi`, and what it adds over the 1.22 `ServeMux`**. `chi` is a router that is `net/http`-compatible to its bones — every `chi` handler is an `http.Handler`, every `chi` middleware is a `func(http.Handler) http.Handler` — so it composes with the standard library rather than replacing it. What it buys you over the bare `ServeMux`: sub-routers and route groups (`r.Route("/notes", ...)`), a clean per-group middleware stack, URL-parameter helpers, and a large library of well-tested middleware (request ID, real-IP, logging, recovery, timeout, CORS) in `github.com/go-chi/chi/v5/middleware`. It adds *zero* magic — no reflection-based handler signatures, no struct tags that secretly mean routes. It is the router a senior Go engineer reaches for when the standard `ServeMux` runs out of ergonomics, precisely because it does not hide the request lifecycle. Citation: the `chi` documentation at <https://pkg.go.dev/github.com/go-chi/chi/v5> and the project README at <https://github.com/go-chi/chi>.

The fourth thing to internalize is **middleware as `func(http.Handler) http.Handler`**. The single most important shape in Go web work: a middleware is a function that takes the next handler and returns a new handler that does something *before* and/or *after* calling next. Because the signature is uniform, middleware *composes* — you stack request-ID, then logging, then recovery, then timeout, and each wraps the next like layers of an onion. A request enters the outermost layer, passes inward to the handler, and the response unwinds back out. This is the same composition idea as a Unix pipe, applied to HTTP. Once the shape is in your fingers, you can write any cross-cutting concern — auth, rate limiting, metrics, tracing — as one more layer. Lecture 2 builds the four canonical middlewares from scratch so you never treat them as magic.

The fifth thing to internalize is the **handler → service → repository seam**. A handler's job is narrow: parse and validate the request, call *one* service method, and translate the result (or error) into an HTTP status and a body. It should contain no business logic and no SQL. The **service** owns the business logic and knows nothing about HTTP — it takes plain Go types and a `context`, and returns plain Go types and errors. The **repository** owns persistence behind a small interface, so the service can be tested against an in-memory fake. This three-layer seam is what makes a Go service testable: you test handlers with `httptest` against a fake service, you test the service against a fake repository, and you test the repository against a real database (Week 6). A handler that talks straight to a database is a handler you cannot test without a database. Lecture 1 and Lab 05 build the seam end to end.

The sixth thing to internalize is **request and response JSON, done carefully**. `encoding/json` marshals and unmarshals Go structs to and from JSON. The careful parts: decode with a `json.Decoder` and call `dec.DisallowUnknownFields()` so a typo'd field is a 400, not a silently-ignored value; cap the request body with `http.MaxBytesReader` so a malicious client cannot OOM you with a 2 GB body; always set `Content-Type: application/json` before writing; and define your wire types as explicit structs with `json:"..."` tags rather than serialising your domain types directly, so the API contract is decoupled from the internal model. Lecture 1 covers the decode/encode discipline and the input-validation step that turns a request into a validated command. Citation: <https://pkg.go.dev/encoding/json>.

The seventh thing to internalize is **HTTP status codes and structured error responses as a contract**. A REST API's status codes *are* its error-handling API: `201 Created` with a `Location` header for a successful POST, `200 OK` for a read, `204 No Content` for a successful DELETE, `400 Bad Request` for malformed input, `404 Not Found` for a missing resource, `409 Conflict` for a duplicate, `422 Unprocessable Entity` for input that parsed but failed validation, `500` for a bug. Every error response carries the same JSON envelope — `{"error": {"code": "...", "message": "..."}}` — so a client can program against it. The mapping from a domain error (a typed error from the service) to a status code happens in *one* place, a small `writeError` helper, not scattered across handlers. Lecture 1 builds the status-code map and the error envelope. Citation: the HTTP semantics RFC 9110 at <https://www.rfc-editor.org/rfc/rfc9110>.

The eighth thing to internalize is **`http.Server` configuration and graceful shutdown**. `http.ListenAndServe` is fine for a demo, but a real service constructs an `http.Server` explicitly so it can set `ReadTimeout`, `WriteTimeout`, `IdleTimeout`, and `ReadHeaderTimeout` (without these, a slow-loris client can tie up your server indefinitely), and so it can call `srv.Shutdown(ctx)` for a graceful drain. `Shutdown` stops accepting new connections and waits for in-flight requests to finish, up to the context's deadline — which is, once again, Week 4's `signal.NotifyContext` cancellation applied to the top of the server. This is the seed of Week 11's full Kubernetes graceful shutdown; we plant it here. Citation: <https://pkg.go.dev/net/http#Server.Shutdown>.

The ninth thing to internalize is **testing handlers with `httptest`**. `net/http/httptest` gives you two tools: `httptest.NewRequest` builds a `*http.Request` without a network, and `httptest.NewRecorder` captures what a handler writes (status, headers, body) without a socket. Together they let you test a handler as a pure function: build a request, call `handler.ServeHTTP(rec, req)`, assert on `rec.Code` and `rec.Body`. For an integration test of the whole router, `httptest.NewServer` stands up a real server on a random port that you hit with a real `http.Client`. This is the testing model the whole rest of the track uses; we establish it this week with table-driven handler tests against the clean seam. Citation: <https://pkg.go.dev/net/http/httptest>.

By the end of the week you will be the person on your team who can answer "why did this endpoint return a 500" with the middleware chain, the recovery log line, and the handler-service-repository layer the error came from — not with "let me add some print statements and redeploy."

## Learning objectives

By the end of this week, you will be able to:

- **Build** a `net/http` server from `http.Handler`, `http.HandlerFunc`, and an explicitly-constructed `http.Server` with sane timeouts. Explain why `http.ListenAndServe` is a demo shortcut. Cite <https://pkg.go.dev/net/http>.
- **Route** with the Go 1.22 `ServeMux` patterns — `"POST /notes/{id}"`, `r.PathValue("id")` — and explain what method-and-path routing the standard library gained in 1.22. Cite <https://go.dev/blog/routing-enhancements>.
- **Route** with `chi`: `r.Route`, `r.Group`, sub-routers, `chi.URLParam(r, "id")`, and a per-group middleware stack. Explain what `chi` adds over the bare `ServeMux` and why it adds no magic. Cite <https://pkg.go.dev/github.com/go-chi/chi/v5>.
- **Write** composable middleware as `func(http.Handler) http.Handler`: a request-ID injector, a structured request logger, a panic-recovery wrapper, and a per-request timeout. Stack them and explain the onion model. Cite the `chi/middleware` package.
- **Structure** a service behind the handler → service → repository seam so the handler holds no business logic and the service holds no HTTP. Define the repository as an interface with an in-memory implementation.
- **Decode** JSON safely: `json.Decoder` with `DisallowUnknownFields`, `http.MaxBytesReader` to cap the body, explicit wire structs with `json` tags, and an input-validation step. Cite <https://pkg.go.dev/encoding/json>.
- **Map** domain errors to HTTP status codes in one `writeError` helper, emitting a consistent `{"error": {...}}` envelope. Choose 201/200/204/400/404/409/422/500 correctly. Cite RFC 9110 at <https://www.rfc-editor.org/rfc/rfc9110>.
- **Thread** `r.Context()` into the service and the repository so a client disconnect or a request timeout cancels the work — the Week 4 cancellation model, server-side.
- **Configure** `http.Server` timeouts (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ReadHeaderTimeout`) and implement graceful shutdown with `srv.Shutdown(ctx)` driven by `signal.NotifyContext`. Cite <https://pkg.go.dev/net/http#Server.Shutdown>.
- **Test** handlers with `httptest.NewRequest` + `httptest.NewRecorder` against a fake service, and test the whole router with `httptest.NewServer`. Write table-driven handler tests. Cite <https://pkg.go.dev/net/http/httptest>.
- **Cite** the `net/http`, `encoding/json`, and `net/http/httptest` package docs, the Go 1.22 routing post, the `chi` docs, and RFC 9110 for every technique covered.

## Prerequisites

- **Week 4 of C30 complete.** You thread `context` through a call tree for cancellation and deadlines. Every request handler receives `r.Context()`; the service and repository take it as their first parameter. The graceful-shutdown story is `signal.NotifyContext` from Week 4 applied to `http.Server`.
- **Week 2 of C30 complete.** You design small consumer-defined interfaces and build wrapped-error chains with `errors.Is` / `errors.As`. The repository is an interface; the domain errors map to status codes via `errors.Is`.
- **Week 1 of C30 complete.** You structure a module, write table-driven tests, and run `go vet` / `staticcheck` clean. This week's handler tests are table-driven; the service must stay clean under the toolchain.
- **A working `go version` of `1.22` or newer.** The method-and-path routing in `ServeMux` and `r.PathValue` require Go 1.22. Earlier Go cannot run the standard-library routing examples.
- **`github.com/go-chi/chi/v5` available.** Add it with `go get github.com/go-chi/chi/v5`. MIT-licensed, zero transitive dependencies beyond the standard library.
- **`curl` and (optionally) `httpie` or `jq`.** For poking the running service from the command line and reading the JSON responses.

## Topics covered

- **The `net/http` server.** `http.Handler`, `http.HandlerFunc`, `http.ResponseWriter`, `*http.Request`, `http.ListenAndServe`, the explicit `http.Server` struct and its timeout fields.
- **Routing.** The Go 1.22 `ServeMux` (`"METHOD /path/{param}"`, `r.PathValue`, the precedence rules, the `{$}` end-of-path wildcard); `chi`'s `Router`, `Route`, `Group`, `Mount`, `chi.URLParam`, and route-group middleware.
- **Middleware.** The `func(http.Handler) http.Handler` shape; the onion model; the standard four (request ID, logging, recovery, timeout); `chi/middleware` (`RequestID`, `RealIP`, `Logger`, `Recoverer`, `Timeout`); writing your own.
- **The handler/service/repository seam.** Handler responsibilities (parse, validate, call one service method, translate result/error); the service as HTTP-ignorant business logic; the repository interface and its in-memory implementation.
- **JSON I/O.** `json.Marshal`/`Unmarshal`, `json.Encoder`/`Decoder`, `DisallowUnknownFields`, `http.MaxBytesReader`, `omitempty`, wire structs vs domain types, the encode/decode helpers.
- **Input validation.** Validating decoded input, returning 422 with field-level errors, the "parse, don't validate" idea applied to request bodies.
- **Status codes and error envelopes.** 200/201/204/400/404/409/422/500; the `Location` header on create; the single `writeError` helper; mapping typed domain errors to codes with `errors.Is`.
- **Cancellation server-side.** `r.Context()`, threading it into the service and repository, what happens on client disconnect, `http.MaxBytesReader` and the body lifecycle.
- **`http.Server` configuration and graceful shutdown.** The timeout fields, `srv.Shutdown(ctx)`, the drain semantics, `signal.NotifyContext` driving the shutdown, the `ErrServerClosed` sentinel.
- **Testing.** `httptest.NewRequest`, `httptest.NewRecorder`, `httptest.NewServer`, table-driven handler tests, testing the middleware chain, testing graceful shutdown.
- **The worked example: a layered `notes` REST API.** A `notes` service with the full CRUD surface, an in-memory repository behind an interface, the four middlewares, JSON I/O with validation, the status-code map, and `httptest` handler tests. The numbers (request latency, the middleware overhead) are in `mini-project/PERF.md`.

## Weekly schedule

The schedule adds up to approximately **36 hours**. Treat it as a target, not a contract. The middleware-composition and error-mapping material rewards an unhurried mind; do not ship a handler that talks straight to a database "just for now."

| Day       | Focus                                                                | Lectures | Exercises | Challenges | Quiz/Read | Homework | Mini-Project | Self-Study | Daily Total |
|-----------|----------------------------------------------------------------------|---------:|----------:|-----------:|----------:|---------:|-------------:|-----------:|------------:|
| Monday    | `net/http`, 1.22 routing, the handler/service/repo seam, JSON I/O    |    2h    |    1.5h   |     0h     |    0.5h   |   1h     |     0h       |    1h      |     6h      |
| Tuesday   | Middleware as `func(http.Handler) http.Handler`, the standard four   |    2h    |    1.5h   |     0h     |    0.5h   |   1h     |     0h       |    1h      |     6h      |
| Wednesday | `chi` router, groups, status codes, error envelopes, graceful drain  |    2h    |    1.5h   |     0h     |    0.5h   |   1h     |     0h       |    1h      |     6h      |
| Thursday  | Challenges, `httptest` deep-dive, validation, server timeouts        |    0.5h  |    0h     |     2h     |    0.5h   |   1h     |     2h       |    0.5h    |     6.5h    |
| Friday    | Mini-project — build the layered `notes-api` with middleware + tests |    0h    |    0h     |     1h     |    0.5h   |   1h     |     3h       |    0.5h    |     6h      |
| Saturday  | Mini-project polish, graceful shutdown, perf write-up                |    0h    |    0h     |     0h     |    0h     |   0h     |     2.5h     |    0h      |     2.5h    |
| Sunday    | Quiz, review, design exercise: "what would you add to the chain"     |    0h    |    0h     |     0h     |    1h     |   0h     |     2h       |    0h      |     3h      |
| **Total** |                                                                      | **6.5h** | **4.5h**  | **3h**     | **3.5h**  | **5h**   | **9.5h**     | **4h**     | **36h**     |

## How to navigate this week

| File | What's inside |
|------|---------------|
| [README.md](./README.md) | This overview (you are here) |
| [resources.md](./resources.md) | The `net/http`, `encoding/json`, and `httptest` docs; the Go 1.22 routing post; the `chi` docs and source; RFC 9110 |
| [lecture-notes/01-net-http-routing-and-the-handler-service-repository-seam.md](./lecture-notes/01-net-http-routing-and-the-handler-service-repository-seam.md) | `net/http` server, 1.22 routing, the three-layer seam, JSON I/O with validation, the status-code map and error envelope |
| [lecture-notes/02-middleware-the-onion-model-and-the-standard-four.md](./lecture-notes/02-middleware-the-onion-model-and-the-standard-four.md) | Middleware as `func(http.Handler) http.Handler`, the onion model, building request-ID / logging / recovery / timeout from scratch |
| [lecture-notes/03-chi-router-groups-server-config-and-graceful-shutdown.md](./lecture-notes/03-chi-router-groups-server-config-and-graceful-shutdown.md) | The `chi` router, route groups and sub-routers, `http.Server` timeouts, graceful shutdown with `Shutdown(ctx)`, `httptest` testing |
| [exercises/exercise-01-router-and-json.go](./exercises/exercise-01-router-and-json.go) | Build a 1.22-routed `ServeMux` with JSON encode/decode helpers and the status-code map; test with `httptest` |
| [exercises/exercise-02-middleware-chain.go](./exercises/exercise-02-middleware-chain.go) | Write the four standard middlewares from scratch, stack them, and prove the onion order with a test that inspects the request-ID and log output |
| [exercises/exercise-03-chi-service-and-shutdown.go](./exercises/exercise-03-chi-service-and-shutdown.go) | Wire `chi` with route groups, a fake service behind an interface, and graceful shutdown driven by `signal.NotifyContext` |
| [exercises/SOLUTIONS.md](./exercises/SOLUTIONS.md) | Annotated solutions for the three exercises, with the exact `curl` calls and responses you should reproduce |
| [challenges/challenge-01-content-negotiation-and-validation.md](./challenges/challenge-01-content-negotiation-and-validation.md) | Add request validation with field-level 422 errors and content negotiation (JSON now, a second format later) behind the same handlers |
| [challenges/challenge-02-graceful-shutdown-under-load.md](./challenges/challenge-02-graceful-shutdown-under-load.md) | Prove graceful shutdown drains in-flight requests: drive load, send SIGTERM, and show zero dropped requests within the grace period |
| [quiz.md](./quiz.md) | 10 multiple-choice questions on `net/http`, routing, middleware, the seam, JSON, status codes, and shutdown |
| [homework.md](./homework.md) | Six practice problems for the week |
| [mini-project/README.md](./mini-project/README.md) | Full spec for `notes-api` — the layered REST service with middleware, validation, the status-code map, and `httptest` tests |

## The "go vet clean" promise — restated, and a new status-code promise

C30 treats a clean toolchain run as a contract:

```
$ go vet ./... && staticcheck ./... && go test -race ./...
ok  	github.com/you/notes-api	0.512s
```

For Week 5 we add a status-code contract: **every response your service can return has an intentional, documented status code, and the mapping from a domain error to that code lives in exactly one place.** A handler that returns a bare `500` for a not-found, or a `200` for a validation failure, is a handler that has not been reviewed. The status code is the API; treat it as deliberately as you treat the JSON body.

We add a seam contract too: **no handler in your service contains business logic or talks to a data store directly.** A handler parses, validates, calls one service method, and translates the result. The most common Go-service mistake is a handler that grew a database query and a business rule until it could no longer be tested without a database. The rule, restated from Week 2's interface lecture: define the seam at the consumer, and keep each layer ignorant of the layers it does not need to know about.

> **Note on packages.** The server, JSON, and testing are all standard library: `net/http`, `encoding/json`, `net/http/httptest`, `log/slog`, `context`, `os/signal`. The router is `github.com/go-chi/chi/v5` (MIT, no transitive dependencies) with its middleware in `github.com/go-chi/chi/v5/middleware`. Add it with `go get github.com/go-chi/chi/v5@latest`. The Go 1.22 routing patterns are in-box — no dependency at all. All free, all open source.
