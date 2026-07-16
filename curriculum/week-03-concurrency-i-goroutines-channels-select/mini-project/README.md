# Mini-Project — Lab 03: `linkcheck`, a Concurrent Link-Checker with a Fan-Out/Fan-In Pipeline

> **Time:** ~9.5 hours across Wednesday-Friday-Saturday. **Prerequisites:** all three lectures, all three exercises, ideally both challenges. **Citations:** the standard-library docs for `net/http`, `encoding/xml`, `sync`, `time`, `flag`, and `context` on <https://pkg.go.dev>; the pipelines blog at <https://go.dev/blog/pipelines>; and `goleak` at <https://github.com/uber-go/goleak>.

## The spec

You are building **`linkcheck`**, a command-line tool that reads a `sitemap.xml`, checks every URL in it with a concurrent HTTP HEAD request, and prints a report of each URL's status, latency, and any error. It is the canonical Week 3 program: a generator (parse the sitemap), a **fan-out** to N worker goroutines making HTTP requests, a **fan-in** that merges their results, a `sync.WaitGroup` for completion, a `done` channel for clean cancellation — and, critically, a **proof that it leaks no goroutines.**

```
$ go run . --workers 16 sitemap.xml
checking 42 URLs with 16 workers...

STATUS  LATENCY   URL
200     38ms      https://example.com/
200     41ms      https://example.com/about
301     55ms      https://example.com/old-page
404     33ms      https://example.com/missing
ERR     —         https://broken.example/    (dial tcp: no such host)
...

summary: 42 checked · 38 ok (2xx/3xx) · 3 client/server errors (4xx/5xx) · 1 request error · 1.9s wall
no leaked goroutines ✓
```

This is, structurally, the fan-out/fan-in pipeline from Lecture 3 with "HTTP HEAD a URL" in place of "square a number." Get the channels and the close-ownership right and it falls out cleanly; get them wrong and it deadlocks or leaks.

## Functional requirements

### F1 — Input: parse the sitemap

- `linkcheck <file>` reads a sitemap XML file. `linkcheck` with no file reads the sitemap from **stdin** (so `curl .../sitemap.xml | linkcheck` works).
- Parse it with `encoding/xml` into a struct (see the starter below). A sitemap is `<urlset>` containing `<url>` elements each with a `<loc>` (the URL).
- A malformed sitemap prints a clear error to **stderr** and exits non-zero. It must **not** panic.

### F2 — Fan-out: N concurrent HTTP HEAD workers

- Fan the URLs out to a fixed pool of **N worker goroutines** (default **16**), each pulling URLs off a shared channel — the bounded fan-out from Challenge 2. No matter how many URLs the sitemap holds, only N requests are ever in flight.
- Each worker issues an HTTP **HEAD** request (`http.NewRequest(http.MethodHead, url, nil)` via an `*http.Client`). HEAD asks for headers only — you want the status code, not the body.
- Each worker measures the request **latency** (`time.Since(start)`), records the **status code**, and records any **error** (DNS failure, connection refused, timeout).
- Set a per-request **timeout** on the `http.Client` (e.g. `Timeout: 10 * time.Second`) so one hung host cannot stall a worker forever.

### F3 — Fan-in: merge results back

- Collect the per-URL results back over a results channel into a single slice — the fan-in merge from Lecture 3. Use the **`WaitGroup` + single closer goroutine** idiom: the workers are the senders on the results channel, so a *separate* goroutine closes it after `wg.Wait()`.
- The main goroutine ranges the results channel until it is closed and collects every result. (Results arrive in nondeterministic order; sort them for the report — by status then URL, or just by URL.)

### F4 — The report

- Print a table to **stdout**: `STATUS`, `LATENCY`, `URL`, and an error note for failed requests. Use `text/tabwriter` (or aligned `Printf`) so the columns line up.
- Classify results in a **summary** line: count `2xx/3xx` as "ok", `4xx/5xx` as "client/server errors", and request-level failures (no HTTP response at all) as "request errors". Print the total wall-clock time.
- **stdout for the report; stderr for diagnostics and fatal errors.** `linkcheck sitemap.xml > report.txt` must produce a clean report with no error text mixed in.

### F5 — The `--workers` flag and clean shutdown

- `--workers N` (default 16) sets the pool size; validate it (a non-positive value is a clear stderr error and non-zero exit). Parse with the `flag` package.
- The program must shut down **cleanly**: when all URLs are checked, the feeder closes the jobs channel, every worker's `range` ends, the closer closes the results channel, and `main` returns. **Every goroutine has a guaranteed exit.**
- Provide a `done` channel (a `make(chan struct{})`, `defer close(done)` in the orchestrator) that workers and the feeder `select` on, so an early exit (e.g. you add Ctrl-C handling as a stretch) stops everything without a leak. (The `context` version of this is next week; this week, the hand-rolled `done` channel is the point.)

### F6 — Prove no leaked goroutines

- The pipeline must leak **zero** goroutines. Prove it two ways:
  - In tests, a `TestMain` with `goleak.VerifyTestMain(m)` (`go get go.uber.org/goleak`), so any leaked goroutine fails the test run; **and/or**
  - a `runtime.NumGoroutine()` before/after measurement in a test (or behind a `--debug` flag) showing the count returns to baseline after a full run.
