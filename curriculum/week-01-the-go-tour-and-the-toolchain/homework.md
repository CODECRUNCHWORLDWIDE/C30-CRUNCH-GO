# Week 1 — Homework

Six practice problems that consolidate the week's material. They are sized to ~45 minutes each. Do them after the lectures and the exercises; do them before (or alongside) the mini-project. Cite the URLs you used while solving each one in the commit message of your homework branch.

## Problem 1 — The `go.mod` and toolchain tour

Create a fresh module (`go mod init github.com/you/hw01`) with a `main` package and one `internal/` library package. Then write a 250-word note answering:

1. What are the two (and only two) things `go mod init` puts in `go.mod`, and what does each gate?
2. What is the difference between `go build`, `go build ./...`, `go run .`, and `go install`? For each, state what artifact (if any) it produces and where.
3. What does the `internal/` directory restrict, and who can import a package under it?
4. Show the output of `go vet ./...` and `staticcheck ./...` on your clean module (both should be empty), and explain in one sentence what each tool catches that the compiler does not.

Cite: <https://go.dev/ref/mod>, <https://pkg.go.dev/cmd/go>, <https://staticcheck.dev>.

Deliverable: `homework/01-toolchain-tour.md` plus the module directory.

## Problem 2 — Predict the slice output

Without running it, predict the exact output of this program. Then run it and reconcile any differences in a short write-up.

```go
package main

import "fmt"

func main() {
	a := make([]int, 3, 5)   // len 3, cap 5, values [0 0 0]
	a[0], a[1], a[2] = 1, 2, 3

	b := a[1:3]              // shares a's backing array
	b[0] = 20

	c := append(b, 99)      // spare capacity? what does this touch?
	c[1] = 88

	d := a[0:2:2]           // full-slice expression
	d = append(d, 777)      // forced realloc?

	fmt.Println("a =", a)
	fmt.Println("b =", b)
	fmt.Println("c =", c)
	fmt.Println("d =", d)
}
```

In your write-up, for each of `a`, `b`, `c`, `d`: state whether it shares a backing array with `a` after all the operations, and explain how `append`'s capacity check decided that.

Cite: <https://go.dev/blog/slices-intro> and the slice-expression spec at <https://go.dev/ref/spec#Slice_expressions>.

Deliverable: `homework/02-slice-prediction.md`.

## Problem 3 — The nil-map and comma-ok drill

Write a small program and a 200-word note that demonstrates, with runnable code:

1. Reading from a nil map returns the zero value and does not panic.
2. Writing to a nil map panics (`recover` it and print the panic message so the program does not crash).
3. The comma-ok form distinguishing "key absent" from "key maps to the zero value" — construct a `map[string]int` where one key genuinely maps to `0` and show that comma-ok tells it apart from an absent key.
4. Why map iteration order is randomized, and the idiomatic fix (collect keys, sort, range the sorted keys) — show the same map printing in a stable order.

Cite: <https://go.dev/blog/maps>.

Deliverable: `homework/03-maps-drill.md` plus the program.

## Problem 4 — `defer` ordering and argument evaluation

Predict, then verify, the output of:

```go
package main

import "fmt"

func main() {
	fmt.Println("start")
	for i := 0; i < 3; i++ {
		defer fmt.Println("deferred", i)
	}
	x := 10
	defer fmt.Printf("x by value = %d\n", x)   // captures x NOW
	defer func() { fmt.Printf("x by closure = %d\n", x) }() // reads x at run time
	x = 20
	fmt.Println("end of main")
}
```

In your write-up: (a) the exact output order, (b) why the loop prints `2 1 0` and not `0 1 2`, and (c) why "x by value" and "x by closure" disagree. Then state the rule for when a deferred call's arguments are evaluated.

Cite: <https://go.dev/blog/defer-panic-and-recover>.

Deliverable: `homework/04-defer-ordering.md`.

## Problem 5 — A table-driven test from scratch

Write a function `func WordCount(s string) map[string]int` that counts word occurrences (lowercased, whitespace-split). Then write a table-driven test suite for it with named subtests covering: empty string, single word, repeated word, mixed case (`"Go go GO"` → `{"go": 3}`), and a sentence with punctuation. Run it with:

```sh
go test -v ./...
go test -run 'TestWordCount/mixed_case' ./...
go test -cover ./...
```

In a 150-word note, report your coverage percentage and explain which line(s) the red bars in `go tool cover -html` point at (or, if 100%, explain how you know every branch is exercised).

Cite: <https://pkg.go.dev/testing>, <https://go.dev/wiki/TableDrivenTests>, <https://go.dev/blog/cover>.

Deliverable: `homework/05-wordcount/` (the module with code and test) plus `homework/05-wordcount.md`.

## Problem 6 — The errors-vs-exceptions essay

In 400 words, argue the case for Go's "errors are values" model to a colleague coming from a `try`/`catch` language (Java, C#, Python, or JavaScript). Cover:

1. **The mechanic.** How a fallible function signals failure (return `error` last) and how the caller responds (`if err != nil`).
2. **The trade.** What you give up (the brevity of letting an exception propagate untouched) and what you gain (the failure path is visible on every line that can fail).
3. **`panic`'s real job.** Why `panic`/`recover` exists and why it is *not* the equivalent of `throw`/`catch` — name a legitimate use (a programmer error like a nil dereference; a process-boundary recovery in middleware) and an illegitimate one (ordinary expected failure like "file not found").
4. **One concrete example.** Take a 10-line function from your old language that uses `try`/`catch` and rewrite it in Go with explicit error returns. Show both.

Cite: Effective Go's errors section at <https://go.dev/doc/effective_go#errors>, the `errors` package at <https://pkg.go.dev/errors>, and <https://go.dev/blog/defer-panic-and-recover>.

Deliverable: `homework/06-errors-essay.md`.

## Submission

Push the six deliverables on a branch named `c30-week01-homework/<your-handle>` and open a PR against the C30 curriculum repository. The PR description should link to each of the six files and include a 100-word summary of the one thing about Go that most surprised you this week.

The teaching staff reviews homework PRs within 5 business days. Reviews focus on whether your predictions matched reality (and whether you reconciled the differences honestly), whether your reasoning holds together, and whether you read the citations. The single most common review comment is "run it and show me the output" — preempt it by pasting the real toolchain output for every claim.

Cited references this homework draws from: <https://go.dev/ref/mod>, <https://go.dev/blog/slices-intro>, <https://go.dev/blog/maps>, <https://go.dev/blog/defer-panic-and-recover>, <https://pkg.go.dev/testing>, <https://go.dev/wiki/TableDrivenTests>, <https://go.dev/doc/effective_go#errors>.
