# Lecture 2 — Benchmarking with `testing.B`, Comparing Runs with `benchstat`, and Profiling with `pprof`

> **Time:** 2.5 hours. Take the `testing.B` section first, the `benchstat` section second, and the `pprof` section — the longest — last, with a terminal open so you run every command as you read it. **Prerequisites:** Lecture 1 (the `testing` package). **Citations:** the `testing` package godoc at <https://pkg.go.dev/testing>, the Go blog "Profiling Go Programs" at <https://go.dev/blog/pprof>, the `runtime/pprof` godoc at <https://pkg.go.dev/runtime/pprof>, `net/http/pprof` at <https://pkg.go.dev/net/http/pprof>, the `pprof` tool at <https://github.com/google/pprof>, and `benchstat` at <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>.

## 1. Why measure instead of guess

The first rule of optimization is that you are wrong about where the time goes. Not sometimes — usually. A function you "know" is the bottleneck turns out to be 2% of the runtime; a `fmt.Sprintf` you never thought about turns out to be 40%, because it allocates on every call and you call it in a loop a million times. The cost of acting on a wrong guess is real: you rewrite the wrong function, the program gets no faster, and now the code is harder to read for no benefit. Go gives you two tools that replace the guess with a measurement — `testing.B` benchmarks (how fast, how many allocations) and `pprof` profiles (where the time and the allocations actually go). This lecture makes you fluent in both, and ties them together in a worked optimization where the profile contradicts the guess.

## 2. `testing.B`: the benchmark

A benchmark is a function `func BenchmarkXxx(b *testing.B)` in a `_test.go` file. The body runs the code under test inside a loop bounded by `b.N`:

```go
func BenchmarkSlugify(b *testing.B) {
	input := "Hello, World! This Is A Title With Punctuation."
	for i := 0; i < b.N; i++ {
		_ = Slugify(input)
	}
}
```

(On Go 1.24+ you may also write `for b.Loop() { ... }`, which is equivalent and slightly clearer; the `for i := 0; i < b.N; i++` form is universal across 1.22+ and is what we use here.) The framework chooses `b.N`: it runs the loop a few times to estimate the per-iteration cost, then runs enough iterations (scaling `b.N` up) to get a statistically stable timing — by default until the benchmark has run for about a second of wall-clock. You do not pick `b.N`; you only ensure your loop body does one unit of the work you want to measure, `b.N` times.

Run it:

```
$ go test -bench=BenchmarkSlugify -benchmem ./internal/notes/
goos: darwin
goarch: arm64
pkg: github.com/crunch/notes/internal/notes
BenchmarkSlugify-10    	 3214874	       372.6 ns/op	     112 B/op	       4 allocs/op
PASS
ok   github.com/crunch/notes/internal/notes   1.612s
```

Read the line:

- **`BenchmarkSlugify-10`** — the name, and the `-10` is `GOMAXPROCS` (10 cores on this machine).
- **`3214874`** — the final `b.N`: the loop ran ~3.2 million times to get a stable measurement.
- **`372.6 ns/op`** — nanoseconds per operation. This is the headline number: one `Slugify` call takes ~373 ns.
- **`112 B/op`** — bytes allocated on the heap per operation (only printed with `-benchmem` or `b.ReportAllocs()`).
- **`4 allocs/op`** — number of distinct heap allocations per operation.

`ns/op` is how fast; `B/op` and `allocs/op` are how much garbage you make. The allocation numbers are frequently the ones that matter, because each allocation is work for the garbage collector, and in a hot path the GC pressure can dominate the actual computation. A change that drops `allocs/op` from 4 to 0 often beats a change that shaves nanoseconds off the arithmetic.

### 2.1 `b.ReportAllocs`, `b.ResetTimer`, `b.StopTimer`/`b.StartTimer`

- **`b.ReportAllocs()`** at the top of the benchmark forces the `B/op`/`allocs/op` columns even without the `-benchmem` flag. Put it in benchmarks where allocation is the thing you care about, so the numbers show up regardless of how someone runs the benchmark.
- **`b.ResetTimer()`** discards everything timed so far. Use it after expensive setup that should not count toward the per-op cost:

```go
func BenchmarkSearchIndex(b *testing.B) {
	idx := buildLargeIndex(100_000) // expensive; must not count
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = idx.Search("golang")
	}
}
```

