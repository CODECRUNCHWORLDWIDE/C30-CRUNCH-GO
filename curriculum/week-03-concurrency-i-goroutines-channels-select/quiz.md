# Week 3 — Quiz

Ten multiple-choice questions covering goroutines, channel semantics (unbuffered vs buffered), closing channels and the "who closes" rule, receiving from a closed channel, ranging over a channel, `select` (default/timeout), `WaitGroup`, deadlocks, and the channel-vs-mutex decision. Treat the quiz as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — Goroutine basics

What happens when `main` returns while a goroutine started with `go f()` is still running?

- (A) The program waits for the goroutine to finish before exiting.
- (B) The program exits immediately, killing the goroutine mid-execution with no cleanup.
- (C) The goroutine is promoted to a new `main` and keeps running.
- (D) The runtime panics with "goroutine still running".

<details>
<summary>Answer</summary>

**(B).** `go f()` returns immediately and runs `f` concurrently, but when `main` returns the whole program exits and every still-running goroutine is killed mid-statement with no deferred calls or cleanup. This is why you coordinate completion with a channel or a `WaitGroup`, never a `time.Sleep`. Citation: <https://go.dev/tour/concurrency/1>, <https://go.dev/doc/effective_go#goroutines>.

</details>

## Question 2 — Unbuffered send semantics

Given an unbuffered channel `ch := make(chan int)`, when does `ch <- 1` (a send) complete?

- (A) Immediately; the value is stored in the channel for later.
- (B) When another goroutine is ready to receive from `ch`; the send and the receive rendezvous at the same instant.
- (C) After a 1-second default timeout.
- (D) Never; you cannot send on an unbuffered channel.

<details>
<summary>Answer</summary>

**(B).** An unbuffered channel is a rendezvous: a send blocks until some goroutine is ready to receive, and the two complete at the same instant. There is no storage — the value is handed directly across. This synchronisation is the channel's primary purpose; carrying the value is almost incidental. Citation: <https://go.dev/tour/concurrency/2>, <https://go.dev/doc/effective_go#channels>.

</details>

## Question 3 — Buffered send semantics

Given `ch := make(chan int, 2)`, after `ch <- 1` and `ch <- 2` with no receiver, what does a third `ch <- 3` do?

- (A) It blocks, because the buffer is full (`len == cap == 2`) and no one is receiving.
- (B) It succeeds and overwrites the value `1`.
- (C) It panics with "buffer overflow".
- (D) It silently drops the value `3`.

<details>
<summary>Answer</summary>

**(A).** A buffered channel's send blocks only when the buffer is full. With capacity 2 and two values buffered (`len == cap == 2`) and no receiver, the third send blocks until something is received and frees a slot. A buffer does not overwrite, panic, or drop. Citation: <https://go.dev/doc/effective_go#channels>.

</details>

## Question 4 — Who closes a channel

Which statement about closing channels is correct?

- (A) The receiver should close the channel when it is done reading.
- (B) Any goroutine may close the channel at any time.
- (C) The sender closes the channel — exactly once, never the receiver; sending on or closing a closed channel panics.
- (D) Channels must always be closed or they leak memory.

<details>
<summary>Answer</summary>

**(C).** The sender owns the send side and is the only goroutine that knows when the last value has been sent, so the sender closes — exactly once. A receiver never closes (a sender might then panic sending into it), and closing an already-closed channel panics. (D) is false: an un-closed channel is garbage-collected like any value; you close only to *signal* receivers. Citation: <https://go.dev/tour/concurrency/4>, <https://go.dev/doc/effective_go#channels>.

</details>

## Question 5 — Receive from a closed channel

After `close(ch)` on `ch := make(chan int)`, what does `v, ok := <-ch` yield?

- (A) It blocks forever, because the channel is closed.
- (B) It panics with "receive on closed channel".
- (C) `v` is the zero value of the element type and `ok` is `false`.
- (D) `v` is the last sent value and `ok` is `true`.

<details>
<summary>Answer</summary>

**(C).** A receive from a closed (and drained) channel returns immediately with the element type's zero value and `ok == false` — every subsequent receive too. This is how `for range` knows to stop and how comma-ok distinguishes a real value from a closed channel. Receiving from a closed channel does *not* panic (only *sending* on or *closing* a closed channel does). Citation: <https://go.dev/tour/concurrency/4>, <https://go.dev/doc/effective_go#channels>.

</details>

## Question 6 — Range over a channel

What ends a `for v := range ch` loop?

