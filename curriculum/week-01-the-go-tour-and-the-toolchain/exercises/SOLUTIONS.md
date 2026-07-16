# Week 1 — Exercise Solutions and Annotations

Read these after attempting the exercises, not before. Every Go snippet here has been built (`go build ./...`) and, where it is a test, run (`go test ./...`) and checked clean under `go vet ./...` and `staticcheck ./...`.

## Exercise 1 — Module and Toolchain

### `internal/greet/greet.go`

```go
package greet

import "strings"

// Salutation returns a lowercase greeting for name.
func Salutation(name string) string {
	return "hello, " + strings.ToLower(name)
}
```

### What success looks like

```
$ go run .
hello, ada
hello, grace
hello, alan

$ go vet ./...
$ staticcheck ./...
$ CGO_ENABLED=0 go build -o ex01
$ ldd ex01
        not a dynamic executable
$ go version -m ex01
ex01: go1.22.4
        path    github.com/you/ex01
        mod     github.com/you/ex01     (devel)
        build   -buildmode=exe
        build   CGO_ENABLED=0
        ...
```

On macOS, `otool -L ex01` will still list `/usr/lib/libSystem.B.dylib` even with `CGO_ENABLED=0`, because macOS does not support truly static binaries linking libSystem — every native Mac binary links it. This is a platform fact, not a Go fact: build for Linux (`GOOS=linux CGO_ENABLED=0 go build`) and `ldd` on a Linux box (or `file`) to see the fully static result. This is exactly why CI for a service builds the Linux binary even on a developer's Mac.

### The acceptance question, answered

**What does `go build ./...` do that `go build` alone does not?**

`go build` with no arguments builds only the package in the *current directory* and, for a `main` package, writes an executable. `go build ./...` builds *every package in the tree, recursively* — it is a compile-check of the whole module. For library packages it produces no output file at all; it just verifies they compile (and caches the result). You run `go build ./...` to answer "does my whole module still compile?" and `go build` (or `go build -o name`) to produce a specific deliverable binary.

### Common pitfalls

1. **Import path mismatch.** The import in `main.go` must be `<module-path>/internal/greet`, where `<module-path>` is exactly the string you passed to `go mod init`. A typo here produces `cannot find package`.
2. **Lowercase function name.** If you write `salutation` (lowercase), `main` cannot import it — the build fails with `undefined: greet.salutation` (or `cannot refer to unexported name`). Capitalization is the access modifier.
3. **Expecting `ldd: not a dynamic executable` on macOS.** See above — that result is a Linux result. On macOS use `GOOS=linux` and inspect the Linux binary.

## Exercise 2 — Slices, Maps, Structs, and defer

### The five predictions, answered

```
after window[0]=99: [1 99 3 4 5]
after append(window,777): [1 99 3 777 5]
after safe append: [1 99 3 777 5] | safe: [99 3 555]
v1=0  v2=0 ok=false  v3=2 present=true
function body done; i is now 99
this runs FIRST  (LIFO)
this runs SECOND (LIFO)
deferred print, i captured at defer-time = 0
```

- **PREDICT 1.** `window := base[1:3]` shares base's backing array; `window[0]` *is* `base[1]`. Writing 99 to it changes `base` → `[1 99 3 4 5]`.
- **PREDICT 2.** `window` had `len 2, cap 4` (it can see to the end of `base`). `append(window, 777)` has spare capacity, so it writes into the existing backing array at index `base[3]`, not a new array → `base` becomes `[1 99 3 777 5]`. This is the aliasing trap: an `append` mutated a slice the caller never handed to this code.
- **PREDICT 3.** `safe := base[1:3:3]` uses the full-slice expression to cap `cap` at `len` (2). Now `append(safe, 555)` has no spare capacity and is *forced to reallocate* a fresh backing array; `base` is untouched. `safe` becomes `[99 3 555]` on its own array.
- **PREDICT 4.** A read of an absent key returns the zero value (`v1=0`), but you cannot tell absence from a real zero without comma-ok: `v2=0, ok=false` says "dog" is absent; `v3=2, present=true` says "cat" maps to 2.
- **PREDICT 5.** Deferred calls run LIFO, so the last-written `defer` runs first. The "i captured at defer-time" line shows **0**, not 99, because the argument `i` was evaluated when the `defer` statement executed (before `i = 99`).

