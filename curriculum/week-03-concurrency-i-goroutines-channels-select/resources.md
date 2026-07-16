# Week 3 — Resources

Every resource on this page is **free**. The Go website (`go.dev`), the package documentation (`pkg.go.dev`), the Go source on GitHub, the Go team's talks, and "Go by Example" are all free and require no account. `goleak` is open-source (MIT/BSD-style). No paywalled material is linked.

## Required reading (work it into your week)

### The tour and the language basics

- **A Tour of Go — Concurrency** — the interactive concurrency module: goroutines, channels, buffered channels, range and close, `select`, the default case, and `sync.Mutex`. Run every snippet in the embedded playground:
  <https://go.dev/tour/concurrency/1>
- **A Tour of Go — Channels** — the specific channels page (send/receive, the rendezvous):
  <https://go.dev/tour/concurrency/2>
- **A Tour of Go — Range and Close** — closing a channel and ranging over it:
  <https://go.dev/tour/concurrency/4>
- **A Tour of Go — Select** — the `select` statement and its default case:
  <https://go.dev/tour/concurrency/5>

### Effective Go (concurrency)

- **Effective Go — Concurrency** — the canonical idiomatic-Go treatment: goroutines, channels, the "share memory by communicating" proverb, parallelization, and a worked example. Read the whole "Concurrency" section this week:
  <https://go.dev/doc/effective_go#concurrency>
- **Effective Go — Goroutines** — what a goroutine is and why it is cheap:
  <https://go.dev/doc/effective_go#goroutines>
- **Effective Go — Channels** — unbuffered vs buffered semantics, the channel as a synchroniser, the semaphore pattern:
  <https://go.dev/doc/effective_go#channels>

### The Go specification (precise semantics)

- **Go statements** — the exact semantics of `go f()`:
  <https://go.dev/ref/spec#Go_statements>
- **Channel types** — the `chan T`, `chan<- T`, and `<-chan T` types and their directionality:
  <https://go.dev/ref/spec#Channel_types>
- **Select statements** — the precise rules: random choice among ready cases, the `default` case, blocking:
  <https://go.dev/ref/spec#Select_statements>
- **Close** — the `close` built-in and exactly when it (and a send/receive on a closed channel) panics:
  <https://go.dev/ref/spec#Close>

### Concurrency patterns — the canonical sources

- **"Go Concurrency Patterns" — Rob Pike (Google I/O 2012)** — the foundational talk on goroutines, channels, `select`, generators, fan-in, and timeouts. The slides and the video:
  - Slides: <https://go.dev/talks/2012/concurrency.slide>
  - Video: <https://www.youtube.com/watch?v=f6kdp27TYZs>
- **"Advanced Go Concurrency Patterns" — Sameer Ajmani** — the follow-on patterns (the nil-channel trick, state machines with `select`, cancellation), as a Go blog post:
  <https://go.dev/blog/advanced-go-concurrency-patterns>
- **"Go Concurrency Patterns: Pipelines and cancellation"** — the canonical blog post on the generator → stage → sink pipeline, fan-out/fan-in, and shutting it all down cleanly. This is the architecture of the mini-project; read it twice:
  <https://go.dev/blog/pipelines>
- **"Share Memory By Communicating"** — the codelab that gives the proverb its name, and (crucially) its caveat that a `sync.Mutex` is sometimes the better fit:
  <https://go.dev/blog/codelab-share>

### The synchronization package and leak detection

- **The `sync` package** — `WaitGroup`, `Mutex`, `RWMutex`, `Once`. Read `WaitGroup` (Add/Done/Wait, "must not be copied after first use") and `Mutex` this week:
  <https://pkg.go.dev/sync> · <https://pkg.go.dev/sync#WaitGroup> · <https://pkg.go.dev/sync#Mutex>
- **`runtime.NumGoroutine`** — the crude leak detector: the count of currently-existing goroutines:
  <https://pkg.go.dev/runtime#NumGoroutine>
- **`go.uber.org/goleak`** — the production goroutine-leak detector: `VerifyTestMain` / `VerifyNone`, used in the challenge and the mini-project to *prove* no leaks:
  <https://github.com/uber-go/goleak> · <https://pkg.go.dev/go.uber.org/goleak>

### The standard library you will read this week

- **`pkg.go.dev` — the package index**:
  <https://pkg.go.dev/std>
