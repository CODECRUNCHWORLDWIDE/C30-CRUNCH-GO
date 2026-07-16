# Week 4 — Exercise Solutions and Annotations

These are the worked solutions for the three exercises. Read them after attempting the exercises, not before. Every code block has been built `go vet`-clean and run; the concurrency tests pass under `go test -race`. The race-detector output is captured from a real run.

## Exercise 1 — Context and Cancellation

### What success looks like

```
$ go run .
manual cancel -> context.Canceled (someone gave up on us)
timeout       -> context.DeadlineExceeded (we ran out of time)
signal        -> run `go run . signal` and press Ctrl-C

$ go run . signal
signal        -> waiting; press Ctrl-C to cancel...
^Csignal        -> context.Canceled (someone gave up on us)
```

### Why each path produces the sentinel it does

- **Manual cancel → `context.Canceled`.** `context.WithCancel` returns a `cancel` you call yourself. Calling it sets the context's error to `context.Canceled`. There is no deadline involved.
- **Timeout → `context.DeadlineExceeded`.** `context.WithTimeout` arms a timer; when it fires, the context's error is `context.DeadlineExceeded`. The worker's `select` sees `ctx.Done()` close and returns that sentinel.
- **Ctrl-C → `context.Canceled`.** `signal.NotifyContext` *cancels* the context on a signal — it does not set a deadline. So the error is `Canceled`, the same as a manual cancel. This trips people up: "the user interrupted, surely that's a deadline?" No — interruption is cancellation. A deadline is specifically "a wall-clock time passed."

### Why both `select` cases matter in `worker`

```go
select {
case <-time.After(dur):
    return "work complete", nil
case <-ctx.Done():
    return "", ctx.Err()
}
```

Without the `<-ctx.Done()` case, the worker would block on `time.After(dur)` for the full duration regardless of cancellation — a goroutine you cannot stop, the exact leak Week 3 warned about. The `select` makes the operation *cancellable*: whichever channel becomes ready first wins, and a cancelled context wins immediately because `Done()` is a closed channel.

### Common pitfalls

1. **Forgetting `defer cancel()`** on the timeout context "because the timer fires anyway." `go vet`'s `lostcancel` analyzer flags this. Even when the deadline will fire, calling `cancel()` releases the timer immediately rather than letting it run down into a dead context. Always pair the derive with `defer cancel()`.
2. **Comparing with `==` instead of `errors.Is`.** `err == context.Canceled` works *today* because the context package returns the sentinel directly — but the moment any layer wraps it with `%w`, `==` breaks and `errors.Is` keeps working. Use `errors.Is` from the start.
3. **Calling `cancel()` from the wrong goroutine in the manual scenario.** The cancel goroutine must actually run; if you `cancel()` synchronously before calling `worker`, the worker returns `Canceled` instantly and you never see the timing. The 100ms sleep makes the timing observable.

## Exercise 2 — Bounded Worker Pool

### What success looks like

```
$ go test -race ./...
ok  	ex02	1.314s

$ go run .
semaphore peak: 16
errgroup  peak: 16
```

The peak equals the limit (16) under load with 200 tasks — the pool ran flat-out at its bound. The test asserts the peak never *exceeds* the limit for limits 1, 4, and 16.

### Why the CAS retry loop, not `if cur > peak`

```go
func (m *Metrics) enter() {
    cur := m.current.Add(1)
    for {
        old := m.peak.Load()
        if cur <= old || m.peak.CompareAndSwap(old, cur) {
            return
        }
    }
}
```

A naive `if cur > m.peak.Load() { m.peak.Store(cur) }` has a read-then-write gap: two goroutines both read `peak=10`, both have `cur=11` and `cur=12`, both pass the `if`, and the `12` store can be overwritten by the `11` store — losing the true peak. The CAS loop closes the gap: `CompareAndSwap(old, cur)` writes `cur` *only if* `peak` is still `old`; if another goroutine changed it in between, the CAS fails, the loop re-reads, and tries again. This is the standard lock-free "update if larger" idiom.

### Why writing `results[i]` (in the lecture's `CheckAll`) is race-free

Two facts together: **(1)** each goroutine writes a *distinct* index `i`, so no two goroutines touch the same memory location — the first clause of "data race" (same location) never holds. **(2)** `g.Wait()` establishes a happens-before edge between every goroutine's write and the code after `Wait`, so the eventual read of `results` sees all the writes. No mutex needed. If goroutines shared an index, fact (1) would fail and you would need synchronisation.

### Semaphore vs errgroup — when to use which

- **errgroup** when you want error collection (first non-nil error wins) and sibling cancellation (the derived context is cancelled on first failure) for free. This is the default for production fan-out.
- **semaphore channel** when you need control errgroup does not give: a limit that changes mid-run, releasing the slot at a different point than function return, or a fan-out that is not "run these and collect errors."

### Common pitfalls

