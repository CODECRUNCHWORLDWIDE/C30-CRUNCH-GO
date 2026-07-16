# Week 3 — Exercise Solutions and Annotations

Read these after attempting the exercises, not before. Every Go snippet here has been built (`go build ./...`), run (`go run .`), and checked clean under `go vet ./...` and `staticcheck ./...`; the concurrent ones run clean under `go test -race ./...` and leave no goroutines behind under `go.uber.org/goleak`.

## Exercise 1 — Goroutines and Channels

### What success looks like

```
--- 1. rendezvous ---
main got: done
goroutine: ran AFTER the send completed
--- 2. buffered ---
len=2 cap=2
10
20
--- 3. range until closed ---
got 1
got 4
got 9
got 16
range ended (channel closed and drained)
--- 4. fan-out / collect ---
doubled: [2 4 6 8 10 12]
```

### The five predictions, answered

- **PREDICT 1.** `main got: done` prints first. The send `ch <- "done"` and the receive `<-ch` complete at the same instant (the rendezvous). When `<-ch` returns, `main` resumes and prints `main got: done`; only *then* is the goroutine free to run its next line. The send *unblocks* the goroutine, but `main` is already running, so `main`'s print wins. (If you saw the order flip on a heavily loaded machine, that is the scheduler's prerogative — never depend on it. What is guaranteed is that the receive cannot complete before the send.)
- **PREDICT 2.** `len=2 cap=2`. `cap` is the buffer size you passed to `make` (2). `len` is how many values are currently buffered (2, because we sent two and received none yet). A third send would block, because `len == cap`.
- **PREDICT 3.** `1 4 9 16` print, then the loop ends. `produce(out, 4)` sends `1*1, 2*2, 3*3, 4*4` and then `defer close(out)` runs. The `for v := range out` loop receives each value and ends when `out` is closed and drained. **If you delete the `defer close(out)`**, the `range` loop blocks forever after `16`, waiting for a value that never comes — a deadlock (here, the *whole-program* deadlock, since `main` is the only other goroutine and it is blocked on the `range`).
- **PREDICT 4.** `[2 4 6 8 10 12]`. Six jobs (`1..6`), each doubled, collected and `sort.Ints`-ed. Before sorting, the order is nondeterministic — which of the three workers handled which job, and which finished first, varies run to run. The `sort` is what makes the output deterministic and testable.
- **PREDICT 5.** It is a deadlock because `ch <- 1` on an unbuffered channel blocks until *some other goroutine* receives, but there is no other goroutine — `main` is the only one, and it is stuck on the send, so it can never reach the `<-ch` on the next line. Every goroutine is asleep, so the runtime aborts: `fatal error: all goroutines are asleep - deadlock!`. The one-line fix is to make the channel buffered (`make(chan int, 1)`), so the send has room and proceeds; then `<-ch` reads it back. (Or, more realistically, do the receive in a separate goroutine.)

### Who closes what (the reviewer's question)

- `rendezvous` / `bufferedDoesNotBlock`: nobody closes — there is no `range`, so no close is needed. Closing is a signal to a `range`/comma-ok receiver; without one, an un-closed channel is just garbage-collected.
- `produce`'s `out`: the **producer** closes it (`defer close(out)`), because the producer owns the send side. The consumer (`main`'s `range`) never closes.
- `fanOutCollect`: the **feeder** closes `jobs` (it owns the send side); a **separate closer goroutine** closes `results` after `wg.Wait()` (because the three workers all send on `results`, none of them may close it alone).

### Common pitfalls

1. **Closing from the receiver.** If `main` had tried to `close(out)` in part 3, a still-running `produce` could panic with `send on closed channel`. The sender closes, always.
2. **`Add` inside the goroutine.** Moving `wg.Add(1)` inside the `go func` races `wg.Wait()` and can let `Wait` return early. Add before the `go`.
3. **Forgetting the closer goroutine for `results`.** Three workers send on `results`; if any single worker tried to `close(results)`, the others would panic on their next send. The `wg.Wait(); close(results)` goroutine is the only safe closer.

## Exercise 2 — select, WaitGroup, and a Goroutine Leak

### What success looks like (after fixing the leak)

```
--- 1. select + timeout ---
fast: got 42
slow: timed out
--- 2. WaitGroup ---
all 4 workers finished
--- 3. leak demo ---
goroutines: before=1 after=1 leaked≈0
```

