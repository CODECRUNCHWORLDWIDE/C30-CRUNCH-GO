# Week 4 — Quiz

Ten multiple-choice questions covering `context`, the `sync` primitives, `sync/atomic`, bounded worker pools, the Go memory model, and the race detector. Treat the quiz as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — Cooperative cancellation

You call `cancel()` on a context whose goroutine is running a tight CPU loop that never checks `ctx.Err()` or selects on `ctx.Done()`. What happens?

- (A) The goroutine is immediately terminated by the runtime.
- (B) The goroutine runs to completion; cancellation is cooperative — `cancel()` closes a channel, it does not stop a goroutine that is not watching.
- (C) The goroutine panics with `context.Canceled`.
- (D) The Go scheduler preempts the goroutine and reschedules it as cancelled.

<details>
<summary>Answer</summary>

**(B).** Cancellation is cooperative. `cancel()` closes the `Done()` channel; a goroutine stops only if it watches that channel (via `select { case <-ctx.Done(): }`) or polls `ctx.Err()`. A tight loop that never checks runs to completion. There is no goroutine `kill` in Go by design. Citation: <https://pkg.go.dev/context>.

</details>

## Question 2 — The two sentinels

A worker returns `ctx.Err()` after its context ended. Which statement is correct?

- (A) `ctx.Err()` is always `context.Canceled`.
- (B) A `WithTimeout` deadline firing yields `context.DeadlineExceeded`; a manual `cancel()` or a `signal.NotifyContext` signal yields `context.Canceled`.
- (C) A signal from `signal.NotifyContext` yields `context.DeadlineExceeded` because a signal is time-based.
- (D) `ctx.Err()` returns `nil` even after cancellation; you must check `ctx.Done()` separately.

<details>
<summary>Answer</summary>

**(B).** `WithTimeout`/`WithDeadline` firing → `context.DeadlineExceeded`. Manual `cancel()` and `signal.NotifyContext` signals → `context.Canceled` (a signal *cancels*; it does not set a deadline). Distinguish with `errors.Is`. Citation: <https://go.dev/blog/context>.

</details>

## Question 3 — `defer cancel()`

Why must you call the `cancel` function returned by `context.WithTimeout`, even when the timeout will fire on its own?

- (A) Calling `cancel()` is optional; the timeout always cleans up after itself.
- (B) `cancel()` releases the context's resources (the timer, the child entry in the parent's tree) immediately; omitting it leaks them until the parent is cancelled, which `go vet`'s `lostcancel` analyzer flags.
- (C) `cancel()` is required to read `ctx.Err()`.
- (D) Without `cancel()`, the context never fires `Done()`.

<details>
<summary>Answer</summary>

**(B).** `cancel()` releases the timer and the child tree entry immediately. Omitting it is a resource leak until the parent is cancelled; `go vet`'s `lostcancel` flags it. Always `defer cancel()`. Citation: <https://pkg.go.dev/context#WithTimeout>.

</details>

## Question 4 — Bounding concurrency

Which of these correctly bounds a fan-out to at most N goroutines running concurrently?

- (A) Start one goroutine per item and hope the scheduler limits them.
- (B) `errgroup.WithContext(ctx)` followed by `g.SetLimit(N)`, then `g.Go(...)` per item.
- (C) A `sync.WaitGroup` with `wg.Add(N)`.
- (D) `runtime.GOMAXPROCS(N)`.

<details>
<summary>Answer</summary>

**(B).** `errgroup.SetLimit(N)` caps concurrent goroutines: `g.Go` blocks until a slot is free. (A) is unbounded. (C) waits for N goroutines but does not cap concurrency. (D) sets the CPU parallelism for the scheduler, not the number of goroutines you may start. Citation: <https://pkg.go.dev/golang.org/x/sync/errgroup>.

</details>

## Question 5 — The semaphore channel

A buffered channel `sem := make(chan struct{}, N)` is used as a counting semaphore. What caps the concurrency at N?

- (A) The channel's element type `struct{}`.
- (B) The channel's *buffer capacity* N: a send blocks once N tokens are outstanding, so at most N goroutines hold a token at once.
- (C) The number of goroutines that call `close(sem)`.
- (D) Nothing; a buffered channel does not bound concurrency.

<details>
<summary>Answer</summary>

**(B).** The buffer *capacity* is the bound. Sending a token blocks once N are outstanding, so at most N goroutines proceed at once. `struct{}` is just the zero-byte payload — it carries the count, not data. Citation: <https://go.dev/blog/pipelines>.

</details>

