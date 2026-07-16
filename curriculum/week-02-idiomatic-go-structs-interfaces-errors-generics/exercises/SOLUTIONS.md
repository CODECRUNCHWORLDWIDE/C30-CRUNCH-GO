# Week 2 — Exercise Solutions and Annotations

Read these after attempting the exercises, not before. Every Go snippet here has been built (`go build ./...`) and, where it is a test, run (`go test ./...`) and checked clean under `go vet ./...` and `staticcheck ./...`.

## Exercise 1 — Receivers, Method Sets, and a Consumer-Defined Interface

### What success looks like

```
$ go run .
counter after bumpThrice(&c): 3
counter after one more inc via interface: 4
counter after c.Inc() (auto-&): 5
string: hello
int: 42
named: @ada
named: bot-0007
named via value: @grace

$ go vet ./...
$ staticcheck ./...
```

### The five predictions, answered

- **PREDICT 1.** `c.Value()` prints **3**. `bumpThrice(&c)` received a `*Counter`, called `Inc()` three times through the `Incrementer` interface, and each `Inc` mutated the original (pointer receiver). The value is 3.
- **PREDICT 2.** `var inc Incrementer = &c` **compiles**. `Inc` has a pointer receiver, so it is in `*Counter`'s method set but *not* in `Counter`'s. A `*Counter` therefore satisfies `Incrementer`; storing `&c` is legal. After `inc.Inc()`, `c.Value()` is 4.
- **PREDICT 3.** `var inc2 Incrementer = c` (the value) does **not compile**. The exact error is:

  ```
  cannot use c (variable of type Counter) as Incrementer value in variable
  declaration: Counter does not implement Incrementer (method Inc has pointer receiver)
  ```

  This is the method-set rule (Lecture 1 §4): the value `Counter`'s method set excludes the pointer-receiver `Inc`, so `Counter` does not satisfy `Incrementer`. The auto-`&` convenience does **not** apply to interface assignment, because the value being boxed has no guaranteed address.
- **PREDICT 4.** `c.Inc()` **compiles and mutates** `c` (now 5). `c` is a local variable — *addressable* — so Go rewrites `c.Inc()` as `(&c).Inc()`. The auto-`&` works here precisely because there is an address to take. (It would *not* work on, say, a map element, which is not addressable.)
- **PREDICT 5.** The four `describe` outputs are `string: hello`, `int: 42`, `named: @ada` (a `User` matches the `Named` case via its value-receiver `Name()`), and `named: bot-0007` (a `Robot` likewise). The `Named` case matches *any* type implementing `Name()`, so both user-defined types route there rather than to `default`.

The lesson the reviewer cares about: **before you assign a value to an interface, ask "is every method this interface needs in the *value's* method set, or only in the *pointer's*?"** If any required method has a pointer receiver, you must hand over `&t`, not `t`.

### Common pitfalls

1. **Expecting the value to satisfy the interface because `c.Inc()` compiled.** Calling a pointer-receiver method on an addressable value works (auto-`&`); *storing that value in an interface* does not. Two different rules; only the second is the method-set rule.
2. **Mixing receiver kinds without noticing.** `Counter` here deliberately has a pointer `Inc` and a value `Value` to expose the rule. In real code, keep them consistent — give the whole type pointer receivers once any method needs one. `go vet` and reviewers flag the mix.
3. **A one-value type assertion in the type switch.** The type switch (`switch x := v.(type)`) is safe — each case binds `x` to the case type with no panic. The panicking form is the *standalone* `x := v.(T)`; never use it on untrusted input.

## Exercise 2 — Error Wrapping: Sentinel, Typed, errors.Is and errors.As

### What success looks like (after implementing `Unwrap`)

```
$ go run .
err == ErrNotFound: false
errors.Is(err, ErrNotFound): true
expired: key="stale" expiredAt=2026-06-19T...
two-layer error: GET ghost: lookup "ghost": store: key not found
errors.Is(query err, ErrNotFound): true
errors.As(query err, &ExpiredError) bound: true

$ go vet ./...
$ staticcheck ./...
```

The `Unwrap` you implement:

