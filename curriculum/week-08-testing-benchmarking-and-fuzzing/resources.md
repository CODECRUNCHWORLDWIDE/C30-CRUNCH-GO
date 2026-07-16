# Week 8 — Resources

Every resource on this page is **free**. The Go documentation, the Go blog, the `pkg.go.dev` reference, the `golang/go` wiki, and the `testcontainers-go` docs are all public and account-free. No paywalled material is linked.

## Required reading (work it into your week)

### The `testing` package and table-driven tests

- **`testing` package godoc** — the canonical reference for `*testing.T`, `*testing.B`, `*testing.F`, `TestMain`, `t.Run`, `t.Parallel`, `t.Helper`, `t.Cleanup`. Required; you will return to it constantly:
  <https://pkg.go.dev/testing>
- **Go blog — "Using Subtests and Sub-benchmarks"** — `t.Run` / `b.Run`, table-driven structure, parallel subtests:
  <https://go.dev/blog/subtests>
- **Go wiki — "TableDrivenTests"** — the idiom, with examples from the standard library (the wiki moved off GitHub to go.dev):
  <https://go.dev/wiki/TableDrivenTests>
- **Dave Cheney — "Prefer table driven tests"** — the doctrine, why the table beats N separate test functions:
  <https://dave.cheney.net/2019/05/07/prefer-table-driven-tests>
- **Go 1.22 release notes — loop variable semantics** — why the `tc := tc` shadow is no longer needed for parallel subtests:
  <https://go.dev/doc/go1.22>

### Assertions and golden files

- **`go-cmp` package godoc** — `cmp.Diff`, `cmp.Equal`, the options model:
  <https://pkg.go.dev/github.com/google/go-cmp/cmp>
- **`go-cmp/cmpopts` godoc** — `IgnoreFields`, `EquateApprox`, `SortSlices`, `AllowUnexported`:
  <https://pkg.go.dev/github.com/google/go-cmp/cmp/cmpopts>
- **`net/http/httptest` godoc** — `NewRecorder`, `NewServer`, `NewRequest` for handler and router tests:
  <https://pkg.go.dev/net/http/httptest>

### Benchmarking and profiling

- **Go blog — "Profiling Go Programs"** — the foundational profiling walkthrough; `top`, `list`, the profile types:
  <https://go.dev/blog/pprof>
- **`runtime/pprof` godoc** — programmatic CPU/heap/block/mutex profile capture, `SetBlockProfileRate`, `SetMutexProfileFraction`:
  <https://pkg.go.dev/runtime/pprof>
- **`net/http/pprof` godoc** — the `/debug/pprof/*` handlers for profiling a running service:
  <https://pkg.go.dev/net/http/pprof>
- **`benchstat` godoc** — the A/B comparison tool, the p-value, the percentage delta:
  <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>
- **`google/pprof` repository** — the profiling tool itself, the flame-graph UI, the README:
  <https://github.com/google/pprof>

### Fuzzing

- **Go fuzzing documentation** — the reference: `testing.F`, `f.Add`, `f.Fuzz`, the corpus, the engine:
  <https://go.dev/security/fuzz/>
- **Go fuzzing tutorial** — step-by-step, write a fuzz target and find a bug:
  <https://go.dev/doc/tutorial/fuzz>
- **Go 1.18 release notes — fuzzing GA** — the announcement that native fuzzing shipped as part of `go test`:
  <https://go.dev/doc/go1.18#fuzzing>
- **Go blog — "Fuzzing is Beta Ready"** — the historical beta announcement (pre-GA; useful background on the design):
  <https://go.dev/blog/fuzz-beta>

### Integration tests

- **`testcontainers-go` documentation** — the in-process container lifecycle, wait strategies, the API:
  <https://golang.testcontainers.org/>
- **`testcontainers-go` Postgres module** — the wrapped `postgres` image, `WithDatabase`/`WithUsername`/`WithPassword`, the default wait strategy:
  <https://golang.testcontainers.org/modules/postgres/>
- **`go` command — build constraints** — the `//go:build integration` tag syntax and resolution rules:
  <https://pkg.go.dev/cmd/go#hdr-Build_constraints>