- **`b.StopTimer()` / `b.StartTimer()`** pause and resume the clock around per-iteration setup that should not count. Use sparingly — frequent stop/start adds overhead and can distort small benchmarks. Prefer hoisting setup out of the loop and using `ResetTimer`.

### 2.2 The dead-code-elimination trap and the sink

Here is the most important pitfall in Go benchmarking. The compiler is allowed to delete code whose result is unused. If your benchmark body is `_ = Slugify(input)` and `Slugify` has no side effects, the optimizer may notice the result is discarded and eliminate the call entirely — and your benchmark reports an impossibly fast `0.25 ns/op`, which is the cost of an empty loop. The fix is to make the result *escape* into a package-level variable the compiler cannot prove is unused — a **sink**:

```go
var sink string

func BenchmarkSlugify(b *testing.B) {
	input := "Hello, World!"
	var s string
	for i := 0; i < b.N; i++ {
		s = Slugify(input)
	}
	sink = s // assign the last result to a package-level var: defeats DCE
}
```

For benchmarks that return different types, use a `var sink any` or a typed sink per benchmark. The rule: **the result of the function under test must reach a package-level variable, or the benchmark may be measuring nothing.** A `ns/op` that is suspiciously close to zero (sub-nanosecond) is almost always dead-code elimination eating the body.

### 2.3 Sub-benchmarks with `b.Run`

Just as `t.Run` gives named subtests, `b.Run` gives named sub-benchmarks — ideal for benchmarking the same code across input sizes:

```go
func BenchmarkBuildTagIndex(b *testing.B) {
	for _, size := range []int{10, 100, 1000, 10_000} {
		b.Run(fmt.Sprintf("n=%d", size), func(b *testing.B) {
			notes := makeNotes(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkIndex = BuildTagIndex(notes)
			}
		})
	}
}
```

Output:

```
BenchmarkBuildTagIndex/n=10-10      	  482106	      2487 ns/op	    2304 B/op	      21 allocs/op
BenchmarkBuildTagIndex/n=100-10     	   47988	     24910 ns/op	   23808 B/op	     205 allocs/op
BenchmarkBuildTagIndex/n=1000-10    	    4791	    249830 ns/op	  238 KB/op	    2009 allocs/op
BenchmarkBuildTagIndex/n=10000-10   	     478	   2498100 ns/op	 2.38 MB/op	   20009 allocs/op
```

This makes the *scaling* visible: `ns/op` and `allocs/op` both grow roughly linearly with `n`, which tells you the algorithm is O(n) — and the ~2 allocs per note hints at where the garbage comes from (Section 6).

### 2.4 `b.RunParallel` and `pb.Next`

To benchmark a concurrent path — something safe to call from many goroutines, like a read from a `sync.Map` or a sharded cache — use `b.RunParallel`:

```go
func BenchmarkCacheGet(b *testing.B) {
	c := newCache()
	c.Set("k", "v")
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = c.Get("k")
		}
	})
}
```

`b.RunParallel` spawns `GOMAXPROCS` goroutines, each running the body until the collective iteration count reaches `b.N`. `pb.Next()` returns `false` when the work is exhausted. This measures throughput under contention — exactly what `b.N` in a single goroutine cannot show you. Documented at <https://pkg.go.dev/testing#B.RunParallel>.

## 3. Comparing runs with `benchstat`

A single benchmark run is noise plus signal. Your laptop thermal-throttles, a background process steals a core, the GC fires at a different point. To make a defensible claim that "B is faster than A," you run each *several times* and compare with `benchstat`, which computes the mean, the variation, and a *p-value* that tells you whether the difference is statistically real or could be noise.

Install it once: `go install golang.org/x/perf/cmd/benchstat@latest`.

The workflow for an A/B comparison:

```
# Capture the baseline, 10 runs:
$ go test -bench=BenchmarkBuildTagIndex -benchmem -count=10 ./internal/notes/ > old.txt

# Make your optimization, then capture the new numbers, 10 runs:
$ go test -bench=BenchmarkBuildTagIndex -benchmem -count=10 ./internal/notes/ > new.txt

# Compare:
$ benchstat old.txt new.txt
```

