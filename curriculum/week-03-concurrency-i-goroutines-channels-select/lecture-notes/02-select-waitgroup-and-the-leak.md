# Lecture 2 — `select`, `sync.WaitGroup`, Deadlocks, and the Goroutine Leak

> **Time:** 2 hours. Take `select` and `WaitGroup` first, then spend the back half on the deadlock-and-leak material with the debugger open. **Prerequisites:** Lecture 1. **Citations:** the Tour's `select` page at <https://go.dev/tour/concurrency/5>, the `sync.WaitGroup` docs at <https://pkg.go.dev/sync#WaitGroup>, `runtime.NumGoroutine` at <https://pkg.go.dev/runtime#NumGoroutine>, and `goleak` at <https://github.com/uber-go/goleak>.

## 1. `select` — the `switch` of concurrency

Lecture 1 gave you one channel and two goroutines. Real concurrent code waits on *several* things at once: "a result arrived OR I was told to stop OR the deadline passed." The construct for that is `select`. It looks like a `switch`, but every `case` is a channel operation, and it blocks until one of them can proceed:

```go
select {
case v := <-in:
	fmt.Println("got", v)
case out <- x:
	fmt.Println("sent x")
case <-done:
	fmt.Println("told to stop")
}
```

The semantics, precisely:

1. `select` evaluates all its `case` channel operations and **blocks until at least one can proceed** (a receive whose channel has a value or is closed, a send whose channel has room).
2. If **several** cases are ready at once, it picks **one uniformly at random**. This is deliberate: you cannot starve a case by listing it last, and you cannot rely on priority by listing it first.
3. It runs the chosen case's body and then falls through past the `select` — it does **not** loop. To keep selecting, wrap it in a `for`.

Citation: <https://go.dev/tour/concurrency/5>, the spec's select statement at <https://go.dev/ref/spec#Select_statements>.

### 1.1 The `default` case makes `select` non-blocking

Add a `default` and `select` no longer blocks: if no other case is ready *right now*, it takes the default immediately.

```go
select {
case v := <-ch:
	fmt.Println("got", v)
default:
	fmt.Println("nothing ready; moving on")
}
```

This is the idiomatic "try to receive (or send) without blocking." A non-blocking *send* into a possibly-full channel — "drop the value if there's no room" — is the same shape:

```go
select {
case results <- r:
	// delivered
default:
	// buffer full; drop r (or count a dropped result)
}
```

Use `default` sparingly. A `for { select { ... default: } }` with no other blocking operation is a **busy-wait** that pins a CPU core spinning — almost always a bug. If you want "do work when there is work, otherwise wait," you want a blocking `select` (no default), not a default that spins.

### 1.2 Timeouts with `time.After`

`time.After(d)` returns a channel that delivers one value after duration `d`. Put it in a `select` and you have a timeout:

```go
select {
case res := <-work:
	fmt.Println("result:", res)
case <-time.After(2 * time.Second):
	fmt.Println("timed out after 2s")
}
```

If `work` produces within two seconds, the first case wins; otherwise the timer fires and the second case wins. This is the single most common `select` pattern in service code. One caveat worth knowing now: `time.After` allocates a timer that is not garbage-collected until it fires, so calling it in a hot loop that usually takes the *other* branch can pile up timers; for that case you reach for a reusable `time.NewTimer` you `Stop` and `Reset`. We use plain `time.After` this week and revisit the timer subtlety in Week 4 with `context.WithTimeout`, which is the production way to express a deadline. Citation: <https://pkg.go.dev/time#After>.

### 1.3 The nil-channel trick

A receive from a `nil` channel blocks **forever**; a send to a `nil` channel blocks **forever**. That sounds like a pure footgun, but it is a precise tool: **a `select` case on a `nil` channel is never selected.** So you can *disable* a branch dynamically by setting its channel variable to `nil`:

```go
func merge(a, b <-chan int, out chan<- int) {
	for a != nil || b != nil { // keep going while either source is live
		select {
		case v, ok := <-a:
			if !ok {
				a = nil // a is drained: disable this case, never select it again
				continue
			}
			out <- v
		case v, ok := <-b:
			if !ok {
				b = nil // b is drained: disable this case
				continue
			}
			out <- v
		}
	}
	close(out) // both inputs drained; this goroutine owns out, so it closes
}
```

When `a` is closed and drained, the comma-ok receive yields `ok == false`; we set `a = nil`, and from then on the `case <-a` can never be chosen (a `nil` channel never proceeds), so the `select` waits only on `b`. When both are `nil`, the `for` condition is false and we close `out`. This is a clean, allocation-free way to merge two channels and know exactly when both are finished — a hand-rolled fan-in for two inputs. Citation: the spec's select statement at <https://go.dev/ref/spec#Select_statements>.

## 2. `sync.WaitGroup` — waiting for completion

A channel coordinates a *value hand-off*. Sometimes you do not need the values, you only need to know **"are all the goroutines I launched finished?"** That is exactly and only what `sync.WaitGroup` is for. It is a counter with three methods:

- `wg.Add(n)` — add `n` to the counter (the number of goroutines you are about to launch).
- `wg.Done()` — subtract one (each goroutine calls it when it finishes; idiomatically `defer wg.Done()` as the first line).
- `wg.Wait()` — block until the counter reaches zero.

```go
func main() {
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1) // BEFORE the go statement — see the rule below
		go func(id int) {
			defer wg.Done() // first line of the goroutine
			fmt.Printf("worker %d running\n", id)
		}(i)
	}
	wg.Wait() // blocks until all four have called Done
	fmt.Println("all workers finished")
}
```

This is the structured replacement for the `time.Sleep` hack: `wg.Wait()` returns at the moment — not a guessed millisecond later — that the last goroutine finishes. Citation: <https://pkg.go.dev/sync#WaitGroup>.

### 2.1 The three rules you must not break

**Rule 1 — `Add` before the `go`.** Call `wg.Add(1)` *before* the `go` statement that launches the goroutine, not inside the goroutine. If you call `Add` inside the goroutine, it races with `Wait`: the scheduler might run `Wait` before the goroutine has had a chance to `Add`, see a zero counter, and return early while goroutines are still launching. The counter must reflect the launched goroutines *before* anyone waits.

```go
// WRONG — Add inside the goroutine races Wait
for i := 0; i < 4; i++ {
	go func() {
		wg.Add(1) // BUG: Wait() may have already returned
		defer wg.Done()
		// ...
	}()
}
wg.Wait()
```

**Rule 2 — pass it by pointer.** A `sync.WaitGroup` must not be copied after first use; its internal counter would be duplicated and the copies would track different counts. Pass `*sync.WaitGroup`, never `sync.WaitGroup` by value:

```go
func worker(wg *sync.WaitGroup) { // pointer
	defer wg.Done()
	// ...
}
```

`go vet` catches a copied `WaitGroup` with its "copies lock value" analyzer — this is one of the concrete reasons the week's contract includes a clean `go vet`. Citation: <https://pkg.go.dev/sync#WaitGroup> ("A WaitGroup must not be copied after first use.").

**Rule 3 — one `Wait`, after all the `Add`s.** The coordinating goroutine calls `Wait` once. Calling `Add` with a positive delta concurrently with `Wait`, or after `Wait` has returned, is a misuse. Establish the full count, launch, then wait.

### 2.2 `WaitGroup` does not move data

A `WaitGroup` answers "are they done?" — it does not collect their results. The results still flow over a channel. The combination from Lecture 1 §7 is the canonical shape: a `WaitGroup` to know when every sender has finished, a closer goroutine that closes the output channel after `wg.Wait()`, and a `range` over the output to collect the values. Keep the two jobs separate in your head: **channels move values; the `WaitGroup` counts completions.**

