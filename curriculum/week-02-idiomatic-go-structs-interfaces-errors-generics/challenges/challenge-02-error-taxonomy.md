# Challenge 2 — Design an Error Taxonomy and Prove It Across a Wrapped Chain

> **Time:** 1.5 hours. **Prerequisites:** Lecture 2 (wrapping, sentinel vs typed, `errors.Is`/`errors.As`). **Citations:** the `errors` package docs at <https://pkg.go.dev/errors>, the Go 1.13 errors blog post at <https://go.dev/blog/go1.13-errors>, the "Error handling and Go" post at <https://go.dev/blog/error-handling-and-go>, and `fmt.Errorf` at <https://pkg.go.dev/fmt#Errorf>.

## The premise

A real service's error surface is not one `error` type — it is a small, deliberate *taxonomy*: a handful of sentinels for "which failure," a handful of typed errors for "with what details," and a wrapping discipline that lets a caller three layers up still ask the right question. This challenge is to *design* that taxonomy for a small domain, *implement* it, and *prove* — with tests, not assertions in prose — that `errors.Is` and `errors.As` behave correctly across a multi-layer wrapped chain. It is the design work behind the mini-project's error handling, isolated so you can think about it on its own.

## The domain

Pick a small domain with genuinely different failure *kinds*. A good one is a **user-account store** with three layers:

```
handler  ->  service  ->  repository (the bottom; produces the raw errors)
```

The repository can fail in ways the caller must distinguish:

- **Not found** — the account does not exist. The caller may want to return 404 / create-on-miss.
- **Already exists** — a create collided with an existing account. The caller may want 409.
- **Validation failed** — the input was malformed, *and the caller needs to know which field*. The caller wants 400 *with the field name in the message*.
- **Backend unavailable** — the store is down, *and there may be a retry-after hint*. The caller wants 503 *and the retry duration*.

## The design task

Classify each failure as **sentinel** or **typed**, and justify each choice:

- `ErrNotFound` and `ErrAlreadyExists` carry no data — the caller only needs *which* failure. ⇒ **sentinels** (`var ErrNotFound = errors.New("account: not found")`), checked with `errors.Is`.
- `ValidationError{Field, Reason}` and `UnavailableError{RetryAfter}` carry *data the caller reads*. ⇒ **typed errors** (structs implementing `error`, with `Unwrap` if they wrap a cause), extracted with `errors.As`.

Write the taxonomy:

```go
package account

import (
	"errors"
	"fmt"
	"time"
)

// Sentinels: identity-only, checked with errors.Is.
var (
	ErrNotFound      = errors.New("account: not found")
	ErrAlreadyExists = errors.New("account: already exists")
)

// ValidationError: typed, carries the offending field.
type ValidationError struct {
	Field  string
	Reason string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("account: invalid %s: %s", e.Field, e.Reason)
}

// UnavailableError: typed, carries a retry hint and wraps the underlying cause.
type UnavailableError struct {
	RetryAfter time.Duration
	Err        error
}

func (e *UnavailableError) Error() string {
	return fmt.Sprintf("account: backend unavailable (retry in %s): %v", e.RetryAfter, e.Err)
}

func (e *UnavailableError) Unwrap() error { return e.Err } // good chain citizen
```

## The wrapping discipline

Each layer wraps with `%w` and adds *its own* context, never re-classifying:

```go
// repository (bottom)
func (r *Repo) Find(id string) (*Account, error) {
	a, ok := r.m[id]
	if !ok {
		return nil, fmt.Errorf("repo.Find %q: %w", id, ErrNotFound)
	}
	return a, nil
}

// service (middle) — wraps, adds context, does NOT re-wrap as a new kind
func (s *Service) Profile(id string) (*Profile, error) {
	a, err := s.repo.Find(id)
	if err != nil {
		return nil, fmt.Errorf("service.Profile %q: %w", id, err)
	}
	return toProfile(a), nil
}
```

Now the handler, three layers up, classifies *once* by inspecting the chain:

```go
func handle(id string) (status int) {
	_, err := svc.Profile(id)
	switch {
	case err == nil:
		return 200
	case errors.Is(err, account.ErrNotFound):
		return 404
	case errors.Is(err, account.ErrAlreadyExists):
		return 409
	default:
		var ve *account.ValidationError
		if errors.As(err, &ve) {
			return 400 // and the message names ve.Field
		}
		var ue *account.UnavailableError
		if errors.As(err, &ue) {
			return 503 // and you can set Retry-After: ue.RetryAfter
		}
		return 500 // unknown
	}
}
```

The handler reads the *chain*, not the *string*, even though `ErrNotFound` was wrapped twice on the way up.

## The proof — required tests

Prove the taxonomy behaves with a table-driven test. For each kind, construct the error *as the service would return it* (wrapped through both layers) and assert:

1. **`errors.Is`** finds each sentinel through the full two-layer wrap.
2. **`errors.As`** binds each typed error through the full wrap, and you **read a field** off it (`ve.Field`, `ue.RetryAfter`) and assert its value — proving extraction, not just detection.
3. **A negative case:** `errors.Is(notFoundErr, ErrAlreadyExists)` is `false` — the taxonomy distinguishes kinds.
4. **The "broke the chain" case:** build one error with `%v` instead of `%w` and show `errors.Is` *fails* to find the sentinel — proving you understand what wrapping buys.

```go
func TestTaxonomy(t *testing.T) {
	wrapped := fmt.Errorf("service: %w",
		fmt.Errorf("repo: %w", ErrNotFound)) // two layers, like production

	if !errors.Is(wrapped, ErrNotFound) {
		t.Error("errors.Is should find ErrNotFound through two wraps")
	}
	if errors.Is(wrapped, ErrAlreadyExists) {
		t.Error("must NOT match a different sentinel")
	}

	ve := fmt.Errorf("service: %w", &ValidationError{Field: "email", Reason: "missing @"})
	var got *ValidationError
	if !errors.As(ve, &got) {
		t.Fatal("errors.As should bind ValidationError")
	}
	if got.Field != "email" {
		t.Errorf("Field = %q, want email", got.Field) // proves extraction
	}

	broken := fmt.Errorf("service: %v", ErrNotFound) // %v, not %w
	if errors.Is(broken, ErrNotFound) {
		t.Error("%v breaks the chain; Is must NOT find the sentinel")
	}
}
```

## Acceptance criteria

1. **A written taxonomy** of at least two sentinels and two typed errors, each with a one-sentence justification of sentinel-vs-typed.
2. **Correct wrapping discipline:** every layer wraps with `%w` and adds context; typed errors that wrap a cause implement `Unwrap`.
3. **A handler/consumer that classifies once** by walking the chain with `errors.Is`/`errors.As` — never by string-matching.
4. **The four required tests** all green: `Is` through the wrap, `As` + field read through the wrap, the negative case, and the `%v`-breaks-the-chain case.
5. **Clean under `go vet ./...`** (it flags a non-error handed to `%w`) **and `staticcheck ./...`**.

## Stretch goals

1. **`errors.Join` for multi-field validation.** Make `Validate` return `errors.Join(perFieldErrors...)` and show `errors.As` finds *a* `*ValidationError` and you can iterate all of them. State how a caller would surface every field error at once.
2. **A `Status() int` method on each error.** Give the typed errors a `Status()` and define a sentinel-to-status map, so the handler becomes a single `statusOf(err)` helper. Decide whether `Status()` belongs on the error (couples the domain to HTTP) or in an HTTP-adapter layer (cleaner) — and defend the boundary.
3. **Custom `Is`.** Implement `Is(target error) bool` on a typed error so that, say, *any* `*ValidationError` matches a sentinel `ErrInvalid` regardless of field. Explain when a custom `Is` is justified versus when it hides intent.
4. **A doc comment contract.** Write the package doc comment that promises which errors the package returns (the public contract). A caller should be able to read the doc and know exactly which `errors.Is`/`As` checks are supported — that promise is the real API.

Cited references: <https://pkg.go.dev/errors>, <https://go.dev/blog/go1.13-errors>, <https://go.dev/blog/error-handling-and-go>, <https://pkg.go.dev/fmt#Errorf>, <https://pkg.go.dev/errors#Join>.