```
goos: darwin
goarch: arm64
pkg: github.com/crunch/notes/internal/notes
                      │   old.txt   │              new.txt               │
                      │   sec/op    │   sec/op     vs base               │
BuildTagIndex/n=1000-10  249.8µ ± 2%   88.4µ ± 1%  -64.61% (p=0.000 n=10)

                      │   old.txt    │              new.txt               │
                      │     B/op     │     B/op      vs base              │
BuildTagIndex/n=1000-10  238.0Ki ± 0%   41.2Ki ± 0%  -82.69% (p=0.000 n=10)

                      │  old.txt   │             new.txt              │
                      │ allocs/op  │ allocs/op   vs base              │
BuildTagIndex/n=1000-10  2009 ± 0%    9 ± 0%  -99.55% (p=0.000 n=10)
```

Read it:

- **`-64.61%`** — the new code is 64.6% faster on time. The `± 2%` and `± 1%` are the run-to-run variation; small percentages mean stable measurements.
- **`(p=0.000 n=10)`** — `n=10` is the number of runs per side; `p` is the p-value. **A p-value below 0.05 means the difference is statistically significant** — the improvement is real, not measurement noise. A p-value above 0.05 (or `benchstat` printing `~`) means "no statistically significant difference," and you should *not* claim a win. This is the discipline: do not ship "an optimization" whose `benchstat` says `~`.
- The allocation row is the story here: `2009 → 9` allocs/op, a 99.55% drop. That is the change that bought the time.

**Always use `-count=10` (or more) and `benchstat` for any optimization claim.** One run before and one run after is not evidence; it is an anecdote. Citation: the `benchstat` documentation, <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>.

## 4. `pprof`: where the time and allocations actually go

`benchstat` tells you a function got faster. `pprof` tells you *why* a function is slow in the first place — which lines, which calls, which allocations. There are two ways to get a profile.

**From a benchmark** (the common case for optimization work): `go test` writes profile files with flags.

```
$ go test -bench=BuildTagIndex -benchmem \
    -cpuprofile=cpu.out -memprofile=mem.out ./internal/notes/
```

**From a running service** (for production-like investigation): import `net/http/pprof` for its side effect, which registers handlers under `/debug/pprof/`:

```go
import _ "net/http/pprof" // registers /debug/pprof/* on http.DefaultServeMux

func main() {
	go func() {
		// expose pprof on a separate, internal-only port
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
	// ... your real server ...
}
```

Then `go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30` captures a 30-second CPU profile from the live process. **Never expose `/debug/pprof` on a public interface** — it leaks internals and lets anyone DoS you by requesting expensive profiles. Bind it to localhost or an internal admin port. Citation: <https://pkg.go.dev/net/http/pprof>.

### 4.1 The five profile types and the symptom each diagnoses

| Profile | Flag / endpoint | Answers the question | Reach for it when |
|---------|-----------------|----------------------|-------------------|
| **CPU** | `-cpuprofile` / `/debug/pprof/profile` | Where are the CPU cycles going? | A function is hot; the program is CPU-bound; latency is high under load with high CPU. |
| **Heap** | `-memprofile` / `/debug/pprof/heap` | What is allocating, and what is still live? | High `allocs/op`; GC running constantly; memory growing. |
| **Goroutine** | `/debug/pprof/goroutine` | How many goroutines, and where are they stuck? | Goroutine count climbs and never falls — a leak. |
| **Block** | `-blockprofile` / `/debug/pprof/block` | What is waiting on channels and locks? | Latency is high but CPU is low — something is blocked. |
| **Mutex** | `-mutexprofile` / `/debug/pprof/mutex` | What is contended? | Many goroutines fight over one lock; throughput stalls under concurrency. |

Block and mutex profiling are **off by default** because they add overhead. Enable them in code:

```go
runtime.SetBlockProfileRate(1)       // sample every blocking event (1 = all; higher = sample 1/N)
runtime.SetMutexProfileFraction(1)   // sample every mutex contention event
```

Set these only while investigating, or with a high `N` in production to keep the overhead small. Documented at <https://pkg.go.dev/runtime#SetBlockProfileRate> and <https://pkg.go.dev/runtime#SetMutexProfileFraction>.

