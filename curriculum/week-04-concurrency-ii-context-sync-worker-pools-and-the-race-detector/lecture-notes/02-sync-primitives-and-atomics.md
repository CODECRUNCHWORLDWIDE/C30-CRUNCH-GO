# Lecture 2 — The `sync` Primitives and `sync/atomic`

> **Time:** 2 hours. Take the mutex family in one sitting and the atomics in a second sitting. **Prerequisites:** Week 3 (`WaitGroup`, the "share memory by communicating" slogan) and Lecture 1 (the worker pool). **Citations:** the `sync` package docs at <https://pkg.go.dev/sync>, the `sync/atomic` docs at <https://pkg.go.dev/sync/atomic>, and Bryan Mills' GopherCon 2018 talk "Rethinking Classical Concurrency Patterns" at <https://www.youtube.com/watch?v=5zXAHh5tJqQ>.

## 1. The honest footnote to "share memory by communicating"

Week 3 taught the channel and the slogan that goes with it: *do not communicate by sharing memory; share memory by communicating.* That slogan is good advice and it is also incomplete. The full picture, which the Go team itself states in the `sync` package and which Bryan Mills argues in his GopherCon talk, is: **channels are the right tool for transferring ownership of data and for signalling events; mutexes are the right tool for protecting shared state that you are not transferring.** A counter that ten goroutines increment, a cache that many goroutines read and a few update, a config struct loaded once and read forever — these are *shared state*, and for them a `sync.Mutex` is simpler, faster, and more obviously correct than wrapping the state in a goroutine and serialising every access through a channel.

The decision rule, which you should be able to recite in a review:

| Situation | Reach for |
|-----------|-----------|
| Hand a value from one goroutine to another (transfer ownership) | a channel |
| Signal that an event happened (done, ready, closed) | a channel (often `chan struct{}`) |
| Protect a small piece of in-memory state many goroutines touch | a `sync.Mutex` |
| Protect state with many readers and few writers | a `sync.RWMutex` |
| Run an initialiser exactly once across goroutines | a `sync.Once` |
| Wait for a known set of goroutines to finish | a `sync.WaitGroup` |
| Increment / load / store a single scalar concurrently | a `sync/atomic` typed atomic |

Picking the wrong tool is not a correctness bug by itself — a channel-guarded counter *works* — but it is a clarity and performance tax, and reviewers will (rightly) ask you to justify it. Citation: the talk above, and the `sync` package overview.

## 2. `sync.Mutex` — the workhorse

A `sync.Mutex` is a mutual-exclusion lock. At most one goroutine holds it at a time; the rest block on `Lock()` until the holder calls `Unlock()`. The idiomatic shape embeds the mutex *next to the state it protects*:

```go
package counter

import "sync"

// SafeCounter is safe for concurrent use by multiple goroutines.
type SafeCounter struct {
	mu     sync.Mutex
	counts map[string]int
}

func NewSafeCounter() *SafeCounter {
	return &SafeCounter{counts: make(map[string]int)}
}

func (c *SafeCounter) Inc(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.counts[key]++
}

func (c *SafeCounter) Get(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.counts[key]
}
```

Five rules:

1. **The mutex protects a *named* piece of state, and lives next to it.** The convention is to put `mu sync.Mutex` immediately above the fields it guards, and a comment like `// guards counts` when it is not obvious. A mutex with no clearly-defined invariant is a mutex you will eventually misuse.
2. **`Lock()` / `defer Unlock()` is the default shape.** `defer` guarantees the unlock runs even if the body panics. Drop the `defer` and lock manually only when you have measured the deferred-unlock overhead in a genuinely hot path *and* the critical section has a single exit — which is rare.
3. **A `Mutex`'s zero value is a ready-to-use unlocked mutex.** You do not initialise it. `var mu sync.Mutex` is immediately usable. This is why embedding it in a struct works with no constructor ceremony.
4. **Never copy a value that contains a `Mutex`.** Copying a locked mutex copies the locked state, producing two mutexes that disagree about reality. This is why methods on a mutex-bearing struct take a *pointer* receiver, and why `go vet`'s `copylocks` analyzer flags passing such a value by value. The map above is held behind a `*SafeCounter` for exactly this reason.
5. **Hold the lock for the shortest correct interval.** Do not do I/O, call out to a slow dependency, or block on a channel while holding a mutex — you serialise every other goroutine behind that slow operation. Compute under the lock, release, then do the slow thing.

