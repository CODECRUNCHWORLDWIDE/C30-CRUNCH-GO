# Week 4 — Resources

Every resource on this page is **free**. The Go package documentation on `pkg.go.dev` is free and account-free. The Go blog and the Go specification at `go.dev` are free. The `golang.org/x/sync` and `golang.org/x/perf` modules are BSD-3-Clause licensed. The GopherCon talks are free on YouTube. No paywalled material is linked.

## Required reading (work it into your week)

### `context`

- **`context` package documentation** — the interface, the constructors, the `Done()` / `Err()` / `Deadline()` / `Value()` methods:
  <https://pkg.go.dev/context>
- **Go Concurrency Patterns: Context** — the canonical blog post introducing `context`, the cancellation tree, and the "pass ctx as the first parameter" convention:
  <https://go.dev/blog/context>
- **`context.WithValue` guidance** — why values are for request-scoped metadata only, and the unexported-key-type convention:
  <https://pkg.go.dev/context#WithValue>

### Worker pools and pipelines

- **`golang.org/x/sync/errgroup`** — `Group`, `WithContext`, `Go`, `Wait`, `SetLimit`; the first-error-wins and sibling-cancellation semantics:
  <https://pkg.go.dev/golang.org/x/sync/errgroup>
- **Go Concurrency Patterns: Pipelines and cancellation** — fan-out / fan-in, bounded concurrency, the semaphore pattern, and explicit cancellation:
  <https://go.dev/blog/pipelines>
- **`golang.org/x/sync/semaphore`** — a weighted counting semaphore for when the buffered-channel pattern needs weights:
  <https://pkg.go.dev/golang.org/x/sync/semaphore>

### The `sync` primitives and atomics

- **`sync` package documentation** — `Mutex`, `RWMutex`, `Once`, `WaitGroup`, `Pool`, the `OnceFunc`/`OnceValue` helpers:
  <https://pkg.go.dev/sync>
- **`sync/atomic` package documentation** — the typed atomics (`Int64`, `Bool`, `Pointer[T]`), `Add`/`Load`/`Store`/`Swap`/`CompareAndSwap`:
  <https://pkg.go.dev/sync/atomic>
- **Rethinking Classical Concurrency Patterns (Bryan Mills, GopherCon 2018)** — the definitive "channel vs mutex, and the times the slogan misleads" talk:
  <https://www.youtube.com/watch?v=5zXAHh5tJqQ>
- **The "atomic maps" FAQ entry** — why a plain `map` is not safe for concurrent access and what to do instead:
  <https://go.dev/doc/faq#atomic_maps>

### The memory model and the race detector

- **The Go Memory Model** — the specification: happens-before, the synchronisation edges, why racy programs are undefined:
  <https://go.dev/ref/mem>
- **Introducing the Go Race Detector** — the blog post explaining what ThreadSanitizer instruments and how to use `-race`:
  <https://go.dev/blog/race-detector>
- **Data Race Detector** — the reference article: usage, the report format, the no-false-positives / has-false-negatives property, the CI recipe:
  <https://go.dev/doc/articles/race_detector>

### Signals and graceful shutdown

- **`os/signal` package, `NotifyContext`** — turn `SIGINT`/`SIGTERM` into context cancellation:
  <https://pkg.go.dev/os/signal#NotifyContext>
- **`os/signal` package overview** — the signal model, `Notify`, `Stop`, the default behaviour:
  <https://pkg.go.dev/os/signal>

### Benchmarking and first `pprof`

- **`testing` package — Benchmarks** — `testing.B`, `b.N`, `b.ReportAllocs`, `b.ResetTimer`, `b.RunParallel`, the `-bench`/`-benchmem`/`-benchtime`/`-count` flags:
  <https://pkg.go.dev/testing#hdr-Benchmarks>
- **Profiling Go Programs** — the canonical `pprof` introduction; CPU profiles, `go tool pprof`, reading the output:
  <https://go.dev/blog/pprof>
- **Diagnostics** — the overview of profiling, tracing, and debugging tools in the Go toolchain:
  <https://go.dev/doc/diagnostics>
- **`benchstat`** — compare benchmark runs with statistical confidence; install with `go install golang.org/x/perf/cmd/benchstat@latest`:
  <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>

### Threading context into I/O

- **`net/http.NewRequestWithContext`** — how cancellation reaches an HTTP request:
  <https://pkg.go.dev/net/http#NewRequestWithContext>
- **`net/http/httptest`** — the in-process test server used to test the worker pool without the real network:
  <https://pkg.go.dev/net/http/httptest>

## Recommended reading (after the required set)

- **Effective Go — Concurrency** — the original idiom guide; "share memory by communicating," goroutines, channels:
  <https://go.dev/doc/effective_go#concurrency>
- **The Go Programming Language Specification — Channels / `select`** — the precise semantics underneath the idioms:
  <https://go.dev/ref/spec#Channel_types>
- **Go Concurrency Patterns (Rob Pike, Google I/O 2012)** — the foundational talk; dated in syntax, timeless in ideas:
  <https://www.youtube.com/watch?v=f6kdp27TYZs>
- **Advanced Go Concurrency Patterns (Sameer Ajmani, GopherCon 2013)** — `select`, cancellation, and the patterns that became `context`:
  <https://go.dev/blog/advanced-go-concurrency-patterns>
- **The `errgroup` source** — read it once; it is ~150 lines and shows exactly how `SetLimit` and `WithContext` work:
  <https://cs.opensource.google/go/x/sync/+/master:errgroup/errgroup.go>

## Tools you will install this week

- **`golang.org/x/sync`** — added per-module: `go get golang.org/x/sync@latest`. Provides `errgroup` and `semaphore`.
- **A C compiler on PATH** — required by the race detector (ThreadSanitizer uses cgo). macOS: `xcode-select --install`. Debian/Ubuntu: `apt install gcc`. Windows: use WSL2 or install mingw `gcc`.
- **`benchstat`** — `go install golang.org/x/perf/cmd/benchstat@latest`. Verify with `benchstat --help`. Compares benchmark runs.
- **`staticcheck`** — `go install honnef.co/go/tools/cmd/staticcheck@latest`. The lint the mini-project must pass clean.

## Citations policy

This curriculum cites the Go package documentation on `pkg.go.dev`, the Go blog and specification on `go.dev`, the Go memory model, and the official `golang.org/x` module docs as the primary references. Every example in the lecture notes and exercises is traced back to one of these. When a conference talk (Bryan Mills, Rob Pike, Sameer Ajmani) is the clearer reference, it is cited explicitly with a URL — never paraphrased without attribution. If a citation is missing from a section of these notes, treat it as a bug and open an issue against the C30 curriculum repository.