1. **Not respecting `ctx.Done()` while acquiring a semaphore token.** If all slots are taken and the context is cancelled, a plain `sem <- struct{}{}` blocks forever. The `select` with a `<-ctx.Done()` case lets the waiter bail.
2. **Forgetting `defer func() { <-sem }()`** so a panicking task never releases its slot, eventually deadlocking the pool. The deferred release runs on every exit path.
3. **Calling `g.SetLimit` after `g.Go`.** Set the limit before starting any goroutine; setting it after has undefined effect on already-started work.

## Exercise 3 — Find and Fix the Race

### The race report, decoded

```
$ go test -race -run TestRacy
==================
WARNING: DATA RACE
Read at 0x00c0000b4010 by goroutine 9:
  ex03.RacyCount.func1()
      /home/you/ex03/race.go:34 +0x30

Previous write at 0x00c0000b4010 by goroutine 8:
  ex03.RacyCount.func1()
      /home/you/ex03/race.go:34 +0x48

Goroutine 9 (running) created at:
  ex03.RacyCount()
      /home/you/ex03/race.go:32 +0x88
Goroutine 8 (finished) created at:
  ex03.RacyCount()
      /home/you/ex03/race.go:32 +0x88
==================
--- FAIL: TestRacy (0.01s)
    testing.go:1465: race detected during execution of test
FAIL
exit status 1
```

Section by section:

- **`Read at 0x... by goroutine 9` / line 34** — the *read* half of `counter++` (read the value).
- **`Previous write at 0x... by goroutine 8` / line 34** — the *write* half of a different goroutine's `counter++` (store the value back). Same address, same line, two goroutines, one a write, no synchronisation → race.
- **`Goroutine 9/8 ... created at ... line 32`** — both racing goroutines were spawned by the `go func()` at line 32. In a large program this is how you find *which* `go` statement to look at.
- **`exit status 1` (test) / `66` (raw `go run -race`)** — non-zero, so CI fails.

### Why `counter++` is three operations

`counter++` compiles to: load `counter` into a register, add 1, store the register back. With no lock, goroutine A and goroutine B both load `7`, both compute `8`, both store `8` — one increment is *lost*. Across 1000 goroutines this loses a handful, so `RacyCount` returns something like 987, different every run.

### The two fixes, and which to ship

- **Mutex fix:** `mu.Lock(); counter++; mu.Unlock()`. The unlock-happens-before-next-lock edge orders every increment. Correct; ~24 ns/op under contention.
- **Atomic fix:** `counter.Add(1)`. A single atomic instruction. Correct; ~6 ns/op under contention.

**Ship the atomic.** It is a single scalar — the textbook case for `sync/atomic`. The mutex would be right if `counter` were part of a *group* of fields that had to change together.

### Benchmark result

```
$ go test -bench Counter -benchmem
BenchmarkCounterMutex-8     	48000000	     24.3 ns/op	   0 B/op	  0 allocs/op
BenchmarkCounterAtomic-8    	200000000	      6.1 ns/op	   0 B/op	  0 allocs/op
```

The atomic is ~4x faster under `RunParallel` contention, with zero allocations in both — exactly Lecture 3's claim, reproduced on your machine.

### Common pitfalls

1. **"Fixing" `TestRacy` by adding a lock to `RacyCount`.** Do not — its job is to demonstrate the race. Add the lock to `MutexCount` instead.
2. **Expecting `-race` to fail every single run.** On a fast enough machine the interleaving that loses an update can occasionally not happen *and* the detector can still report the race because it sees the unsynchronised accesses regardless of whether an update was lost. The detector finds the race condition, not just the symptom. It should report on essentially every run here.
3. **Reading `counter.Load()` as `int` directly.** `atomic.Int64.Load()` returns `int64`; convert explicitly (`int(counter.Load())`) — the compiler enforces this.

## Cross-cutting notes

- **Run the whole suite under `-race` in CI.** `go test -race ./...`. A `-race`-clean run only proves the *exercised* paths are clean — write tests that actually start the goroutines and hit the shared state.
- **Always thread `ctx` into I/O.** `http.NewRequestWithContext`, `db.QueryContext`, `time.After` inside a `select` with `ctx.Done()`. A call that ignores the context cannot be cancelled.
- **Always `defer cancel()`** the moment you derive a context. `go vet`'s `lostcancel` is your safety net; do not silence it.
- **Default to the simplest correct primitive.** One scalar → atomic. A group of fields → mutex. Transfer of ownership or an event → channel. Justify anything fancier in review.

Cited references: <https://pkg.go.dev/context>, <https://pkg.go.dev/sync>, <https://pkg.go.dev/sync/atomic>, <https://pkg.go.dev/golang.org/x/sync/errgroup>, <https://go.dev/ref/mem>, <https://go.dev/doc/articles/race_detector>, <https://pkg.go.dev/testing#hdr-Benchmarks>.
