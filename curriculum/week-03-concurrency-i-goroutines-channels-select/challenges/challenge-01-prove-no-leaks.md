# Challenge 1 — Prove No Leaks: Count Goroutines, Introduce a Leak, Then Fix It

> **Time:** 1.5 hours. **Prerequisites:** Lectures 1–2, Exercises 1–2. **Citations:** `runtime.NumGoroutine` at <https://pkg.go.dev/runtime#NumGoroutine>, the `goleak` repository at <https://github.com/uber-go/goleak> and its docs at <https://pkg.go.dev/go.uber.org/goleak>, the pipelines/cancellation blog at <https://go.dev/blog/pipelines>, and the `testing` package at <https://pkg.go.dev/testing>.

## The premise

A goroutine leak is a bug the compiler accepts, `go vet` ignores, `staticcheck` ignores, and the happy-path tests pass right over. The only way to catch one is to *look for it* — to count goroutines, or to assert that none survive a test. In this challenge you build a tiny concurrent program, instrument it to count goroutines before and after, then deliberately introduce a leak, watch the count climb, and fix it — first by hand, then with the production tool, `uber-go/goleak`.

## The program

A small worker pool that processes jobs and returns results. Start from this shape (`go mod init github.com/you/leakproof`, save as `pool.go`):

```go
// pool.go
package leakproof

import "sync"

// Process fans `jobs` out across `workers` goroutines, applies fn to each, and
// returns the results (unordered). This version is CORRECT and leak-free.
func Process(jobs []int, workers int, fn func(int) int) []int {
	if workers < 1 {
		workers = 1
	}
	in := make(chan int)
	out := make(chan int)

	// Feeder owns `in`, so the feeder closes it.
	go func() {
		defer close(in)
		for _, j := range jobs {
			in <- j
		}
	}()

	// Fan out.
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for j := range in {
				out <- fn(j)
			}
		}()
	}

	// The only safe closer of `out`.
	go func() {
		wg.Wait()
		close(out)
	}()

	// Fan in.
	var results []int
	for r := range out {
		results = append(results, r)
	}
	return results
}
```

Confirm it works with a trivial driver or test (`Process([]int{1,2,3}, 2, func(n int)int{return n*n})` returns the squares, unordered).

## Part A — Count goroutines by hand

Write a test (`pool_test.go`) that captures `runtime.NumGoroutine()` before and after calling `Process`, allowing a beat for goroutines to unwind:

```go
func TestProcess_NoLeak_ByCount(t *testing.T) {
	before := runtime.NumGoroutine()

	for i := 0; i < 20; i++ { // run it many times to amplify any leak
		_ = Process([]int{1, 2, 3, 4, 5}, 3, func(n int) int { return n * n })
	}

	time.Sleep(50 * time.Millisecond) // let tail goroutines return
	runtime.GC()
	after := runtime.NumGoroutine()

	if after > before+1 { // +1 tolerance for scheduler/runtime noise
		t.Errorf("goroutine count grew: before=%d after=%d", before, after)
	}
}
```

Run it; it should pass. Record the `before`/`after` numbers.

## Part B — Introduce a leak

Now break it on purpose. Write a `ProcessLeaky` that drops the closer goroutine *and* makes a worker abandon a send when a (never-signalled) condition is hit — for example, a worker that, on the first job, starts a helper goroutine sending on an unbuffered channel nobody reads:

```go
// ProcessLeaky LEAKS one goroutine per call. Do not ship this — it is the bug.
func ProcessLeaky(jobs []int, fn func(int) int) []int {
	results := make([]int, 0, len(jobs))
	for _, j := range jobs {
		leak := make(chan int) // unbuffered, never read
		go func(v int) {
			leak <- fn(v) // BLOCKS FOREVER: nobody receives on leak
		}(j)
		results = append(results, fn(j)) // we compute synchronously and ignore the goroutine
	}
	return results
}
```

Add a counting test for `ProcessLeaky` like Part A's. It should **fail** (or, if you assert the leak, *pass by detecting it*) — the goroutine count grows by roughly one per job. Record the numbers and confirm `runtime.GC()` does *not* bring them back down (a blocked goroutine is live, not garbage).

## Part C — Catch it with `goleak`

Add the production tool. `go get go.uber.org/goleak`, then add a `TestMain` to the package:

```go
import "go.uber.org/goleak"

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
```

Run `go test ./...`. With `ProcessLeaky` exercised in a test, `goleak` fails the run and names the leaked goroutine — its state (`chan send`) and the line that created it. Capture that output. Then either delete the `ProcessLeaky` test or change it to use `defer goleak.VerifyNone(t)` so you can see the per-test failure precisely.

## Part D — Fix the leak

Fix `ProcessLeaky` so it leaks nothing. The minimal fix is to give the helper a buffered channel of one (so its send always succeeds) — but the *better* fix, if the goroutine is supposed to do real work whose result you want, is to actually receive the value (and close channels from the right owner). Re-run the `goleak` test and the count test; both must pass.

## Acceptance criteria

1. **Part A passes:** a `runtime.NumGoroutine()` before/after test on the correct `Process` shows the count returns to baseline (within ±1).
2. **Part B demonstrates the leak:** numbers showing the goroutine count growing roughly one-per-job for `ProcessLeaky`, and a note that `runtime.GC()` does not reclaim them, with the one-sentence reason (a blocked goroutine is live).
3. **Part C catches it:** the captured `goleak` failure output, with the leaked goroutine's state and creation site identified.
4. **Part D fixes it:** the fixed code, plus a clean `go test ./...` (with the `goleak` `TestMain` in place) and a clean `go test -race ./...`.

## Stretch goals

1. **`goleak.VerifyNone(t)` per test.** Convert from the package-wide `TestMain` to a `defer goleak.VerifyNone(t)` at the top of each concurrent test. Discuss the trade-off: `TestMain` catches a leak from *any* test but does not tell you which; per-test pins the blame but is more boilerplate.
2. **A leak that needs `done` to fix, not a buffer.** Write a worker that loops forever waiting for jobs on a channel that is never closed — a buffer cannot fix this (the goroutine is not stuck on a send, it is stuck on a receive). Fix it by adding a `done := make(chan struct{})` the worker selects on, and `close(done)` from the caller. This is the cancellation pattern `context` standardises next week — note in your write-up exactly which lines `context.Context` would replace.
3. **Quantify the cost of a leak.** Run `ProcessLeaky` 100,000 times in a loop and capture `runtime.NumGoroutine()` and the process RSS (`/usr/bin/time -v` on Linux, or read `runtime.MemStats`). Put a number on "a leak-per-call in a long-running service": how many goroutines and how much memory after 100k calls?

Cited references: <https://pkg.go.dev/runtime#NumGoroutine>, <https://github.com/uber-go/goleak>, <https://pkg.go.dev/go.uber.org/goleak>, <https://go.dev/blog/pipelines>, <https://pkg.go.dev/testing>.
