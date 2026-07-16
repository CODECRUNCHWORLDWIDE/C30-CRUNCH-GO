# Challenge 2 — Profile and Optimize a Slow `notes` Hot Path

> **Estimated time:** 2 hours. **Prerequisite:** Lecture 2 (benchmarking and `pprof`) and `benchstat` installed (`go install golang.org/x/perf/cmd/benchstat@latest`). `graphviz` (`brew install graphviz`) is optional, for the graphical flame graph. **Citations:** the Go blog "Profiling Go Programs" at <https://go.dev/blog/pprof>, the `runtime/pprof` godoc at <https://pkg.go.dev/runtime/pprof>, and `benchstat` at <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>.

## The premise

Optimization without measurement is vandalism: you make the code harder to read and the program no faster. This challenge enforces the discipline — **measure, locate, fix, prove** — on a deliberately slow `notes` hot path. You will benchmark it, capture a CPU and a heap profile, read the profile to find where the time *actually* goes (not where you guessed), ship exactly one optimization, and prove with `benchstat` that the improvement is real and statistically significant. The deliverable is not just faster code; it is the *evidence* that it is faster, which is what a senior engineer attaches to the pull request.

## The slow path

Use the `RenderFeed` function below (or, if you prefer, the `BuildTagIndexSlow` from Exercise 2 — either is a legitimate target). `RenderFeed` renders a list of notes into a single HTML feed string with naive `+=` string concatenation and a `fmt.Sprintf` per note:

```go
// RenderFeed renders notes into one HTML string. Deliberately slow: it grows a
// string with += (quadratic copying) and allocates a Sprintf per note.
func RenderFeed(notes []Note) string {
	html := ""
	for _, n := range notes {
		html += fmt.Sprintf("<article id=%q><h2>%s</h2><p>%s</p></article>\n",
			n.ID, n.Title, n.Body)
	}
	return "<section class=feed>\n" + html + "</section>\n"
}
```

There are *two* problems hiding here, and the profile will show you both: the `+=` on a string copies the entire accumulated buffer on every iteration (O(n²) total copying), and the `fmt.Sprintf` allocates a fresh string per note. Your guess about which dominates is probably wrong — that is the point of profiling.

## Steps

### 1. Write the benchmark with a sink

```go
var feedSink string

func BenchmarkRenderFeed(b *testing.B) {
	for _, size := range []int{100, 1000, 5000} {
		b.Run("n="+strconv.Itoa(size), func(b *testing.B) {
			notes := MakeNotes(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				feedSink = RenderFeed(notes)
			}
		})
	}
}
```

Run it and record the baseline:

```bash
go test -bench=RenderFeed -benchmem -count=10 ./... > old.txt
```

Note the *shape* across sizes: if `ns/op` grows faster than linearly (4x the size → much more than 4x the time), that is the `+=` quadratic copying showing up.

### 2. Capture CPU and heap profiles

```bash
go test -bench='RenderFeed/n=5000' -benchmem \
        -cpuprofile=cpu.out -memprofile=mem.out ./...
```

### 3. Read the profiles

```bash
go tool pprof cpu.out
# (pprof) top         — look for runtime.memmove (the += copying) and runtime.mallocgc
# (pprof) list RenderFeed   — see the time on the += line vs the Sprintf line
go tool pprof -alloc_space mem.out
# (pprof) top         — see what allocated the most bytes
```

Optionally render the flame graph: `go tool pprof -http=:8080 cpu.out` (needs `graphviz`). Write down, *before you change anything*, what the profile says is the dominant cost — and compare it to what you guessed.

### 4. Ship one optimization

Replace the `+=` and `Sprintf` with a single pre-sized `strings.Builder`:

```go
func RenderFeed(notes []Note) string {
	var b strings.Builder
	b.Grow(64 * len(notes)) // pre-size to avoid repeated buffer growth
	b.WriteString("<section class=feed>\n")
	for _, n := range notes {
		b.WriteString(`<article id="`)
		b.WriteString(n.ID)
		b.WriteString(`"><h2>`)
		b.WriteString(n.Title)
		b.WriteString("</h2><p>")
		b.WriteString(n.Body)
		b.WriteString("</p></article>\n")
	}
	b.WriteString("</section>\n")
	return b.String()
}
```

The `strings.Builder` keeps one growing buffer (no per-iteration copy), `b.Grow` pre-sizes it so it does not reallocate as it fills, and the `WriteString` calls replace the allocating `Sprintf`. (If your real output needs HTML escaping, use `html/template` or `template.HTMLEscapeString` — but keep the optimization a *like-for-like* output change, verified by a test.)

### 5. Prove it

Add a correctness test first — the optimized version must produce *byte-identical* output to the original (snapshot the old output as a golden file, or assert `cmp.Diff(old, new) == ""`). Then:

```bash
go test -bench=RenderFeed -benchmem -count=10 ./... > new.txt
benchstat old.txt new.txt
```

Read the delta: the time and allocation drops, the `±` variation, and the `p` value. A `p < 0.05` means the improvement is real; a `~` means you cannot claim a win.

## Acceptance criteria

- [ ] The benchmark has a package-level sink; `ns/op` is plausible (not sub-nanosecond).
- [ ] You captured both a CPU profile and a heap profile from the slow path.
- [ ] You recorded the profile's dominant cost (`top` output) *before* optimizing, and noted whether it matched your guess.
- [ ] A correctness test proves the optimized output is byte-identical to the original.
- [ ] You shipped exactly **one** optimization (resist the urge to rewrite five things; isolate the change so the `benchstat` delta is attributable).
- [ ] `benchstat old.txt new.txt` shows the improvement with `p < 0.05`.
- [ ] `RESULTS.md` includes the `top` output, one `list` snippet pointing at the hot line, and the `benchstat` table.

## Reflection (write into `RESULTS.md`)

1. **The profile vs. your guess.** Before profiling, which line did you think was the bottleneck — the `+=` or the `Sprintf`? What did the profile actually say? `runtime.memmove` high in the CPU profile points at the `+=` copying; `runtime.mallocgc` and `fmt.Sprintf` high point at the allocation. Which dominated, and at what `n`? (The `+=` quadratic cost grows with `n`; at small `n` the `Sprintf` allocation may dominate, at large `n` the copying does — the crossover is itself interesting.)

2. **The scaling shape.** Plot or tabulate `ns/op` across your three sizes for the old and new versions. The old version should grow super-linearly (quadratic copying); the new version should grow linearly. What does the `benchstat` delta look like at `n=100` vs `n=5000`, and why is the win *bigger* at larger `n`?

3. **Why `b.Grow`?** Run the new version with and without the `b.Grow(64 * len(notes))` line and compare with `benchstat`. How much does pre-sizing buy you? What is the failure mode of guessing the size wrong (too small → some reallocation; too large → wasted memory)?

4. **When to stop.** After this one optimization, is the function fast enough? How would you decide — what `ns/op` target, derived from what request budget? Optimization has diminishing returns; name the point at which a second optimization would not be worth the loss of readability.

## Stretch goals (optional)

- **Allocation deep-dive.** Run `go tool pprof -alloc_objects mem.out` (object count, not bytes) and compare with `-alloc_space` (bytes). When would the object count matter more than the byte count? (GC cost scales with object count.)
- **`-race` under load.** If your real hot path is called concurrently, add a `BenchmarkRenderFeedParallel` with `b.RunParallel` and run it under `-race`. Does the optimization hold up under contention?
- **A second target.** Profile a different `notes` hot path — JSON encoding of a list response, or the tag-index build from Exercise 2 — and apply the same measure-locate-fix-prove loop. Two worked optimizations, two `benchstat` deltas.

## Submission

Place under your repo:

- The slow and optimized `RenderFeed` (in git history, so the diff is visible), the benchmark, and the correctness test.
- `old.txt`, `new.txt`, and the `RESULTS.md` with the `top`/`list`/`benchstat` evidence and the four reflection answers.

Commit with the message:

```
challenge-02: profile and optimize the notes feed render path
```

Push and open a PR. The PR description must include the `benchstat` table — the reviewer should be able to see the `p < 0.05` win without running anything.
