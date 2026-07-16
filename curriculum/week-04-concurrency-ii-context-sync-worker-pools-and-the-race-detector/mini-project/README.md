# Mini-Project — `linkcheck`: A Bounded, Race-Free, `context`-Cancellable Link-Checker (the Phase I Gate Artifact)

> **Time:** ~9.5 hours across Friday-Saturday-Sunday. **Prerequisites:** Exercises 1-3, ideally both challenges, and Lab 03's link-checker from Week 3. **Citations:** every package doc referenced in the three lecture notes, plus the Go memory model and the race-detector article.

## The spec

You are taking **Lab 03's concurrent link-checker** — which fanned out to N HTTP HEAD requests through a channel pipeline — and hardening it into the artifact you will demo at the **Phase I gate**. The hardened tool:

- runs as a bounded worker pool (at most `--workers` requests in flight),
- threads `context` cancellation through everything (graceful Ctrl-C *and* a `--timeout` deadline),
- has a deliberately-introduced data race that you find with `go test -race` and fix,
- ships clean under `go vet`, `staticcheck`, and `go test -race`,
- and comes with a benchmark sweep across three pool sizes and a perf write-up.

```
                    sitemap.xml / URL list
                            |
                            v
                  +-------------------+
                  |   read & parse    |  (one goroutine)
                  +---------+---------+
                            |  urls
                            v
                  +-------------------+
                  | errgroup pool     |  (at most --workers goroutines)
                  |  HEAD each URL    |  (each request carries ctx)
                  +---------+---------+
                            |  results[i] (written by index, race-free)
                            v
                  +-------------------+
                  |  report (Markdown)|  ctx cancellation drains in-flight work
                  +-------------------+
```

The CLI:

```
linkcheck [flags] <sitemap.xml | -]
  --workers N     max concurrent requests (default 16)
  --timeout D     overall deadline, e.g. 30s (default: none)
  --format F      report format: table | json (default table)
```

Ctrl-C drains in-flight requests and prints a partial report; the `--timeout` deadline does the same when it fires.

## Functional requirements

### F1 — Input

- Read a `sitemap.xml` from a path argument, or from stdin when the argument is `-`.
- Parse `<loc>` elements with `encoding/xml`; tolerate a plain newline-delimited URL list too (detect by sniffing the first non-space byte: `<` → XML, else line list).
- Validate each URL with `net/url.Parse`; record an invalid URL as a result with an error, do not abort the run.

### F2 — Bounded concurrency

- Check URLs concurrently with `errgroup.WithContext` + `g.SetLimit(--workers)`.
- Each check issues an `http.MethodHead` request built with `http.NewRequestWithContext(ctx, ...)`, so cancellation reaches the wire.
- A shared `*http.Client` with a sane `Timeout` is reused across all requests (never one client per request).
- Instrument the pool with an `atomic.Int64` high-water mark; the peak must never exceed `--workers`. Assert this in a test.

### F3 — Cancellation

- The root context comes from `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`.
- When `--timeout` is set, derive a `context.WithTimeout` from the signal context; the earlier of the two wins.
- On cancellation, in-flight requests are aborted (the context error surfaces as the per-URL result), and the program prints the partial report and exits 0 (cancellation is not a crash).

### F4 — The race, found and fixed

- The starter ships a *deliberate* data race in the result-aggregation path (e.g. a shared `stats` struct mutated from every goroutine without synchronisation, or a slice appended from goroutines).
- You must reproduce it under `go test -race`, paste the report into `PERF.md`, and fix it — with an atomic for the scalar counters and/or by writing per-index, never appending.
- After the fix, `go test -race ./...` is green.

### F5 — Report

- `table` format: a Markdown table with columns URL, Status, OK?, Error, sorted by input order.
- `json` format: a JSON array of `{url, status, ok, error}` objects.
- A summary footer: total, OK (2xx/3xx), broken (4xx/5xx), errored (network/cancelled), and the peak concurrency observed.

### F6 — Toolchain cleanliness

- `go vet ./...` clean, `staticcheck ./...` clean, `go test -race ./...` green.
- Table-driven tests for the parser and the classifier; an `httptest.Server`-backed test for the pool (no real network in tests).

## Non-functional requirements

### NF1 — No goroutine leaks

- Every goroutine has a defined stop: the pool's workers return on completion or on `ctx.Done()`; the parser goroutine closes its channel; nothing is left blocked. A test counts goroutines before and after a run (with a settle delay) and asserts no growth.

### NF2 — Code quality

- File-scoped, small functions; a clean `parse → check → report` seam (three packages or three clearly-separated files).
- Every blocking call takes `ctx` as its first parameter. No `http.NewRequest` (the context-less constructor) anywhere.
- No fire-and-forget goroutines: every `go`/`g.Go` is joined.

### NF3 — Citations

- Every non-obvious choice has a one-line comment citing the relevant package doc or the memory model.
- `README.md` lists `golang.org/x/sync` with its version and license; everything else is standard library.

## Suggested project layout

```
linkcheck/
├── go.mod
├── README.md            <-- build, run, the flags, an example
├── PERF.md              <-- the race report + the benchmark sweep (see below)
├── main.go              <-- flag parsing, signal context, wiring
├── parse.go             <-- sitemap/URL-list parsing
├── parse_test.go
├── check.go             <-- the errgroup pool, the http.Client, the atomic peak
├── check_test.go        <-- httptest-backed; the leak check; the peak assertion
├── report.go            <-- table + json formatting, the summary footer
└── report_test.go
```

