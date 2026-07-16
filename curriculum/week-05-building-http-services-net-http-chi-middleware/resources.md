# Week 5 — Resources

Every resource on this page is **free**. The Go package docs on `pkg.go.dev` are free and account-free. The Go blog and specification on `go.dev` are free. `chi` is MIT-licensed. RFC 9110 is a public IETF standard. No paywalled material is linked.

## Required reading (work it into your week)

### `net/http` and the server

- **`net/http` package documentation** — `Handler`, `HandlerFunc`, `ResponseWriter`, `Request`, `Server`, `ListenAndServe`, `Shutdown`:
  <https://pkg.go.dev/net/http>
- **`http.Server` reference** — the timeout fields (`ReadTimeout`, `WriteTimeout`, `IdleTimeout`, `ReadHeaderTimeout`), `Shutdown`, `Close`:
  <https://pkg.go.dev/net/http#Server>
- **`Server.Shutdown` reference** — the graceful-drain semantics and the relationship to `ErrServerClosed`:
  <https://pkg.go.dev/net/http#Server.Shutdown>
- **The complete guide to Go net/http timeouts (Filippo Valsorda)** — the canonical explanation of which timeout defends against which failure mode:
  <https://blog.cloudflare.com/the-complete-guide-to-golang-net-http-timeouts/>

### Routing

- **Routing Enhancements for Go 1.22 (the Go blog)** — method-and-path patterns, `PathValue`, precedence, the `{$}` anchor:
  <https://go.dev/blog/routing-enhancements>
- **`http.ServeMux` reference** — the standard-library router, the pattern syntax, the conflict rules:
  <https://pkg.go.dev/net/http#ServeMux>
- **`chi` router documentation** — `Router`, `Route`, `Group`, `Mount`, `Use`, `URLParam`:
  <https://pkg.go.dev/github.com/go-chi/chi/v5>
- **The `chi` project README** — the design philosophy ("`net/http`-compatible, no magic") and routing examples:
  <https://github.com/go-chi/chi>

### Middleware

- **`chi/middleware` package** — the well-tested standard middlewares (`RequestID`, `RealIP`, `Logger`, `Recoverer`, `Timeout`, `Compress`):
  <https://pkg.go.dev/github.com/go-chi/chi/v5/middleware>
- **`log/slog` package** — the standard structured logger; handlers, levels, attributes, `LogAttrs`:
  <https://pkg.go.dev/log/slog>
- **Structured Logging with slog (the Go blog)** — the guide to `slog` design and usage:
  <https://go.dev/blog/slog>
- **`net/http.TimeoutHandler`** — the standard-library response-timeout wrapper, and how it differs from a context timeout:
  <https://pkg.go.dev/net/http#TimeoutHandler>

### JSON I/O

- **`encoding/json` package** — `Marshal`, `Unmarshal`, `Encoder`, `Decoder`, struct tags, `omitempty`:
  <https://pkg.go.dev/encoding/json>
- **`json.Decoder.DisallowUnknownFields`** — reject unexpected fields instead of silently dropping them:
  <https://pkg.go.dev/encoding/json#Decoder.DisallowUnknownFields>
- **`http.MaxBytesReader`** — cap a request body to defend against memory exhaustion:
  <https://pkg.go.dev/net/http#MaxBytesReader>
- **JSON and Go (the Go blog)** — the foundational article on marshalling and unmarshalling in Go:
  <https://go.dev/blog/json>

### HTTP semantics

- **RFC 9110 — HTTP Semantics** — the authoritative reference for methods, status codes, headers, and content negotiation:
  <https://www.rfc-editor.org/rfc/rfc9110>
- **RFC 9110 §15 — Status Codes** — the definitive list and the meaning of each:
  <https://www.rfc-editor.org/rfc/rfc9110#name-status-codes>
- **RFC 9110 §12 — Content Negotiation** — `Accept`, quality values, the negotiation algorithm (Challenge 1):
  <https://www.rfc-editor.org/rfc/rfc9110#name-content-negotiation>

### Testing

- **`net/http/httptest` package** — `NewRequest`, `NewRecorder`, `NewServer`, `NewTLSServer`:
  <https://pkg.go.dev/net/http/httptest>
- **`testing` package** — table-driven tests, subtests with `t.Run`, `t.Parallel`:
  <https://pkg.go.dev/testing>

### Cancellation and signals (recap from Week 4)

- **`os/signal.NotifyContext`** — turn `SIGINT`/`SIGTERM` into a context cancellation for graceful shutdown:
  <https://pkg.go.dev/os/signal#NotifyContext>
- **`context` package** — `WithTimeout`, `WithValue`, the request-scoped-value convention:
  <https://pkg.go.dev/context>

## Recommended reading (after the required set)

- **Effective Go** — the idiom guide; the relevant sections on interfaces, errors, and `defer`:
  <https://go.dev/doc/effective_go>
- **How I write HTTP services in Go (Mat Ryer)** — a widely-cited essay on structuring a Go HTTP service; the `NewServer` constructor pattern and the handler-returns-handler trick:
  <https://grafana.com/blog/2024/02/09/how-i-write-http-services-in-go-after-13-years/>
- **`golang.org/x/time/rate`** — the standard token-bucket rate limiter (homework Problem 3):
  <https://pkg.go.dev/golang.org/x/time/rate>
- **The `chi` source** — read `mux.go` once; it is small and shows exactly how routing and middleware composition work with no reflection:
  <https://github.com/go-chi/chi/blob/master/mux.go>
- **REST API design — status code best practices (the RFC 9110 status-code registry)** — the authoritative registry of every code:
  <https://www.iana.org/assignments/http-status-codes/http-status-codes.xhtml>

## Tools you will install this week

- **`github.com/go-chi/chi/v5`** — added per-module: `go get github.com/go-chi/chi/v5@latest`. MIT-licensed, no transitive dependencies beyond the standard library.
- **`curl`** — almost certainly already installed; the workhorse for poking the running service. Use `-i` to see the status line and headers.
- **`jq`** (optional) — pretty-print and query JSON responses: `curl -s localhost:8080/v1/notes | jq`.
- **`staticcheck`** — `go install honnef.co/go/tools/cmd/staticcheck@latest`. The lint the mini-project must pass clean.

## Citations policy

This curriculum cites the Go package documentation on `pkg.go.dev`, the Go blog on `go.dev`, RFC 9110 for HTTP semantics, and the `chi` docs and source as the primary references. Every example in the lecture notes and exercises is traced back to one of these. When a third-party essay (Filippo Valsorda on timeouts, Mat Ryer on service structure) is the clearer reference, it is cited explicitly with a URL — never paraphrased without attribution. If a citation is missing from a section of these notes, treat it as a bug and open an issue against the C30 curriculum repository.
