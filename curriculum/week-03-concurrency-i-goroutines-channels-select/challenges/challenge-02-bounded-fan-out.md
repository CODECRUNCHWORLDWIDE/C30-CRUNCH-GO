# Challenge 2 — Bounded Fan-Out: From One-Goroutine-Per-Item to an N-Worker Pool

> **Time:** 1.5 hours. **Prerequisites:** Lectures 1–3, Exercise 3. **Citations:** the pipelines/cancellation blog at <https://go.dev/blog/pipelines>, Effective Go's concurrency section at <https://go.dev/doc/effective_go#concurrency>, the `runtime` docs at <https://pkg.go.dev/runtime#NumGoroutine>, and the `time` package at <https://pkg.go.dev/time>.

## The premise

The naive way to "do all these in parallel" is to start one goroutine per item:

```go
for _, item := range items {
	go process(item) // one goroutine PER item — unbounded
}
```

For ten items this is fine. For ten thousand items where `process` opens a file or makes an HTTP request, it is a disaster: ten thousand goroutines all reaching for a file descriptor or a socket at once will exhaust the OS's file-descriptor limit, blow up memory, and very likely get you rate-limited or refused by the remote server. **Unbounded fan-out is a bug waiting for a big enough input.** The fix is a *bounded* fan-out — a fixed pool of N worker goroutines pulling items off a channel — so that no matter how many items there are, only N are ever in flight. In this challenge you build both, break the unbounded one, and measure the difference.

## Part A — The unbounded version (and why it breaks)

Write `ProcessUnbounded(items []int, work func(int) int) []int` that starts one goroutine per item, collecting results over a channel:

```go
func ProcessUnbounded(items []int, work func(int) int) []int {
	out := make(chan int)
	var wg sync.WaitGroup
	wg.Add(len(items))
	for _, it := range items {
		go func(v int) {
			defer wg.Done()
			out <- work(v) // one goroutine per item, all alive at once
		}(it)
	}
	go func() { wg.Wait(); close(out) }()

	var results []int
	for r := range out {
		results = append(results, r)
	}
	return results
}
```

Now make `work` *expensive in a way that exposes the problem*. The cleanest demonstration is file descriptors: have `work` open a file (e.g. `os.Open("/dev/null")`), hold it briefly with a `time.Sleep`, then close it. Call `ProcessUnbounded` with a large slice (say 50,000 items), and instrument it to print the **peak** `runtime.NumGoroutine()` mid-run (sample it from a separate goroutine on a ticker). You should see the goroutine count spike to ~50,000, and — depending on your `ulimit -n` and the timing — you may see `too many open files` errors. Record the peak goroutine count and any FD errors. (If your OS limit is high, lower it for the demo with `ulimit -n 256` in the shell before running.)

## Part B — The bounded version (the worker pool)

Now write `ProcessBounded(items []int, workers int, work func(int) int) []int` that starts exactly `workers` goroutines, each pulling items off a shared channel — the fan-out/fan-in pattern from Lecture 3:

```go
func ProcessBounded(items []int, workers int, work func(int) int) []int {
	if workers < 1 {
		workers = 1
	}
	in := make(chan int)
	out := make(chan int)

	// Feeder owns `in`.
	go func() {
		defer close(in)
		for _, it := range items {
			in <- it
		}
	}()

	// Exactly `workers` goroutines — the BOUND. No matter how many items.
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for v := range in { // each worker pulls the next item; at most `workers` in flight
				out <- work(v)
			}
		}()
	}

	go func() { wg.Wait(); close(out) }()

	var results []int
	for r := range out {
		results = append(results, r)
	}
	return results
}
```

The key difference: the channel `in` *is* the work queue, and exactly `workers` goroutines drain it, so at most `workers` calls to `work` run at once — the open file descriptors are bounded by `workers`, not by `len(items)`. Run the same 50,000-item, `/dev/null`-opening workload with `workers = 16` and sample the peak goroutine count. It should stay around `16 + overhead`, never spike, and never hit the FD limit.

## Part C — Measure the difference

Build a small table comparing the two, on the same input:

| Version | Peak goroutines | Peak open FDs (approx) | FD errors? | Wall-clock |
|---|---:|---:|:---:|---:|
| `ProcessUnbounded` (50k items) | ~50,000 | up to ~50,000 | likely | ? |
| `ProcessBounded` (50k items, 16 workers) | ~16 | ~16 | none | ? |

Time both with `time.Now()`/`time.Since` around the call (not `go test -bench` yet — that is Week 8). You will often find the bounded version is *also faster* for I/O-bound work, because the unbounded version thrashes the scheduler and the OS, while the bounded version keeps a steady, sustainable level of concurrency.

## Acceptance criteria

1. **Both functions work** and return the same result set (as a multiset — order differs) for the same input. A small table test proving this.
2. **Part A demonstrates the problem:** the recorded peak goroutine count for the unbounded version on a large input, and either FD-exhaustion errors or a clear statement of the `ulimit` you set to provoke them.
3. **Part B demonstrates the fix:** the recorded peak goroutine count for the bounded version staying at ~`workers`, with no FD errors, on the *same* large input.
4. **Part C is the comparison table**, filled in with your measured numbers and one paragraph interpreting them (why bounded is safer, and often faster, for I/O-bound work).
5. Clean under `go vet ./...`, `staticcheck ./...`, and `go test -race ./...`.

## Stretch goals

1. **A `--workers` sweep.** Parameterise the worker count and run the bounded version at 1, 4, 16, 64, 256 workers on an I/O-bound workload (open `/dev/null`, `time.Sleep`, close). Plot or tabulate wall-clock vs worker count. You will see throughput climb, then plateau, then sometimes *degrade* past the sweet spot — the same curve you will measure rigorously with benchmarks in Week 8. Identify your machine's sweet spot and explain it.
2. **The semaphore-channel alternative.** Instead of N long-lived workers, keep one-goroutine-per-item but bound concurrency with a buffered channel used as a semaphore: `sem := make(chan struct{}, N)`, and each goroutine does `sem <- struct{}{}` before `work` and `<-sem` after. Implement it, confirm the peak goroutine count is still ~`len(items)` (goroutines exist but most are blocked on `sem`) while peak *concurrency* is N. Discuss the trade-off vs the worker-pool shape (more goroutines, but simpler to add per-item context).
3. **The production version (Week 4 preview).** Note in your write-up what changes when you reach Week 4: `errgroup.Group` with `SetLimit(N)` gives you a bounded pool *and* first-error propagation *and* `context` cancellation in a few lines — replacing the hand-rolled `WaitGroup` + closer + `done`-channel machinery here. Sketch (do not implement) which parts of `ProcessBounded` `errgroup` would subsume. Citation: <https://pkg.go.dev/golang.org/x/sync/errgroup>.

Cited references: <https://go.dev/blog/pipelines>, <https://go.dev/doc/effective_go#concurrency>, <https://pkg.go.dev/runtime#NumGoroutine>, <https://pkg.go.dev/time>, <https://pkg.go.dev/golang.org/x/sync/errgroup>.