## Authoritative deep dives

- **Go blog — "Go's testing infrastructure"** and the testing-related entries on the blog index:
  <https://go.dev/blog/>
- **`go help test` and `go help testflag`** — the full flag reference for `go test`: `-run`, `-bench`, `-benchmem`, `-count`, `-race`, `-cpuprofile`, `-fuzz`, `-fuzztime`, `-cover`, `-coverprofile`. Run them locally, or read the rendered docs:
  <https://pkg.go.dev/cmd/go#hdr-Testing_flags>
- **The Go memory model** — required background for understanding why `go test -race` matters and what a data race is:
  <https://go.dev/ref/mem>
- **Russ Cox / Caleb Spare — `golang.org/x/perf`** — the home of `benchstat` and the benchmark-analysis tooling:
  <https://pkg.go.dev/golang.org/x/perf>
- **`go.uber.org/mock`** — the maintained successor to `golang/mock`, for the rare case a generated mock earns its keep (read it to understand *why* you usually prefer a hand-written fake):
  <https://github.com/uber-go/mock>

## Source you should read

The standard library's own tests are the best table-driven-test corpus in existence — they are MIT-style licensed and source-link works. When a lecture says "the idiom is in the standard library," it means literally that; open a `_test.go` file and read it.

- **`strings` package tests** — a dense, well-organized set of table-driven tests:
  <https://github.com/golang/go/blob/master/src/strings/strings_test.go>
- **`net/url` package** — `Parse` plus its fuzz target, a real-world parser-with-fuzzing example:
  <https://github.com/golang/go/blob/master/src/net/url/url_test.go>
- **The `testing` package source** — `testing.go`, `benchmark.go`, `fuzz.go`; the framework is readable Go:
  <https://github.com/golang/go/tree/master/src/testing>
- **`go-cmp` source** — the diff engine and the options:
  <https://github.com/google/go-cmp>
- **`testcontainers-go` Postgres module source** — see exactly what the module's wait strategy and options do:
  <https://github.com/testcontainers/testcontainers-go/tree/main/modules/postgres>

## Tools (all free, first-party or first-party-adjacent)

- **`benchstat`** — `go install golang.org/x/perf/cmd/benchstat@latest`. The A/B benchmark comparison tool; computes the delta and the p-value:
  <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>
- **`go tool pprof`** — ships with the Go toolchain; `go tool pprof cpu.out` for `top`/`list`/`web`, `go tool pprof -http=:8080 cpu.out` for the interactive flame graph:
  <https://github.com/google/pprof>
- **`graphviz`** — `brew install graphviz`; required only for the graphical `pprof -web` / `-http` call-graph and flame-graph rendering. `top` and `list` work without it:
  <https://graphviz.org/>
- **`go tool cover`** — ships with the toolchain; `go tool cover -func=cover.out` and `go tool cover -html=cover.out` for coverage reports:
  <https://pkg.go.dev/cmd/cover>
- **Docker / Colima** — the container runtime `testcontainers-go` drives. Docker Desktop (<https://docs.docker.com/get-docker/>) or the lighter Colima (`brew install colima`):
  <https://github.com/abiosoft/colima>

## How to use this resource list

The lectures cite specific URLs from this page at decision points. When a lecture says "see the fuzzing docs," the URL is above. The links you should read end-to-end this week are:

1. **`testing` package godoc** — the reference. Plan for 45 minutes, spread across the week.
2. **"Using Subtests and Sub-benchmarks" (Go blog)** — the table-driven and sub-benchmark idiom. 20 minutes.
3. **"Profiling Go Programs" (Go blog)** — required before Wednesday's `pprof` work. 30 minutes.
4. **Go fuzzing tutorial** — required before Friday's fuzzing. Do it hands-on; 45 minutes.
5. **`testcontainers-go` Postgres module page** — required before the integration challenge. 20 minutes.

The rest are reference material. Bookmark and return to them when a specific question arises.

---

*Bookmarks decay. If a link rots, search the title — these are all canonical pieces and they reappear on the same authors' new homes.*