Before fixing the leak, part 3 instead prints something like `before=1 after=51 leaked≈50` — fifty goroutines parked forever on their send.

### The three predictions, answered

- **PREDICT 1.** The timeout wins: `slow: timed out`. `fetch(500ms)` will not deliver for half a second, but `time.After(200ms)` fires first, so the second `select` case is chosen. (Because `fetch` uses a buffered channel of 1, the slow goroutine's eventual send still succeeds into the buffer and the goroutine returns — no leak. That is the point of buffering it.)
- **PREDICT 2.** No. `wg.Wait()` blocks until the counter hits zero, which happens only after all four workers have run `defer wg.Done()`. The print is *after* `Wait()`, so it cannot run until every worker has finished. That guarantee is exactly why a `WaitGroup` replaces a `time.Sleep` guess.
- **PREDICT 3.** Before the fix: `leaked≈50` (one leaked goroutine per timed-out call; all 50 time out because 20ms < 100ms). After the fix: `leaked≈0`. `runtime.GC()` does **not** reclaim the leaked goroutines because a goroutine blocked on a send is *live*, not garbage.

### The leak, located and explained

The leaking line is:

```go
result <- 42 // in leakyFetch's goroutine
```

with `result := make(chan int)` (unbuffered). The caller's `select` times out at 20ms and `leakyFetch` returns. At 100ms the goroutine wakes, executes `result <- 42`, and finds no receiver — the caller is long gone — so it blocks on that send *forever*. Each timed-out call leaves one such goroutine. Nothing crashes; the count just climbs.

### The fix

**Option (a) — buffer of one (abandon the result safely):**

```go
result := make(chan int, 1) // the send always has room, receiver or not
```

The goroutine's `result <- 42` now succeeds into the buffer even when nobody receives, so the goroutine returns and is collected. This is the simplest fix when you just want to *discard* a late result.

**Option (b) — a `done` channel (cancel the work):**

```go
func leakyFetch() (int, error) {
	result := make(chan int, 1)
	done := make(chan struct{})
	defer close(done) // signal the worker on every return path

	go func() {
		time.Sleep(100 * time.Millisecond)
		select {
		case result <- 42:
		case <-done: // caller gave up; stop trying to send and return
		}
	}()

	select {
	case r := <-result:
		return r, nil
	case <-time.After(20 * time.Millisecond):
		return 0, errors.New("timeout")
	}
}
```

`close(done)` (deferred, so it fires on timeout *and* on success) makes the worker's `<-done` case ready, so its `select` proceeds and the goroutine returns. This generalises to *cancelling* the work, not just discarding the result — and it is exactly the pattern `context.Context` standardises next week.

### Common pitfalls

1. **"GC will clean it up."** It will not. A blocked goroutine is reachable and runnable-in-principle, so it is not garbage. Only making it *return* frees it.
2. **`time.After` in a hot loop.** Each `time.After` allocates a timer that lives until it fires. In a tight loop that usually takes the other branch, prefer a reusable `time.NewTimer` you `Stop`/`Reset` — or, next week, `context.WithTimeout`.
3. **A `default` busy-wait.** Adding `default:` to a `select` in a `for` with no other blocking op spins a CPU core. If you want to wait, do not add a `default`.

## Exercise 3 — Fan-Out / Fan-In, and Proving No Leaks

### What success looks like

```
squares: [1 4 9 16 25 36 49 64]
goroutines: before=1 after=1 delta=0
empty: []
single: [81]
```

### The three predictions, answered

- **PREDICT 1.** `[1 4 9 16 25 36 49 64]` — the squares of `1..8`, sorted. Unsorted, the order would be nondeterministic (four workers race to finish).
- **PREDICT 2.** `delta=0`. A leak-free pipeline returns every goroutine it started: `gen` returns after closing `source`; each `square` worker returns when `source` is drained and fires its deferred close; each `merge` forwarder returns when its worker's channel closes; the `merge` closer returns after `wg.Wait()`. So the count comes back to baseline. (Tiny transient deltas of ±1 from the runtime's own goroutines are possible; a *growing* delta across repeated calls is the real leak signal.)
- **PREDICT 3.** `SquareAll(nil, 4)` returns `[]` (the empty/nil slice prints as `[]`): `gen` sends nothing and closes `source` immediately, every worker's `range` ends at once, `merge` closes `out`, and the collect loop appends nothing. `SquareAll([]int{9}, 1)` returns `[81]`.

