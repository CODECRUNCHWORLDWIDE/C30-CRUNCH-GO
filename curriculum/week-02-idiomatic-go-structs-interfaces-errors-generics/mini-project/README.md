# Mini-Project — Lab 02: A Generic `Cache[K comparable, V any]` with a Pluggable Policy, Typed Errors, and a Two-Backend Store

> **Time:** ~7.5 hours across Wednesday-Friday-Saturday. **Prerequisites:** all three lectures, all three exercises, ideally both challenges. **Citations:** the generics tutorial at <https://go.dev/doc/tutorial/generics>, Effective Go's interfaces at <https://go.dev/doc/effective_go#interfaces>, the `errors` package at <https://pkg.go.dev/errors>, the Go 1.13 errors post at <https://go.dev/blog/go1.13-errors>, and the testing docs at <https://pkg.go.dev/testing>.

## The spec

You are building **`cache`**, a generic, in-process, TTL (time-to-live) cache. It is the week distilled into one artifact: a *generic container* (`Cache[K comparable, V any]`), a *pluggable eviction policy* behind a *small interface*, *typed and sentinel errors* checked with `errors.Is`/`errors.As`, and a `Store[K, V]` *interface* with **two** implementations — in-memory and file-backed — proving the "accept interfaces, return structs" discipline at the seam where it matters most. Every concept from Lectures 1–3 appears, and the design choices (container ⇒ generic, behaviour ⇒ interface) are exactly the ones the syllabus says you must defend in review.

```go
c := cache.New[string, int](
	cache.WithCapacity(3),
	cache.WithTTL(5*time.Minute),
	cache.WithPolicy(cache.NewLRU[string]()),
	cache.WithStore(cache.NewMemStore[string, int]()),
)

c.Put("a", 1)
v, err := c.Get("a")          // v == 1, err == nil

_, err = c.Get("missing")
errors.Is(err, cache.ErrMiss) // true — sentinel

time.Sleep(6 * time.Minute)
_, err = c.Get("a")
var ee *cache.ExpiredError
errors.As(err, &ee)           // true — typed; ee.Key == "a", ee.ExpiredAt set
```

## Functional requirements

### F1 — The generic cache type

- `Cache[K comparable, V any]` — `K` is `comparable` (it is a map key); `V` is `any`.
- A constructor `New[K comparable, V any](opts ...Option[K, V]) *Cache[K, V]` returns the **concrete** `*Cache` (return structs), configured by functional options (`WithCapacity`, `WithTTL`, `WithPolicy`, `WithStore`). Sensible zero-config defaults: unbounded-ish capacity, no TTL, an LRU policy, a `MemStore`.
- Core methods: `Put(key K, val V)`, `Get(key K) (V, error)`, `Delete(key K)`, `Len() int`.

### F2 — TTL and expiry (typed error)