The lesson the reviewer cares about: **always be able to answer "does this slice alias its parent, and can an `append` here reach back into a caller's array?"** When in doubt, `slices.Clone(s)` or `s[a:b:b]`.

### Common pitfalls

1. **Forgetting that `append` can mutate the parent.** This is the bug that ships. If a function receives a slice and appends to it, and the caller still holds the original, the caller may see surprise mutations. Document it or clone.
2. **Writing to a nil map.** Not in this exercise, but the sibling trap: `var m map[string]int; m["x"]=1` panics. Always `make` a map before writing.
3. **Depending on map iteration order.** The `sort.Strings(keys)` step is mandatory for stable output; remove it and the program prints `[a b c]` in a different order on different runs.

## Exercise 3 — Errors as Values and Table-Driven Tests

### What success looks like

```
$ go test -v ./...
=== RUN   TestParseTopN
=== RUN   TestParseTopN/valid
=== RUN   TestParseTopN/zero_is_invalid
=== RUN   TestParseTopN/negative_is_invalid
=== RUN   TestParseTopN/non-numeric_is_invalid
--- PASS: TestParseTopN (0.00s)
    --- PASS: TestParseTopN/valid (0.00s)
    ...
=== RUN   TestNormalizeWord
--- PASS: TestNormalizeWord (0.00s)
PASS
ok      github.com/you/ex03     0.004s

$ go test -run 'TestParseTopN/zero_is_invalid' -v ./...
=== RUN   TestParseTopN
=== RUN   TestParseTopN/zero_is_invalid
--- PASS: TestParseTopN/zero_is_invalid (0.00s)
PASS

$ go test -cover ./...
ok      github.com/you/ex03     0.004s  coverage: 100.0% of statements
```

Note the subtest name in `-run` replaces spaces with underscores: `zero is invalid` → `zero_is_invalid`.

### Explaining the coverage number

With the four starter cases plus the two TODO cases each, both functions hit 100% statement coverage: every `return`, including both error branches in each function, is exercised. Open the HTML view (`go tool cover -html`) and confirm there are no red lines. If you *remove* the "punctuation only" case from `TestNormalizeWord`, the `if trimmed == ""` error branch goes red and coverage drops — proving the point that coverage's real value is showing you the **untested error branch**, which is almost always where the bug hides.

### Why `(err != nil) != tc.wantErr` is the canonical check

The table carries a boolean `wantErr`, not a specific error string, because this week we only assert "an error happened (or did not)." `(err != nil)` is a bool: true if we got an error. `(err != nil) != tc.wantErr` is true exactly when reality disagrees with the table — we got an error and did not want one, or wanted one and did not get it. In Week 2 we tighten this to `errors.Is(err, ErrInvalidTopN)` to assert *which* error, but the structure stays identical.

### Common pitfalls

1. **`reflect.DeepEqual` on a nil vs empty slice.** Not in this exercise (we compare ints and strings), but worth flagging: `reflect.DeepEqual([]int(nil), []int{})` is `false`. When your function can return either, your test must be deliberate about which it expects. `cmp.Diff` (Week 8) is clearer.
2. **Asserting the error *message* string.** `if err.Error() != "..."` is brittle — the message format is not part of the API contract. Assert the *behaviour* (`wantErr`) this week and the *identity* (`errors.Is`) in Week 2; never the exact string unless the string genuinely is the contract.
3. **Not naming the subtests.** A table with `{name: ""}` produces unnamed subtests you cannot target with `-run`. Always name every case.

## Cross-cutting notes

- **Run the toolchain after every change.** `go test ./... && go vet ./... && staticcheck ./...` is the loop. Wire it to a key in your editor. A finding is a bug you have not fixed yet.
- **`gofmt` is not optional.** Configure format-on-save. A PR with unformatted code is a PR that has not run `go fmt`, and a reviewer will say so before reading the logic.
- **Read the standard library.** Every function you reached for — `strings.ToLower`, `strconv.Atoi`, `sort.Strings`, `strings.Trim` — has its source one click away on <https://pkg.go.dev>. Read one per exercise; it is the cheapest idiomatic-Go education there is.

Cited references: <https://go.dev/ref/mod>, <https://go.dev/blog/slices-intro>, <https://go.dev/blog/maps>, <https://go.dev/blog/defer-panic-and-recover>, <https://pkg.go.dev/testing>, <https://go.dev/wiki/TableDrivenTests>, <https://go.dev/blog/cover>.
