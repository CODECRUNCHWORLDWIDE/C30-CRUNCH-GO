# Week 8 — Homework

Six practice problems. Allocate roughly 1 hour per problem; the last two are longer and may need 90 minutes. Submit one `.zip` of code + a single `homework.md` write-up. Rubric at the bottom.

---

## Problem 1 — A table-driven suite with subtests and parallel (60 min)

You are given (or write) this function on the `notes` domain:

```go
// NormalizeTags lowercases each tag, trims whitespace, drops empties and
// duplicates, and returns them sorted. It errors if any tag exceeds 32 runes.
func NormalizeTags(tags []string) ([]string, error)
```

Write a table-driven test for it. **Required:**

- A `[]struct{ name string; in []string; want []string; wantErr bool }` table.
- A `t.Run(tc.name, ...)` loop with `t.Parallel()` on each subtest (and on the parent).
- At least eight cases: empty input, single tag, mixed case, duplicates, surrounding whitespace, an empty/whitespace-only tag dropped, a tag at exactly 32 runes (ok), a tag at 33 runes (error).
- `slices.Equal` (or `cmp.Diff`) for the value comparison; `(err != nil) != tc.wantErr` for the error.
- A `got, want` failure message that prints the input.

**Deliverable:** `normalize_test.go` and a one-paragraph `notes.md` explaining why no `tc := tc` shadow is needed on Go 1.22+.

---

## Problem 2 — A golden-file test with `-update` (60 min)

Take a function that renders structured output — `RenderNoteMarkdown` from Exercise 1, or your own `notes` HTML/JSON renderer. Write a golden-file test for it.

**Required:**

- A `var update = flag.Bool("update", false, "update golden files")`.
- On `-update`: render and `os.WriteFile` the output to `testdata/<name>.golden`.
- On a normal run: `os.ReadFile` the golden and compare with `cmp.Diff`.
- The render must be **deterministic** (sort any maps/slices) so the golden is stable.
- A second test case with *different* input and a *second* golden file, to prove the pattern scales.

**Deliverable:** the test, the two `.golden` files under `testdata/`, and a `notes.md` describing the code-review workflow when the golden output legitimately changes (what does the reviewer see, what do you run to bless it).

---

## Problem 3 — A benchmark + `benchstat` A/B for two implementations (60 min)

Write two implementations of the same function — for example, "count word frequencies in a note body": one using `strings.Fields` + a `map[string]int`, one using a single pass with manual word boundaries; or the slow/fast tag index from Exercise 2.

**Required:**

- A correctness test proving the two implementations return identical results for several inputs (differential testing).
- `BenchmarkA` and `BenchmarkB`, each with a package-level sink and `b.ReportAllocs()`.
- Run each with `-count=10`, capture `a.txt` and `b.txt`, and run `benchstat a.txt b.txt`.

**Deliverable:** the code, the benchmark, `a.txt`, `b.txt`, and a `notes.md` with the `benchstat` table and one sentence interpreting the p-value (is the difference significant?). If the two are statistically indistinguishable (`~`), say so — that is a valid and useful result.

---

## Problem 4 — Capture and interpret a `pprof` profile (60 min)

Take a deliberately slow function (the `RenderFeed` with `+=` from Challenge 2, or your own). Benchmark it, then capture a CPU profile and a heap profile.

**Required:**

- `go test -bench=... -benchmem -cpuprofile=cpu.out -memprofile=mem.out ./...`.
- `go tool pprof cpu.out` → run `top` and `list <YourFunc>`; capture the output.
- `go tool pprof -alloc_space mem.out` → run `top`; capture the output.
- Identify **one** finding: the single line or call that dominates, and whether the cost is CPU (arithmetic), memory copying (`runtime.memmove`), or allocation (`runtime.mallocgc`).