A plain Go `map` is **not** safe for concurrent access — concurrent read+write of a map is a data race that the runtime will sometimes detect and `panic` with "concurrent map writes." The `SafeCounter` above is the standard fix. (`sync.Map` exists for a narrow set of access patterns — append-mostly, disjoint key sets per goroutine — and is usually *slower* than a mutex-guarded map for the common case; reach for it only when its docs describe your exact pattern.) Citation: <https://pkg.go.dev/sync#Mutex> and the map-concurrency note at <https://go.dev/doc/faq#atomic_maps>.

## 3. `sync.RWMutex` — many readers, few writers

When reads vastly outnumber writes — a config loaded once and read on every request, a routing table updated rarely — a `sync.RWMutex` lets *any number* of readers hold the read lock simultaneously, while a writer gets exclusive access:

```go
type Config struct {
	mu   sync.RWMutex
	data map[string]string // guarded by mu
}

func (c *Config) Get(key string) (string, bool) {
	c.mu.RLock()         // shared read lock: many readers concurrently
	defer c.mu.RUnlock()
	v, ok := c.data[key]
	return v, ok
}

func (c *Config) Set(key, value string) {
	c.mu.Lock()          // exclusive write lock: blocks all readers and writers
	defer c.mu.Unlock()
	c.data[key] = value
}
```

Three points:

1. **`RLock` / `RUnlock` for reads, `Lock` / `Unlock` for writes.** Multiple `RLock` holders coexist; a `Lock` waits for all readers to release and then excludes everyone.
2. **`RWMutex` is only a win when reads genuinely dominate and the critical section is non-trivial.** For a tiny critical section (a single map lookup), the bookkeeping overhead of the reader/writer accounting can make `RWMutex` *slower* than a plain `Mutex`. Measure before assuming it helps. The Go team's own guidance is "default to `Mutex`; reach for `RWMutex` only when profiling shows read contention."
3. **Do not upgrade a read lock to a write lock.** There is no atomic "promote `RLock` to `Lock`" — releasing the read lock and acquiring the write lock has a gap where another writer can sneak in. Design so you know up front whether you need read or write access.

Citation: <https://pkg.go.dev/sync#RWMutex>.

## 4. `sync.Once` — run exactly once

`sync.Once` runs a function exactly once, no matter how many goroutines call it concurrently. The canonical use is lazy, thread-safe initialisation:

```go
type Service struct {
	once   sync.Once
	client *http.Client
}

func (s *Service) Client() *http.Client {
	s.once.Do(func() {
		s.client = &http.Client{Timeout: 5 * time.Second}
	})
	return s.client
}
```

Three properties:

1. **`Do(f)` runs `f` on the first call and blocks all other callers until `f` returns.** Every caller after the first returns immediately without running `f`. After `Do` returns once, `s.client` is fully initialised and visible to every goroutine — `Once` establishes the happens-before edge.
2. **It runs even if `f` panics** in the sense that the `Once` is marked "done" and will never run `f` again — so a panicking initialiser leaves you with a half-built value and no retry. If initialisation can fail, prefer an explicit constructor with error return over lazy `Once`.
3. **The Go 1.21+ helpers `sync.OnceFunc`, `sync.OnceValue`, and `sync.OnceValues`** wrap the common shapes: `OnceValue(func() T) func() T` gives you a function that computes a value once and returns the cached result thereafter. Reach for them when you would otherwise write the `Once`-plus-field boilerplate by hand. Citation: <https://pkg.go.dev/sync#OnceValue>.

## 5. `sync.WaitGroup` — recap and the one rule that bites

