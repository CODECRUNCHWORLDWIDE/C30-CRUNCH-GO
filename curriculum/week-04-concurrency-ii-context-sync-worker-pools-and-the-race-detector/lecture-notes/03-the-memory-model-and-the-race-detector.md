# Lecture 3 — The Go Memory Model, the Race Detector, and First Contact with Benchmarks and `pprof`

> **Time:** 2 hours. Take the memory-model and race-detector material in one sitting and the benchmarking material in a second. **Prerequisites:** Lectures 1 and 2. **Citations:** the Go Memory Model at <https://go.dev/ref/mem>, "Introducing the Go Race Detector" at <https://go.dev/blog/race-detector>, the data-race-detector article at <https://go.dev/doc/articles/race_detector>, the `testing` benchmarks docs at <https://pkg.go.dev/testing#hdr-Benchmarks>, and the profiling post at <https://go.dev/blog/pprof>.

## 1. What a data race actually is

A **data race** has a precise definition, and you should be able to recite it: *two goroutines access the same memory location concurrently, at least one of the accesses is a write, and there is no synchronisation event that orders the two accesses.* All three clauses matter. Two concurrent *reads* are not a race. A write and a read that are *ordered* by a synchronisation event (a channel send/receive, a mutex unlock/lock, an atomic store/load) are not a race. It takes all three — concurrency, a write, and the absence of ordering — to make a race.

The reason a race is serious, and not merely "occasionally returns a stale value," is that the Go memory model declares a racy program to have **undefined behaviour**. The compiler and the CPU are both allowed to reorder memory operations as long as a *single goroutine* cannot tell. When two goroutines race, those reorderings become visible, and you can observe things that look impossible: a pointer that is non-nil but points at a half-initialised struct, an integer that is neither its old nor its new value (a "torn" read on a platform where the write is not atomic), a loop that never sees a flag another goroutine set. The Go memory model spec states it plainly: "Programs that modify data being simultaneously accessed by multiple goroutines must serialize such access. ... If you must read the rest of this document to understand the behavior of your program, you are being too clever. Don't be clever." Citation: <https://go.dev/ref/mem>.

## 2. Happens-before — the only ordering guarantee you get

The memory model defines a partial order called **happens-before**. If event A happens-before event B, then the effects of A (the writes it made) are guaranteed visible to B. Within a single goroutine, statements happen-before in program order. *Across* goroutines, happens-before edges are established only by synchronisation events. The ones you use:

- **A channel send happens-before the corresponding receive completes.** Everything the sender did before the send is visible to the receiver after the receive.
- **A channel close happens-before a receive that returns the zero value because the channel is closed.** (This is why ranging over a closed channel sees all prior sends.)
- **A `sync.Mutex.Unlock` happens-before the next `Lock` that observes it.** Everything done under the first critical section is visible in the next.
- **A `sync/atomic` store happens-before the load that reads the stored value.**
- **`sync.Once.Do(f)` completing happens-before any other `Do` returns.** Everything `f` wrote is visible to every caller.
- **`sync.WaitGroup.Wait` returns after all `Done` calls;** the goroutines' writes before `Done` are visible after `Wait`.

If you mutate shared state in goroutine A and read it in goroutine B, and you cannot point at a happens-before edge between the write and the read, **you have a race** — even if it "works" today, even if it passes a thousand test runs. The fix is always the same in kind: add the missing synchronisation. Recall from Lecture 1 why the `errgroup` worker pool writing `results[i]` is race-free: each goroutine writes a distinct index (no two goroutines touch the same location), and `g.Wait()` establishes a happens-before edge between every write and the subsequent read. Both conditions hold; no race. Citation: <https://go.dev/ref/mem#synchronization>.

## 3. A real race, reproduced

Here is the simplest race that bites real code — an unsynchronised counter incremented from many goroutines:

```go
package main

import (
	"fmt"
	"sync"
)

func main() {
	var counter int // shared, unsynchronised — the bug
	var wg sync.WaitGroup

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter++ // RACE: read-modify-write with no synchronisation
		}()
	}
	wg.Wait()
	fmt.Println(counter) // almost never 1000
}
```

`counter++` is three operations — read `counter`, add one, write it back. With 1000 goroutines doing that with no lock, two goroutines routinely read the same value, both add one, and both write back the *same* result, losing an increment. Run it normally and you get a number a little under 1000, different every time:

```
$ go run main.go
987
$ go run main.go
991
```

The bug is invisible to the compiler and to `go vet` — it is a *runtime* property of how the goroutines interleave. That is what the race detector is for.

## 4. Running the race detector

Compile with `-race` and run:

```
$ go run -race main.go
==================
WARNING: DATA RACE
Read at 0x00c0000a4018 by goroutine 8:
  main.main.func1()
      /home/you/main.go:15 +0x2c

Previous write at 0x00c0000a4018 by goroutine 7:
  main.main.func1()
      /home/you/main.go:15 +0x44

Goroutine 8 (running) created at:
  main.main()
      /home/you/main.go:13 +0x7c

Goroutine 7 (finished) created at:
  main.main()
      /home/you/main.go:13 +0x7c
==================
987
Found 1 data race(s)
exit status 66
```

Read the report top to bottom — this is the single most important skill of the week:

1. **`WARNING: DATA RACE`** — the detector found a pair of unsynchronised accesses to the same address (`0x00c0000a4018`).
2. **`Read at ... by goroutine 8`** with a stack — one of the two racing accesses, here the read half of `counter++`, at `main.go:15`.
3. **`Previous write at ... by goroutine 7`** with a stack — the other access, the write half, at the same line.
4. **`Goroutine N created at`** — for each goroutine, *where it was spawned* (`main.go:13`, the `go func()` statement). This is how you find which `go` statement started the racing goroutine when your code has many.
5. **`exit status 66`** — the magic exit code the race detector uses for a detected race. In CI, a non-zero exit fails the build. That is the contract: **`-race` failures break the build.**

The same flag works on `go test -race`, `go build -race`, and `go install -race`. In CI you run `go test -race ./...` over the whole module. Citation: <https://go.dev/doc/articles/race_detector>.

## 5. What the detector can and cannot prove

This is the nuance that separates someone who *runs* `-race` from someone who *understands* it:

- **No false positives.** Every race the detector reports is a real race. If it fires, you have a bug. There is no "ignore this one, it is spurious." (The rare exception is racing with C code through cgo that the detector cannot see into — not relevant to pure-Go work.)
- **False negatives for un-exercised paths.** The detector instruments the accesses that *actually execute* on the run it observes. A race on a branch your test never takes, a goroutine your test never starts, an interleaving that did not happen this run — invisible. The detector cannot prove the *absence* of races; it can only find races on the paths you exercise.

The operational consequence is the week's discipline: **run your whole suite under `-race` in CI, and write tests that actually exercise the concurrent paths.** A `-race`-clean run of a test that never starts two goroutines proves nothing. A `-race`-clean run of a test that hammers your shared structure from 64 goroutines for 10,000 iterations is real evidence. The detector also adds 5-10x CPU cost and 5-10x memory, so you run it in CI and in targeted local runs, not in production. Citation: <https://go.dev/blog/race-detector> and the limitations section of <https://go.dev/doc/articles/race_detector>.

## 6. Fixing the race three ways

The counter race has three correct fixes, each illustrating a tool from Lecture 2.

**Fix 1 — a mutex:**

```go
var (
	mu      sync.Mutex
	counter int
)
// inside the goroutine:
mu.Lock()
counter++
mu.Unlock()
```

The `Unlock`-happens-before-next-`Lock` edge orders every increment. Correct, slightly heavier under contention.

**Fix 2 — an atomic** (the right tool for a single scalar):

```go
var counter atomic.Int64
// inside the goroutine:
counter.Add(1)
// after Wait:
fmt.Println(counter.Load())
```