- This is the headline grading criterion for the lab. "It works on my small sitemap" is not a leak proof.

### F7 — Tests

- The **pure logic** — sitemap parsing, status classification, result sorting — lives in functions that take inputs and return values (no network, no `os.Exit`), and is covered by a **table-driven test suite** with named subtests.
- The **HTTP-using** code is tested against an `httptest.Server` (standard library) that serves canned responses (a 200, a 404, a redirect, a hang) — not the live internet, so the tests are fast and deterministic.
- Clean under `go vet ./...` and `staticcheck ./...`; `go test ./...` and `go test -race ./...` both green.

## Non-functional requirements

### NF1 — Idiomatic, leak-free concurrency

- Use channel **direction types** on every pipeline-stage function (`<-chan` / `chan<-`) so the compiler enforces who sends and who receives.
- Obey the **"who closes" rule**: the feeder closes the jobs channel; a dedicated closer goroutine closes the results channel after `wg.Wait()`. No receiver ever closes.
- Pass the `WaitGroup` **by pointer**; `Add` before each `go`. No `time.Sleep` for coordination anywhere.

### NF2 — One shared HTTP client, sane defaults

- Create **one** `*http.Client` (with a `Timeout`) and share it across all workers — `http.Client` is safe for concurrent use, and sharing it reuses connections. Do **not** create a client per request. Citation: <https://pkg.go.dev/net/http#Client>.
- Drain and close the response body even on HEAD (`defer resp.Body.Close()`), so connections are returned to the pool. (HEAD has no body, but closing is still correct and `staticcheck` will flag an unclosed body.)

### NF3 — Citations

- Every non-obvious standard-library choice carries a one-line `pkg.go.dev` citation in the source (e.g. `// http.MethodHead: https://pkg.go.dev/net/http#pkg-constants`).

## Suggested project layout

```
linkcheck/
├── go.mod                         (go mod init github.com/you/linkcheck)
├── README.md                      <-- build, run, sample report, the leak proof, one design note
├── testdata/
│   └── sitemap.xml                <-- a small sitemap to test against
├── main.go                        <-- CLI: flags, stdin/file, orchestration, output, exit codes
└── internal/
    └── checker/
        ├── sitemap.go             <-- ParseSitemap(r io.Reader) ([]string, error)
        ├── check.go               <-- the fan-out/fan-in pipeline + Result type + classify
        └── checker_test.go        <-- table tests + httptest server tests + goleak TestMain
```

A starting point for the sitemap struct and the worker shape (complete the pipeline):

```go
// internal/checker/sitemap.go
package checker

import (
	"encoding/xml" // https://pkg.go.dev/encoding/xml
	"fmt"
	"io"
)

// urlset mirrors the sitemap.org schema's <urlset><url><loc>...</loc></url></urlset>.
type urlset struct {
	XMLName xml.Name `xml:"urlset"`
	URLs    []struct {
		Loc string `xml:"loc"`
	} `xml:"url"`
}

// ParseSitemap reads a sitemap XML document and returns its <loc> URLs.
func ParseSitemap(r io.Reader) ([]string, error) {
	var set urlset
	if err := xml.NewDecoder(r).Decode(&set); err != nil {
		return nil, fmt.Errorf("parsing sitemap: %w", err)
	}
	locs := make([]string, 0, len(set.URLs))
	for _, u := range set.URLs {
		if u.Loc != "" {
			locs = append(locs, u.Loc)
		}
	}
	return locs, nil
}
```

```go
// internal/checker/check.go (sketch — you complete the pipeline)
package checker

import (
	"net/http" // https://pkg.go.dev/net/http
	"sync"
	"time"
)

// Result is one URL's outcome.
type Result struct {
	URL     string
	Status  int           // HTTP status code; 0 if the request itself failed
	Latency time.Duration
	Err     error         // non-nil on a request-level failure (DNS, refused, timeout)
}

// Check fans `urls` out across `workers` goroutines making HEAD requests, fans
// the results back in, and returns them. It leaks no goroutines.
func Check(client *http.Client, urls []string, workers int, done <-chan struct{}) []Result {
	if workers < 1 {
		workers = 1
	}
	jobs := make(chan string)
	results := make(chan Result)

	// Feeder owns `jobs`. It also watches `done` so it never blocks on a send
	// after the workers have left.
	go func() {
		defer close(jobs)
		for _, u := range urls {
			select {
			case jobs <- u:
			case <-done:
				return
			}
		}
	}()

	// Fan out: exactly `workers` goroutines.
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for u := range jobs {
				r := checkOne(client, u) // TODO: HEAD u, time it, fill Result
				select {
				case results <- r:
				case <-done:
					return
				}
			}
		}()
	}

	// The only safe closer of `results`.
	go func() {
		wg.Wait()
		close(results)
	}()

	// Fan in.
	var out []Result
	for r := range results {
		out = append(out, r)
	}
	return out
}
```

