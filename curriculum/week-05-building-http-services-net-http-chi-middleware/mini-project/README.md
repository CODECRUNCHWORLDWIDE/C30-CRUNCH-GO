# Mini-Project — `notes-api`: A Layered REST Service with Middleware, Validation, the Status-Code Map, and `httptest` Tests

> **Time:** ~9.5 hours across Friday-Saturday-Sunday. **Prerequisites:** Exercises 1-3 and ideally both challenges. **Citations:** every package doc referenced in the three lecture notes, plus RFC 9110.

## The spec

You are building **`notes-api`** (Lab 05), the REST service the rest of Phase II evolves: Week 6 swaps its in-memory repository for Postgres, Week 7 adds a gRPC twin over the same service layer, Week 8 hardens it with benchmarks and fuzzing. This week it is REST-only, in-memory, but built to the layered standard those later weeks depend on.

```
   HTTP client
        |
        v
   +-------------------------------------------+
   |  chi router                               |
   |   RequestID -> RealIP -> Logger(slog) ->  |  middleware (onion)
   |   Recoverer -> Timeout                    |
   +-------------------+-----------------------+
                       |
                       v
   +-------------------+-----------------------+
   |  Handler   (parse, validate, translate)   |  HTTP only
   +-------------------+-----------------------+
                       |
                       v
   +-------------------+-----------------------+
   |  Service   (business logic, ctx-threaded) |  HTTP-free
   +-------------------+-----------------------+
                       |
                       v
   +-------------------+-----------------------+
   |  Repository (interface)                   |  persistence
   |   MemRepo (this week) | PgRepo (Week 6)   |
   +-------------------------------------------+
```

The CRUD surface:

```
POST   /v1/notes          -> 201 + Location, the created note
GET    /v1/notes          -> 200, the list (paginated)
GET    /v1/notes/{id}     -> 200 the note | 404
PATCH  /v1/notes/{id}     -> 200 the updated note | 404 | 422
DELETE /v1/notes/{id}     -> 204 | 404
GET    /healthz           -> 200 "ok"   (outside /v1)
```

## Functional requirements

### F1 — Routing

- `chi` router with a `/v1` sub-router and a `/v1/notes` group; `/healthz` outside `/v1`.
- URL params via `chi.URLParam(r, "id")`. Method-specific registration (`r.Post`, `r.Get`, ...).
- A wrong method on a valid path returns 405; an unknown path returns 404.

### F2 — The seam

- Three layers: `Handler` (HTTP), `Service` (logic, ctx-threaded, HTTP-free), `Repository` (interface).
- The repository is an interface with a `MemRepo` (mutex-guarded map) implementation.
- The handler holds no business logic; the service imports no `net/http`; `git grep "net/http"` in the service package returns nothing.

### F3 — JSON I/O

- Decode with `http.MaxBytesReader` + `json.Decoder` + `DisallowUnknownFields` + the `dec.More()` trailing-garbage check.
- Encode through a `writeJSON(w, status, v)` helper; content type then status then body.
- Wire structs (`createRequest`, `noteResponse`) decoupled from the domain `Note`, with explicit `json` tags.

### F4 — Validation

- A create/update request that fails a rule returns 422 with field-level errors (collect *all* violations, not the first).
- A malformed body (bad JSON, unknown field, too large) returns 400.
- Validation lives in the handler layer, not the service.

### F5 — Status codes and errors

- One `writeError(w, err)` maps domain errors (`ErrNotFound`→404, `ErrConflict`→409, validation→422, bad request→400, else→500) via `errors.Is`.
- 201 with a `Location` header on create; 204 (no body) on delete.
- A 500 logs the real error at `slog.LevelError` and returns a generic message — never leak internals.

### F6 — Middleware

- The chain: `RequestID` → `RealIP` → a custom `slog` logger (one structured line per request, with `request_id`, `method`, `path`, `status`, `latency`) → `Recoverer` → `Timeout`.
- A panic in any handler becomes a logged 500; the chain does not crash.

### F7 — Server and shutdown

- `http.Server` constructed explicitly with `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`.
- Graceful shutdown via `signal.NotifyContext` + `srv.Shutdown(freshCtxWithDeadline)`; `http.ErrServerClosed` treated as success.
- `r.Context()` threaded into the service so client disconnect and shutdown cancel in-flight work.

### F8 — Pagination

- `GET /v1/notes?limit=20&cursor=<id>` returns at most `limit` notes and a `next_cursor` for the next page. Cap `limit` at 100; default 20.

## Non-functional requirements

### NF1 — Tests

- Table-driven handler tests with `httptest.NewRequest` + `httptest.NewRecorder` against a fake or in-memory service.
- One end-to-end test with `httptest.NewServer` exercising the full router + middleware chain.
- A test that a panic becomes a 500; a test that graceful shutdown drains an in-flight request.
- `go test -race ./...` green.

### NF2 — Code quality

- File-scoped, small functions; the three-layer seam in separate packages or clearly-separated files.
- Every service/repository method takes `ctx` first. No `http.NewRequest` (context-less) anywhere.
- `go vet ./...` and `staticcheck ./...` clean.

### NF3 — Citations

