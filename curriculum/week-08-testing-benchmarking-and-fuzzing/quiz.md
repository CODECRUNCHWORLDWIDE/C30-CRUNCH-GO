# Week 8 — Quiz

Ten multiple-choice questions. Take it with your lecture notes closed. Aim for 9/10 before moving to Week 9. Answer key at the bottom — do not peek.

---

**Q1.** Inside a `t.Run(tc.name, func(t *testing.T) { ... })` subtest, you call `t.Parallel()` as the first line. What does that actually do?

- A) It runs the body on a new OS thread immediately.
- B) It pauses this subtest until all non-parallel work in the parent finishes, then runs it concurrently with the other parallel subtests of that parent.
- C) It splits the table into `GOMAXPROCS` shards and runs each shard sequentially.
- D) It has no effect unless you also pass `-parallel=N` on the command line.

<details>
<summary>Answer</summary>

**B.** `t.Parallel()` signals that the subtest is safe to run concurrently. The testing framework pauses it until the parent's serial work is done, then runs all the parent's parallel subtests together. It does not spawn a thread immediately, and it does not require a command-line flag (though `-parallel=N` caps how many run at once). Documented at <https://pkg.go.dev/testing#T.Parallel>.

</details>

---

**Q2.** You are reviewing a pre-Go-1.22 table-driven test that wrote `tc := tc` at the top of the loop body before calling `t.Parallel()`. On Go 1.22+, is that shadow still needed?

- A) Yes — the shadow is always required for parallel subtests, in every Go version.
- B) No — as of Go 1.22 each `range` iteration binds a fresh loop variable, so a parallel subtest no longer captures a shared, mutated variable. The shadow is harmless but unnecessary.
- C) No — `t.Parallel()` was removed in Go 1.22, so the question is moot.
- D) Yes — without it, parallel subtests deadlock.

<details>
<summary>Answer</summary>

**B.** Before Go 1.22 the `range` loop reused a single variable across iterations, so a parallel subtest — which runs *after* the loop finished — captured that one variable at its final value, and every parallel subtest tested the last case. The `tc := tc` shadow worked around it. Go 1.22 changed loop semantics so each iteration binds a fresh variable; the shadow is now unnecessary (and `go vet` no longer flags the missing shadow). See <https://go.dev/doc/go1.22>.

</details>

---

**Q3.** A benchmark body is `_ = Slugify(input)` with no other use of the result, and `Slugify` has no side effects. The benchmark reports `0.28 ns/op`. What is the most likely explanation?

- A) `Slugify` is genuinely that fast.
- B) The compiler eliminated the call as dead code because its result is unused; the benchmark is measuring an empty loop. You need a package-level sink.
- C) `b.N` was set too low.
- D) The CPU was idle, so the OS clock under-counted.

<details>
<summary>Answer</summary>

**B.** A sub-nanosecond `ns/op` is the signature of dead-code elimination: the optimizer saw the result was unused and removed the call. Assign the result to a package-level sink so the compiler cannot prove it is unused. This is the single most common benchmarking mistake.

</details>

---

**Q4.** A benchmark line reads `BenchmarkBuild-10  4791  249830 ns/op  243712 B/op  2009 allocs/op`. Which statement is correct?

- A) The function was called 249,830 times.
- B) `2009 allocs/op` is the number of distinct heap allocations per call; `243712 B/op` is the bytes allocated per call; `249830 ns/op` is the time per call. The `4791` is the final `b.N`.
- C) `B/op` is the number of bytes the function returned.
- D) `allocs/op` counts stack allocations, which is why it is high.

<details>
<summary>Answer</summary>

**B.** Read the columns: `4791` is the final `b.N` (the loop ran ~4,791 times for the last measurement), `249830 ns/op` is time per call, `243712 B/op` is heap bytes per call, `2009 allocs/op` is distinct heap allocations per call. `allocs/op` counts *heap* allocations, not stack — stack allocations are free and not counted.

</details>

---

**Q5.** `benchstat old.txt new.txt` prints, for a metric, `-64.61% (p=0.000 n=10)`. What does the `p=0.000` tell you?

- A) The new code is 0% slower.
- B) The difference is statistically significant — well below the 0.05 threshold — so the 64.61% improvement is real, not measurement noise.
- C) Zero of the ten runs improved.
- D) The p-value must be above 0.05 to claim a win, so this result is inconclusive.

<details>
<summary>Answer</summary>

**B.** `benchstat` runs a statistical test across the `n=10` runs per side and reports the p-value. A p-value below 0.05 means the difference is statistically significant — unlikely to be noise. `p=0.000` is strong evidence the 64.61% improvement is real. (If `benchstat` instead prints `~`, the difference is not significant and you should not claim a win.)

</details>