```go
func (e *QueryError) Unwrap() error { return e.Err }
```

### The five predictions, answered

- **PREDICT 1.** `err == ErrNotFound` is **false**. `lookup` returned `fmt.Errorf("lookup %q: %w", key, ErrNotFound)` — a *new* `fmt` wrapping value, not the sentinel itself. Direct `==` compares the outer wrapper to the sentinel and they are different values.
- **PREDICT 2.** `errors.Is(err, ErrNotFound)` is **true**. `errors.Is` walks the chain via `Unwrap` and finds the wrapped `ErrNotFound` underneath the `fmt` wrapper. This is exactly why you use `errors.Is` instead of `==` the moment anything wraps a sentinel.
- **PREDICT 3.** `errors.As(err, &ee)` **binds** `ee` to the wrapped `*ExpiredError`, and `ee.Key` is `"stale"`. `As` walked the one-layer chain, found a `*ExpiredError`, and assigned it into `ee` — so you can read its fields. (Note the `&ee` pointer-to-target argument; `As` must be able to set it.)
- **PREDICT 4.** *Before* implementing `Unwrap` on `QueryError`: `errors.Is(query err, ErrNotFound)` is **false**, because the chain stops at `QueryError` (no `Unwrap`, so `Is` cannot descend to the wrapped `ErrNotFound`). *After* implementing `Unwrap`: it becomes **true** — `QueryError.Unwrap()` exposes `e.Err`, and `Is` continues down to the sentinel. This is the whole point of `Unwrap`: it makes your typed error a good chain citizen.
- **PREDICT 5.** Same story for the typed error: with `Unwrap` implemented, `errors.As(query err, &ee2)` is **true** — `As` descends through `QueryError` to the wrapped `*ExpiredError` two layers down. Without `Unwrap`, it would be `false`.

The lesson: **`==` is layer-zero only; `errors.Is`/`errors.As` are wrap-aware — and your custom error must implement `Unwrap` to be transparent to them.**

### Common pitfalls

1. **Forgetting `Unwrap` on a custom wrapping error.** Without it, `Is`/`As` cannot see past your type, and a caller's "is this a not-found?" silently returns false. If your error stores a cause, expose it with `Unwrap`.
2. **Passing the target by value to `errors.As`.** `errors.As(err, ee)` (without `&`) is a compile-time panic-class mistake — `As` needs `&ee` so it can assign into it. The argument is always a pointer to the target variable.
3. **`%v` where you meant `%w`.** `fmt.Errorf("...: %v", err)` formats the cause into the string and *breaks the chain* — `errors.Is` will no longer find anything underneath. Use `%w` when you want the cause inspectable; `%v` only when you deliberately want to hide it.
4. **String-matching.** `if strings.Contains(err.Error(), "not found")` is the anti-pattern this whole exercise exists to kill. Assert identity (`Is`) or type (`As`); never the message.

## Exercise 3 — Generics: a Set[T comparable] and Map/Filter/Reduce

### The completed `Reduce`

```go
func Reduce[T, A any](s []T, init A, f func(A, T) A) A {
	acc := init
	for _, v := range s {
		acc = f(acc, v)
	}
	return acc
}
```

### What success looks like

```
$ go test -v ./...
=== RUN   TestSet
=== RUN   TestSet/dedupes
=== RUN   TestSet/empty
=== RUN   TestSet/absent_probe
--- PASS: TestSet (0.00s)
    --- PASS: TestSet/dedupes (0.00s)
    ...
=== RUN   TestMap
--- PASS: TestMap (0.00s)
=== RUN   TestFilter
--- PASS: TestFilter (0.00s)
=== RUN   TestReduce
--- PASS: TestReduce (0.00s)
PASS
ok      github.com/you/ex03     0.005s

$ go test -run 'TestSet/dedupes' -v ./...
=== RUN   TestSet
=== RUN   TestSet/dedupes
--- PASS: TestSet/dedupes (0.00s)
PASS

$ go vet ./... && staticcheck ./...
```

### The three predictions, answered