### Who closes each of the three channel kinds

1. **`source`** (from `gen`): the **generator** closes it (`defer close(out)`), because it is the sole sender.
2. **Each `square` stage's output**: that **stage's goroutine** closes its own output (`defer close(out)`), again the sole sender on it.
3. **`merge`'s `out`**: a **dedicated closer goroutine** closes it after `wg.Wait()`, because `len(inputs)` forwarders all send on it — no single forwarder may close it.

### The table test you should write

The whole point of keeping `SquareAll` pure (no I/O, no `os.Exit`) is that it is table-testable just like Week 1. Put this in `main_test.go`:

```go
package main

import (
	"reflect"
	"testing"
)

func TestSquareAll(t *testing.T) {
	tests := []struct {
		name    string
		in      []int
		workers int
		want    []int
	}{
		{"empty", nil, 4, nil},
		{"single, one worker", []int{9}, 1, []int{81}},
		{"eight, four workers", []int{1, 2, 3, 4, 5, 6, 7, 8}, 4, []int{1, 4, 9, 16, 25, 36, 49, 64}},
		{"more workers than items", []int{2, 3}, 16, []int{4, 9}},
		{"workers clamped to 1", []int{2, 3}, 0, []int{4, 9}}, // workers<1 -> 1
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SquareAll(tc.in, tc.workers)
			// SquareAll sorts, so the comparison is deterministic despite parallelism.
			if len(got) == 0 && len(tc.want) == 0 {
				return // both empty: pass (nil vs []int{} differ under DeepEqual)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("SquareAll(%v, %d) = %v, want %v", tc.in, tc.workers, got, tc.want)
			}
		})
	}
}
```

Run it three ways:

```
$ go test ./...
ok      github.com/you/ex03     0.006s
$ go test -race ./...          # the race detector finds nothing — no shared state
ok      github.com/you/ex03     1.2s
$ go test -run TestSquareAll/eight,_four_workers -v ./...
```

The `len(got)==0 && len(tc.want)==0` guard sidesteps the Week 1 trap that `reflect.DeepEqual(nil, []int{})` is `false`; for the empty case we only care that the result is empty.

### The `goleak` proof (the stretch, and the mini-project's bar)

Add `go.uber.org/goleak` (`go get go.uber.org/goleak`) and a `TestMain`:

```go
func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}
```

If `SquareAll` leaked a goroutine, `go test` would now fail with the leaked goroutine's state and creation site. Because the pipeline closes every channel from the right owner, it passes — this is how you *prove* "no leaked goroutines" rather than asserting it.

### Common pitfalls

1. **Forgetting `merge`'s closer goroutine.** Without `go func(){ wg.Wait(); close(out) }()`, `out` is never closed, the collect `range` blocks forever, and the program deadlocks. (Or, if you tried to close `out` from a forwarder, the others panic.)
2. **`Add` after the `go` in `merge`.** Use `wg.Add(len(inputs))` once *before* the launch loop (or `wg.Add(1)` immediately before each `go`), never inside the forwarder.
3. **Comparing parallel output without sorting.** If `SquareAll` did not sort, the table test would be flaky — the result order changes run to run. Either sort (as here) or compare as a multiset.

## Cross-cutting notes

- **The three reviewer questions.** For every concurrent program, answer: *who closes each channel, who waits for whom, and what happens to every goroutine when the work is done or cancelled?* If you cannot answer all three, the code is not ready.
- **The "who closes" rule is non-negotiable.** Sender closes, once, never the receiver. When several goroutines send on one channel, a `WaitGroup` + a single closer goroutine is the only correct close.
- **Leak detection is a habit, not an afterthought.** Count with `runtime.NumGoroutine()` while iterating, and add `goleak.VerifyTestMain` to any package with concurrent code. A leak passes `go vet`, `staticcheck`, and the happy-path tests — only a leak check catches it.
- **Run `-race` now.** Deep race coverage is Week 4, but `go test -race ./...` is nearly free and catches the moment two goroutines touch the same variable without a channel or mutex between them.

Cited references: <https://go.dev/tour/concurrency/1>, <https://go.dev/tour/concurrency/5>, <https://go.dev/doc/effective_go#channels>, <https://go.dev/blog/pipelines>, <https://pkg.go.dev/sync#WaitGroup>, <https://pkg.go.dev/runtime#NumGoroutine>, <https://github.com/uber-go/goleak>.