## 3. Deadlocks

A deadlock is a state where every goroutine is blocked and none can make progress. The Go runtime detects the *total* deadlock — when **every** goroutine is asleep — and aborts the program:

```go
func main() {
	ch := make(chan int) // unbuffered
	ch <- 1              // blocks forever: no receiver, and no other goroutine
}
```

```
fatal error: all goroutines are asleep - deadlock!

goroutine 1 [chan send]:
main.main()
	/tmp/x/main.go:5 +0x...
```

The message is precise and worth memorising: `all goroutines are asleep - deadlock!`. The runtime can prove no progress is possible because *every* goroutine is parked, so it dumps each goroutine's stack and its blocking reason (`[chan send]`, `[chan receive]`, `[semacquire]`) and exits. Read the dump bottom-up: each goroutine and what it is waiting on usually points straight at the cycle.

Common causes, all of which you will cause on purpose in the exercises:

- **Unbuffered send with no receiver** (above) — the textbook case.
- **Receive with no sender** — `<-ch` where nothing will ever send and the channel is never closed.
- **`range` over a never-closed channel** — the producer forgot to `close`, so the consumer's `for range` blocks forever after the last value.
- **Everyone waiting on everyone** — goroutine A waits on a channel B will write only after reading one A will write only after... a cycle.

The crucial limitation, which motivates the next section: **the runtime only catches the case where *all* goroutines are blocked.** If one goroutine is stuck forever but the rest of the program keeps running, the runtime says nothing. That is not a deadlock — it is a *leak*, and it is far more dangerous because nothing crashes.

## 4. Goroutine leaks

A goroutine leak is a goroutine that blocks forever while the program runs on. It never returns, so its stack and everything its closures capture stay alive for the life of the process. Leaks accumulate: a leak-per-request in a long-running service is a slow memory climb and a goroutine count that only goes up until something falls over. The compiler, `go vet`, and `staticcheck` will all happily let you ship one.

### 4.1 A worked leak: the stuck `select`

Here is the canonical leak. A function starts a goroutine to fetch a result and uses a `select` with a timeout to wait for it. If the timeout fires, the function returns — but the worker goroutine is still blocked trying to send on an **unbuffered** channel that now has no receiver:

```go
// LEAKY. Do not ship this.
func fetchWithTimeout() (int, error) {
	result := make(chan int) // unbuffered

	go func() {
		// Simulate slow work.
		time.Sleep(2 * time.Second)
		result <- 42 // BLOCKS FOREVER if the receiver already gave up
	}()

	select {
	case r := <-result:
		return r, nil
	case <-time.After(1 * time.Second):
		return 0, errors.New("timeout")
		// We return here. Nobody will ever receive on `result`.
		// The goroutine wakes after 2s, tries `result <- 42`, and blocks forever.
	}
}
```

Every call to `fetchWithTimeout` that times out leaks one goroutine, parked forever on `result <- 42`. The function *looks* correct and tests pass on the happy path. This is the live reproduction the week's lecture promised: a stuck `select` (well, a stuck *send* after a `select` gave up) that leaks silently.

### 4.2 Detecting the leak with `runtime.NumGoroutine`

The crudest detector is a count. `runtime.NumGoroutine()` returns the number of goroutines that currently exist; measure it before and after, allowing a moment for scheduling to settle:

```go
func main() {
	before := runtime.NumGoroutine()

	for i := 0; i < 100; i++ {
		_, _ = fetchWithTimeout() // each timeout leaks one goroutine
	}

	time.Sleep(3 * time.Second)   // let the slow goroutines wake and block on the send
	runtime.GC()                  // GC will NOT collect a live (blocked) goroutine
	after := runtime.NumGoroutine()

	fmt.Printf("before=%d after=%d leaked≈%d\n", before, after, after-before)
}
```

