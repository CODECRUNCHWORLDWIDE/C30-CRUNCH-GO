# Week 2 — Quiz

Ten multiple-choice questions covering receivers and method sets, interface satisfaction, "accept interfaces return structs", `%w` wrapping, `errors.Is` vs `errors.As`, sentinel vs typed errors, type parameters, constraints, `comparable`, and the generics-vs-interfaces decision. Treat the quiz as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — Receivers

A method `func (c *Counter) Inc()` has a pointer receiver. Which statement is correct?

- (A) `Inc` cannot mutate the `Counter`; pointer receivers are read-only.
- (B) `Inc` operates on the original `Counter` and can mutate it; a value receiver would operate on a copy and lose the mutation.
- (C) You may not call `Inc` on a `Counter` value, only on a `*Counter`, with no exceptions.
- (D) Pointer and value receivers are interchangeable and never affect behaviour.

<details>
<summary>Answer</summary>

**(B).** A pointer receiver operates on the original through a pointer, so it can mutate; a value receiver operates on a copy and mutations are lost. (C) is wrong because you *can* call `Inc` on an addressable `Counter` value — Go inserts the `&` automatically. Citation: <https://go.dev/doc/effective_go#methods>.

</details>

## Question 2 — Method sets

`Logfile` has one method, `func (l *Logfile) Write(p []byte) (int, error)` (pointer receiver). Which satisfies `io.Writer`?

- (A) Both `Logfile` (value) and `*Logfile`.
- (B) `Logfile` (value) only.
- (C) `*Logfile` only — the value `Logfile`'s method set excludes pointer-receiver methods.
- (D) Neither; `Write` must have a value receiver to satisfy `io.Writer`.

<details>
<summary>Answer</summary>

**(C).** The method-set rule: a value `T`'s method set contains only its value-receiver methods; `*T`'s method set contains both. `Write` has a pointer receiver, so only `*Logfile` satisfies `io.Writer`; the value `Logfile` does not. Citation: <https://go.dev/ref/spec#Method_sets>.

</details>

## Question 3 — Interface satisfaction

How does a type declare that it satisfies an interface in Go?

- (A) With an `implements` keyword: `type T implements Iface`.
- (B) By registering with the interface at `init()` time.
- (C) It does not declare anything; satisfaction is implicit and structural — having the right methods *is* satisfying the interface.
- (D) By embedding the interface as a field.

<details>
<summary>Answer</summary>

**(C).** Satisfaction is implicit and structural — there is no `implements` keyword and no registration. A type satisfies an interface exactly by having the required methods, even if its author never knew the interface existed. Citation: <https://go.dev/doc/effective_go#interfaces>.

</details>

## Question 4 — Accept interfaces, return structs

Why does idiomatic Go prefer to *return* a concrete struct rather than an interface?

- (A) Interfaces cannot be returned from functions in Go.
- (B) Returning the concrete type keeps the full surface for the caller, lets you add methods later without breaking callers, and avoids an unnecessary boxing allocation; returning an interface is lossy and brittle.
- (C) Returning a struct is faster only because structs are always stack-allocated.
- (D) There is no preference; returning interfaces is equally idiomatic in all cases.

<details>
<summary>Answer</summary>

**(B).** "Accept interfaces, return structs": returning the concrete type gives the caller the full documented surface, lets you add methods without breaking callers, and avoids boxing the value into an interface. Returning an interface is lossy (caller must assert back to capability) and brittle (adding a method breaks implementers). Citation: <https://go.dev/wiki/CodeReviewComments#interfaces>.

</details>

## Question 5 — `%w` wrapping

What is the difference between `fmt.Errorf("ctx: %w", err)` and `fmt.Errorf("ctx: %v", err)`?

- (A) None; `%w` and `%v` are aliases for errors.
- (B) `%w` *wraps* `err`, keeping it inspectable by `errors.Is`/`errors.As`; `%v` formats `err` into the message and breaks the chain (the cause is no longer findable).
- (C) `%v` wraps; `%w` only formats.
- (D) `%w` panics if `err` is nil; `%v` does not.

<details>
<summary>Answer</summary>

**(B).** `%w` wraps the error so it stays inspectable via the `Unwrap` chain that `errors.Is`/`errors.As` walk; `%v` formats it into the message text and stops the chain there. Wrapping also makes the wrapped error part of your API contract — a deliberate choice. Citation: <https://go.dev/blog/go1.13-errors>, <https://pkg.go.dev/fmt#Errorf>.

</details>

## Question 6 — `errors.Is`

Given `err := fmt.Errorf("load: %w", ErrNotFound)`, which is true?

- (A) `err == ErrNotFound` is true.
- (B) `err == ErrNotFound` is true but `errors.Is(err, ErrNotFound)` is false.
- (C) `err == ErrNotFound` is false, but `errors.Is(err, ErrNotFound)` is true because `Is` walks the wrapped chain.
- (D) Both are false; once wrapped, a sentinel can never be detected.