- **`net/http`** — `Client`, `NewRequest`, `MethodHead`; the client is safe for concurrent use (the mini-project shares one):
  <https://pkg.go.dev/net/http> · <https://pkg.go.dev/net/http#Client>
- **`net/http/httptest`** — `Server` for testing the HTTP path without the live internet:
  <https://pkg.go.dev/net/http/httptest>
- **`encoding/xml`** — `Decoder`, struct tags; used to parse the `sitemap.xml`:
  <https://pkg.go.dev/encoding/xml>
- **`time`** — `After`, `NewTimer`, `AfterFunc`, `Since`; the timeout and latency tools:
  <https://pkg.go.dev/time> · <https://pkg.go.dev/time#After>
- **`text/tabwriter`** — aligned columnar output for the report:
  <https://pkg.go.dev/text/tabwriter>
- **`runtime`** — `NumGoroutine`, `GOMAXPROCS`, `GC` (the scheduler knobs touched in the lectures):
  <https://pkg.go.dev/runtime>

## Recommended reading (after the required set)

- **Go by Example** — short, runnable examples for every construct this week:
  - Goroutines: <https://gobyexample.com/goroutines>
  - Channels: <https://gobyexample.com/channels>
  - Channel buffering: <https://gobyexample.com/channel-buffering>
  - Channel directions: <https://gobyexample.com/channel-directions>
  - Select: <https://gobyexample.com/select>
  - Timeouts: <https://gobyexample.com/timeouts>
  - Closing channels: <https://gobyexample.com/closing-channels>
  - Range over channels: <https://gobyexample.com/range-over-channels>
  - WaitGroups: <https://gobyexample.com/waitgroups>
  - Worker pools: <https://gobyexample.com/worker-pools>
- **The Go Memory Model** — the formal "happens-before" rules; a channel send happens-before the corresponding receive completes. You consult it for exact ordering guarantees (we go deeper in Week 4):
  <https://go.dev/ref/mem>
- **Go Code Review Comments — Synchronous functions / Goroutine lifetimes** — the reviewer's instincts on goroutine ownership and when a function should be synchronous:
  <https://go.dev/wiki/CodeReviewComments>
- **The Go Blog index** — the pipelines, advanced-patterns, and share-memory posts above all live here:
  <https://go.dev/blog/>
- **"The Go Programming Language" (Donovan & Kernighan)** — Chapters 8 ("Goroutines and Channels") and 9 ("Concurrency with Shared Variables") cover this week in depth. Not free, but the canonical text:
  <https://www.gopl.io/>

## Tools you will install this week

- **The Go toolchain** (1.22 or newer): from <https://go.dev/dl/>; verify with `go version`. The 1.22 loop-variable scoping fix matters for goroutines launched in a loop.
- **`staticcheck`** (already from Week 1): `go install honnef.co/go/tools/cmd/staticcheck@latest`. Its concurrency checks (copied `WaitGroup`/`Mutex`, unreachable `select` case) earn their keep this week.
- **(Optional) `go.uber.org/goleak`**: `go get go.uber.org/goleak` *inside your module* — it is a test-only dependency added to `go.mod`/`go.sum`, not a global install. Required only for the leak-detection challenge and the mini-project's leak proof (you may instead use a `runtime.NumGoroutine` check, but `goleak` is the right tool). This is the single external dependency the week introduces.
- **The race detector** (built into the toolchain): run `go test -race ./...`. No install needed. Deep race coverage is Week 4, but run it now.

## Citations policy

This curriculum cites `go.dev` (the tour, Effective Go, the spec, the blog, the talks, the memory model), `pkg.go.dev` (the package documentation and source), the Go GitHub source, and the `uber-go/goleak` repository as the primary references for this week. Every example in the lecture notes and exercises traces back to one of these. The Rob Pike "Go Concurrency Patterns" talk (Google I/O 2012) is cited with its correct title and both its slide URL (<https://go.dev/talks/2012/concurrency.slide>) and video URL (<https://www.youtube.com/watch?v=f6kdp27TYZs>) because it is the clearest source for the fan-in, generator, and timeout patterns. When a third-party reference (Go by Example, the GOPL book, a Go team member's talk) is the clearer source, it is cited explicitly with a URL — never paraphrased without attribution. If a citation is missing from a section of these notes, treat it as a bug and open an issue against the C30 curriculum repository.