`Add` is a single atomic instruction; every increment is ordered. This is the *idiomatic* fix for a counter — one scalar, so an atomic, not a mutex.

**Fix 3 — do not share at all** (the channel / aggregation approach):

```go
results := make(chan int, 1000)
for i := 0; i < 1000; i++ {
	wg.Add(1)
	go func() { defer wg.Done(); results <- 1 }()
}
go func() { wg.Wait(); close(results) }()
total := 0
for n := range results {
	total += n
}
```

No goroutine mutates shared state; each *sends* its contribution and a single goroutine sums them. Heavier than the atomic for a bare counter, but the right shape when the per-item work produces a richer value than `1`.

Run each fix under `-race`; all three report `0 data race(s)`. Which to ship: **Fix 2 (atomic) for a counter.** Fix 1 (mutex) if the shared state grows beyond one scalar. Fix 3 if you are already collecting per-item results anyway. Exercise 3 walks you through Fix 1 and Fix 2 and benchmarks both.

## 7. Benchmarking with `testing.B`

A benchmark is a function `BenchmarkXxx(b *testing.B)` in a `_test.go` file. The framework calls it with an ever-larger `b.N` until the per-operation time is stable:

```go
package counter

import (
	"sync"
	"sync/atomic"
	"testing"
)

func BenchmarkCounterMutex(b *testing.B) {
	var mu sync.Mutex
	var n int
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			n++
			mu.Unlock()
		}
	})
}

func BenchmarkCounterAtomic(b *testing.B) {
	var n atomic.Int64
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n.Add(1)
		}
	})
}
```

Five things:

1. **`b.N` is set by the framework, not you.** You write a loop that runs `b.N` times (here via `RunParallel`/`pb.Next()`); the framework grows `b.N` until the measurement is stable, then reports nanoseconds *per operation*.
2. **`b.RunParallel`** runs the body on `GOMAXPROCS` goroutines, which is the right shape for measuring a *concurrent* primitive under contention — exactly what a counter's cost depends on.
3. **`b.ReportAllocs()`** adds the `B/op` and `allocs/op` columns. For these counters both should be `0` — no allocation per increment. A surprise allocation in a benchmark is a finding.
4. **`b.ResetTimer()`** (not shown) is what you call after expensive setup, so the setup cost is excluded from the measured loop. Use it whenever a benchmark seeds data before the loop.
5. **Run with `go test -bench . -benchmem`:**

```
$ go test -bench . -benchmem
BenchmarkCounterMutex-8     	48000000	     24.3 ns/op	   0 B/op	  0 allocs/op
BenchmarkCounterAtomic-8    	200000000	      6.1 ns/op	   0 B/op	  0 allocs/op
```

The `-8` suffix is `GOMAXPROCS`. The first column is the final `b.N`. The atomic is ~4x faster under contention — the measured basis for Lecture 2's "atomic beats mutex for a single scalar" claim. Citation: <https://pkg.go.dev/testing#hdr-Benchmarks>.

## 8. A parameter sweep — finding where the pool stops scaling

The worker pool's interesting benchmark is a *sweep* across pool sizes, to find the point where more workers stop helping. Use `b.Run` sub-benchmarks:

```go
func BenchmarkPool(b *testing.B) {
	items := makeItems(1000)
	for _, limit := range []int{1, 4, 16, 64, 256} {
		b.Run(fmt.Sprintf("limit=%d", limit), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				_, _ = pool.Run(context.Background(), items, limit, fakeWork)
			}
		})
	}
}
```

```
BenchmarkPool/limit=1-8     	     200	   5_900_000 ns/op
BenchmarkPool/limit=4-8     	     800	   1_500_000 ns/op
BenchmarkPool/limit=16-8    	    3000	     410_000 ns/op
BenchmarkPool/limit=64-8    	    3200	     390_000 ns/op
BenchmarkPool/limit=256-8   	    3100	     400_000 ns/op
```