```
before=1 after=101 leaked≈100
```

A hundred goroutines that will never return. Note that `runtime.GC()` does *not* reclaim them — a blocked goroutine is *live*, not garbage, precisely because it could (in principle) still run. The count is a blunt instrument (other transient goroutines add noise), but it is enough to *see* a leak. Citation: <https://pkg.go.dev/runtime#NumGoroutine>.

### 4.3 Detecting the leak with `goleak`

The production tool is `go.uber.org/goleak`, which snapshots the set of goroutines and fails a test if any unexpected ones survive. The idiomatic use is a `TestMain` that wraps the whole package's tests:

```go
package fetch

import (
	"testing"

	"go.uber.org/goleak"
)

// TestMain runs after all tests in the package and fails if goroutines leaked.
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
```

```
$ go test ./...
--- FAIL: TestMain (0.00s)
    leaks.go:78: found unexpected goroutines:
    [Goroutine 34 in state chan send, ... created by fetch.fetchWithTimeout]
FAIL
```

`goleak` names the leaked goroutine, its state (`chan send`), and where it was created — pointing straight at the bug. You can also assert at the end of a single test with `defer goleak.VerifyNone(t)`. This is the tool the challenge and the mini-project use to *prove* "no leaked goroutines." It is the only external dependency the week introduces, and it is test-only. Citation: <https://github.com/uber-go/goleak> and <https://pkg.go.dev/go.uber.org/goleak>.

### 4.4 Fixing the leak

Two fixes, both standard. The first: **give the worker a buffered channel of capacity 1** so its send never blocks even if the receiver is gone:

```go
func fetchWithTimeout() (int, error) {
	result := make(chan int, 1) // capacity 1: the send never blocks

	go func() {
		time.Sleep(2 * time.Second)
		result <- 42 // always succeeds: the buffer has room, receiver or not
	}()

	select {
	case r := <-result:
		return r, nil
	case <-time.After(1 * time.Second):
		return 0, errors.New("timeout")
		// The goroutine's send still succeeds into the buffer; it then returns. No leak.
	}
}
```

The buffer of one is the classic "abandon the result safely" idiom: the worker can always deposit its single value and exit, and if nobody reads it the buffered channel is simply garbage-collected.

The second fix, which generalises to *cancelling* the work rather than just abandoning it, is a **`done` channel** the worker watches with its own `select`:

```go
func fetchWithCancel() (int, error) {
	result := make(chan int, 1)
	done := make(chan struct{}) // closed to tell the worker to stop

	go func() {
		// Imagine real work that periodically checks done.
		select {
		case <-time.After(2 * time.Second):
			select {
			case result <- 42:
			case <-done: // receiver gave up; stop trying to send
			}
		case <-done: // told to stop before the work finished
			return
		}
	}()

	defer close(done) // on every return path, signal the worker to stop

	select {
	case r := <-result:
		return r, nil
	case <-time.After(1 * time.Second):
		return 0, errors.New("timeout") // close(done) (deferred) unblocks the worker
	}
}
```

`close(done)` is a broadcast: every goroutine selecting on `<-done` wakes at once (a receive from a closed channel returns immediately). The deferred `close(done)` guarantees the signal fires on *every* return path. This `done`-channel cancellation is exactly the pattern `context.Context` standardises — and replacing this hand-rolled `done` channel with `context` is the headline of next week. This week, the hand-rolled version is the point: build it once by hand so you understand what `context` is doing for you. Citation: the pipelines blog's cancellation section at <https://go.dev/blog/pipelines>.

## 5. Putting it together — a `WaitGroup` + `select` + timeout worker pool

A small program that ties the lecture together: a pool of workers, coordinated for completion by a `WaitGroup`, with a `select` that imposes an overall deadline and a `done` channel that stops the workers cleanly when the deadline hits:

```go
package main

import (
	"fmt"
	"sync"
	"time"
)

func worker(id int, jobs <-chan int, done <-chan struct{}, wg *sync.WaitGroup) {
	defer wg.Done()
	for {
		select {
		case j, ok := <-jobs:
			if !ok {
				return // jobs closed and drained: clean exit
			}
			fmt.Printf("worker %d did job %d\n", id, j)
			time.Sleep(50 * time.Millisecond)
		case <-done:
			return // told to stop early (deadline hit): clean exit
		}
	}
}

func main() {
	jobs := make(chan int)
	done := make(chan struct{})

	var wg sync.WaitGroup
	for i := 1; i <= 3; i++ {
		wg.Add(1)            // Add before go
		go worker(i, jobs, done, &wg) // WaitGroup by pointer
	}

	// Feed jobs, but stop the whole thing after a deadline.
	go func() {
		defer close(jobs) // feeder owns jobs, feeder closes it
		for j := 1; j <= 100; j++ {
			select {
			case jobs <- j:
			case <-done:
				return // deadline hit: stop feeding
			}
		}
	}()

	// Overall deadline: after 300ms, tell everyone to stop.
	time.AfterFunc(300*time.Millisecond, func() { close(done) })

	wg.Wait() // returns when all workers have exited (drained or signalled)
	fmt.Println("all workers stopped cleanly")
}
```

Every goroutine has two guaranteed exits: jobs-drained, or done-signalled. `close(done)` broadcasts to all of them at once. The feeder also watches `done` so it does not block on `jobs <- j` after the workers have left. This is a leak-free, deadline-bounded pool built entirely from this week's primitives — and it is one `context.Context` away from the production version you build in Week 4.

## 6. Exercise pointer

Now do **Exercise 2 — `select` and `WaitGroup`**. You will write a `select` with a `time.After` timeout, coordinate a set of workers with a `WaitGroup` (obeying Add-before-`go` and pass-by-pointer), and then study a program that *deliberately* leaks a goroutine — you must spot the stuck send, predict what `runtime.NumGoroutine()` reports, and fix it with a buffered channel or a `done` channel. The acceptance criterion is that you can point at the exact line where a goroutine blocks forever and explain the fix.

## 7. Summary

- **`select`** waits on multiple channel operations and proceeds with one that is ready; **ties are broken at random**. It runs once — wrap it in `for` to keep selecting. A **`default`** case makes it non-blocking (beware busy-waits). A **`case <-time.After(d)`** is a timeout. A **`nil` channel** case is never selected — set a channel to `nil` to disable a branch dynamically.
- **`sync.WaitGroup`** answers "are all my goroutines done?": `Add(n)` / `Done()` / `Wait()`. The rules: **`Add` before the `go`**, **pass it by pointer** (`go vet` catches a copy), and **`Wait` once**. A `WaitGroup` counts completions; channels move the values.
- A **deadlock** is all goroutines blocked; the runtime detects it and aborts with `all goroutines are asleep - deadlock!` plus per-goroutine stacks. Common causes: unbuffered send with no receiver, receive with no sender, `range` over a never-closed channel.
- A **goroutine leak** is one goroutine blocked forever while the program runs on — the runtime does **not** catch it. The classic leak is a worker stuck sending on an unbuffered channel whose receiver gave up after a timeout. Detect it by counting with **`runtime.NumGoroutine()`** or, properly, with **`goleak`** in a `TestMain`. Fix it with a **buffered channel of one** (abandon the result safely) or a **`done` channel** the worker selects on (cancel the work) — the pattern `context` will standardise next week.

Cited references this lecture pulled from: <https://go.dev/tour/concurrency/5>, <https://go.dev/ref/spec#Select_statements>, <https://pkg.go.dev/sync#WaitGroup>, <https://pkg.go.dev/time#After>, <https://pkg.go.dev/runtime#NumGoroutine>, <https://github.com/uber-go/goleak>, <https://go.dev/blog/pipelines>.