You write `checkOne` (issue the HEAD with the shared client, measure latency, fill `Result`), the `classify` helper (2xx/3xx vs 4xx/5xx vs request error), the report printing in `main.go`, and the tests.

## The README write-up (`README.md`)

Treat the README as part of the deliverable. It must contain:

### W1 — Build and run

Every command, copy-pasteable: `go build`, `go run . --workers 16 testdata/sitemap.xml`, the stdin form (`cat testdata/sitemap.xml | go run .`).

### W2 — A sample report

The actual table + summary output on your `testdata/sitemap.xml` (point it at a small public site you control or at an `httptest` server you capture).

### W3 — The leak proof

Paste the proof that the pipeline leaks no goroutines: either the `goleak`-clean `go test ./...` output, or the `runtime.NumGoroutine()` before/after numbers, with one sentence on *why* every goroutine has a guaranteed exit (name who closes each channel).

### W4 — One design note

~200 words on one decision: why the workers are bounded (and what would break with one-goroutine-per-URL on a 100k-URL sitemap), or why a separate goroutine closes the results channel, or why the `done` channel exists even though the happy path does not strictly need it.

## Grading rubric

- **35 points: functional correctness.** F1–F5: sitemap parse (file + stdin), bounded N-worker HEAD fan-out, fan-in merge, the report with status/latency/error and the summary line, the `--workers` flag, clean stdout/stderr separation.
- **25 points: leak-free, idiomatic concurrency.** F6 + NF1: the "who closes" rule obeyed, `WaitGroup` by pointer with Add-before-`go`, channel direction types, a `done` channel for clean shutdown, no `time.Sleep` coordination, **and a real proof of zero leaked goroutines** (this sub-criterion is worth half the points here — no proof, no points).
- **15 points: tests.** F7: table-driven tests of the pure logic as named subtests; `httptest.Server`-based tests of the HTTP path (200/404/redirect/hang); `go test ./...` and `go test -race ./...` green.
- **10 points: the shared client and HTTP hygiene.** NF2: one shared `*http.Client` with a timeout; response bodies closed; HEAD used correctly.
- **10 points: the README write-up.** W1–W4 present, with a real sample report and the leak proof.
- **5 points: citations + clean toolchain.** NF3 one-line citations in source; clean `go vet` and `staticcheck`.

## Stretch goals

1. **`-race` clean under load.** Run `go test -race ./...` with a test that checks 200 URLs against an `httptest.Server` across 32 workers. A `-race`-clean run under real concurrency is the proof that no two goroutines touch the same variable unguarded. (If you keep per-worker counts or a shared summary, this will catch a missing mutex.)
2. **Retry with backoff.** On a request error or a 5xx, retry up to 2 times with an exponential backoff (`time.Sleep(base * 2^attempt)` plus a little jitter). Add a `--retries` flag. Note in the README that the *production* version of timeouts-and-retries uses `context` (next week) rather than a bare `time.Sleep`.
3. **Graceful Ctrl-C.** Catch `SIGINT` (`os/signal.Notify`) and `close(done)` on it, so a `Ctrl-C` mid-run stops every worker and the feeder cleanly, prints a partial report, and still proves no leak. This is the hand-rolled version of the graceful shutdown you build with `context` in Week 4 and on a service in Week 11.
4. **The `context`/`errgroup` preview (Week 4).** Sketch (in the README, do not implement) how `errgroup.Group` with `SetLimit(N)` and a `context.Context` would replace the `WaitGroup` + closer-goroutine + `done`-channel machinery — and which exact lines it subsumes. You implement this conversion as Lab 04. Citation: <https://pkg.go.dev/golang.org/x/sync/errgroup>.
5. **Follow redirects vs report them.** By default Go's `http.Client` follows redirects. Add a `--no-follow` mode (set `client.CheckRedirect` to return `http.ErrUseLastResponse`) that reports the `301`/`302` itself instead of following it, and discuss which behaviour a link-checker should default to.

## Submission

Push the project on a branch named `c30-week03-linkcheck/<your-handle>` and open a PR against the C30 curriculum repository. The PR description must link to the README and paste (a) the sample report + summary line and (b) the leak proof (the `goleak`-clean test output or the before/after goroutine counts).

The teaching staff reviews mini-project PRs within 7 business days. Reviews focus on (a) whether the pipeline is correct and the seven functional requirements are met, (b) whether the concurrency is leak-free and idiomatic — the "who closes" rule, `WaitGroup` by pointer, direction types, the `done` channel — and whether the **leak proof is real**, (c) whether the tests cover the pure logic as named subtests and the HTTP path via `httptest`, and (d) whether the program is clean under `go vet`, `staticcheck`, and `go test -race`.

Cited references: <https://pkg.go.dev/net/http>, <https://pkg.go.dev/net/http#Client>, <https://pkg.go.dev/net/http/httptest>, <https://pkg.go.dev/encoding/xml>, <https://pkg.go.dev/sync#WaitGroup>, <https://pkg.go.dev/text/tabwriter>, <https://pkg.go.dev/flag>, <https://go.dev/blog/pipelines>, <https://github.com/uber-go/goleak>.
