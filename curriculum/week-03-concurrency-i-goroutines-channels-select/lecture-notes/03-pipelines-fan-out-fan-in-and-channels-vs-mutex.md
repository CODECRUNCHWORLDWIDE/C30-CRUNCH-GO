# Lecture 3 — Pipelines, Fan-Out/Fan-In, and Channels vs. Mutex

> **Time:** 2 hours. Take the pipeline and fan-out/fan-in material first, then the channel-vs-mutex decision matrix. **Prerequisites:** Lectures 1 and 2. **Citations:** the canonical pipelines blog at <https://go.dev/blog/pipelines>, the "Share Memory By Communicating" codelab at <https://go.dev/blog/codelab-share>, Effective Go's concurrency section at <https://go.dev/doc/effective_go#concurrency>, and the `sync.Mutex` docs at <https://pkg.go.dev/sync#Mutex>.

## 1. The pipeline as the unit of concurrent design

The first two lectures gave you goroutines, channels, `select`, and `WaitGroup` as parts. This lecture assembles them into the shape that organises most real Go concurrency: the **pipeline**. A pipeline is a series of stages connected by channels, where each stage is a group of goroutines running the same function. Each stage:

- receives values from an inbound channel,
- does something with each value,
- sends values on an outbound channel.

The first stage (no inbound channel) is a **generator** or **source**; the last (no outbound channel) is a **sink**. The rule for every stage from Lecture 1 holds throughout: a stage **closes its outbound channel when it is done sending** (so the next stage's `range` can end), and it **keeps receiving until its inbound channel is closed**. Get those two right at every stage and the whole pipeline shuts down cleanly from the front to the back. Citation: <https://go.dev/blog/pipelines>.

### 1.1 A three-stage pipeline

A complete, runnable pipeline: generate integers, square them, then sum them. Each stage is a function that takes a receive-only input and returns a receive-only output:

```go
package main

import "fmt"

// gen is the SOURCE: it emits nums onto a channel and closes it.
func gen(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // source owns out, source closes it
		for _, n := range nums {
			out <- n
		}
	}()
	return out
}

// sq is a STAGE: receive ints, send their squares, close on the way out.
func sq(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in { // ends when in is closed and drained
			out <- n * n
		}
	}()
	return out
}

func main() {
	// Compose the pipeline: gen -> sq -> sink.
	for v := range sq(gen(2, 3, 4)) { // the sink: range the final stage
		fmt.Println(v)
	}
}
```

```
4
9
16
```

Notice the shape that repeats: each stage **makes its own output channel, launches a goroutine that ranges its input and writes its output, defers the close of its output, and returns the channel immediately.** The caller composes stages by nesting — `sq(gen(...))` — and the data flows through. Shutdown propagates automatically: `gen` closes its output when its loop ends, which lets `sq`'s `range` end, which fires `sq`'s deferred close, which lets `main`'s `range` end. Close the front and the whole pipe drains and closes behind it. Citation: <https://go.dev/blog/pipelines>.

## 2. Fan-out: parallelising a stage

When a stage's work is expensive (an HTTP request, a hash, a parse) and the values are independent, you parallelise that stage by running **multiple goroutines that all read from the same inbound channel**. That is **fan-out**: N copies of the stage competing to pull the next value off one channel. Because each value is delivered to exactly one receiver, the N goroutines naturally share the work with no extra coordination — the channel *is* the work queue.

```go
// slowSquare is the expensive stage we want to parallelise.
func slowSquare(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in {
			time.Sleep(100 * time.Millisecond) // pretend it's expensive
			out <- n * n
		}
	}()
	return out
}
```

To fan out across `workers` goroutines, start `workers` of these reading the *same* `in`. Each returns its own `out` channel — which gives us N output channels to recombine. That recombination is fan-in.

## 3. Fan-in: merging N channels into one

**Fan-in** merges multiple channels into a single channel. It is the inverse of fan-out: N stage goroutines produce N output channels, and `merge` folds them back into one so the sink can `range` a single channel. The canonical implementation uses the Lecture 1 "WaitGroup + closer goroutine" idiom — one forwarding goroutine per input channel, a `WaitGroup` that counts them, and a single closer that closes the merged output once every forwarder is done:

```go
// merge fans in: forward every input channel onto one out channel.
func merge(inputs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup

	// One forwarder goroutine per input channel.
	forward := func(c <-chan int) {
		defer wg.Done()
		for v := range c { // drain c
			out <- v
		}
	}

	wg.Add(len(inputs)) // Add before the go statements
	for _, c := range inputs {
		go forward(c)
	}

	// The ONLY safe closer: close out after every forwarder has returned.
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}
```

This is the rule from Lecture 1 §6.1 made concrete: there are `len(inputs)` senders on `out`, so none of them may close it; instead, a separate goroutine waits for all of them (`wg.Wait()`) and closes `out` exactly once. Citation: the pipelines blog's fan-in section at <https://go.dev/blog/pipelines>.

## 4. The full fan-out/fan-in pipeline

Now assemble it: a generator, a fan-out of `workers` slow-square stages, a fan-in merge, and a sink. This is the exact architecture of the mini-project's link-checker (substitute "HTTP HEAD a URL" for "square a number"):

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func gen(nums ...int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for _, n := range nums {
			out <- n
		}
	}()
	return out
}