The matching of symptom to profile is the senior skill. "Goroutines climbing" → goroutine profile, look for thousands of goroutines parked on the same `chan receive`. "CPU pegged at 100%" → CPU profile, find the hot loop. "p99 latency high, CPU low" → block profile, find what is waiting. Picking the wrong profile wastes an afternoon staring at a graph that cannot show the problem.

### 4.2 Reading a CPU profile with `go tool pprof`

```
$ go tool pprof cpu.out
File: notes.test
Type: cpu
(pprof) top
Showing nodes accounting for 1.48s, 92.50% of 1.60s total
      flat  flat%   sum%        cum   cum%
     0.62s 38.75% 38.75%      0.62s 38.75%  runtime.mallocgc
     0.28s 17.50% 56.25%      0.94s 58.75%  fmt.Sprintf
     0.21s 13.13% 69.38%      0.21s 13.13%  runtime.memmove
     0.18s 11.25% 80.63%      0.18s 11.25%  strings.(*Builder).WriteString
     0.10s  6.25% 86.88%      1.42s 88.75%  notes.BuildTagIndex
     ...
```

Read the columns:

- **`flat`** — time spent *in this function itself*, not its callees. `runtime.mallocgc` at 38.75% flat means the program spent more than a third of its CPU *allocating memory*. That is the smoking gun — too many allocations.
- **`cum`** (cumulative) — time in this function *and everything it called*. `notes.BuildTagIndex` at 88.75% cum is the entry point that drives almost all the work; `fmt.Sprintf` at 58.75% cum says that one call (and its callees, including `mallocgc`) accounts for more than half the runtime.

`fmt.Sprintf` showing up high, with `runtime.mallocgc` dominating flat, is a classic pattern: the code is building strings with `Sprintf` in a hot loop, and each `Sprintf` allocates. The fix is to stop allocating — `strings.Builder`, pre-sized slices, byte buffers (Section 6).

**`list`** annotates a function line-by-line with the time on each line:

```
(pprof) list BuildTagIndex
Total: 1.60s
ROUTINE ======================== notes.BuildTagIndex
      10ms      1.42s (flat, cum) 88.75% of Total
         .          .     12:func BuildTagIndex(notes []Note) map[string][]string {
         .          .     13:	idx := map[string][]string{}
         .       940ms     15:		key := fmt.Sprintf("%s:%d", tag, len(tag))   // <-- the cost is here
      10ms      480ms     16:		idx[key] = append(idx[key], n.ID)
         .          .     17:	}
         .          .     18:	return idx
         .          .     19:}
```

Line 15 — the `fmt.Sprintf` — carries 940ms of the 1.42s. The profile has pointed at the exact line. **`web`** renders a call graph (and `go tool pprof -http=:8080 cpu.out` opens an interactive flame graph in the browser), but `top` and `list` are usually enough and need no `graphviz`.

### 4.3 Reading the flame graph

`go tool pprof -http=:8080 cpu.out` opens a browser view; the "Flame Graph" tab shows a stacked-bar visualization where **width is proportional to time** and **stacking is the call hierarchy** (callers below, callees above). You read it top-down for "what is the program doing" and you look for the *widest* boxes — those are where the time is. A wide `runtime.mallocgc` box near the top, fed by a wide `fmt.Sprintf` below it, fed by `BuildTagIndex` below that, is the same story `top` and `list` told, drawn as rectangles. The flame graph is best for seeing the *shape* — one dominant spike vs. time spread thinly across many call paths — at a glance. Citation: the Go blog "Profiling Go Programs," <https://go.dev/blog/pprof>, and the `pprof` tool README, <https://github.com/google/pprof>.

## 5. Reading a heap (memory) profile

The CPU profile said `mallocgc` is hot; the heap profile says *what* is being allocated.

```
$ go tool pprof -alloc_space mem.out
(pprof) top
Showing nodes accounting for 238MB, 99.10% of 240MB total
      flat  flat%   sum%        cum   cum%
     201MB 83.75% 83.75%      201MB 83.75%  fmt.Sprintf
      37MB 15.42% 99.17%       37MB 15.42%  notes.BuildTagIndex
```