---

**Q6.** What governs how many times a `func BenchmarkX(b *testing.B)` runs its `for i := 0; i < b.N; i++` loop?

- A) You set `b.N` yourself at the top of the benchmark.
- B) The framework chooses `b.N`: it runs a few iterations to estimate the per-op cost, then scales `b.N` up until the benchmark has run long enough (about a second by default) for a stable measurement.
- C) `b.N` is always 1,000,000.
- D) `b.N` equals `GOMAXPROCS`.

<details>
<summary>Answer</summary>

**B.** You never set `b.N`. The framework estimates the per-iteration cost with a short run, then scales `b.N` up until the benchmark has run for its target duration (about a second by default, adjustable with `-benchtime`), which gives a stable timing. Your job is only to make the loop body do one unit of the work you want to measure.

</details>

---

**Q7.** Your service's goroutine count climbs steadily and never falls, and you suspect a leak. Which `pprof` profile do you reach for, and what do you expect to see?

- A) The CPU profile; a hot loop.
- B) The goroutine profile; thousands of goroutines parked at the same location (e.g. a `chan receive`), revealing where they are stuck and never resume.
- C) The heap profile with `-inuse_space`; a single large allocation.
- D) The mutex profile; a contended lock.

<details>
<summary>Answer</summary>

**B.** A goroutine leak is a goroutine-profile problem. The profile shows you *how many* goroutines exist and *where each is stuck* — a leak appears as a large, growing cluster all parked at the same line (often a `chan receive` or a `select` with no progress). CPU/heap/mutex profiles answer different questions and would not reveal the leak's location.

</details>

---

**Q8.** In a fuzz target, what is the relationship between `f.Add(...)` and the function passed to `f.Fuzz(...)`?

- A) `f.Add` registers seed-corpus inputs whose argument types must match the non-`*testing.T` parameters of the `f.Fuzz` function; seeds run as ordinary subtests and serve as starting points the engine mutates.
- B) `f.Add` registers assertions that the `f.Fuzz` function must satisfy.
- C) `f.Add` sets the number of fuzz iterations.
- D) `f.Add` and `f.Fuzz` are unrelated; `f.Add` configures logging.

<details>
<summary>Answer</summary>

**A.** `f.Add` seeds the corpus; its arguments must match — in type and order — the fuzzed parameters of the `f.Fuzz` function (everything after the leading `*testing.T`). Seeds run as ordinary subtests on every `go test` (acting as regression tests) and are the starting points the engine mutates when fuzzing with `-fuzz`. See <https://go.dev/security/fuzz/>.

</details>

---

**Q9.** The fuzzer finds an input that crashes your parser. Where does it write that input, and what happens to it afterward?

- A) It prints it to stdout and discards it; you must copy it down by hand.
- B) It writes the minimized input to a file under `testdata/fuzz/FuzzXxx/<hash>`, which becomes part of the seed corpus and runs as a regression test on every `go test` (no `-fuzz` needed) once committed.
- C) It deletes the input to avoid polluting the corpus.
- D) It writes it to `/tmp` and the file is gone after reboot.

<details>
<summary>Answer</summary>

**B.** The engine minimizes the failing input and writes it to `testdata/fuzz/FuzzXxx/<hash>`. Once you commit that file, it is part of the seed corpus: it runs on every `go test` with no `-fuzz` flag, so the bug becomes a permanent regression test. The engine also prints a `go test -run=FuzzXxx/<hash>` command to re-run just that crasher.

</details>

---

**Q10.** A teammate proposes "every package must hit 100% test coverage." What is the strongest objection?

- A) 100% coverage is impossible in Go.
- B) Coverage measures lines *executed*, not behaviour *verified* — a test that calls every line and asserts nothing hits 100% and proves nothing; chasing the last few percent produces brittle tests that assert implementation. Coverage is a hole-finder, not a target.
- C) The `-cover` flag only works on Linux.
- D) 100% coverage makes the test suite run too slowly to be useful.

<details>
<summary>Answer</summary>

**B.** Coverage measures executed statements, not verified behaviour, so it is trivially gamed (call everything, assert nothing → 100%, proves nothing). The last few percent are usually hard-to-trigger error paths and getters; covering them produces contorted tests that assert implementation details and break on refactors. Use coverage to find the holes you forgot, not as a number to maximize.

---

</details>

---

## Scoring

- **10/10**: You can teach this material. Move to Week 9 with confidence.
- **8–9**: Solid. Re-read the lecture sections corresponding to the questions you missed, then move on.
- **6–7**: Re-read all three lectures and retake. The benchmarking and profiling discipline is dense; do not skim it.
- **≤5**: Slow down. Spend an extra evening on the lectures and the SOLUTIONS.md before attempting the mini-project.