- **PREDICT 1.** `NewSet("a", "b", "a")` works and yields a set of length 2 (the duplicate `"a"` is stored once). `NewSet([]int{1}, []int{2})` does **not compile**: the element type would be `[]int`, and slices are **not comparable** (you cannot use them as map keys or compare with `==`), so they violate the `comparable` constraint. The error is roughly `[]int does not implement comparable`.
- **PREDICT 2.** The zero value of `A` is whatever `A`'s zero is for the instantiated type — `0` for `int`, `""` for `string`, `nil` for a pointer/slice/map. In `Reduce`, if a caller passes `init` equal to `A`'s zero, the fold simply starts from there; `Reduce(nil-slice, init, f)` returns `init` unchanged because the loop runs zero times. (That "empty slice returns init" property is the case the TODO asks you to add.)
- **PREDICT 3.** `Map` constrains `T` and `U` only to `any` because it never *inspects* the elements — it just passes each `T` to `f` and stores the resulting `U`. It needs no `==`, no `<`, no map-key usage, so the unconstrained `any` is correct. `Set` needs `comparable` because it uses `T` as a *map key* (`m[v]`), which requires `==`. The constraint is exactly "the operations the body performs," no more.

### The container/algorithm question, answered

- **`Set[T comparable]`** — a **container**. Generics are right: the storage logic is identical for every comparable element type, and an interface could not give you a *typed* `Add(T)`/`Has(T)` without `any` and a runtime assertion.
- **`Map` / `Filter` / `Reduce`** — **type-parametric algorithms**. Generics are right: the algorithm is the same for every element type; only the type varies. An interface here would force `[]any` and lose the element types.
- Would an interface have been better for any of them? **No** — none of these needs to *behave differently per type*; they treat every type the same. That is the signal for a type parameter, not an interface (Lecture 3 §6).

### Common pitfalls

1. **Constraining too tightly.** Don't write `Map[T comparable, U any]` "to be safe" — `Map` never compares, so `comparable` would needlessly reject `[]int` elements. Constrain to exactly what the body uses.
2. **`reflect.DeepEqual` on nil vs empty slices.** `Filter` returns `make([]T, 0, ...)` — an *empty, non-nil* slice — even when nothing matches. `reflect.DeepEqual([]int{}, []int(nil))` is `false`, so a "filter matches nothing" test must expect `[]int{}` (empty), not `nil`. Be deliberate; `cmp.Diff` (Week 8) is clearer.
3. **Trying to put a type parameter on a method.** Go has no method type parameters — you cannot write `func (s *Set[T]) Map[U any](...)`. Make cross-type transforms free functions (`Map(slice, f)`), not methods.

## Cross-cutting notes

- **Run the toolchain after every change.** `go test ./... && go vet ./... && staticcheck ./...` is the loop. A finding is a bug you have not fixed yet — and `go vet` earns its keep this week on receivers (it flags a lock copied by a value receiver) and on `%w` (it flags a non-error handed to `%w`).
- **Assert behaviour and identity, never the message.** Week 1 asserted `wantErr bool`; Week 2 tightens it to `errors.Is(err, want)` (sentinel) or `errors.As` (typed). Never `err.Error() == "..."`.
- **Smallest interface, narrowest constraint.** The same instinct governs both: an interface lists only the methods the consumer calls; a constraint lists only the operations the generic body performs. Anything more is speculative and a reviewer will trim it.
- **Read the standard library.** `slices`, `maps`, and `cmp` are the canonical generic Go — `slices.Sort`, `slices.Contains`, `maps.Keys`, `cmp.Compare`. Read one per exercise; it is the cheapest modern-Go education there is. <https://pkg.go.dev/slices>, <https://pkg.go.dev/maps>, <https://pkg.go.dev/cmp>.

Cited references: <https://go.dev/ref/spec#Method_sets>, <https://go.dev/doc/effective_go#interfaces>, <https://pkg.go.dev/errors>, <https://go.dev/blog/go1.13-errors>, <https://go.dev/doc/tutorial/generics>, <https://go.dev/blog/when-generics>, <https://pkg.go.dev/cmp>, <https://go.dev/wiki/TableDrivenTests>.
