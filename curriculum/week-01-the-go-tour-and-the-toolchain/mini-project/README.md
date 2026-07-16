# Mini-Project — Lab 01: `wordfreq`, a Word-Frequency CLI with Table Tests and a Static Binary

> **Time:** ~9.5 hours across Wednesday-Friday-Saturday. **Prerequisites:** all three lectures, all three exercises, ideally both challenges. **Citations:** the standard-library docs for `bufio`, `os`, `strings`, `sort`, and `unicode` on <https://pkg.go.dev>, plus the testing docs at <https://pkg.go.dev/testing>.

## The spec

You are building **`wordfreq`**, a command-line tool that counts word frequencies in text and prints the top *N* as a Markdown table. It is small enough to finish in a week and rich enough to exercise every Week 1 concept: a module, an `internal/` library package, idiomatic slices and maps, errors as values, `defer` for file cleanup, a table-driven test suite, and a `CGO_ENABLED=0` static binary you can hand to anyone.

```
$ go run . sample.txt --top 5
| rank | word  | count |
|-----:|-------|------:|
|    1 | the   |    42 |
|    2 | of    |    31 |
|    3 | and   |    27 |
|    4 | to    |    19 |
|    5 | a     |    15 |

$ cat sample.txt | go run . --top 3      # reads stdin when given no file
| rank | word | count |
|-----:|------|------:|
|    1 | the  |    42 |
...
```

## Functional requirements

### F1 — Input

- `wordfreq <file>` reads from the named file.
- `wordfreq` with **no file argument** reads from **stdin**, so `cat x.txt | wordfreq` works. Detect this by checking whether a non-flag argument was supplied.
- Opening a non-existent file prints a clear error to **stderr** and exits with a non-zero status code (`os.Exit(1)`). It must *not* panic.

### F2 — Counting

- Split input into words on Unicode whitespace (use `strings.Fields` or a `bufio.Scanner` with `bufio.ScanWords`).
- Normalize each word: lowercase it and strip surrounding punctuation, so `"Cat,"`, `"cat"`, and `"(cat)"` all count as `cat`. (Reuse the `NormalizeWord` idea from Exercise 3.)
- Tokens that normalize to the empty string (pure punctuation like `"---"`) are skipped, not counted as an empty word.
- Count with a `map[string]int`.

### F3 — Ranking

- Produce the top *N* words by frequency, most-frequent first.
- **Ties broken alphabetically** (so output is deterministic and testable — never depend on map iteration order).
- If *N* exceeds the number of distinct words, print all of them (clamp, do not error).

### F4 — The `--top N` flag

- Default *N* is **20**.
- `--top N` overrides it. Parse and validate with the standard library `flag` package (or your Exercise 3 `ParseTopN`); a non-positive or non-numeric value is a clear error to stderr and a non-zero exit.

### F5 — Output

- Print a GitHub-flavoured Markdown table with columns `rank | word | count`, the rank and count right-aligned (`-----:`), the word left-aligned.
- Output goes to **stdout**; errors and diagnostics go to **stderr**. (This separation matters: `wordfreq big.txt > report.md` must produce a clean `report.md` with no error text in it.)

### F6 — Tests

- The counting/ranking logic lives in an `internal/` package and is covered by a **table-driven test suite** with named subtests.
- Cover at least: empty input, single repeated word, ties broken alphabetically, *N* larger than the vocabulary, punctuation stripping, and a pure-punctuation token that gets skipped.
- The package is clean under `go vet ./...` and `staticcheck ./...`, and `go test ./...` is green.

### F7 — The binary

- `CGO_ENABLED=0 go build -o wordfreq` produces a static binary.
- The `README.md` records its size (`ls -lh`) and a proof it has no dynamic dependencies (`ldd` on Linux, or a `GOOS=linux` cross-build inspected with `file`).

## Non-functional requirements

### NF1 — Idiomatic Go

- File-level package layout: `main.go` (the CLI: flags, I/O, exit codes) and `internal/count/count.go` (the pure logic: count, rank). The pure logic takes a `string` (or an `io.Reader`) and returns data — it does no I/O and calls no `os.Exit`, so it is trivially testable.
- Every error is checked; no error is silently discarded. The program never `panic`s on bad input — bad input is an `error` returned and reported.
- `defer f.Close()` immediately after a successful `os.Open`.

### NF2 — Streaming-friendly (stretch-ready)

- Prefer reading via a `bufio.Scanner` so the tool does not load a 1 GB file entirely into memory at once. (Counting into a map still holds the vocabulary in memory, but not the whole file text.)

### NF3 — Citations

- Every non-obvious standard-library choice has a one-line comment pointing at its `pkg.go.dev` page (e.g. `// sort.Slice: https://pkg.go.dev/sort#Slice`).

## Suggested project layout

```
wordfreq/
├── go.mod                       (go mod init github.com/you/wordfreq)
├── README.md                    <-- build, run, the binary-size note
├── sample.txt                   <-- a public-domain text to test against
├── main.go                      <-- CLI: flags, file/stdin, exit codes, output
└── internal/
    └── count/
        ├── count.go             <-- Count(text) map, Top(text, n) []Pair
        └── count_test.go        <-- the table-driven suite
```

