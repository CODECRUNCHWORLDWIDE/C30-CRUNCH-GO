# Week 2 — Homework

Six practice problems, roughly **45 minutes each**. They reinforce the lectures and de-risk the mini-project: by the time you start Lab 02 you will have already written receivers, a small interface, a wrapped-error chain, and a generic function. Put every deliverable under a `homework/` directory in your week-02 working copy, one subfolder per problem (`homework/hw1/`, …), each its own module (`go mod init github.com/you/hw1`). Every problem must end clean under `go vet ./...` and `staticcheck ./...`.

## HW1 — Receivers: prove the method-set rule to yourself (~45 min)

Write a type `Account` with a balance and two methods: `Balance() int` (value receiver) and `Deposit(n int)` (pointer receiver). Then:

1. Write a one-method interface `Depositor` (`Deposit(int)`).
2. In `main`, attempt `var d Depositor = Account{}` and `var d Depositor = &Account{}`. One compiles, one does not — comment each with *why*, citing the method-set rule.
3. Show that `a.Deposit(10)` works on a local `Account` value (auto-`&`) but that you cannot call `Deposit` on a *map element* (`m["x"].Deposit(10)` — map elements are not addressable). Comment the exact error.

**Deliverable:** `hw1/main.go` with the two interface assignments (the failing one commented out with its error) and the addressability demonstration. Cite <https://go.dev/ref/spec#Method_sets>.

## HW2 — Design a small consumer-defined interface (~45 min)

You have a function that needs to write progress messages somewhere — a file in production, a buffer in tests. Write:

1. A `report.Summarize(w io.Writer, items []Item) error` that writes a summary line per item, accepting `io.Writer` (the narrowest thing it uses).
2. A test that passes a `*bytes.Buffer` (not a file) and asserts on the buffer's contents — no disk, no mocks-library.
3. A one-paragraph comment explaining why `Summarize` accepts `io.Writer` rather than `*os.File`, and why it returns the concrete error (not an interface).

**Deliverable:** `hw2/report.go` + `hw2/report_test.go`, green. Cite <https://go.dev/doc/effective_go#interfaces> and <https://go.dev/wiki/CodeReviewComments#interfaces>.

## HW3 — Error wrapping through two layers (~45 min)

Model a config loader with two layers: `readRaw(path) ([]byte, error)` (wraps `os.ReadFile`'s error with `%w`) and `Load(path) (*Config, error)` (calls `readRaw`, wraps its error with `%w` again, and on a parse failure returns a sentinel `var ErrBadFormat = errors.New("config: bad format")`).

1. Demonstrate that `errors.Is(err, fs.ErrNotExist)` is `true` for a missing file *through both wraps* (import `io/fs`).
2. Demonstrate that `errors.Is(err, ErrBadFormat)` is `true` for a malformed file.
3. Add a PREDICT comment: is `errors.Is(missingFileErr, ErrBadFormat)` true or false? Confirm.

**Deliverable:** `hw3/config.go` + a `main` or test proving both `errors.Is` checks. Cite <https://pkg.go.dev/errors> and <https://go.dev/blog/go1.13-errors>.

## HW4 — `errors.Is` vs `errors.As` (~45 min)

Define a sentinel `ErrClosed` and a typed `RateLimitError{RetryAfter time.Duration}` (implementing `error`). Write a `do() error` that randomly (or by argument) returns one wrapped with `%w`. Then write a `classify(err) string` that:

1. Uses `errors.Is(err, ErrClosed)` to detect the sentinel and returns `"closed"`.
2. Uses `errors.As(err, &rle)` to detect the typed error, **reads `rle.RetryAfter`**, and returns `"rate-limited, retry in <d>"`.
3. Returns `"unknown"` otherwise.

Write a table test for `classify` with one case per branch. State in a comment *why* you used `Is` for one and `As` for the other.

**Deliverable:** `hw4/limit.go` + `hw4/limit_test.go`, green. Cite <https://pkg.go.dev/errors#Is> and <https://pkg.go.dev/errors#As>.

## HW5 — A generic function and its table test (~45 min)

Write a generic `Keys[K comparable, V any](m map[K]V) []K` that returns a map's keys (then compare yours to the standard-library `maps.Keys`), and a generic `Max[T cmp.Ordered](xs []T) (T, bool)` that returns the maximum element (`ok=false` for an empty slice).

1. Table-test `Max` at *two* element types (`int` and `string`) — the same test table shape, instantiated twice.
2. Add a PREDICT comment: does `Max([][]int{...})` compile? Why or why not?
3. One sentence: is each function a container, an algorithm, or neither — and would an interface have served better?

**Deliverable:** `hw5/gen.go` + `hw5/gen_test.go`, green. Cite <https://go.dev/doc/tutorial/generics>, <https://pkg.go.dev/cmp>, and <https://pkg.go.dev/maps>.

## HW6 — Essay: generics vs interfaces (~45 min, written)

Write **400–600 words** answering: *"When do generics earn their keep over an interface, and when is an interface the better tool?"* You must:

1. State the Go team's "treats every type the same ⇒ type parameter; behaves differently per type ⇒ interface; neither ⇒ neither" rule and cite <https://go.dev/blog/when-generics>.
2. Give one concrete example where generics win (a container or algorithm) and one where an interface wins (polymorphism over behaviour), each with a few lines of illustrative Go.
3. Name one anti-pattern (e.g. a type parameter used by exactly one type, or a type switch inside a generic) and explain the fix.
4. Tie it to Lab 02: explain *in your own words* why the cache is generic but the policy and store are interfaces.

**Deliverable:** `homework/hw6-essay.md`. No code required to run, but any snippets must compile if extracted. Cite at least <https://go.dev/blog/when-generics> and one of <https://go.dev/doc/effective_go#interfaces> / <https://go.dev/doc/tutorial/generics>.

---

## Submitting

Push your `homework/` directory on your week-02 branch. Each subfolder should build and (where it has tests) pass `go test ./...`, and all must be clean under `go vet ./...` and `staticcheck ./...`. The essay (HW6) is read for whether you can *defend a design choice with a reason and a citation* — the exact skill the mini-project review and the Phase I gate test.

Cited references across this set: <https://go.dev/ref/spec#Method_sets>, <https://go.dev/doc/effective_go#interfaces>, <https://go.dev/wiki/CodeReviewComments#interfaces>, <https://pkg.go.dev/errors>, <https://go.dev/blog/go1.13-errors>, <https://go.dev/doc/tutorial/generics>, <https://go.dev/blog/when-generics>, <https://pkg.go.dev/cmp>, <https://pkg.go.dev/maps>.