func slowSquare(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in {
			time.Sleep(100 * time.Millisecond)
			out <- n * n
		}
	}()
	return out
}

func merge(inputs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup
	forward := func(c <-chan int) {
		defer wg.Done()
		for v := range c {
			out <- v
		}
	}
	wg.Add(len(inputs))
	for _, c := range inputs {
		go forward(c)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}

func main() {
	const workers = 4
	source := gen(1, 2, 3, 4, 5, 6, 7, 8)

	// FAN OUT: start `workers` slowSquare stages, all reading `source`.
	stages := make([]<-chan int, workers)
	for i := 0; i < workers; i++ {
		stages[i] = slowSquare(source)
	}

	// FAN IN: merge the worker outputs into one channel; range it (the sink).
	start := time.Now()
	var got []int
	for v := range merge(stages...) {
		got = append(got, v)
	}
	fmt.Printf("results=%v in %v\n", got, time.Since(start).Round(10*time.Millisecond))
}
```

Eight items, each 100 ms of "work," across four workers: roughly 200 ms wall-clock instead of the 800 ms a single stage would take. The results arrive in nondeterministic order (which worker finished first?), which is the price of parallelism — if you need ordered output, you carry an index with each value and re-sort at the sink. Every goroutine has a guaranteed exit: `gen` closes `source`, so each worker's `range` ends and fires its deferred close; each forwarder's `range` ends when its worker's channel closes; the merge closer closes `out` after `wg.Wait()`; `main` ends its `range`. No leaks, no `time.Sleep` for coordination. Run it under `go test -race` and `goleak` and it is clean. Citation: <https://go.dev/blog/pipelines>, <https://go.dev/doc/effective_go#concurrency>.

## 5. "Share memory by communicating" — and its limits

The Go proverb, from the codelab of the same name, is:

> Do not communicate by sharing memory; instead, share memory by communicating.

The idea: rather than have many goroutines touch one shared variable behind a lock, give the variable's *ownership* to one goroutine and have everyone else send it messages over a channel. At any instant exactly one goroutine owns the data, so there is no data race to guard against — the channel hand-off *transfers ownership*. The fan-out/fan-in pipeline above is this principle in its natural habitat: each integer is owned by exactly one worker at a time, passed along by value over channels, never shared. Citation: <https://go.dev/blog/codelab-share>.

But the proverb is a *default*, not a law — and reading it as a law produces some of the worst Go you will ever review. The codelab itself says: "Don't overdo it... a `sync.Mutex` ... may be the best fit." The failure mode is real: a junior engineer reads the proverb, decides locks are forbidden, and "protects" a single integer counter by spawning a goroutine that owns it and a channel that every other goroutine sends increment-messages on. That is dozens of lines, a goroutine, and a channel to replace `mu.Lock(); n++; mu.Unlock()` — slower, harder to read, and easier to leak. The senior instinct is to know *which tool fits the shape of the problem*, and that is the decision matrix.

## 6. The channel-vs-mutex decision matrix

| Use a **channel** when… | Use a **`sync.Mutex`** when… | Use **neither (a plain call)** when… |
|---|---|---|
| You are **transferring ownership** of data between goroutines | You are **guarding a small piece of shared state** read/written by several goroutines | There is **no concurrency** — one goroutine does the work |
| The structure is a **pipeline / fan-out / fan-in** | The state is a **counter, map, cache, or struct field** touched briefly | The "parallel" work is so cheap that a goroutine costs more than it saves |
| You need to **signal an event** ("done", "stop", "a result is ready") | The critical section is **short** and does not block | You only *thought* you needed concurrency |
| You need to **coordinate timing** (a rendezvous, a deadline via `select`) | You want the **simplest, fastest** mutual exclusion | |
| Multiple goroutines should **wait on multiple sources** at once | Performance matters and a lock is cheaper than channel ops | |

The two-sentence version to carry into review: **a channel is for moving a value from one goroutine to another and coordinating in time; a mutex is for letting several goroutines safely touch the same small thing.** When the answer is "I have one shared map that a few goroutines update," the answer is a mutex, and reaching for a channel is the over-engineering the codelab warns against.

### 6.1 The mutex, concretely

A `sync.Mutex` is the simplest answer for a shared counter or a small in-memory cache. It is useful at its zero value (no constructor — the Week 1 "make the zero value useful" lesson), and the idiom is `Lock` then `defer Unlock`:

```go
// SafeCounter is a counter several goroutines can increment.
type SafeCounter struct {
	mu sync.Mutex
	n  map[string]int
}

func NewSafeCounter() *SafeCounter {
	return &SafeCounter{n: make(map[string]int)}
}

func (c *SafeCounter) Inc(key string) {
	c.mu.Lock()
	defer c.mu.Unlock() // unlocks however Inc returns
	c.n[key]++          // the critical section: short, does not block
}

func (c *SafeCounter) Value(key string) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n[key]
}
```

Compare the channel version of the same job — a goroutine owning the map and a channel of commands — and you will write three times the code for no benefit. The map here is shared, the critical section is two lines that never block, and several goroutines update it: that is the mutex's home turf. (Recall from Week 1 that a plain `map` is *not* safe for concurrent writes — concurrent map access without a lock is a data race the runtime may even detect and crash on with "concurrent map writes." The mutex is what makes it safe.) A `sync.Mutex` must not be copied after first use — embed it in a struct and pass the struct by pointer, and `go vet` will catch a copy. There is also `sync.RWMutex` for the read-heavy case (many readers, rare writers), which Week 4 covers. Citation: <https://pkg.go.dev/sync#Mutex>, Effective Go's concurrency section at <https://go.dev/doc/effective_go#concurrency>.

### 6.2 A worked judgement call

Three problems, the right tool for each:

1. **"Sum the results of 1,000 HTTP requests."** Channel (fan-out/fan-in). You are transferring each result from a worker to a collector, and the structure is a pipeline. This is the mini-project.
2. **"Count how many requests each status code produced, across the workers."** Mutex (or a per-worker local count merged at the end). The shared state is a small `map[int]int`; guarding it with a mutex is simpler than routing every count through a channel. Even better: each worker keeps a *local* map and merges at the end — no sharing at all, which beats both.
3. **"Tell every worker to stop because the deadline passed."** Channel (a closed `done` channel broadcasts to all selectors at once) — and next week, `context`, which is the standardised version of exactly this signal.

The thread through all three: pick the primitive that matches the *shape* of the coordination, not the one a proverb made you feel guilty about not using.

## 7. A note on channel-of-channels and ordered results

Two patterns you will meet again. First, **channel-of-channels** (`chan chan T`) lets a requester hand the worker a private reply channel — "here's where to send *my* answer" — which is how you do request/response over a worker pool while keeping each caller's result separate. Second, **ordered fan-in**: because merged results arrive in nondeterministic order, when order matters you attach an index to each value (`type indexed struct { i int; v T }`), collect into a slice sized to the input, and place each result at its index. The link-checker does not need ordering (a report sorted by URL or status is fine), but recognise the pattern — it is how you keep "process in parallel, report in order" honest. Citation: <https://go.dev/blog/pipelines>.

## 8. Exercise pointer

Now do **Exercise 3 — Fan-Out/Fan-In**. You will build a small pipeline that squares N numbers across W worker goroutines, merge their outputs with the `WaitGroup`-plus-closer fan-in, collect and sort the results, and prove the program leaks no goroutines (a `runtime.NumGoroutine` check, with `goleak` as the stretch). A table-test skeleton is provided in comments. The acceptance criterion is a correct result set for any `(N, W)` and a goroutine count that returns to its baseline after the pipeline drains.

## 9. Summary

- A **pipeline** is stages connected by channels: a **source** (no input), intermediate **stages**, and a **sink** (no output). Each stage **closes its output when done** and **ranges its input until closed**; closing the front drains and closes the whole pipe behind it.
- **Fan-out** parallelises a stage: N goroutines read the **same** input channel, sharing the work because each value goes to exactly one receiver. **Fan-in** (`merge`) folds N output channels back into one using the **`WaitGroup` + single closer goroutine** idiom — one forwarder per input, closer closes the merged channel after `wg.Wait()`.
- The full **fan-out/fan-in pipeline** (generator → N workers → merge → sink) is the architecture of the mini-project; results arrive unordered (carry an index to re-order), every goroutine has a guaranteed exit, and it runs clean under `-race` and `goleak`.
- **"Share memory by communicating"** is the default: transfer data *ownership* over channels so only one goroutine touches it at a time. But it is **not a law** — do not overdo it.
- The **decision matrix**: a **channel** moves a value between goroutines and coordinates in time (pipelines, fan-out/fan-in, signalling, deadlines); a **`sync.Mutex`** guards a small shared piece of state (a counter, a map, a cache field) with a short critical section; **neither** when there is no concurrency to coordinate. Pick by the shape of the problem, and prefer the simpler primitive.

Cited references this lecture pulled from: <https://go.dev/blog/pipelines>, <https://go.dev/blog/codelab-share>, <https://go.dev/doc/effective_go#concurrency>, <https://pkg.go.dev/sync#Mutex>.
