# Week 1 — Quiz

Ten multiple-choice questions covering modules, the toolchain, the zero value, slices, maps, `defer`, errors-as-values, and table-driven tests. Treat the quiz as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — The module file

What does the `go.mod` file contain for a typical project, and what is it for?

- (A) A list of every `.go` source file in the project, like a `.csproj`.
- (B) The module path (import prefix) and the Go version the module targets; it is the entire build configuration.
- (C) Compiler optimization flags and the output binary name.
- (D) A copy of every dependency's source code, vendored for reproducibility.

<details>
<summary>Answer</summary>

**(B).** `go.mod` holds the module path (the import prefix for the module's packages) and the targeted Go version — that is the whole build configuration. There is no source-file list (the toolchain walks the tree), no compiler-flag block, and dependencies live in a shared cache, not vendored by default. Citation: <https://go.dev/ref/mod>.

</details>

## Question 2 — `go build ./...`

What does `go build ./...` do that a bare `go build` does not?

- (A) Nothing; they are aliases.
- (B) It runs the tests in addition to building.
- (C) It compiles every package in the directory tree recursively (a whole-module compile check), whereas bare `go build` builds only the current directory's package.
- (D) It strips debug symbols from the resulting binary.

<details>
<summary>Answer</summary>

**(C).** `./...` is the recursive package pattern: "this directory and everything under it." `go build ./...` compile-checks the whole module (no output for library packages); bare `go build` builds only the current directory's package. Citation: <https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns>.

</details>

## Question 3 — Visibility

In Go, an identifier named `parseConfig` (lowercase `p`) is:

- (A) Exported and visible to any package that imports this one.
- (B) Package-private — visible only within its own package.
- (C) A compile error; Go function names must be capitalized.
- (D) Visible to packages in the same `internal/` directory only.

<details>
<summary>Answer</summary>

**(B).** Capitalization is visibility in Go. A lowercase initial letter means package-private; uppercase means exported. There is no `public`/`private` keyword. (`internal/` adds a *separate* module-boundary restriction, but that is about directory placement, not the name's case.) Citation: <https://go.dev/ref/spec#Exported_identifiers>.

</details>

## Question 4 — The zero value

Which statement about the zero value is correct?

- (A) Only numeric types have a defined zero value; structs and pointers are undefined until assigned.
- (B) Every type has a zero value, and a freshly declared variable *is* its zero value; for a struct it is a struct whose every field is its own zero value.
- (C) The zero value of a slice is an empty slice with `len 0` and a non-nil backing array.
- (D) You must call a constructor to initialize a variable before reading it.

<details>
<summary>Answer</summary>

**(B).** Every type has a zero value and a declared variable *is* it — `0`, `""`, `false`, `nil`, or a struct of zero-valued fields. This is why Go favours "make the zero value useful" over constructors. (C) is wrong: a nil slice has a *nil* backing pointer, not an allocated one. Citation: <https://go.dev/tour/basics/12>, <https://go.dev/doc/effective_go#allocation_new>.

</details>

## Question 5 — `append` and the backing array

After `s := make([]int, 0, 2); s = append(s, 1); s = append(s, 2); t := append(s, 3)`, which is true?

- (A) `t` shares `s`'s backing array, because `append` never reallocates.
- (B) `t` has a new backing array, because appending the third element exceeded `cap 2` and forced a reallocation.
- (C) The third `append` panics with an out-of-capacity error.
- (D) `s` now has length 3.

<details>
<summary>Answer</summary>

**(B).** `s` had `cap 2`; appending a third element exceeds capacity, so `append` allocates a new larger backing array, copies, and returns a header pointing at it. `t` therefore does not share `s`'s array. `s` is still length 2 (the `t := append(s, 3)` did not reassign `s`). Citation: <https://go.dev/blog/slices-intro>.

</details>

## Question 6 — The nil map

Given `var m map[string]int` (a nil map), which operation panics?

- (A) Reading `m["x"]` — returns 0.
- (B) `len(m)` — returns 0.
- (C) `for k := range m {}` — iterates zero times.
- (D) Writing `m["x"] = 1` — assignment to entry in nil map.

<details>
<summary>Answer</summary>

**(D).** A nil map is safe to read (returns zero values), to `len`, and to `range` (zero iterations) — but *writing* to it panics with "assignment to entry in nil map." You must `make` a map before writing. Citation: <https://go.dev/blog/maps>.

</details>

## Question 7 — Map iteration order

What order does `for k, v := range m` visit a map's keys in?

- (A) Insertion order.
- (B) Sorted key order.
- (C) A randomized order that can differ run to run; depending on it is a bug, and the fix is to collect keys into a slice and sort them.
- (D) Reverse insertion order.

<details>
<summary>Answer</summary>

**(C).** Map iteration order is deliberately randomized so programs cannot accidentally depend on an order that was never guaranteed. For stable output, collect the keys into a slice and `sort` them. Citation: <https://go.dev/blog/maps>, <https://go.dev/ref/spec#For_statements>.

</details>

## Question 8 — `defer` argument evaluation

In `i := 0; defer fmt.Println(i); i = 99`, what does the deferred call print when the function returns?

- (A) 99, because `defer` reads the variable at return time.
- (B) 0, because the argument `i` is evaluated at the `defer` statement, not when the deferred call runs.
- (C) Nothing; deferred calls cannot take arguments.
- (D) Both 0 and 99.

<details>
<summary>Answer</summary>

**(B).** A deferred call's *arguments* are evaluated at the moment the `defer` statement executes, not when the deferred call later runs. So `i`'s value (0) is captured before `i = 99`. To read the value at run time, defer a closure that references `i`. Citation: <https://go.dev/blog/defer-panic-and-recover>.

</details>

## Question 9 — Errors as values

Which statement reflects idiomatic Go error handling?

- (A) Throw an exception with `panic` for any failure and `recover` it in the caller.
- (B) A fallible function returns an `error` as its last return value; the caller checks `if err != nil`; `panic` is reserved for programmer bugs and process-boundary recovery, not ordinary failure.
- (C) Return a boolean `ok` for failure; the `error` type is only for the standard library.
- (D) Ignore errors that "can't happen"; the compiler enforces handling of the rest.

<details>
<summary>Answer</summary>

**(B).** Errors are values: returned last, checked explicitly. `panic`/`recover` is for programmer mistakes (nil dereference, out-of-bounds) and boundary recovery (HTTP middleware), not ordinary expected failure. (A) inverts the model; (D) is false — the compiler does not force error handling, but `staticcheck` flags ignored errors. Citation: <https://go.dev/doc/effective_go#errors>.

</details>

## Question 10 — Table-driven tests

What is the idiomatic Go shape for testing a function across many inputs?

- (A) One `TestXxx` function per input case, copy-pasting the assertion each time.
- (B) A single `TestXxx` function holding a slice of `{name, input, want}` case structs, iterated with `t.Run(tc.name, ...)` so each case is a named subtest you can run individually.
- (C) A third-party assertion framework with a fluent DSL, required because the standard library has no test support.
- (D) A single test that calls the function once with a representative input.

<details>
<summary>Answer</summary>

**(B).** The idiomatic shape is a single test function holding a table of named cases iterated with `t.Run`, giving named subtests you can target with `go test -run TestX/case`, cheap-to-add cases, and assertion logic written once. The standard `testing` package needs no external assertion framework. Citation: <https://go.dev/wiki/TableDrivenTests>, <https://pkg.go.dev/testing>.

</details>

---

## Self-assessment

- 9-10: you are fluent in the Week 1 foundations; ship the mini-project without further reading.
- 7-8: re-read the lecture notes on the questions you missed; the citations point to the exact Go docs.
- 5-6: re-read all three lecture notes and redo the exercises before the mini-project.
- 0-4: rewind to Lecture 1 and work all three lecture notes and exercises carefully. The mini-project depends on every concept here.