`pprof` has two heap views worth knowing: **`-alloc_space`** (total bytes allocated over the run — what creates GC work) and **`-inuse_space`** (bytes still live at the sample — what a memory leak shows up in). For an allocation-rate problem you want `-alloc_space`; for a "memory keeps growing" leak you want `-inuse_space`. Here `-alloc_space` confirms `fmt.Sprintf` allocated 201MB across the benchmark — the GC pressure the CPU profile blamed.

## 6. A worked optimization: benchmark, profile, fix, re-benchmark

Put it together. The slow code:

```go
// BuildTagIndex maps each tag to the IDs of the notes that carry it.
func BuildTagIndex(notes []Note) map[string][]string {
	idx := map[string][]string{}
	for _, n := range notes {
		for _, tag := range n.Tags {
			key := fmt.Sprintf("%s:%d", tag, len(tag)) // builds a string per (note,tag)
			idx[key] = append(idx[key], n.ID)
		}
	}
	return idx
}
```

**Step 1 — benchmark.** `BuildTagIndex/n=1000-10` reports `249.8µs/op, 238 KB/op, 2009 allocs/op` (Section 3, `old.txt`).

**Step 2 — profile.** The CPU profile shows `runtime.mallocgc` at 38% flat and `fmt.Sprintf` at 58% cum; `list` points at the `fmt.Sprintf` line (Section 4.2). The guess might have been "the map is slow" or "append is slow" — the profile says no, it is the `Sprintf` allocating a fresh string on every inner-loop iteration.

**Step 3 — fix.** The `Sprintf` is doing string formatting in the hottest loop. We do not need `Sprintf` at all — the key can be built without allocation-heavy formatting, and we can pre-size the maps and slices so `append` does not repeatedly grow and copy:

```go
func BuildTagIndex(notes []Note) map[string][]string {
	idx := make(map[string][]string, len(notes)) // pre-size the map
	var b strings.Builder
	for _, n := range notes {
		for _, tag := range n.Tags {
			b.Reset()
			b.WriteString(tag)
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(len(tag))) // no Sprintf, no per-call alloc churn
			key := b.String()
			idx[key] = append(idx[key], n.ID)
		}
	}
	return idx
}
```

The `strings.Builder` reuses one backing buffer across the loop (with `b.Reset()` between keys), `strconv.Itoa` is cheaper than `Sprintf`, and pre-sizing the map avoids rehashing. (If the key is genuinely needed as a stable string for map lookup you still allocate it once per distinct key — but you have eliminated the per-iteration `Sprintf` allocation churn.)

**Step 4 — re-benchmark and compare.** `benchstat old.txt new.txt` reports `-64.61%` on time, `-82.69%` on bytes, `-99.55%` on allocations, all with `p=0.000` (Section 3, the `benchstat` block). The optimization is real and significant — and you can prove it, line by line, from the profile that pointed you at the `Sprintf` in the first place. That is the loop: **measure (benchmark) → locate (profile) → fix → prove (benchstat).** No guessing at any step.

## 7. Wrap-up — the measurement checklist

When you benchmark and profile this week:

- [ ] Every benchmark has a package-level *sink*; no sub-nanosecond `ns/op` (that is dead-code elimination).
- [ ] `b.ReportAllocs()` is on for benchmarks where allocation matters; you read `B/op` and `allocs/op`, not just `ns/op`.
- [ ] Expensive setup is hoisted out of the `b.N` loop and the timer is reset with `b.ResetTimer()`.
- [ ] Optimization claims are backed by `benchstat` over `-count=10` runs, with `p < 0.05`.
- [ ] You picked the profile that matches the symptom (CPU / heap / goroutine / block / mutex).
- [ ] You read the profile (`top`, `list`) before changing code — the profile chose the target, not your intuition.
- [ ] Block and mutex profiling are enabled with `SetBlockProfileRate` / `SetMutexProfileFraction` only while investigating.
- [ ] `net/http/pprof` is bound to localhost / an internal port, never a public interface.

Read "Profiling Go Programs" before Friday — <https://go.dev/blog/pprof>. The exercise for this lecture (`exercise-02-benchmark-and-pprof`) gives you a slow path, a fast path, and the benchmark scaffolding to reproduce this exact `benchstat` delta.

Next lecture: fuzzing with `testing.F` to find the inputs you did not think of, and integration tests against a real Postgres with `testcontainers-go`.
