# Challenge 2 — Benchmark Sweep and First `pprof`: Find Where the Pool Stops Scaling

> **Time:** 90 minutes. **Prerequisites:** Exercise 2 (the bounded pool) and Lecture 3 (benchmarking, first `pprof`). **Deliverable:** a benchmark that sweeps the worker-pool size, a throughput curve you can read, a CPU profile, and a one-page write-up naming the frame where scaling stops.

## Statement of the problem

A bounded worker pool has a "right" size, and it is rarely "as many as possible." Past some point, adding workers stops improving throughput — you have saturated the downstream, or the CPU, or you are paying more in scheduling and contention than you gain in parallelism. Your job is to *measure* that point for a given workload rather than guess it, and to use `pprof` to name what the bottleneck is.

The headline question: for a workload that is part CPU and part simulated I/O, at what pool size does throughput flatten, and which function is on top of the CPU profile at that size?

## What you will build

A benchmark over the pool from Exercise 2, sweeping the limit, plus a workload that is deliberately a *mix* of CPU work and a simulated I/O wait.

```
src/sweep/
  sweep.go          // the pool + a mixed CPU/IO workload
  sweep_test.go     // the benchmark sweep
  SWEEP.md          // your write-up
```

The workload — some real CPU (a hash loop) plus a short sleep standing in for network I/O:

```go
package sweep

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"time"
)

// work does ~CPU-bound hashing plus a 1ms simulated I/O wait, the shape of a
// real task: compute something, then wait on a downstream.
func work(ctx context.Context, seed uint64) error {
	buf := make([]byte, 8)
	binary.LittleEndian.PutUint64(buf, seed)
	for i := 0; i < 2000; i++ { // CPU: repeated hashing
		sum := sha256.Sum256(buf)
		buf = sum[:8]
	}
	select { // I/O: 1ms simulated downstream wait
	case <-time.After(time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
```

The sweep benchmark:

```go
package sweep

import (
	"context"
	"fmt"
	"testing"
)

func BenchmarkPoolSweep(b *testing.B) {
	const items = 2000
	for _, limit := range []int{1, 2, 4, 8, 16, 32, 64, 128} {
		b.Run(fmt.Sprintf("limit=%d", limit), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				_ = RunPool(context.Background(), items, limit, work)
			}
		})
	}
}
```

(`RunPool` is your Exercise 2 `errgroup` pool, generalised to take the `work` func.)

## The measurement plan

### M1 — the throughput sweep

Run `go test -bench BenchmarkPoolSweep -benchmem`. Record the ns/op for each limit. Expect a curve like:

```
BenchmarkPoolSweep/limit=1-8     	      40	  30_000_000 ns/op
BenchmarkPoolSweep/limit=2-8     	      80	  15_200_000 ns/op
BenchmarkPoolSweep/limit=4-8     	     150	   7_900_000 ns/op
BenchmarkPoolSweep/limit=8-8     	     290	   4_100_000 ns/op
BenchmarkPoolSweep/limit=16-8    	     420	   2_900_000 ns/op
BenchmarkPoolSweep/limit=32-8    	     440	   2_700_000 ns/op
BenchmarkPoolSweep/limit=64-8    	     440	   2_750_000 ns/op
BenchmarkPoolSweep/limit=128-8   	     430	   2_800_000 ns/op
```

Find the knee: throughput improves steeply up to ~16, then flattens (and may slightly *worsen* past 64 as scheduling overhead grows). On an 8-core machine with a mixed CPU/IO workload, the knee sits a little above the core count — because the I/O wait lets more than `GOMAXPROCS` tasks make progress, but the CPU portion caps the benefit.

### M2 — the CPU profile at the knee

Capture a profile at a representative limit:

```bash
go test -bench 'BenchmarkPoolSweep/limit=16' -cpuprofile cpu.out
go tool pprof -top cpu.out
```

Read the top `flat` frame. With this workload it should be the SHA-256 hashing (`crypto/sha256.(*digest).Write` or `block`), confirming the CPU portion dominates. Open the flame graph with `go tool pprof -http=:8080 cpu.out` and confirm the widest box is the hashing path.

### M3 — vary the CPU/IO mix

Change the hash loop from 2000 to 200 iterations (more I/O-bound) and re-run the sweep. The knee should move *right* — an I/O-bound workload benefits from more workers than a CPU-bound one, because workers spend more time parked on `time.After` and less time fighting for the CPU. Report both knees and explain the shift.

## Acceptance criteria

1. `go test -bench BenchmarkPoolSweep` runs and produces a monotonic-then-flat curve.
2. `SWEEP.md` reports the ns/op for all eight limits and identifies the knee (the smallest limit within ~5% of the best time).
3. A CPU profile is captured and `go tool pprof -top` output is pasted into `SWEEP.md` with the top frame named and explained.
4. The CPU-heavy vs I/O-heavy comparison (M3) reports two knees and explains why the I/O-heavy one is further right.
5. The write-up states the pool size you would ship for this workload and why.

## A trap to watch for

`b.N` grows the *outer* loop, not the item count. Each benchmark iteration runs the whole 2000-item pool once. If you accidentally seed or allocate inside the timed loop, the allocation noise swamps the signal — move setup before the `for i := 0; i < b.N` loop and call `b.ResetTimer()` after it. The `-benchmem` `allocs/op` column should be stable across limits; a column that climbs with the limit means you are allocating per-worker inside the timed region.

## A second trap: measuring on a busy machine

A laptop running a browser, a video call, and an editor will give you a noisy curve. Close everything, run with `-count=5`, and feed the output to `benchstat`:

```bash
go test -bench BenchmarkPoolSweep -count=5 | tee sweep.txt
benchstat sweep.txt
```

`benchstat` reports the mean and the variance; a high variance (±20%) means your numbers are not trustworthy and you should re-run on a quiet machine. Install it with `go install golang.org/x/perf/cmd/benchstat@latest`.

## Submission

Submit the `sweep` package (runnable with `go test -bench .`), `cpu.out`, and `SWEEP.md` with the curve, the knee, the `pprof -top` output, the CPU-vs-IO comparison, and your shipping recommendation. A short comment block in `sweep.go` should link to the testing-benchmarks docs and the pprof post.

The rubric:

- (35%) The sweep is correctly structured (setup outside the timed loop, `ReportAllocs`, sensible limits) and produces a readable curve.
- (30%) The knee is correctly identified and the CPU profile's top frame is named and explained.
- (25%) The CPU-vs-IO comparison is run and the rightward knee shift is correctly explained.
- (10%) Citations present; `benchstat` used to justify the numbers are not noise.

Cited references: <https://pkg.go.dev/testing#hdr-Benchmarks>, <https://go.dev/blog/pprof>, <https://go.dev/doc/diagnostics>, the `benchstat` docs at <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>.