**Deliverable:** `cpu.out` (and/or its text), the `top`/`list` output pasted into `pprof-findings.md`, and a one-paragraph interpretation: what is the bottleneck, and what optimization would you try (you do **not** have to implement it for this problem — Problem and Challenge 2 do that).

---

## Problem 5 — A fuzz target that finds a real crash (90 min)

You are given a buggy parser. Use this one (a frontmatter-style `key=value` header parser) — it has a real crash:

```go
// ParseHeader parses lines like "title=Hello" into a map. BUG: a line that is
// exactly "=" (empty key) or that ends without a value is mishandled.
func ParseHeader(s string) (map[string]string, error) {
	out := map[string]string{}
	for _, line := range strings.Split(s, "\n") {
		if line == "" {
			continue
		}
		eq := strings.IndexByte(line, '=')
		key := line[:eq]      // BUG: eq is -1 when there is no '=', so line[:-1] panics
		val := line[eq+1:]
		out[key] = val
	}
	return out, nil
}
```

**Required:**

- A `FuzzParseHeader(f *testing.F)` with `f.Add` seeds (`"title=Hi"`, `"a=1\nb=2"`, `""`, `"noequals"`, `"="`).
- An `f.Fuzz` body asserting at minimum never-panic, and ideally that every returned key is non-empty.
- Run `go test -run='^$' -fuzz=FuzzParseHeader -fuzztime=30s` and let it find the crash.
- Fix the bug (handle `eq < 0`: treat a line with no `=` as an error or skip it; reject an empty key).
- Re-run the committed crasher and confirm it passes; commit it under `testdata/fuzz/`.

**Deliverable:** the fixed `ParseHeader`, the fuzz target, the committed crasher file, and a `notes.md` documenting the **minimal crashing input** the engine found, the panic message, and your fix in one sentence.

---

## Problem 6 — Integration test against a `testcontainers-go` Postgres (90 min, stretch)

If you have Docker installed (or are willing to install it: <https://docs.docker.com/get-docker/>), write an integration test for your Week-6 `notes` repository.

**Required:**

- A file whose first line is `//go:build integration`.
- A `TestMain` that starts a `testcontainers-go` Postgres (`postgres:16-alpine`), runs your Week-6 migrations, runs the suite, and terminates the container.
- A clean `t.Skip` / `os.Exit(0)` when Docker is unavailable.
- One create-and-get round-trip through real SQL, compared with `cmp.Diff`.
- Proof that `go test ./...` (no tags) does not start a container and `go test -tags=integration ./...` does.

**Deliverable:** the integration test file and a `notes.md` answering: what bug could this test catch that a fake repository could not, specific to your schema?

If you do not have Docker and do not want to install it, substitute: **write a transaction-rollback isolation helper** (`withTx(t, pool) pgx.Tx` that begins a transaction and rolls it back in `t.Cleanup`) and a short `notes.md` explaining how it makes integration tests `t.Parallel()`-safe — written against the API even if you cannot run it.

---

## Rubric

For each problem (max 100 points):

| Tier | Points | Description |
|------|--------|-------------|
| Master | 90–100 | Compiles and passes. Every requirement met. The `notes.md` shows reasoning beyond the literal answer — at least one observation the spec did not ask for. |
| Solid | 75–89 | Compiles and passes. Every requirement met. The `notes.md` answers what was asked, no more. |
| Working | 60–74 | Compiles. Most requirements met; one or two missed. |
| Partial | 40–59 | Compiles in places but with significant gaps; the spec was not fully read. |
| Submitted | 0–39 | Submission exists; substantial parts are missing or broken. |

Total: **600 points** across the six problems. **480** is the C30-passing threshold for this week's homework. The mini-project is graded separately.

## Submission

Zip the six problem folders together as `week-08-homework-<your-name>.zip`. Include a top-level `homework.md` that links to each problem's `notes.md` and lists your self-assigned score in each tier.

Submit by Sunday 11:59 PM local time. Late submissions are accepted with a one-tier markdown per 24h past the deadline.