Read the curve: throughput improves sharply from 1 to 16 workers, then *flattens* — at `limit=64` and `limit=256` the time per run is essentially the same as `limit=16`. That flat region is the signal: you have saturated whatever the bottleneck is (here, the simulated downstream), and adding workers only adds scheduling overhead and contention. The right pool size for this workload is around 16. This is precisely the kind of evidence the Phase I gate asks you to present. Use `benchstat` (`golang.org/x/perf/cmd/benchstat`) to compare two runs with statistical confidence rather than eyeballing one. Citation: the benchmarking section of <https://go.dev/blog/pprof>.

## 9. First contact with `pprof`

When a benchmark is slow and you want to know *which function* eats the time, capture a CPU profile:

```
$ go test -bench BenchmarkPool -cpuprofile cpu.out
$ go tool pprof -top cpu.out
Showing nodes accounting for 2.31s, 92.40% of 2.50s total
      flat  flat%   sum%        cum   cum%
     0.78s 31.20% 31.20%      0.78s 31.20%  runtime.cgocall
     0.41s 16.40% 47.60%      1.12s 44.80%  linkcheck.check
     0.22s  8.80% 56.40%      0.22s  8.80%  runtime.futex
     ...
```

The `flat` column is time spent *in that function itself*; `cum` (cumulative) is time in it *plus everything it called*. The top `flat` frame is where the CPU actually went. Here `linkcheck.check` is the hot path — expected, since it does the HTTP work. `go tool pprof -http=:8080 cpu.out` opens an interactive flame graph in the browser; the widest box at the bottom is the entry point, and width is cumulative time. We do not go deeper this week — `pprof` (CPU, heap, goroutine, block, mutex profiles, reading flame graphs) is the spine of Week 8. The goal here is *first contact*: capture a profile, read the top frame, and know that the tool exists. Citation: <https://go.dev/blog/pprof> and the profiling guide at <https://go.dev/doc/diagnostics>.

## 10. Exercise pointer

Now do **Exercise 3 — Find and Fix the Race**: take a program with a deliberate race, find it with `-race`, read the report end to end, fix it with a mutex and then with an atomic, and benchmark both fixes to reproduce the ~4x gap. Then attempt **Challenge 2 — Benchmark Sweep and pprof**, which has you sweep the pool size, plot the throughput curve, and name the frame where scaling stops.

## 11. Summary

- A data race is concurrent access to one location, at least one a write, with no happens-before ordering. It is undefined behaviour, not a flaky bug.
- Happens-before edges come only from synchronisation: channel send/receive, mutex unlock/lock, atomic store/load, `Once`, `WaitGroup.Wait`.
- `-race` works on `test`, `run`, `build`, `install`. It has **no false positives** and **false negatives for un-exercised paths**.
- Read a ThreadSanitizer report: the two access stacks (read / previous write), the "created at" lines that name the `go` statements, exit status 66.
- The CI discipline: `go test -race ./...` over the whole module, with tests that exercise the concurrent paths.
- Fix a race with the right tool: atomic for one scalar, mutex for a group, channel/aggregation when collecting per-item values.
- `BenchmarkXxx(b *testing.B)` with `b.N`, `b.RunParallel`, `b.ReportAllocs`, `b.ResetTimer`; run with `-bench . -benchmem`; read ns/op and allocs/op.
- A pool-size sweep finds where throughput flattens — the right concurrency cap. `benchstat` compares runs with confidence.
- `go test -cpuprofile` + `go tool pprof -top` gives first contact with profiling; the top `flat` frame is where the CPU went. Depth comes in Week 8.

Cited references this lecture pulled from: <https://go.dev/ref/mem>, <https://go.dev/blog/race-detector>, <https://go.dev/doc/articles/race_detector>, <https://pkg.go.dev/testing#hdr-Benchmarks>, <https://go.dev/blog/pprof>, <https://go.dev/doc/diagnostics>.
