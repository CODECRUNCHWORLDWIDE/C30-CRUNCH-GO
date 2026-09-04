# Week 2 — Idiomatic Go: Methods and Receivers, Consumer-Defined Interfaces, the Error-Wrapping Contract, and Generics

Welcome to **C30 · Crunch Go**, Week 2. Last week you stood up a module, built a static binary, wrote slices and maps that did not surprise you, and gave a `(T, error)` function a table-driven test suite that ran clean under `go vet` and `staticcheck`. You established two reflexes — check every `err`, and make the zero value useful — and you ended on the promise that we would go deep on *constructing* error chains. This week we cash that promise, and we add the other three pillars of idiomatic Go: methods and the receiver rules, the interface-at-the-consumer discipline, and generics. By Friday you will have built a generic `Cache[K comparable, V any]` with a pluggable eviction policy behind a small interface, typed errors you check with `errors.Is`, and two implementations of a `Store[K, V]` interface — and you will be able to defend every design choice in review. This is the week Go stops looking like "a smaller Java" and starts looking like itself.

The first thing to internalize is that **a method is just a function with a receiver, and the receiver is either a value or a pointer — and that choice is load-bearing**. `func (c Counter) Value() int` takes a *copy* of the `Counter`; `func (c *Counter) Inc()` takes a *pointer* and can mutate the original. The rule you will repeat all week: a method that mutates the receiver, or whose receiver is large, takes a pointer receiver; everything else can take a value receiver — but you keep the receiver kind *consistent across a type's whole method set*, because mixing them is a smell a reviewer flags immediately. Coming from a class-based language, the instinct is "everything is a method on the object"; in Go the instinct is "is this a copy or a reference, and why." Citation: Effective Go's methods section at <https://go.dev/doc/effective_go#methods> and the Tour's methods module at <https://go.dev/tour/methods/1>.

The second thing to internalize is **the method-set rule, which is the single most-missed detail in Go interview prep**: the method set of a value `T` contains only its value-receiver methods, while the method set of a pointer `*T` contains *both* its value-receiver and its pointer-receiver methods. The consequence is concrete and bites in real code: if `Inc()` has a pointer receiver, then `*Counter` satisfies an interface that requires `Inc()`, but `Counter` (the value) does *not*. You can call `c.Inc()` on an addressable value because Go inserts the `&c` for you, but you cannot store that value in the interface. This is why "does my type satisfy this interface" sometimes depends on whether you hand over `t` or `&t`. We make you predict this in Exercise 1. Citation: the spec's "Method sets" at <https://go.dev/ref/spec#Method_sets>.

The third thing to internalize is **interfaces in Go are small, satisfied implicitly, and defined where they are consumed — not where they are implemented**. There is no `implements` keyword: a type satisfies an interface by having the right methods, full stop, and it never declares that it does. The corollary that separates idiomatic Go from ported-Java is that you define the interface in the *consumer* package — the package that calls the method — typed as narrowly as that consumer needs (often one method). The standard library's `io.Reader` and `io.Writer` are one method each, and that is why everything composes. The reviewer's heuristic, straight from the Go team: "the bigger the interface, the weaker the abstraction." Citation: Effective Go's interfaces section at <https://go.dev/doc/effective_go#interfaces> and the Go Code Review Comments interface guidance at <https://go.dev/wiki/CodeReviewComments#interfaces>.

The fourth thing to internalize is the maxim **"accept interfaces, return structs."** A function should take the *narrowest interface* it actually needs as a parameter (so any caller can pass anything that fits), but return a *concrete struct* (so the caller gets the full, documented type with all its methods and fields, not a lossy abstraction). Returning an interface forces every caller to type-assert their way back to capability and locks you out of adding methods without breaking the abstraction; returning the concrete type keeps your API honest and extensible. This one sentence resolves a surprising fraction of Go design arguments, and we restate it in the closing promise of this week because it is the discipline the whole track leans on. Citation: the Go Code Review Comments at <https://go.dev/wiki/CodeReviewComments#interfaces> and the FAQ on interface design at <https://go.dev/doc/faq#guarantee_satisfies_interface>.

