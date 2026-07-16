# Challenge 1 — A Pluggable Eviction Policy Behind One Small Interface

> **Time:** 1.5 hours. **Prerequisites:** Lecture 1 (interfaces, "accept interfaces return structs"), Lecture 3 (generics). **Citations:** Effective Go's interfaces section at <https://go.dev/doc/effective_go#interfaces>, the Go Code Review Comments interface guidance at <https://go.dev/wiki/CodeReviewComments#interfaces>, the generics tutorial at <https://go.dev/doc/tutorial/generics>, and the container/list docs at <https://pkg.go.dev/container/list>.

## The premise

A cache that never forgets is a memory leak. A *bounded* cache holds at most `N` entries and, when full, *evicts* one to make room — but *which* one is a policy decision, not a cache decision. **LRU** (least-recently-used) evicts the entry untouched longest; **FIFO** (first-in-first-out) evicts the oldest *inserted* entry regardless of use. This challenge is to design the seam between the cache and the policy so that the cache does not know or care which policy it runs — exactly the "accept interfaces" discipline from Lecture 1, applied to a real structural decision you will reuse in the mini-project.

The deliverable is a *design plus a working spike*: a small `EvictionPolicy` interface, two implementations (LRU and FIFO), and a bounded cache that holds the policy behind that interface and evicts through it.

## The interface — design it small

Design `EvictionPolicy` as the *smallest* interface the cache actually needs. The cache must tell the policy three things and ask it one:

- "I **touched** key `k`" (a `Get` or `Put` hit) — the policy updates its recency/order bookkeeping.
- "I **added** key `k`" — the policy starts tracking it.
- "I **removed** key `k`" (the caller deleted it) — the policy stops tracking it.
- "I am full — **which key should I evict**?" — the policy returns the victim.

A reasonable shape (you may refine it — defend your version):

```go
// EvictionPolicy decides which key to evict when the cache is full.
// It is consumer-defined (the cache consumes it) and small.
type EvictionPolicy[K comparable] interface {
	Touch(key K)          // key was accessed (Get/Put hit)
	Add(key K)            // key was newly inserted
	Remove(key K)         // key was explicitly deleted
	Evict() (K, bool)     // choose a victim; ok=false when empty
}
```

Note the type parameter: the policy is generic over the key type `K comparable`, matching the cache. The *values* never reach the policy — a policy only orders *keys*, which is itself a design point worth stating: the policy has no business seeing the stored values.

## Two implementations

### FIFO

FIFO is the simpler one — a queue of keys in insertion order. `Add` enqueues; `Evict` dequeues the front; `Touch` is a *no-op* (FIFO does not care about access, only insertion); `Remove` deletes the key from the queue. A slice or `container/list` both work; note the cost of `Remove` from the middle.

### LRU

LRU evicts the *least recently used* key, so `Touch` must move the key to the "most recent" end in O(1). The canonical implementation is a **doubly linked list of keys plus a map from key to its list element**: `Touch` and `Add` move/insert at the front; `Evict` removes from the back; `Remove` unlinks by the mapped element. The standard library's `container/list` (<https://pkg.go.dev/container/list>) gives you the doubly linked list; the map gives you O(1) lookup of a key's node. Aim for all four operations O(1).

## The cache that holds the policy

The cache stores the data and delegates *all* eviction decisions to the interface:

```go
type Cache[K comparable, V any] struct {
	cap    int
	data   map[K]V
	policy EvictionPolicy[K] // the seam: cache never names LRU or FIFO
}

// New accepts the policy as an INTERFACE (accept interfaces!) and returns the
// concrete *Cache (return structs!).
func New[K comparable, V any](capacity int, policy EvictionPolicy[K]) *Cache[K, V] {
	return &Cache[K, V]{cap: capacity, data: make(map[K]V, capacity), policy: policy}
}

func (c *Cache[K, V]) Put(key K, val V) {
	if _, exists := c.data[key]; exists {
		c.data[key] = val
		c.policy.Touch(key)
		return
	}
	if len(c.data) >= c.cap {
		if victim, ok := c.policy.Evict(); ok {
			delete(c.data, victim)
		}
	}
	c.data[key] = val
	c.policy.Add(key)
}
```

The cache body has **zero** mention of LRU or FIFO. Swapping policies is a one-line change at construction (`New[string, int](100, NewLRU[string]())` vs `NewFIFO`). That is the win the challenge proves.

## Acceptance criteria

1. **The `EvictionPolicy` interface is small and defended.** Four methods (or your justified variant), generic over `K comparable`, consumer-defined next to the cache. You can explain in one sentence why values never reach the policy.
2. **Two working implementations**, LRU and FIFO, each behind the same interface, with a compile-time assertion (`var _ EvictionPolicy[string] = (*LRU[string])(nil)`).
3. **The cache names neither policy** in its body — swapping is a construction-site change only.
4. **A demonstration** (a `main` or a test) that fills a cap-3 cache past capacity and shows LRU and FIFO evicting *different* keys for the *same* access sequence. The classic sequence: `Put a, Put b, Put c, Get a, Put d` — FIFO evicts `a` (oldest inserted), LRU evicts `b` (oldest *used*, since `a` was just touched).
5. **Clean under `go vet ./...` and `staticcheck ./...`**, and any tests green.

## Stretch goals

1. **Table-test the divergence.** Write a table-driven test whose cases are `{name, policyFactory, ops, wantEvicted}` and run the *same* operation sequence through both policies, asserting each evicts the expected key. This is the test the mini-project will want — build it here.
2. **LRU in O(1) for every operation.** Prove it: no operation scans the list. Use `container/list` plus `map[K]*list.Element`. Write one sentence per method stating its complexity.
3. **A third policy: LFU (least-frequently-used).** Add a counter per key; `Evict` returns the lowest-count key (ties broken by recency). The point: a third policy slots in with *no cache change* — that is the abstraction paying off. Defend whether LFU still fits the same interface or needs a fifth method.
4. **`Stats()`** on the cache: hits, misses, evictions. Decide whether stats belong on the cache (yes) or the policy (no — values/hits are the cache's concern) and explain the boundary.
5. **Concurrency note (preview of Week 3/4).** State, in prose, what would break if two goroutines called `Put` concurrently and where the mutex would go. Do *not* implement it yet — just locate the hazard. The map and the policy's bookkeeping are both unsynchronised; name them.

Cited references: <https://go.dev/doc/effective_go#interfaces>, <https://go.dev/wiki/CodeReviewComments#interfaces>, <https://go.dev/doc/tutorial/generics>, <https://pkg.go.dev/container/list>, <https://pkg.go.dev/cmp>.