## Question 6 — Atomic vs mutex

For a single `int64` counter incremented from many goroutines and occasionally read, the idiomatic and fastest correct choice is:

- (A) A `sync.RWMutex` guarding the `int64`.
- (B) A `sync/atomic.Int64` with `Add` and `Load`.
- (C) A channel that every goroutine sends `1` to.
- (D) An unsynchronised `int64`, since `int64` writes are atomic on 64-bit platforms anyway.

<details>
<summary>Answer</summary>

**(B).** A single scalar → `sync/atomic.Int64`. It is a single hardware instruction, faster than a mutex under contention, and correct. (D) is wrong: an unsynchronised increment is a read-modify-write race regardless of platform word size — `int64++` is three operations, not one. Citation: <https://pkg.go.dev/sync/atomic>.

</details>

## Question 7 — Mutex rules

Which statement about `sync.Mutex` is correct?

- (A) You must initialise a `Mutex` with `sync.NewMutex()` before use.
- (B) The zero value of a `Mutex` is a ready-to-use unlocked mutex; you must never copy a value containing one (`go vet`'s `copylocks` enforces this), and you should not do I/O while holding it.
- (C) `RLock` and `Lock` on a plain `Mutex` both allow concurrent readers.
- (D) Holding a mutex while making a slow network call is fine because the mutex makes it safe.

<details>
<summary>Answer</summary>

**(B).** The zero `Mutex` is ready to use; never copy a mutex-bearing value (`copylocks`); never do slow I/O while holding it. (A) there is no `NewMutex`. (C) describes `RWMutex`, not `Mutex`. (D) is exactly the anti-pattern — holding a lock across I/O serialises everyone behind the slow call. Citation: <https://pkg.go.dev/sync#Mutex>.

</details>

## Question 8 — The data race definition

Which of the following is a data race under the Go memory model?

- (A) Two goroutines both read the same variable and neither writes it.
- (B) Two goroutines access the same variable, at least one writes, and there is no synchronisation ordering the two accesses.
- (C) One goroutine writes a variable, then a channel send/receive orders a second goroutine's read after it.
- (D) A single goroutine reads and writes a variable in program order.

<details>
<summary>Answer</summary>

**(B).** All three clauses are required: concurrency, at least one write, no synchronisation ordering. (A) two reads are fine. (C) the channel orders the accesses (happens-before), so no race. (D) a single goroutine in program order never races with itself. Citation: <https://go.dev/ref/mem>.

</details>

## Question 9 — What `-race` proves

Your CI runs `go test -race ./...` and it is green. Which statement is correct?

- (A) Your program is provably free of all data races.
- (B) The detector found no races on the code paths your tests exercised; it has no false positives but does have false negatives for paths that did not run, so a race on an un-exercised path is still possible.
- (C) The race detector has false positives, so a green run might still hide a reported race.
- (D) `-race` proves the absence of races but only on 64-bit platforms.

<details>
<summary>Answer</summary>

**(B).** `-race` instruments the accesses that actually execute. No false positives (every report is real), but false negatives for un-exercised paths. A green run proves the *exercised* paths are clean, not the whole program. Write tests that hit the concurrent paths. Citation: <https://go.dev/doc/articles/race_detector>.

</details>

## Question 10 — Why this aggregation is race-free

In the lecture's worker pool, each goroutine writes `results[i]` for its own distinct `i`, and the main goroutine reads `results` after `g.Wait()`. Why is this race-free without a mutex?

- (A) Because slices are inherently thread-safe in Go.
- (B) Because each goroutine writes a *distinct* index (no shared location), and `g.Wait()` establishes a happens-before edge between every write and the subsequent read.
- (C) Because the writes are atomic.
- (D) It is not race-free; this code has a data race.

<details>
<summary>Answer</summary>

**(B).** Two facts: each goroutine writes a *distinct* location (no shared write), and `g.Wait()` creates a happens-before edge between every write and the post-`Wait` read. Slices are *not* inherently thread-safe (A is wrong); the writes are plain assignments, not atomics (C is wrong). Citation: <https://go.dev/ref/mem#synchronization>.

</details>

---

## Self-assessment

- 9-10: you can demo the Phase I gate's worker pool and defend every concurrency choice.
- 7-8: re-read the lecture notes on the questions you missed; the citations point to the exact Go docs.
- 5-6: re-read all three lecture notes and redo the exercises, especially Exercise 3 (the race).
- 0-4: rewind to Lecture 1. The mini-project and the Phase I gate will not go well without the `context` and memory-model foundation.