- (A) It never ends; you must `break` manually.
- (B) The channel being closed *and* drained of any buffered values.
- (C) Receiving a `nil` value.
- (D) The first receive, since `range` reads exactly one value.

<details>
<summary>Answer</summary>

**(B).** `for v := range ch` receives values until the channel is closed and emptied of any buffered values, then ends. A producer that forgets to `close` leaves the consumer's `range` blocked forever — a leak (or a whole-program deadlock if it is the only other goroutine). `nil` is a normal value and does not end the loop. Citation: <https://go.dev/tour/concurrency/4>.

</details>

## Question 7 — `select` default and timeout

In a `select` statement, what is the effect of adding a `default` case?

- (A) It makes the `select` block until the default's channel is ready.
- (B) It makes the `select` non-blocking: if no other case is ready immediately, the default runs.
- (C) It is required; a `select` without a `default` is a compile error.
- (D) It sets a one-second timeout on all the other cases.

<details>
<summary>Answer</summary>

**(B).** A `default` case makes a `select` non-blocking: if no other case can proceed at that instant, the `default` runs immediately. Without a `default`, `select` blocks until a case is ready. Beware a `default` inside a tight `for` with no other blocking operation — it busy-waits and pins a CPU. Citation: <https://go.dev/tour/concurrency/5>, <https://go.dev/ref/spec#Select_statements>.

</details>

## Question 8 — Send on a closed channel

What happens when a goroutine executes `ch <- 1` on a channel that has already been closed?

- (A) The send is silently ignored.
- (B) The value is buffered until the channel is reopened.
- (C) It panics with "send on closed channel".
- (D) It returns an error you can check.

<details>
<summary>Answer</summary>

**(C).** Sending on a closed channel panics with `send on closed channel`. (Closing an already-closed channel and closing a `nil` channel also panic.) This is precisely why only the sender, and only once, may close: it guarantees no send can ever race a close. Citation: <https://go.dev/ref/spec#Close>, <https://go.dev/doc/effective_go#channels>.

</details>

## Question 9 — `WaitGroup` Add-before-go and deadlocks

Which of these is a correct use of `sync.WaitGroup`, and which mistake causes a problem?

- (A) Call `wg.Add(1)` *before* the `go` statement and pass the `WaitGroup` by pointer; calling `Add` *inside* the goroutine races `Wait` and can let `Wait` return early.
- (B) Call `wg.Add(1)` inside each goroutine; the counter is more accurate that way.
- (C) Pass the `WaitGroup` by value to each worker so each gets its own copy.
- (D) Call `wg.Wait()` inside every worker so they all synchronize.

<details>
<summary>Answer</summary>

**(A).** Call `Add` before the `go` (so the counter reflects the launched goroutines before anyone `Wait`s) and pass the `WaitGroup` by pointer (a copy is a separate counter — `go vet` flags it via "copies lock value"). Calling `Add` inside the goroutine races `Wait`, which may observe a zero counter and return while goroutines are still launching. Citation: <https://pkg.go.dev/sync#WaitGroup>.

</details>

## Question 10 — Channel vs mutex

You have a single shared integer counter incremented by many goroutines and read occasionally. What is the simplest correct primitive?

- (A) A channel of increment messages and a dedicated owner goroutine — channels are always preferred in Go.
- (B) A `sync.Mutex` guarding the counter (or `sync/atomic`): the state is small and shared, the critical section is short, and a lock is simpler and faster than a goroutine plus a channel.
- (C) No synchronization; integer increments are atomic in Go.
- (D) A buffered channel sized to the number of goroutines.

<details>
<summary>Answer</summary>

**(B).** A single small shared counter touched by many goroutines with a short critical section is the textbook case for a `sync.Mutex` (or `sync/atomic` for a plain integer). Building a channel and an owner goroutine for this is the over-engineering the "share memory by communicating" codelab itself warns against. (C) is false — concurrent increments without synchronisation are a data race the detector will flag. Citation: <https://go.dev/blog/codelab-share>, <https://pkg.go.dev/sync#Mutex>.

</details>

---

## Self-assessment

- 9-10: you are fluent in the Week 3 primitives; build the link-checker and prove it leaks nothing.
- 7-8: re-read the lecture notes on the questions you missed — especially the "who closes" rule and channel-vs-mutex. The citations point to the exact Go docs.
- 5-6: re-read all three lecture notes and redo the exercises before the mini-project; concurrency bugs compound when the basics are shaky.
- 0-4: rewind to Lecture 1 and work all three lecture notes and exercises carefully. The link-checker depends on every concept here, and a leak in it costs half the lab's concurrency points.