- Each entry records an expiry time (`now + TTL`); a TTL of zero means "never expires."
- `Get` on an *expired* entry returns the zero `V` and a **typed** `*ExpiredError{Key K, ExpiredAt time.Time}` (you will need `Key any` or a generic error type — decide and defend; a non-generic `ExpiredError` holding `fmt.Sprintf`'d key info is acceptable and simpler). The caller extracts it with `errors.As` and can read `ExpiredAt`.
- An expired entry is *removed* on access (lazy expiry); document this choice versus active sweeping.

### F3 — Cache miss (sentinel error)

- `Get` on a *never-stored* (or evicted, or deleted) key returns the zero `V` and the **sentinel** `var ErrMiss = errors.New("cache: key not present")`, wrapped with context via `%w` (`fmt.Errorf("get %v: %w", key, ErrMiss)`).
- The caller checks with `errors.Is(err, ErrMiss)`. A miss and an expiry are **distinct** — `errors.Is(expiredErr, ErrMiss)` must be `false`.

### F4 — Pluggable eviction policy (interface)

- A small, consumer-defined `EvictionPolicy[K comparable]` interface (see Challenge 1): `Touch(K)`, `Add(K)`, `Remove(K)`, `Evict() (K, bool)`.
- Ship **two** implementations: `NewLRU[K]()` and `NewFIFO[K]()`, each with a compile-time assertion `var _ EvictionPolicy[string] = (*LRU[string])(nil)`.
- When `Put` would exceed capacity, the cache asks the policy for a victim and evicts it from the store. The cache body must **not** name LRU or FIFO.

### F5 — Pluggable store (interface, two backends)

- A `Store[K comparable, V any]` interface — the persistence seam — with the minimal methods the cache needs: `Load(key K) (entry[V], bool, error)`, `Save(key K, e entry[V]) error`, `Remove(key K) error`, `Keys() ([]K, error)`. (Refine the method set; defend it.)
- **`MemStore[K, V]`** — an in-memory `map[K]entry[V]`. Fast path; never errors in practice.
- **`FileStore[K, V]`** — persists entries to a file (one JSON object, or one line per entry) so the cache survives a process restart. Requires `K` and `V` to be JSON-serialisable; state that constraint in the doc. Loading a corrupt file returns a **wrapped** error, not a panic.
- The cache holds `Store[K, V]` as an interface (accept interfaces) and never names which backend it has.

### F6 — Tests (the heart of the grade)

- A **table-driven** suite over the cache covering: hit, miss (assert `errors.Is(err, ErrMiss)`), expiry (assert `errors.As(err, &ExpiredError)` and read the field), capacity eviction (assert the *right* key was evicted under LRU vs FIFO — run the same sequence through both), overwrite-resets-TTL, and `Delete`.
- A suite over **both** `Store` implementations using the *same* table (write the test against the `Store` interface, run it once per backend) — this is the "accept interfaces makes testing trivial" payoff made concrete.
- Every error checked with `errors.Is`/`errors.As`, **never** by string.
- Clean under `go vet ./...` and `staticcheck ./...`; `go test ./...` green; report coverage.

### F7 — A tiny demo `main`

- A `cmd/cachedemo/main.go` that constructs a cap-3 LRU cache, puts four keys, and prints which key was evicted; then constructs a `FileStore`-backed cache, writes, "restarts" (constructs a fresh cache over the same file), and reads a surviving value. Proves F4 and F5 end-to-end.

## Non-functional requirements

### NF1 — Idiomatic Go

- **Container is generic; behaviour is an interface.** The cache is `Cache[K, V]`; the policy and the store are interfaces. Be ready to defend each in review (Lecture 3 §6).
- **Accept interfaces, return structs.** `New` returns `*Cache`; it *accepts* `EvictionPolicy` and `Store` as interfaces. Constructors for the implementations return their concrete types (`*LRU`, `*MemStore`).
- **Errors:** a sentinel (`ErrMiss`) for miss, a typed (`*ExpiredError`) for expiry; wrap with `%w`; never branch on `err.Error()`.
- Every error checked; no panic on bad input (a corrupt `FileStore` file is a returned error). Receivers consistent across each type's method set.

### NF2 — Small interfaces

- `EvictionPolicy` and `Store` each list only the methods the cache calls. If the cache never calls it, it is not on the interface. A reviewer will ask "who consumes this and what is the smallest method set?" for both.

### NF3 — Citations

- Every non-obvious standard-library choice carries a one-line comment pointing at its `pkg.go.dev` page (e.g. `// encoding/json: https://pkg.go.dev/encoding/json`).

## Suggested project layout

```
cache/
├── go.mod                       (go mod init github.com/you/cache)
├── README.md                    <-- design notes (see W1–W4 below)
├── cache.go                     <-- Cache[K,V], New, Put/Get/Delete/Len, Options
├── errors.go                    <-- ErrMiss (sentinel), ExpiredError (typed)
├── policy.go                    <-- EvictionPolicy interface, LRU, FIFO
├── store.go                     <-- Store interface, MemStore, FileStore
├── cache_test.go                <-- table tests for the cache
├── store_test.go                <-- one table run against BOTH Store impls
└── cmd/
    └── cachedemo/
        └── main.go              <-- the F7 demo
```

A starting point for the core type and the seams (complete `Get`/`Put` and the stores):

```go
// cache.go
package cache

import (
	"fmt"
	"time"
)

type entry[V any] struct {
	val      V
	expireAt time.Time // zero == never expires
}

type Cache[K comparable, V any] struct {
	cap    int
	ttl    time.Duration
	policy EvictionPolicy[K]
	store  Store[K, V]
	now    func() time.Time // injected for testable expiry (defaults to time.Now)
}

func (c *Cache[K, V]) Get(key K) (V, error) {
	var zero V
	e, ok, err := c.store.Load(key)
	if err != nil {
		return zero, fmt.Errorf("get %v: %w", key, err)
	}
	if !ok {
		return zero, fmt.Errorf("get %v: %w", key, ErrMiss) // sentinel
	}
	if !e.expireAt.IsZero() && c.now().After(e.expireAt) {
		_ = c.store.Remove(key) // lazy expiry
		c.policy.Remove(key)
		return zero, fmt.Errorf("get %v: %w", key,
			&ExpiredError{Key: fmt.Sprintf("%v", key), ExpiredAt: e.expireAt}) // typed
	}
	c.policy.Touch(key)
	return e.val, nil
}
```

```go
// errors.go
package cache

import (
	"errors"
	"fmt"
	"time"
)

var ErrMiss = errors.New("cache: key not present") // sentinel

type ExpiredError struct {
	Key       string
	ExpiredAt time.Time
}

func (e *ExpiredError) Error() string {
	return fmt.Sprintf("cache: key %q expired at %s", e.Key, e.ExpiredAt.Format(time.RFC3339))
}
```

The `now func() time.Time` field is the key testability trick: inject a fake clock so the expiry tests are instant and deterministic instead of `time.Sleep`-ing.

## The README write-up (`README.md`)

Treat the README as part of the deliverable. It must contain:

### W1 — Build and run

Every command, copy-pasteable: `go test ./...`, `go run ./cmd/cachedemo`, and the demo's output showing LRU eviction and file-backed survival.

### W2 — The three design defenses (the graded heart of the write-up)

200–400 words total answering, with reasons:
- **Why is the cache generic but the policy and store interfaces?** (Container vs behaviour — Lecture 3 §6.)
- **Why is miss a sentinel and expiry a typed error?** (Identity vs data — Lecture 2 §7.)
- **What does each interface's method set deliberately *exclude*, and why?** (Smallest interface — Lecture 1 §5.)

### W3 — The two-backend test story

One paragraph: how you wrote the store test *once against the `Store` interface* and ran it against both `MemStore` and `FileStore`, and why that is the payoff of accepting an interface.

### W4 — One trade-off note

200 words on a real decision: lazy vs active expiry, or O(1) LRU via `container/list` vs a simpler O(n) scan, or whether `ExpiredError.Key` should be generic `K` (and why you chose `string`).

## Grading rubric

- **35 points: functional correctness.** F1–F7: the generic cache, TTL expiry with a typed error, miss with a sentinel, the pluggable LRU/FIFO policy evicting the right key, the two `Store` backends, and the demo.
- **25 points: idiomatic Go.** Container-generic / behaviour-interface split defended; accept-interfaces-return-structs honoured; compile-time satisfaction assertions; receivers consistent; small interfaces; no panic on bad input.
- **20 points: tests.** Table-driven; miss checked with `errors.Is`, expiry with `errors.As` (and a field read); LRU-vs-FIFO eviction divergence proven; the store suite run against *both* backends; green and clean under `go vet`/`staticcheck`; coverage reported.
- **10 points: errors.** Correct sentinel vs typed choice, `%w` wrapping, `errors.Is`/`As` (never string-matching), a typed error with `Unwrap` where it wraps a cause.
- **5 points: the README write-up.** W1–W4 present, with the three design defenses actually argued.
- **5 points: citations.** Standard-library choices carry one-line `pkg.go.dev` citations in the source.

## Stretch goals

1. **Generic typed error.** Make `ExpiredError[K]` carry the real key type `K` instead of a stringified key. Decide whether the added type parameter is worth the API friction, and write up the call. (Hint: `errors.As` with a generic target type is fiddly — that friction is itself the lesson.)
2. **Active expiry sweeper.** Add a background sweep that evicts expired entries on a ticker. Note in prose where a `sync.Mutex` must guard the store and policy (you implement the goroutine in Week 3/4; here just locate the hazard).
3. **A third policy (LFU).** Slot a least-frequently-used policy behind the *unchanged* `EvictionPolicy` interface — proving the abstraction. Add a table case.
4. **`GetOrLoad`.** Add `GetOrLoad(key K, load func() (V, error)) (V, error)`: on a miss, call `load`, store the result, and return it (the cache-aside pattern). Decide how a `load` error propagates and whether it is wrapped.
5. **Benchmark LRU eviction** (`go test -bench`). Compare O(1) `container/list` LRU against an O(n)-scan LRU at capacities 100 and 10,000. Report ns/op and allocs/op. (Previews Week 8.)

## Submission

Push the project on a branch named `c30-week02-cache/<your-handle>` and open a PR against the C30 curriculum repository. The PR description must link to the README and paste (a) the `cachedemo` output showing an LRU eviction and a file-backed survival, and (b) the `go test ./... -cover` line.

The teaching staff reviews mini-project PRs within 7 business days. Reviews focus on (a) whether F1–F7 are met, (b) whether the *generic-container / interface-behaviour* split and the *sentinel / typed* error split are correct and *defended* in W2, (c) whether the store test was written once against the interface and run against both backends, and (d) whether every error is checked with `errors.Is`/`errors.As` rather than by string. A PR that string-matches an error message, or returns an interface from a constructor, or puts a method on an interface the cache never calls, will be sent back with a pointer to the relevant lecture section.

Cited references: <https://go.dev/doc/tutorial/generics>, <https://go.dev/doc/effective_go#interfaces>, <https://go.dev/wiki/CodeReviewComments#interfaces>, <https://pkg.go.dev/errors>, <https://go.dev/blog/go1.13-errors>, <https://pkg.go.dev/encoding/json>, <https://pkg.go.dev/container/list>, <https://pkg.go.dev/testing>, <https://pkg.go.dev/cmp>.
