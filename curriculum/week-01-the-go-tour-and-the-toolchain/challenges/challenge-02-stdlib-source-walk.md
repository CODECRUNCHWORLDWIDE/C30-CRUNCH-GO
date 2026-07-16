# Challenge 2 — Standard-Library Source Walk: Read `strings.Fields` and `bufio.Scanner`

> **Time:** 1.5 hours. **Prerequisites:** Lecture 2 (slices, strings) and Lecture 3 (functions). **Citations:** the `strings` package docs and source at <https://pkg.go.dev/strings>, the `bufio` package at <https://pkg.go.dev/bufio>, the `unicode` package at <https://pkg.go.dev/unicode>, and Effective Go at <https://go.dev/doc/effective_go>.

## The premise

The standard library *is* the canonical idiomatic-Go reference. Not a blog, not a style guide — the source the Go team wrote and reviews. The two functions you will lean on most in the `wordfreq` mini-project are `strings.Fields` (split text into words on whitespace) and `bufio.Scanner` (read input line by line or word by word). In this challenge you read both from source, on `pkg.go.dev` (click the function name to jump to the source), and write up *why* each is written the way it is.

## Part A — `strings.Fields`

Open <https://pkg.go.dev/strings#Fields> and click through to the source.

Answer, with evidence from the code:

1. **The signature.** `func Fields(s string) []string`. Why does it return a `[]string` and not, say, an iterator? (Think about the era it was written and the simplicity-over-cleverness instinct.)
2. **The two-pass structure.** `Fields` makes a first pass to *count* the fields, then allocates the result slice with exactly the right capacity (`make([]string, 0, n)`), then a second pass to fill it. Why count first instead of `append`-ing and letting the slice grow? (Connect this to the reallocation mechanic from Lecture 2 — what does the count-first approach save?)
3. **`unicode.IsSpace`.** `Fields` splits on any Unicode whitespace via `unicode.IsSpace`, not just ASCII spaces. Find where it calls it. What is the difference between `strings.Fields` and `strings.Split(s, " ")` for the input `"a\t b\n c"`? (Run both and compare.)
4. **The fast path.** Modern `Fields` has an ASCII fast path before falling back to the Unicode-aware path. Find it. Why is the fast path worth the extra code? (Performance for the common case — most text is ASCII.)

Write a 250-word note titled "Why `strings.Fields` is idiomatic Go" covering the count-then-allocate pattern, the Unicode-correctness choice, and the readability-over-cleverness style.

## Part B — `bufio.Scanner`

Open <https://pkg.go.dev/bufio#Scanner> and read the type, plus `NewScanner`, `Scan`, `Text`, `Bytes`, `Err`, and the split functions `ScanLines` / `ScanWords`.

Answer, with evidence:

1. **The Scan loop idiom.** The canonical use is:
   ```go
   sc := bufio.NewScanner(r)
   for sc.Scan() {
       line := sc.Text()
       // ... use line ...
   }
   if err := sc.Err(); err != nil { /* handle */ }
   ```
   Why is the error checked *after* the loop with `sc.Err()` rather than inside it? (`Scan` returns a bool, not an error — explain the design: `Scan` returns false on both EOF and error, and `Err()` disambiguates.)
2. **`ScanWords`.** `sc.Split(bufio.ScanWords)` makes the scanner yield whitespace-separated words instead of lines. How does this relate to `strings.Fields` from Part A? When would you choose the streaming `Scanner` over `strings.Fields` on a whole string? (Hint: memory — `Scanner` never holds the whole input in memory; `strings.Fields` requires the whole string already in memory.)
3. **The buffer limit.** `Scanner` has a maximum token size (`bufio.MaxScanTokenSize`, 64 KiB by default) and returns `bufio.ErrTooLong` if a single token exceeds it. Find where this is enforced. Why does a streaming scanner need a maximum token size at all? (A pathological input — a 4 GB "line" with no newline — would otherwise OOM the process.)
4. **`Text()` vs `Bytes()`.** `Bytes()` returns a slice that *points into the scanner's internal buffer* and is overwritten on the next `Scan()`. `Text()` returns a fresh string copy. When is each correct? (Connect to the slice-aliasing lesson from Lecture 2 — `Bytes()` is the aliasing trap in standard-library form.)

Write a 250-word note titled "The `bufio.Scanner` contract" covering the Scan-then-Err idiom, the streaming-vs-whole-string memory trade, and the `Bytes()` aliasing hazard.

## Acceptance criteria

1. Both 250-word notes, each citing the specific source location (file + function) you read.
2. A small runnable program demonstrating the `strings.Fields` vs `strings.Split(s, " ")` difference on `"a\t b\n c"`, with its output.
3. A small runnable program demonstrating the `Bytes()` aliasing hazard: scan two words, hold the first `Bytes()` result, then scan again, and show the held slice changed. Then show `Text()` does not have this problem.

## Stretch goals

1. **Implement a custom split function.** Write a `bufio.SplitFunc` that splits on commas (a tiny CSV-field scanner) and use it with `sc.Split(yourFunc)`. Read `ScanLines` as your template; the `SplitFunc` signature is `func(data []byte, atEOF bool) (advance int, token []byte, err error)`. Explain the `advance`/`token`/`err` contract.
2. **Benchmark the two approaches.** Write a `BenchmarkFields` and a `BenchmarkScanWords` over a 10 MB text file. Which allocates less? (`go test -bench . -benchmem`.) This is a preview of Week 4 / Week 8 benchmarking; the point here is to *see* that the streaming approach holds less memory.
3. **Read `strings.Builder`.** You used it in Exercise 1. Read its source: why does it have a `copyCheck` that panics if you copy a `Builder` by value? (Connect to the "copies lock value" `go vet` finding from Lecture 1 — a `Builder` holds a pointer to itself to detect illegal copies.)

Cited references: <https://pkg.go.dev/strings#Fields>, <https://pkg.go.dev/bufio#Scanner>, <https://pkg.go.dev/unicode#IsSpace>, <https://pkg.go.dev/strings#Builder>, <https://go.dev/doc/effective_go>.