<details>
<summary>Answer</summary>

**(C).** The wrapped error is a *new* `fmt` value, so `err == ErrNotFound` is false. But `errors.Is(err, ErrNotFound)` walks the chain via `Unwrap` and finds the wrapped sentinel — which is exactly why you use `errors.Is` instead of `==` once anything wraps the sentinel. Citation: <https://pkg.go.dev/errors#Is>.

</details>

## Question 7 — `errors.Is` vs `errors.As`

You need to detect a failure *and read a field off it* (a `RetryAfter` duration on a typed `*RateLimitError`). Which do you use, and how?

- (A) `errors.Is(err, RateLimitError{})` — it returns the matched value.
- (B) `errors.As(err, &rle)` with `var rle *RateLimitError` — it walks the chain, binds `rle` to the typed error, and lets you read `rle.RetryAfter`.
- (C) `err.(*RateLimitError)` is the only way; `errors.As` cannot read fields.
- (D) `err.Error()` and parse the duration out of the message string.

<details>
<summary>Answer</summary>

**(B).** `errors.As(err, &rle)` searches the chain for an error assignable to `*rle`'s type, binds it, and lets you read its fields. Note the pointer-to-target argument. `errors.Is` only tests identity (yes/no); it does not give you the value. (D) string-parses the message — the anti-pattern. Citation: <https://pkg.go.dev/errors#As>.

</details>

## Question 8 — Sentinel vs typed

When is a *sentinel* error (`var ErrX = errors.New(...)`) the right choice over a *typed* error?

- (A) Always; typed errors are discouraged in Go.
- (B) When the caller only needs to know *which* failure occurred (identity), with no extra data to read — checked with `errors.Is`. A typed error is for when the caller needs *details* (fields), read with `errors.As`.
- (C) Only inside the standard library.
- (D) When the error must carry a stack trace.

<details>
<summary>Answer</summary>

**(B).** A sentinel answers *which* failure (identity), checked with `errors.Is`; a typed error answers *with what details* (fields), read with `errors.As`. Both are valid and both join your public contract; choose per error based on whether the caller needs data. Citation: <https://go.dev/blog/error-handling-and-go>, <https://go.dev/blog/go1.13-errors>.

</details>

## Question 9 — Type parameters and constraints

In `type Cache[K comparable, V any]`, why is `K` constrained to `comparable` while `V` is `any`?

- (A) `comparable` makes `K` faster to store.
- (B) `K` is used as a map key, which requires `==` support — exactly what `comparable` provides; `V` is only stored and retrieved, never compared, so it needs no constraint.
- (C) `any` is not allowed for the second type parameter.
- (D) It is an arbitrary convention with no functional reason.

<details>
<summary>Answer</summary>

**(B).** `K` is a map key, and map keys must support `==` — precisely what `comparable` requires. `V` is only stored and returned, never compared, so it needs no constraint and `any` is correct. A constraint should list exactly the operations the body performs, no more. Citation: <https://go.dev/ref/spec#Comparison_operators>, <https://go.dev/doc/tutorial/generics>.

</details>

## Question 10 — Generics vs interfaces

You are writing code that must *behave differently depending on the concrete type* — write to a file one way, to a socket another. Generics or an interface?

- (A) Generics — a type parameter dispatches to the right code per type.
- (B) An interface — when the function must behave differently per type, that is polymorphism over behaviour, which interfaces express and a type parameter cannot. Generics are for when the logic is *identical* across types (containers, algorithms).
- (C) Neither; use a giant type switch over `any`.
- (D) Always generics in modern Go; interfaces are deprecated.

<details>
<summary>Answer</summary>

**(B).** Behaviour that differs per concrete type is polymorphism over behaviour — an interface, with each type implementing the method its own way. A type parameter abstracts over types whose handling is *identical* (containers like `Cache[K,V]`, algorithms like `Map`). The Go team's rule: same logic for every type ⇒ generics; different behaviour per type ⇒ interface; neither ⇒ neither. Citation: <https://go.dev/blog/when-generics>.

</details>

---

## Self-assessment

- 9-10: you are fluent in the Week 2 idioms; ship the mini-project without further reading.
- 7-8: re-read the lecture-note sections on the questions you missed — the method-set rule (L1 §4), `Is` vs `As` (L2 §5–§6), and the decision matrix (L3 §6) are the usual gaps.
- 5-6: re-read all three lecture notes and redo the exercises before the mini-project; the cache touches every concept here.
- 0-4: rewind to Lecture 1 and work all three lecture notes and exercises carefully. Lab 02 depends on receivers, small interfaces, wrapped errors, and generics together.