- Every non-obvious choice has a one-line comment citing the relevant package doc, RFC 9110, or the routing post.
- `README.md` lists `github.com/go-chi/chi/v5` with its version and license; everything else is standard library.

## Suggested project layout

```
notes-api/
├── go.mod
├── README.md            <-- build, run, the routes, example curls
├── PERF.md              <-- the latency + middleware-overhead write-up
├── cmd/notesapi/
│   └── main.go          <-- flag parsing, server construction, shutdown wiring
└── internal/
    ├── notes/           <-- domain + service + repository interface + MemRepo
    │   ├── note.go
    │   ├── service.go
    │   ├── service_test.go
    │   ├── repo.go
    │   └── memrepo.go
    ├── http/            <-- handlers, router, JSON helpers, error map
    │   ├── handler.go
    │   ├── handler_test.go
    │   ├── router.go
    │   ├── json.go
    │   └── errors.go
    └── middleware/      <-- RequestID, Logger, Recoverer (or use chi's), Timeout
        ├── middleware.go
        └── middleware_test.go
```

## Starter

A starter scaffold is provided in `mini-project/starter/`. Copy it as your starting point:

- `cmd/notesapi/main.go` — the server construction and graceful-shutdown wiring, complete.
- `internal/notes/repo.go` — the `Repository` interface and the domain `Note`, complete.
- `internal/http/router.go` — the `chi` router with the route group, complete; the handlers are stubs you fill in.
- `internal/middleware/middleware.go` — the request-ID and timeout middlewares, complete; `Logger` and `Recoverer` are stubs.

The starter compiles and serves `/healthz`; the `notes` handlers return `501 Not Implemented` until you fill them in.

## The perf write-up (`PERF.md`)

Treat it as part of the deliverable.

### M1 — request latency

Benchmark a `GET /v1/notes/{id}` against an `httptest.Server` with the full middleware chain, and again with *no* middleware. Report the per-request latency for each and the middleware overhead (ns/request). One sentence: is the overhead acceptable for your use? (For reference, a slog log line plus the request-ID and timeout middlewares typically add single-digit microseconds.)

### M2 — body-size guard

Send a 2 MiB body to `POST /v1/notes` (the `MaxBytesReader` cap is 1 MiB). Confirm a 400 and that the process memory does not balloon. Report what happens with the cap removed (do not ship that; just observe).

### M3 — validation

Send a create request violating two rules. Confirm a 422 with both field errors in the body. Paste the response JSON.

### M4 — graceful shutdown

Drive 20 concurrent clients against the slow path, send `SIGTERM`, and report how many in-flight requests drained vs dropped, and the drain time. (Reuse Challenge 2's harness.)

### M5 — the request-log line

Paste one `slog` request-log line and annotate every field: `request_id`, `method`, `path`, `status`, `latency`. Confirm the `request_id` matches the `X-Request-Id` response header.

## Grading rubric

- **35 points: functional correctness.** F1-F8 implemented and demonstrable; the full CRUD + pagination + health surface works with correct status codes.
- **20 points: the seam.** Three layers, the service HTTP-free (verified by `git grep`), the repository an interface; handlers thin.
- **15 points: middleware + shutdown.** The chain works in onion order; a panic becomes a 500; graceful shutdown drains in-flight work; `ErrServerClosed` handled as success.
- **15 points: the perf write-up.** All five measurements (M1-M5) with real numbers and one-sentence interpretations.
- **10 points: tests.** Table-driven handler tests, an `httptest.NewServer` end-to-end test, a panic-to-500 test, `go test -race` green.
- **5 points: citations.** At least eight distinct citations in the source pointing at Go package docs or RFC 9110.

## Stretch goals

1. **`If-Match` / optimistic concurrency.** Add an `ETag` to each note (a hash of its content) and require `If-Match` on `PATCH`; return 412 (Precondition Failed) on a mismatch. Discuss why this prevents the lost-update problem (a preview of Week 6's transaction hazards).
2. **A `/metrics`-shaped counter middleware.** Add a middleware that counts requests by method and status into an `atomic.Int64` map (Week 4) and exposes them at `/metrics` as plain text. This is the seed of Week 9's Prometheus RED metrics.
3. **Structured 404 for unknown routes.** Override `chi`'s `NotFound` and `MethodNotAllowed` handlers to emit the same `{"error":{...}}` envelope as the rest of the API, so a client gets a consistent error shape everywhere.

## Submission

Push the project on a branch named `week05-mini-project/<your-handle>` and open a PR against the C30 curriculum repository. The PR description must link to `PERF.md` and paste the green `go test -race ./...` line and one example `curl` showing a 201 with a `Location` header.

The teaching staff reviews mini-project PRs within 7 business days. Reviews focus on: (a) whether the eight functional requirements are met, (b) whether the seam genuinely holds (the service must be HTTP-free), (c) whether middleware and shutdown are correct, and (d) whether the perf write-up has real measurements.

Cited references: every page referenced in the three lecture notes, plus <https://www.rfc-editor.org/rfc/rfc9110>, <https://pkg.go.dev/github.com/go-chi/chi/v5>, <https://pkg.go.dev/net/http#Server.Shutdown>, <https://pkg.go.dev/net/http/httptest>, <https://pkg.go.dev/log/slog>.