The fifth thing to internalize is that **`errors.New` and `fmt.Errorf("%w", err)` build a chain, and `errors.Is` / `errors.As` are how you inspect it without string-matching**. An error wrapped with `%w` remembers the error underneath it; `errors.Is(err, ErrNotFound)` walks that chain looking for a *sentinel* you compare against, and `errors.As(err, &target)` walks it looking for a *typed* error you can extract fields from. The cardinal sin you are unlearning this week is `if err.Error() == "not found"` — string-matching an error message, which is brittle because the message is not part of the contract. You assert *identity* (`Is`) or *type* (`As`), never the string. This is the contract that makes Go's verbose-but-honest error handling actually maintainable at scale. Citation: the `errors` package docs at <https://pkg.go.dev/errors> and the Go 1.13 errors blog post at <https://go.dev/blog/go1.13-errors>.

The sixth thing to internalize is **the choice between a sentinel error and a typed error, and when to wrap versus annotate**. A *sentinel* is a package-level `var ErrNotFound = errors.New("not found")` — a single comparable value callers test with `errors.Is`, perfect when the only thing the caller needs to know is *which* failure. A *typed* error is a struct implementing `error` (and usually `Unwrap`) that carries *data* — a `*ExpiredError{Key, ExpiredAt}` the caller extracts with `errors.As` to learn *how* it failed. You wrap (`%w`) when an upstream caller might reasonably want to inspect the cause; you annotate with `%v` (or a fresh `errors.New`) when the cause is an implementation detail you do *not* want to leak into your API surface. Choosing wrong leaks abstraction or hides cause; choosing right is a senior-level habit. Citation: the error-handling blog post at <https://go.dev/blog/error-handling-and-go> and <https://go.dev/blog/go1.13-errors>.

The seventh thing to internalize is that **generics add type parameters to functions and types, constrained by interfaces, so you can write one `Map`/`Filter`/`Stack`/`Set` that works for every element type without `interface{}` and without reflection**. A type parameter is written `func Map[T, U any](s []T, f func(T) U) []U`; a *constraint* is an interface that says what operations the type must support — `comparable` for things you can use as map keys or compare with `==`, `cmp.Ordered` for things you can `<`-compare, or a custom constraint interface. Generics give you compile-time type safety and no boxing, where before you had `interface{}` plus a runtime type assertion that could panic. They arrived in Go 1.18 and are now load-bearing across the standard library (`slices`, `maps`, `cmp`). Citation: the generics tutorial at <https://go.dev/doc/tutorial/generics> and the "Intro to generics" blog post at <https://go.dev/blog/intro-generics>.

The eighth thing to internalize is **the generics-versus-interfaces decision, which is a real fork and not a style preference**. Reach for generics when you are writing a *container* (a `Cache[K, V]`, a `Set[T]`, a `Stack[T]`) or a *type-parametric algorithm* (`Map`, `Filter`, `Min`) where the element type varies but the logic is identical — generics give you type safety with no runtime cost. Reach for an *interface* when you have many concrete types that share *behaviour* but whose implementations differ (a `Store` with in-memory and file-backed bodies, an `io.Writer` to a file or a socket) — that is polymorphism over *behaviour*, which is exactly what interfaces are for. The Go team's own guidance, which you will cite in review, is "if the body of a function treats every type the same, use a type parameter; if it must behave differently per type, use an interface; if it does neither, use neither." Lab 02 makes the contrast vivid: the cache *container* is generic, and its eviction policy and store are *interfaces*. Citation: the "When to use generics" blog post at <https://go.dev/blog/when-generics> and the `cmp` package at <https://pkg.go.dev/cmp>.

By the end of the week you will be the person who, handed a design, can say "that interface is too big — split it and define it at the consumer," "that should be a typed error so the caller can read the key," and "that's a container, so it's generic, but the policy behind it is an interface" — and back each call with a `go.dev` citation. That fluency is what Phase I exists to build, and it is the foundation the concurrency model (Weeks 3–4) and the service layer (Weeks 5–8) are written in.

## Learning objectives

By the end of this week, you will be able to:

- **Write** methods with value and pointer receivers, choose the receiver kind deliberately (mutation or large struct ⇒ pointer; consistency across the type's method set), and explain the copy-vs-reference consequence. Cite <https://go.dev/doc/effective_go#methods> and <https://go.dev/tour/methods/1>.
- **Apply** the method-set rule — a value's method set excludes its pointer-receiver methods — to predict exactly when `T` versus `*T` satisfies an interface, and explain the auto-`&` on addressable values. Cite <https://go.dev/ref/spec#Method_sets>.
- **Design** a small, consumer-defined interface (often one method), explain why implicit satisfaction and the no-`implements`-keyword rule matter, and defend "the bigger the interface, the weaker the abstraction" in review. Cite <https://go.dev/doc/effective_go#interfaces> and <https://go.dev/wiki/CodeReviewComments#interfaces>.
- **Articulate and apply** "accept interfaces, return structs," and say what goes wrong when you return an interface instead of a concrete type. Cite <https://go.dev/wiki/CodeReviewComments#interfaces>.
- **Use** the empty interface / `any`, type assertions (`v, ok := x.(T)`), and the type switch (`switch v := x.(type)`) — and explain why each appears far less in idiomatic generic-era Go. Cite <https://go.dev/tour/methods/14> and <https://go.dev/ref/spec#Type_assertions>.
- **Build** a wrapped-error chain with `fmt.Errorf("...: %w", err)` and inspect it with `errors.Is` (sentinel identity) and `errors.As` (typed extraction); never string-match an error message. Cite <https://pkg.go.dev/errors> and <https://go.dev/blog/go1.13-errors>.
- **Decide** between a sentinel error (`var ErrX = errors.New(...)`) and a typed error (a struct implementing `error` and `Unwrap`), and between wrapping (`%w`) and annotating (`%v`), with reasons. Cite <https://go.dev/blog/error-handling-and-go>.
- **Write** generic functions and generic types with type parameters and constraints — `any`, `comparable`, `cmp.Ordered`, and a custom constraint — and explain type inference and explicit instantiation. Cite <https://go.dev/doc/tutorial/generics> and <https://go.dev/blog/intro-generics>.
- **Decide** between generics and interfaces with the container/algorithm/neither matrix, and articulate when *not* to reach for generics. Cite <https://go.dev/blog/when-generics> and <https://pkg.go.dev/cmp>.
- **Keep** every artifact clean under `go vet`, `staticcheck`, and `go test` — including a table-driven suite that checks errors with `errors.Is`. Cite <https://pkg.go.dev/testing> and <https://staticcheck.dev>.

## Standards this week meets

| Bar | What this week is measured against |
| --- | --- |
| University | `SWE 432` — Past the outcome set: neither ledger course assesses receiver choice, consumer-defined interfaces or type parameters on their own. Week 2 builds the domain error taxonomy that the Server-Side Web Programming outcome "return failure to a client in one consistent shape" depends on from Week 5 onward. |
| Industry | Design the seam a team has to live behind for years: one small interface defined by the code that consumes it, a receiver choice you can defend in review, and an error surface a caller three layers up can still interrogate without matching on message text. |
| Beyond the bar | The error taxonomy is designed, implemented and then *proved* — a test suite walks `errors.Is` and `errors.As` across a multi-layer wrapped chain, so the contract is demonstrated rather than asserted in prose — `challenges/challenge-02-error-taxonomy.md` |

## Prerequisites

- **Week 1 complete.** You have a module muscle-memory, you check every `err`, you write table-driven tests with `t.Run`, and your code is clean under `go vet` and `staticcheck`. If `go build ./...` versus `go build` still feels fuzzy, re-read Week 1 Lecture 1 first.
- **The Go toolchain, 1.22 or newer.** Generics require 1.18+; the track standardises on 1.22+ for the loop-variable fix, `slog`, and 1.22 routing. Verify with `go version`. The `cmp.Ordered` constraint and the `slices`/`maps` packages used here are standard-library as of 1.21.
- **`staticcheck`** installed (`go install honnef.co/go/tools/cmd/staticcheck@latest`) and on your `PATH`. The week's contract — clean under `go vet` and `staticcheck` — carries forward unchanged.
- **An editor with `gopls`.** Generics-aware completion and inline `go vet` make the receiver and constraint material far easier to learn by experiment. <https://pkg.go.dev/golang.org/x/tools/gopls>.

## Topics covered

- **Methods.** Method declaration syntax, the receiver, methods on any local named type (not just structs), the method value and method expression.
- **Value vs pointer receivers.** Copy vs reference semantics; the "mutate or large ⇒ pointer" rule; consistency across a type's method set; the nil-pointer-receiver pattern.
- **Method sets.** The value set excludes pointer-receiver methods; the auto-`&` on addressable values; why this governs interface satisfaction.
- **Interfaces.** Implicit satisfaction (no `implements`); small, single-method interfaces; the consumer-defined-interface rule; interface embedding (`io.ReadWriter`); the empty interface and `any`.
- **"Accept interfaces, return structs."** Narrow parameters, concrete returns; what breaks when you return an interface; the compile-time satisfaction assertion `var _ Iface = (*T)(nil)`.
- **Type assertions and type switches.** `v, ok := x.(T)`, the panic on the one-value form, the `switch v := x.(type)` form, and why generics shrank their footprint.
- **Embedding vs inheritance.** Struct embedding promotes fields and methods (composition, not subtyping); no virtual dispatch; the difference from an `extends` relationship.
- **The error interface revisited.** `error` as a one-method interface; `errors.New`; `fmt.Errorf` with and without `%w`.
- **Wrapping and unwrapping.** `%w` builds a chain; `Unwrap() error`; `errors.Unwrap`; multi-error wrapping with `errors.Join` (preview).
- **`errors.Is` vs `errors.As`.** Sentinel-identity testing vs typed extraction; how each walks the chain; implementing `Is`/`As` on a custom error.
- **Sentinel vs typed errors.** `var ErrNotFound = errors.New(...)` vs a struct error carrying data; when to wrap vs annotate; not leaking implementation detail into the API.
- **Generics — type parameters.** `[T any]`, `[K comparable, V any]`, multiple type parameters, generic functions and generic types.
- **Constraints.** `any`, `comparable`, `cmp.Ordered`, the `constraints` package, custom constraint interfaces, union elements (`~int | ~string`), the `~` (underlying-type) token.
- **Instantiation and inference.** Explicit instantiation (`Map[int, string]`), type inference from arguments, where inference fails.
- **Generics vs interfaces.** The container/algorithm/neither matrix; the runtime-cost and type-safety trade; when *not* to use generics.

## Weekly schedule

The schedule adds up to approximately **36 hours**. Treat it as a target, not a contract. The interface and error material rewards drawing the type relationships by hand; the generics material rewards typing the code into the playground and watching inference succeed and fail.

| Day       | Focus                                                              | Lectures | Exercises | Challenges | Quiz/Read | Homework | Mini-Project | Self-Study | Daily Total |
|-----------|-------------------------------------------------------------------|---------:|----------:|-----------:|----------:|---------:|-------------:|-----------:|------------:|
| Monday    | Methods, receivers, method sets, small interfaces                 |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Tuesday   | "Accept interfaces, return structs", embedding, type switches     |    2h    |    1.5h   |     0h     |    0.5h   |   1h     |     0.5h     |    0.5h    |     6h      |
| Wednesday | Error values, `%w` wrapping, `errors.Is` / `errors.As`, sentinels | 2h       |    2h     |     0h     |    0.5h   |   1h     |     0.5h     |    0.5h    |     6.5h    |
| Thursday  | Generics — type parameters, constraints, the decision matrix      |    2h    |    1.5h   |     1.5h   |    0.5h   |   1h     |     0.5h     |    0.5h    |     7.5h    |
| Friday    | Mini-project — build the generic `Cache[K, V]`                    |    0h    |    0h     |     0.5h   |    0.5h   |   0h     |     3.5h     |    0.5h    |     5h      |
| Saturday  | Mini-project polish, file-backed store, the `errors.Is` test pass |    0h    |    0h     |     0h     |    0h     |   0h     |     3h       |    0h      |     3h      |
| Sunday    | Quiz, review, "where does an interface beat a type parameter"     |    0h    |    0h     |     0h     |    1h     |   0h     |     0h       |    1h       |    2h      |
| **Total** |                                                                   | **8h**   | **7h**    | **2h**     | **3.5h**  | **4h**   | **8h**       | **3.5h**   | **36h**     |

## How to navigate this week

| File | What's inside |
|------|---------------|
| [README.md](./README.md) | This overview (you are here) |
| [resources.md](./resources.md) | Effective Go (methods, interfaces, embedding), the `errors` docs, the Go 1.13 errors post, the generics tutorial and the "when generics" post, `cmp` |
| [lecture-notes/01-methods-receivers-and-interfaces.md](./lecture-notes/01-methods-receivers-and-interfaces.md) | Methods, value vs pointer receivers, the method-set rule, small consumer-defined interfaces, "accept interfaces return structs", `any`, type assertions, type switches, interface embedding |
| [lecture-notes/02-errors-wrapping-sentinel-and-typed.md](./lecture-notes/02-errors-wrapping-sentinel-and-typed.md) | The error interface, `errors.New`, `fmt.Errorf("%w", err)`, `errors.Is` vs `errors.As`, sentinel vs typed errors, `Unwrap`, when to wrap vs annotate |
| [lecture-notes/03-generics-type-parameters-and-constraints.md](./lecture-notes/03-generics-type-parameters-and-constraints.md) | Type parameters, constraints (`any`, `comparable`, `cmp.Ordered`, custom), generic functions and types (`Stack[T]`, `Set[T]`), inference, the generics-vs-interfaces decision matrix, when not to use generics |
| [exercises/exercise-01-receivers-and-interfaces.go](./exercises/exercise-01-receivers-and-interfaces.go) | Pointer vs value receiver method sets, a consumer-defined interface, a type switch — with PREDICT comments |
| [exercises/exercise-02-error-wrapping.go](./exercises/exercise-02-error-wrapping.go) | Sentinel errors, `%w` wrapping, `errors.Is` and `errors.As` over a wrapped chain |
| [exercises/exercise-03-generics.go](./exercises/exercise-03-generics.go) | A generic `Set[T comparable]` plus `Map`/`Filter`/`Reduce`, with a table-test skeleton to complete |
| [exercises/SOLUTIONS.md](./exercises/SOLUTIONS.md) | Annotated solutions, "what success looks like" transcripts, the PREDICT answers, common pitfalls, cross-cutting notes |
| [challenges/challenge-01-pluggable-eviction-policy.md](./challenges/challenge-01-pluggable-eviction-policy.md) | Design a small `EvictionPolicy` interface (LRU vs FIFO) behind a cache; acceptance + stretch |
| [challenges/challenge-02-error-taxonomy.md](./challenges/challenge-02-error-taxonomy.md) | Design a typed error taxonomy for a small domain and prove `errors.Is`/`errors.As` behaviour across a wrapped chain |
| [quiz.md](./quiz.md) | 10 multiple-choice questions on receivers, method sets, interface satisfaction, `%w`, `errors.Is`/`As`, sentinel vs typed, type parameters, constraints, `comparable`, generics vs interfaces |
| [homework.md](./homework.md) | Six ~45-minute practice problems for the week |
| [mini-project/README.md](./mini-project/README.md) | Full spec for **Lab 02 — generic `Cache[K comparable, V any]`**: TTL cache, pluggable eviction policy, typed errors checked with `errors.Is`, a `Store[K, V]` interface with in-memory and file-backed implementations, full table tests |

## The "accept interfaces, return structs" promise

C30 adds one design rule to the toolchain contract this week, and it is the rule a Go reviewer applies before they read your logic:

> **Accept interfaces, return structs** — take the narrowest interface you actually use as a parameter, and return the concrete type so callers keep the full, documented surface.

A function that accepts `io.Reader` instead of `*os.File` works with a file, a network connection, a `strings.Reader`, and a test fixture, all for free. A function that *returns* `io.Reader` instead of `*bytes.Buffer` forces every caller to type-assert their way back to the methods they need and locks you out of adding a method later without breaking them. The interfaces you define live in the *consumer* package and are as small as the consumer needs — often one method — because **the bigger the interface, the weaker the abstraction.** In review you will be asked, of every interface you introduce, "who consumes this, and what is the smallest set of methods they call?" — and "is this returning a concrete type?"

And the Week 1 contract carries forward unchanged. Every artifact you ship in this week's exercises and the mini-project must produce empty output from all three commands:

```
$ go vet ./...
$ staticcheck ./...
$ go test ./...
ok      github.com/you/cache        0.018s
```

A `go vet` or `staticcheck` finding is not a style nit; it is a bug you have not fixed yet. This week vet earns its keep on receivers specifically — it flags a lock copied by a value receiver and a `%w` verb handed something that is not an error — so run it after every change.

> **Note on dependencies.** Everything this week is standard-library only. `errors`, `fmt`, `cmp` (1.21+), `slices`, and `maps` are all in the standard library; the generics you write need no third-party packages. The one place you may *optionally* reach outside is `golang.org/x/exp/constraints` for the pre-1.21 `constraints.Ordered`, but with 1.21+ you use the built-in `cmp.Ordered` and need nothing external. Reaching for a generics utility library this early is a decision you justify, not a default — the standard `slices` and `maps` packages already cover most of what a beginner reaches for.