A starting point for the pure logic (complete `Top`'s ranking and add `Count`):

```go
// internal/count/count.go
package count

import (
	"sort"
	"strings"
)

// Pair is one word and its frequency.
type Pair struct {
	Word  string
	Count int
}

// Top returns the n most-frequent words in text, most-frequent first,
// ties broken alphabetically. If n exceeds the vocabulary size it returns all.
func Top(text string, n int) []Pair {
	freq := make(map[string]int)
	for _, raw := range strings.Fields(text) {
		w := normalize(raw)
		if w == "" {
			continue // pure-punctuation token: skip, don't count ""
		}
		freq[w]++
	}

	pairs := make([]Pair, 0, len(freq))
	for w, c := range freq {
		pairs = append(pairs, Pair{Word: w, Count: c})
	}

	// sort.Slice: https://pkg.go.dev/sort#Slice
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count != pairs[j].Count {
			return pairs[i].Count > pairs[j].Count
		}
		return pairs[i].Word < pairs[j].Word
	})

	if n > len(pairs) {
		n = len(pairs)
	}
	return pairs[:n]
}

func normalize(s string) string {
	return strings.Trim(strings.ToLower(s), ".,!?;:\"'()[]{}-")
}
```

## The README write-up (`README.md`)

Treat the README as part of the deliverable. It must contain:

### W1 — Build and run

Every command to build and run, copy-pasteable: `go build`, `go run . sample.txt --top 5`, the stdin form.

### W2 — The binary

The output of `ls -lh wordfreq` and the dependency proof:
```
$ ls -lh wordfreq
-rwxr-xr-x  1.9M  wordfreq
$ GOOS=linux CGO_ENABLED=0 go build -o wordfreq-linux && file wordfreq-linux
wordfreq-linux: ELF 64-bit ... statically linked
```
One sentence interpreting the size (it is mostly the embedded Go runtime — a fact, not a problem; Week 10 makes it small and distroless).

### W3 — A sample run

The Markdown table output on `sample.txt`, pasted in. (Pick a public-domain text — a Project Gutenberg book works well.)

### W4 — One design note

200 words on one decision you made: why the pure logic does no I/O, or why you chose `strings.Fields` over a `bufio.Scanner` (or vice versa), or how you guaranteed deterministic output despite map randomization.

## Grading rubric

- **40 points: functional correctness.** F1–F7 all implemented and demonstrable: file + stdin input, normalization, top-N with alphabetical tie-break, the `--top` flag, the Markdown table, stdout/stderr separation, and the static binary.
- **20 points: idiomatic Go.** Clean handler/logic separation; errors checked everywhere; no panic on bad input; `defer` for cleanup; clean under `go vet` and `staticcheck`.
- **15 points: tests.** The table-driven suite covers all six required cases (empty, repeated, ties, clamp, punctuation, skipped token) as named subtests; `go test ./...` is green; coverage of the `count` package is reported.
- **10 points: the binary.** Static `CGO_ENABLED=0` build with the size and dependency proof in the README.
- **10 points: the README write-up.** W1–W4 present, with a real sample run and a real design note.
- **5 points: citations.** Standard-library choices carry one-line `pkg.go.dev` citations in the source.

## Stretch goals

1. **`--min-count N`.** Add a flag that drops words appearing fewer than N times before ranking. Add a table case.
2. **`--ignore-stopwords`.** Add a flag that excludes common English stop words (`the`, `of`, `and`, …) from the count, reading the stop-word list from an embedded file with `//go:embed` (a preview of how Go bundles static assets into the binary — <https://pkg.go.dev/embed>).
3. **Benchmark the hot path.** Add a `BenchmarkTop` over a large input and run `go test -bench . -benchmem`. Report allocations per operation. Then try the two-pass count-first allocation pattern you read in Challenge 2 and measure whether it helps. (This previews Week 4 and Week 8.)
4. **Read from many files.** Accept multiple file arguments (`wordfreq a.txt b.txt c.txt`) and aggregate the counts across all of them. Decide and document how an unreadable file mid-list is handled (skip with a stderr warning, or fail fast?).
5. **Output formats.** Add `--format json` that emits the top-N as a JSON array instead of a Markdown table, using `encoding/json` (a preview of Week 5). Keep Markdown the default.

## Submission

Push the project on a branch named `c30-week01-wordfreq/<your-handle>` and open a PR against the C30 curriculum repository. The PR description must link to the README and paste the sample-run Markdown table and the binary-size line.

The teaching staff reviews mini-project PRs within 7 business days. Reviews focus on (a) whether the seven functional requirements are met, (b) whether the code reads like the editorial style of the lecture-note examples (clean logic/I/O separation, errors checked, no panic), (c) whether the table tests cover the required cases as named subtests, and (d) whether the binary is genuinely static with the proof to show it.

Cited references: <https://pkg.go.dev/bufio>, <https://pkg.go.dev/strings#Fields>, <https://pkg.go.dev/sort#Slice>, <https://pkg.go.dev/flag>, <https://pkg.go.dev/testing>, <https://pkg.go.dev/embed>, <https://pkg.go.dev/encoding/json>.