You met `WaitGroup` in Week 3. The recap, plus the rule people get wrong:

```go
var wg sync.WaitGroup
for _, item := range items {
	wg.Add(1)              // increment BEFORE starting the goroutine
	go func() {
		defer wg.Done()    // decrement when the goroutine finishes
		work(item)         // (Go 1.22+: `item` is per-iteration; pre-1.22 capture it)
	}()
}
wg.Wait()                  // block until the counter hits zero
```

The rule that bites: **call `wg.Add(n)` before you start the goroutine, never inside it.** If you write `go func() { wg.Add(1); ... }()`, there is a race between the goroutine getting scheduled (and calling `Add`) and the parent reaching `Wait()` — `Wait` can see a zero counter and return before the goroutine has even incremented it. `Add` happens in the parent, synchronously, before the `go` statement. Citation: <https://pkg.go.dev/sync#WaitGroup> — the docs state this explicitly. (In Go 1.25+, `WaitGroup.Go` packages the `Add`/`Done` pair safely; where available, prefer it.)

## 6. `sync/atomic` — lock-free single-scalar operations

For a *single* scalar — an `int64` counter, a `bool` flag, a pointer swapped wholesale — a `sync/atomic` operation is both simpler and faster than a mutex, because the operation is a single hardware instruction with no lock to contend. Go 1.19 introduced *typed* atomics that make misuse impossible:

```go
package metrics

import "sync/atomic"

type Stats struct {
	requests atomic.Int64 // total requests; incremented from many goroutines
	inFlight atomic.Int64 // currently in flight; up on enter, down on exit
	degraded atomic.Bool  // a flag flipped by a health check
}

func (s *Stats) Begin() {
	s.requests.Add(1)
	s.inFlight.Add(1)
}

func (s *Stats) End() {
	s.inFlight.Add(-1)
}

func (s *Stats) Snapshot() (total, current int64) {
	return s.requests.Load(), s.inFlight.Load()
}

func (s *Stats) SetDegraded(v bool) { s.degraded.Store(v) }
func (s *Stats) Degraded() bool     { return s.degraded.Load() }
```

Five things to know:

1. **The typed atomics — `atomic.Int64`, `atomic.Uint64`, `atomic.Int32`, `atomic.Bool`, `atomic.Pointer[T]`, `atomic.Value`** — are structs whose only valid access is through their methods (`Load`, `Store`, `Add`, `Swap`, `CompareAndSwap`). You *cannot* accidentally read the underlying field non-atomically, which was the foot-gun of the old free-function API (`atomic.AddInt64(&x, 1)`). Prefer the typed forms in all new code.
2. **`Add` returns the new value; `Load` reads; `Store` writes; `Swap` writes and returns the old value; `CompareAndSwap` (CAS) writes only if the current value matches an expected one.** CAS is the building block of lock-free algorithms: "set it to B, but only if it is still A" lets you detect and retry on contention.
3. **An atomic protects exactly one value.** If two fields must change *together* — "decrement balance and append to the ledger" — atomics cannot express the invariant; you need a mutex around both. The rule: **one scalar → atomic; a group that changes together → mutex.**
4. **Do not copy a value containing an atomic** (same `copylocks` reason as a mutex). Hold it behind a pointer.
5. **`atomic.Pointer[T]` is how you swap an immutable snapshot atomically** — a common pattern for hot-reloadable config: build a new `*Config`, then `cfg.Store(newPtr)`, and every reader does `cfg.Load()` to get a consistent snapshot with no lock. This is faster than `RWMutex` when the value is read enormously more often than it is written. Citation: <https://pkg.go.dev/sync/atomic#Pointer>.

## 7. Atomic vs mutex — the measured difference

Why prefer an atomic for a counter? Because under contention, a mutex serialises every increment through a lock acquire/release, while an atomic increment is a single CPU instruction (`LOCK XADD` on x86) with no parking of goroutines. The difference is small per-operation but compounds under high contention. A representative benchmark, which you reproduce in Exercise 3:

```
BenchmarkCounter/mutex-8     	48000000	     24.3 ns/op	     0 B/op	   0 allocs/op
BenchmarkCounter/atomic-8    	200000000	      6.1 ns/op	     0 B/op	   0 allocs/op
```

The atomic is roughly 4x faster for a bare counter under contention on an 8-core machine. But the headline number is not the point; the point is the *decision rule*: an atomic is the right tool **only when the shared state is a single scalar.** The moment you need to update two things consistently, the atomic is wrong regardless of speed, and a mutex is correct. Premature atomics, like premature anything, cost you correctness for a speedup you did not need. Citation: the `sync/atomic` overview and the benchmark methodology from Lecture 3.

## 8. A worked example: a bounded, instrumented worker pool

Putting Lecture 1 and this lecture together — a pool that bounds concurrency with `errgroup`, counts throughput with an atomic, and tracks a concurrency high-water mark to *prove* the bound:

```go
package pool

import (
	"context"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

type Metrics struct {
	processed atomic.Int64
	current   atomic.Int64 // in-flight right now
	peak      atomic.Int64 // high-water mark of `current`
}

func (m *Metrics) enter() {
	cur := m.current.Add(1)
	// Bump the peak with a compare-and-swap retry loop: set peak to cur if
	// cur is larger than the current peak, retrying on a concurrent update.
	for {
		old := m.peak.Load()
		if cur <= old || m.peak.CompareAndSwap(old, cur) {
			break
		}
	}
}

func (m *Metrics) exit() {
	m.processed.Add(1)
	m.current.Add(-1)
}

func Run(ctx context.Context, items []int, limit int, do func(context.Context, int) error) (*Metrics, error) {
	m := &Metrics{}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	for _, item := range items {
		item := item
		g.Go(func() error {
			m.enter()
			defer m.exit()
			return do(ctx, item)
		})
	}
	err := g.Wait()
	return m, err
}
```

Two teaching points: **(a)** `m.peak` uses a CAS retry loop — the canonical lock-free "update if larger" — because `current.Add(1)` returning a value does not, by itself, let two goroutines agree on the maximum. **(b)** After `Run`, `m.peak.Load()` must be `<= limit`. That is your *proof* the bound held; Exercise 2 asserts exactly this. If `peak > limit`, your bound is broken. Citation: <https://pkg.go.dev/sync/atomic#Int64.CompareAndSwap>.

## 9. Exercise pointer

Now do **Exercise 2 — Bounded Worker Pool**. Build the pool two ways (semaphore channel and `errgroup.SetLimit`), instrument it with an atomic high-water mark, and assert in a test that the peak never exceeds the limit. Then do **Exercise 3 — Find and Fix the Race**, which uses the mutex-vs-atomic material here to repair a racy counter two ways and benchmark both.

## 10. Summary

- Channels transfer ownership and signal events; mutexes protect shared state you are not transferring. Pick deliberately.
- `sync.Mutex`: one holder at a time; embed it next to the state it guards; `Lock`/`defer Unlock`; never copy it; never do I/O under it.
- `sync.RWMutex`: many readers or one writer; only a win when reads dominate and the section is non-trivial; never upgrade a read lock.
- `sync.Once` / `OnceValue`: run an initialiser exactly once; not a retry mechanism for fallible init.
- `sync.WaitGroup`: `Add` in the parent before `go`; never inside the goroutine.
- `sync/atomic` typed atomics (`Int64`, `Bool`, `Pointer[T]`): one scalar, lock-free, faster than a mutex under contention.
- The rule: one scalar → atomic; a group of fields that change together → mutex.
- `CompareAndSwap` is the lock-free building block; the "update if larger" peak tracker is the canonical CAS loop.

Cited references this lecture pulled from: <https://pkg.go.dev/sync>, <https://pkg.go.dev/sync/atomic>, <https://go.dev/doc/faq#atomic_maps>, and Bryan Mills' "Rethinking Classical Concurrency Patterns" at <https://www.youtube.com/watch?v=5zXAHh5tJqQ>.
