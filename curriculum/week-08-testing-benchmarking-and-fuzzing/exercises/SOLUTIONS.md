# Exercise Solutions — Week 8

These annotated solutions assume you have made a serious attempt at each exercise. Read your own attempt against the explanations below; do not copy without trying first. All three exercises live in one package (`notesx`) so they share the `Note` type and compile together — run everything with `go test ./...` from the `exercises/` directory (after `go mod init` and `go get github.com/google/go-cmp/cmp`).

---

## Exercise 1 — Table-driven tests and a golden file

### Key correctness properties

- **The validation table uses `(err != nil) != tc.wantErr` plus `errors.Is`.** The boolean check confirms you got an error iff you expected one; `errors.Is(err, tc.wantErrIs)` confirms it is the *right* error. Two separate assertions, because "errored" and "errored for the right reason" are different claims. (The exercise's `errorIs` helper is a hand-rolled `errors.Is`; in real code call `errors.Is` directly.)
- **`t.Parallel()` is called on the parent test and on each subtest.** Because C30 targets Go 1.22+, no `tc := tc` shadow is needed — each `range` iteration already binds a fresh `tc`. On Go 1.21 and earlier this code would have tested the last case eleven times.
- **Boundary cases are in the table.** `boundary title length ok` (exactly `MaxTitleLen`) and `boundary tag length ok` (exactly `MaxTagLen`) prove the comparison is `>` and not `>=`. Off-by-one in a length check is the classic validation bug; the table catches it for free.
- **The golden test renders a *fixed* note and compares with `cmp.Diff`.** `RenderNoteMarkdown` sorts tags, so the output is deterministic — a precondition for a stable golden file.

### Expected test output

```
$ go test ./...
ok      notesx  0.021s
```

### The golden file (`testdata/render_note.golden`)

After `go test -run TestRenderNoteMarkdown -update`, the file contains exactly:

```
# Release Notes

We shipped **fuzzing** support.

See the [docs](https://go.dev).

**Tags:** `announce`, `go`, `release`
```

(Note the tags are sorted alphabetically, and there is a single trailing newline.) Read it, confirm it is what you intended, then commit it. On every subsequent `go test`, the render is compared against this file; a drift shows up as a unified diff in the failure output.

### Reflection answers

1. **Why `cmp.Diff` over `reflect.DeepEqual` for the golden comparison?** `reflect.DeepEqual(string(want), got)` returns a bare `false` on mismatch — you then have to print both strings and eyeball the difference. `cmp.Diff` returns the *unified diff*, so the failure log shows you the exact line that changed. For a 40-line Markdown document, that is the difference between a useful failure and a useless one.

2. **Why trim the title before the empty check?** A title of `"   "` is, for a human, empty — it carries no information. Trimming first means `ErrEmptyTitle` fires for both `""` and whitespace-only titles, which is the behaviour a user expects. The table tests both (`empty title` and `whitespace title`) and asserts the same sentinel.

3. **When would you *not* use a golden file?** When the output is small and stable enough to write inline (a one-line string), or when it is non-deterministic in a way you cannot normalize (embeds a timestamp or a random ID you cannot strip). Golden files shine for large, deterministic output; they are overkill for a single short string.

---

## Exercise 2 — Benchmark, profile, optimize

### Key correctness properties

- **`BenchmarkBuildTagIndexSlow` / `Fast` both assign to the package-level `sinkIndex`.** Without the sink, the compiler can prove the result is unused and delete the call, reporting a meaningless sub-nanosecond `ns/op`. The sink forces the work to happen.
- **`b.ReportAllocs()` and `b.ResetTimer()` are both present**, the first so `B/op`/`allocs/op` always print, the second so the `MakeNotes(size)` setup does not count toward the per-op time.
- **`TestTagIndexImplementationsAgree` is the differential test.** A "faster" function that returns a *different* answer is not an optimization, it is a regression. The test asserts `cmp.Diff(slow, fast) == ""` for several sizes — including `n=0`, the empty case that off-by-one bugs love.

### Expected benchmark output (illustrative; numbers vary by machine)

```
$ go test -bench='BuildTagIndex' -benchmem ./...
goos: darwin
goarch: arm64
pkg: notesx
BenchmarkBuildTagIndexSlow/n=100-10      	   47988	     24910 ns/op	   23808 B/op	     205 allocs/op
BenchmarkBuildTagIndexSlow/n=1000-10     	    4791	    249830 ns/op	  243712 B/op	    2009 allocs/op
BenchmarkBuildTagIndexSlow/n=10000-10    	     478	   2498100 ns/op	 2437120 B/op	   20009 allocs/op
BenchmarkBuildTagIndexFast/n=100-10      	  142806	      8390 ns/op	    8208 B/op	       9 allocs/op
BenchmarkBuildTagIndexFast/n=1000-10     	   13561	     88420 ns/op	   42240 B/op	       9 allocs/op
BenchmarkBuildTagIndexFast/n=10000-10    	    1364	    878300 ns/op	  401664 B/op	       9 allocs/op
PASS
```

The headline is the `allocs/op` column: the slow path allocates ~2 per note (one `Sprintf` string per tag, two tags per note), the fast path allocates a near-constant 9 regardless of size (the map plus a handful of slice grows). Allocation count, not raw arithmetic, is what separates them.

### Expected pprof finding

```
$ go test -bench='BuildTagIndexSlow/n=10000' -benchmem \
        -cpuprofile=cpu.out -memprofile=mem.out ./...
$ go tool pprof cpu.out
(pprof) top
      flat  flat%   sum%        cum   cum%
     0.62s 38.75% 38.75%      0.62s 38.75%  runtime.mallocgc
     0.28s 17.50% 56.25%      0.94s 58.75%  fmt.Sprintf
     ...
(pprof) list BuildTagIndexSlow
         .       940ms     ...:		key := fmt.Sprintf("%s:%d", tag, len(tag))
```

`runtime.mallocgc` dominating `flat` is the signature of an allocation problem; `list` points at the `fmt.Sprintf` line as the source. The heap profile confirms it:

```
$ go tool pprof -alloc_space mem.out
(pprof) top
     201MB 83.75% 83.75%      201MB 83.75%  fmt.Sprintf
```

### Expected benchstat delta

```
$ go test -bench='BuildTagIndexSlow/n=1000' -benchmem -count=10 ./... > old.txt
# (edit BuildTagIndexSlow to match BuildTagIndexFast, or compare the two benchmark
#  names directly by renaming) — then:
$ benchstat old.txt new.txt
BuildTagIndex/n=1000-10   sec/op   249.8µ ± 2%   88.4µ ± 1%   -64.61% (p=0.000 n=10)
BuildTagIndex/n=1000-10   B/op     238.0Ki ± 0%  41.2Ki ± 0%  -82.69% (p=0.000 n=10)
BuildTagIndex/n=1000-10   allocs/op  2009 ± 0%       9 ± 0%   -99.55% (p=0.000 n=10)
```

`p=0.000` is well below 0.05: the improvement is statistically significant, not noise.

### Reflection answers

1. **What did the profile reveal that a guess might have missed?** A reasonable guess is "map operations are slow" or "append reallocates." The profile says no — the map and append are cheap; the cost is `fmt.Sprintf` *allocating a string on every iteration*. Optimizing the map would have bought nothing. The profile redirected the effort to the line that actually mattered.

2. **Why does `allocs/op` matter more than `ns/op` here?** Each allocation is work for the garbage collector, and in a service under load that GC work competes with request handling — it shows up as tail-latency on *unrelated* requests. Dropping `allocs/op` from 2009 to 9 removes that pressure. The `ns/op` win is a consequence of the allocation win, not a separate effect.

3. **When is the slow version acceptable?** When `n` is small and the function is called rarely — a config-load path that runs once at startup over ten items does not need a `strings.Builder`. Optimize the hot path the profile points at; leave the cold path readable. Premature optimization of cold code is a cost (less readable) with no benefit.

---

## Exercise 3 — Write a fuzz target that finds a real crash

### The bug

`ParseQuery`'s `tag` branch reads `val[0]` to reject a leading `#`, without first checking `len(val) > 0`:

```go
case "tag":
	if val[0] == '#' { // panics when val == "" (input "tag:")
		return Query{}, fmt.Errorf("tag value must not start with '#': %q", field)
	}
	q.Tags = append(q.Tags, val)
```

The input `"tag:"` produces `val == ""`, and `val[0]` panics with `index out of range [0] with length 0`. No hand-written test thinks of `tag:` with an empty value; the fuzzer reaches it in milliseconds by deleting the `go` from the seed `tag:go`.

### The fix

Check the length before indexing, and reject the empty value explicitly so it does not become an empty tag:

```go
case "tag":
	if val == "" {
		return Query{}, fmt.Errorf("empty tag value in %q", field)
	}
	if val[0] == '#' { // now safe: val is guaranteed non-empty
		return Query{}, fmt.Errorf("tag value must not start with '#': %q", field)
	}
	q.Tags = append(q.Tags, val)
```

The equivalent one-liner `if len(val) > 0 && val[0] == '#'` also avoids the panic, but it would then `append` an empty tag for input `tag:`, which violates the "no empty tags" invariant the fuzz target asserts — so the engine would *still* fail, just on invariant 2 instead of a panic. The complete fix rejects the empty value outright, which is why the solution above checks `val == ""` first.

### Expected output before the fix

```
$ go test -run='^$' -fuzz=FuzzParseQuery -fuzztime=30s ./...
fuzz: elapsed: 0s, gathering baseline coverage: 0/7 completed
fuzz: elapsed: 0s, gathering baseline coverage: 7/7 completed, now fuzzing with 10 workers
fuzz: minimizing 38-byte failing input file
fuzz: elapsed: 1s, minimizing
--- FAIL: FuzzParseQuery (1.04s)
    --- FAIL: FuzzParseQuery (0.00s)
        testing.go:1591: panic: runtime error: index out of range [0] with length 0
        [signal SIGSEGV: segmentation violation]

        goroutine 12 [running]:
        notesx.ParseQuery({...})
            /Users/you/notesx/exercise-03-fuzz-target.go:82

    Failing input written to testdata/fuzz/FuzzParseQuery/3f8a1c9b2e7d4506
    To re-run:
    go test -run=FuzzParseQuery/3f8a1c9b2e7d4506
FAIL
exit status 1
```

The crasher file `testdata/fuzz/FuzzParseQuery/3f8a1c9b2e7d4506` contains:

```
go test fuzz v1
string("tag:")
```

Four characters. Minimized by the engine from whatever larger mutation first triggered the panic.

### Expected output after the fix

```
$ go test -run=FuzzParseQuery/3f8a1c9b2e7d4506 ./...
ok      notesx  0.014s

$ go test -run='^$' -fuzz=FuzzParseQuery -fuzztime=30s ./...
fuzz: elapsed: 30s, execs: 4988201 (166273/sec), new interesting: 0 (total: 25)
PASS
ok      notesx  30.142s
```

The committed crasher now passes (it is a permanent regression test), and a fresh 30-second fuzz run finds nothing new.

### Reflection answers

1. **Why does the never-panic invariant need no explicit assertion?** A panic inside the `f.Fuzz` body is caught by the testing framework and reported as a failure automatically. So simply *calling* `ParseQuery(in)` is a never-panic assertion: if any input panics, the target fails. The explicit assertions (no empty tags, round-trip) catch the *logic* bugs that do not panic.

2. **Why compare `q.canonical()` rather than `q` directly in the round-trip check?** `Query.String()` sorts tags and terms for a canonical serialization, so re-parsing yields the same *set* of tags/terms but possibly in a different *slice order* than the original parse. Comparing the canonical (sorted) forms isolates the invariant you care about — "the same query came back" — from the irrelevant ordering difference. Comparing raw slices would produce false failures.

3. **Why commit the crasher file?** It turns a bug the fuzzer found *once* into a regression test that runs *forever* on every `go test` (no `-fuzz` needed — corpus entries run as ordinary subtests). If someone later reintroduces the bug, the committed crasher fails immediately, before the change merges. The fuzzer found it; the corpus keeps it found.

---

## Common mistakes across the three exercises

- **Benchmarking without `b.ReportAllocs()`.** You see `ns/op` but not `B/op`/`allocs/op`, and you miss that the slow path's problem is allocation, not arithmetic. Turn it on in any benchmark where memory could be the issue — which is most of them.
- **Dead-code elimination eating the benchmark.** A benchmark body of `_ = Fn(x)` with no sink can be deleted by the optimizer, reporting a sub-nanosecond `ns/op`. If your benchmark is implausibly fast, you are measuring an empty loop. Assign to a package-level sink.
- **One run before, one run after — no `benchstat`.** A single before/after pair is an anecdote, not evidence; laptop thermal throttling alone can swing a run 15%. Always `-count=10` and `benchstat`, and do not claim a win when `benchstat` prints `~` (no significant difference).
- **A fuzz target with no invariant.** `f.Fuzz(func(t *testing.T, s string) { ParseQuery(s) })` with the result discarded can only catch panics, not logic bugs. Add the round-trip or output-validity invariant so the target can actually *find* a wrong answer, not just a crash.
- **Over-mocking.** Reaching for a generated mock with `EXPECT().Foo().Times(1)` when a twenty-line in-memory fake would do. The mock couples the test to call counts and order you did not mean to assert, and breaks on refactors that do not change behaviour. Hand-write the fake; reserve mocks for the rare case where the interaction itself is under test.
- **Integration tests with no Docker guard.** An integration test that fails (rather than skips) when Docker is absent breaks `go test ./...` for every teammate without a daemon running. Gate behind `//go:build integration` *and* `t.Skip`/`os.Exit(0)` when the daemon is unreachable.
- **Chasing 100% coverage.** The last few percent are error paths and getters; covering them produces contorted tests that assert implementation and break on refactor. Read coverage to find the holes you forgot, not to hit a dashboard number.

Next: the challenges. Challenge 1 stands up the full `testcontainers-go` Postgres integration suite; Challenge 2 takes a slow `notes` hot path through the measure-profile-optimize-prove loop.