## Starter

A starter scaffold is provided in `mini-project/starter/`. Copy it as your starting point:

- `main.go` — flag parsing and the `signal.NotifyContext` wiring, with a `TODO`-free skeleton that compiles.
- `check.go` — the pool, **with the deliberate data race in the aggregation path**. Your job is to find it under `-race` and fix it.
- `parse.go` — the XML/line-list parser, complete.
- `check_test.go` — the `httptest`-backed test and the peak-concurrency assertion (the peak assertion will *fail* until you fix the race that corrupts the counter).

The starter compiles and runs, but the race makes the peak counter and the summary unreliable until you fix it.

## The perf write-up (`PERF.md`)

Treat it as part of the deliverable.

### M1 — the race report

Paste the `go test -race` output for the unfixed starter. Annotate it: the read line, the previous-write line, the two stacks, the `go` statement they were created at. State the fix you applied and why (atomic vs mutex vs per-index write).

### M2 — the benchmark sweep

Benchmark the pool against an `httptest.Server` (so the "network" is local and stable) across three worker counts, e.g. `{4, 16, 64}` over 500 URLs:

```
BenchmarkCheck/workers=4-8     	      30	  41_000_000 ns/op
BenchmarkCheck/workers=16-8    	     100	  11_500_000 ns/op
BenchmarkCheck/workers=64-8    	     110	  10_800_000 ns/op
```

Report the numbers and identify the knee (where adding workers stops paying). One sentence interpreting why the curve flattens (you have saturated the local test server / the loopback).

### M3 — cancellation timing

With a deliberately-slow `httptest` handler (each request sleeps 100ms) and `--timeout 300ms`, run 100 URLs at `--workers 8`. Report: how many completed before the deadline, how many were recorded as cancelled, and the wall-clock time from deadline-fire to program exit (should be small — in-flight requests abort promptly because they carry `ctx`).

### M4 — leak check

Report the goroutine count before the run, immediately after, and after a 100ms settle. Confirm no growth. State which goroutine *would* leak if you removed the `ctx.Done()` case from the worker's `select`.

### M5 — peak concurrency

Report the observed `atomic.Int64` peak for each of the three worker counts. Confirm `peak <= --workers` in every case. This is your proof the bound held.

## Grading rubric

- **35 points: functional correctness.** F1-F6 implemented and demonstrable; the tool checks a real sitemap and produces a correct report.
- **20 points: the race, found and fixed.** The report is pasted and annotated in `PERF.md`; the fix is correct and idiomatic; `go test -race` is green after.
- **15 points: bounded, leak-free concurrency.** Peak ≤ workers (asserted in a test); no goroutine leak (asserted in a test); Ctrl-C and `--timeout` both drain cleanly.
- **15 points: the perf write-up.** All five measurements (M1-M5) reported with real numbers and one-sentence interpretations.
- **10 points: toolchain cleanliness.** `go vet`, `staticcheck`, `go test -race` all clean; tests are table-driven and use `httptest`, not the real network.
- **5 points: citations.** At least eight distinct citations in the source pointing at Go package docs or the memory model.

## The Phase I gate

This mini-project **is** the Phase I gate artifact. At the gate you will:

1. Demo the tool checking a real sitemap, then Ctrl-C mid-run and show the partial report.
2. Run `go vet ./... && staticcheck ./... && go test -race ./...` live and show it green.
3. Walk the race report from `PERF.md` and explain the fix.
4. Show the benchmark sweep and name the knee.
5. Draw the context tree on a whiteboard and point at the line that stops each goroutine.

## Stretch goals

1. **Retry with backoff and jitter.** Wrap each check in a retry (up to 3 attempts) on a 5xx or a network error, with exponential backoff plus jitter, all under the same `ctx` deadline. This is a preview of Week 11's reliability patterns. Discuss why jitter matters (the thundering-herd problem).
2. **A progress meter.** Stream a live "checked 412 / 1000, 14 broken" line to stderr using an `atomic.Int64` counter, updated without blocking the workers.
3. **Crawl one level deep.** For each OK HTML page, GET it (not HEAD), extract `<a href>` links with `golang.org/x/net/html`, and check those too — bounded by the *same* pool, so the total concurrency stays capped even as the work grows. Prove the peak still never exceeds `--workers`.

## Submission

Push the project on a branch named `week04-mini-project/<your-handle>` and open a PR against the C30 curriculum repository. The PR description must link to `PERF.md` and paste the green `go test -race ./...` line.

The teaching staff reviews mini-project PRs within 7 business days. Reviews focus on: (a) whether the six functional requirements are met, (b) whether the race was genuinely found, reported, and fixed, (c) whether the bound and leak-freedom are *asserted in tests* not just claimed, and (d) whether the perf write-up has real measurements.

Cited references: every page referenced in the three lecture notes, plus <https://go.dev/ref/mem>, <https://go.dev/doc/articles/race_detector>, <https://pkg.go.dev/golang.org/x/sync/errgroup>, <https://pkg.go.dev/net/http#NewRequestWithContext>, <https://pkg.go.dev/net/http/httptest>, <https://pkg.go.dev/os/signal#NotifyContext>.
