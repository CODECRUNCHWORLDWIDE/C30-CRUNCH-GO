# Week 3 — Concurrency I: Goroutines, Channels, `select`, and the "Who Closes" Rule

Welcome to **C30 · Crunch Go**, Week 3. Last week (interfaces, error values, generics) you learned the small set of strong opinions that make Go code read like Go: consumer-defined interfaces, wrapped error chains inspected with `errors.Is`/`errors.As`, and the deliberate choice between a generic and an interface. This week we reach the feature that the whole rest of the track is built on — Go's concurrency model — and the central claim of the week is a sentence to tape to your monitor: **channels are a synchronisation primitive, not a queue.** A channel's job is to coordinate two goroutines in time, to make one wait for the other; that it also moves a value across is almost a side effect. The cohort that internalizes that one sentence writes pipelines that do not leak; the cohort that treats a channel as "a thread-safe list I push onto" writes deadlocks and leaked goroutines that pass code review and fall over in production. By Friday you will have a concurrent link-checker that fans out to sixteen workers, fans the results back in, finishes cleanly, and — provably — leaks no goroutines.

The first thing to internalize is that **a goroutine is not a thread; it is a function call you asked the runtime to run concurrently**. You write `go f(x)` and `f(x)` runs concurrently with the rest of your function. The goroutine starts with a tiny stack (a couple of kilobytes) that grows and shrinks on demand, so a Go program can hold hundreds of thousands of goroutines where it could hold only thousands of OS threads. The runtime multiplexes those goroutines onto a small pool of OS threads with its own scheduler — the GMP model (goroutines, machine threads, processors) — so you never manage threads directly. The catch, the one that defines this whole week: `go f()` returns *immediately*, and `main` returning kills every goroutine still running, mid-line, with no cleanup. Concurrency is about coordination, and the rest of the week is the coordination primitives. Citation: the Tour's concurrency intro at <https://go.dev/tour/concurrency/1> and Effective Go's goroutines section at <https://go.dev/doc/effective_go#goroutines>.

The second thing to internalize is that **an unbuffered channel is a rendezvous: the send and the receive happen at the same instant, and each blocks until the other is ready**. `ch := make(chan int)` is unbuffered. A `ch <- 1` blocks until some other goroutine executes `<-ch`; the receive blocks until some goroutine sends. The transfer is a synchronisation point — when the send completes, you *know* the receiver has the value, and the receiver knows the sender reached that line. This is the property that makes a channel a synchronisation primitive: it is not "put a value in a box for later," it is "hand a value to someone, and neither of us proceeds until the hand-off happens." Treating an unbuffered channel as a queue you can push onto without a reader is the fastest route to a deadlock. Citation: the Tour's channels page at <https://go.dev/tour/concurrency/2> and Effective Go's channels section at <https://go.dev/doc/effective_go#channels>.

The third thing to internalize is that **a buffered channel decouples sender and receiver up to its capacity — and that capacity is a tuning knob, not a correctness mechanism**. `make(chan int, 8)` holds up to eight values; a send blocks only when the buffer is full, a receive blocks only when it is empty. A buffer is the right tool when you have a known burst to absorb or want to bound how far ahead a producer may run from a consumer. It is the *wrong* tool when you reach for it to "fix a deadlock" — a buffer that hides a deadlock for small inputs just moves the deadlock to a larger input. The discipline: choose unbuffered by default (it forces you to think about the rendezvous), and add a buffer only when you can state, in one sentence, the burst it absorbs. Citation: Effective Go's channels section at <https://go.dev/doc/effective_go#channels> and the pipelines blog at <https://go.dev/blog/pipelines>.

The fourth thing to internalize is **the "who closes" rule, because it is the single most-violated channel rule in code review: a channel is closed by its sender, never by a receiver, and never by more than one goroutine**. Closing a channel is a broadcast that says "no more values are coming." A receiver ranging over a channel (`for v := range ch`) stops when the channel is closed and drained. But sending on a closed channel *panics*, and closing an already-closed channel *panics*, so the only goroutine that can safely close is the one that owns the sending side — and only once. The corollary, which you will use all week: you never *need* to close a channel just to free it (the garbage collector handles that); you close it only to signal "done" to receivers. Citation: the Tour's range-and-close page at <https://go.dev/tour/concurrency/4> and Effective Go's channels section at <https://go.dev/doc/effective_go#channels>.

The fifth thing to internalize is that **`select` is the `switch` of concurrency: it waits on multiple channel operations and proceeds with whichever is ready first**. A `select` with several `case`s blocks until one of its channel operations can proceed; if several are ready, it picks one at random (so you cannot starve a case by ordering). Add a `default` case and the `select` becomes non-blocking — it takes the default if nothing else is ready. Add a `case <-time.After(d)` and you have a timeout. The nil-channel trick — a `case` whose channel is `nil` is never selected — lets you dynamically disable a branch. `select` is how a goroutine listens for "work arrived OR I was told to stop OR the deadline passed," and it is the heart of every cancellation pattern (which we generalise with `context` next week). Citation: the Tour's `select` page at <https://go.dev/tour/concurrency/5>.

The sixth thing to internalize is that **`sync.WaitGroup` answers exactly one question — "are all the goroutines I launched finished?" — and it has three rules you must not break**. `wg.Add(n)` says "I am about to launch n goroutines," each goroutine calls `wg.Done()` (idiomatically `defer wg.Done()`) when it finishes, and `wg.Wait()` blocks until the counter reaches zero. The rules: **call `Add` before you launch the goroutine** (calling `Add` inside the goroutine races with `Wait`), **pass the `WaitGroup` by pointer** (a copied `WaitGroup` is a different counter — `go vet` will catch this), and **`Wait` exactly once** in the coordinating goroutine. A `WaitGroup` is for completion, not for moving data; the data still flows over channels. Citation: the `sync.WaitGroup` docs at <https://pkg.go.dev/sync#WaitGroup>.

The seventh thing to internalize is that **a goroutine that blocks forever is a leak, and a leak is a bug that the compiler, `go vet`, and `staticcheck` will all happily let you ship**. A goroutine blocked on a send to a channel nobody reads, or a receive from a channel nobody sends to, never returns — its stack, its captured variables, and everything they reference stay alive for the life of the process. The runtime tells you about the *total* deadlock ("all goroutines are asleep - deadlock!") because it can prove the program can make no progress; it cannot tell you about a *partial* leak where the rest of the program runs on. You find leaks deliberately: count goroutines with `runtime.NumGoroutine()` before and after, or — the production tool — assert zero leaks in a test with `uber-go/goleak`. The fix is almost always a `done` channel (or, next week, a `context`) that unblocks the stuck goroutine. Citation: `runtime.NumGoroutine` at <https://pkg.go.dev/runtime#NumGoroutine> and `goleak` at <https://github.com/uber-go/goleak>.

The eighth thing to internalize is the slogan and its limit: **"Do not communicate by sharing memory; instead, share memory by communicating" — except when a `sync.Mutex` or a plain function call is simpler, and it often is**. The Go proverb says that instead of guarding a shared variable with a lock, you should pass the variable's *ownership* down a channel so that only one goroutine touches it at a time. That model is a beautiful fit for pipelines, fan-out/fan-in, and signalling. But it is not a law: protecting a single shared counter or a small in-memory cache with a `sync.Mutex` is simpler, faster, and easier to review than spinning up a goroutine and a channel to serialise access. The senior instinct is to know the decision matrix — channel for *transferring ownership and coordinating in time*, mutex for *guarding a small piece of shared state*, plain function call when there is no concurrency to coordinate at all. Citation: the "Share Memory By Communicating" codelab at <https://go.dev/blog/codelab-share> and Effective Go's concurrency section at <https://go.dev/doc/effective_go#concurrency>.

The ninth thing to internalize is **the pipeline as the unit of concurrent design: a generator stage produces values onto a channel, intermediate stages read from one channel and write to the next, and a sink consumes the last channel — with fan-out and fan-in as the parallelism dials**. A pipeline stage is just a function that takes an input channel and returns an output channel, closing its output when its input is drained. Fan-out is starting N goroutines that all read from the same channel (parallelising a stage); fan-in is merging N channels back into one (collecting their results). This is the shape of Lab 03's link-checker, and it is the shape of most real Go concurrency: the structure is dictated by the data flow, completion is coordinated with a `WaitGroup`, and shutdown is coordinated by closing channels in the right order from the right goroutine. Citation: the canonical "Go Concurrency Patterns: Pipelines and cancellation" blog post at <https://go.dev/blog/pipelines>.

By the end of the week you will be the person who can look at a concurrent program and answer the three questions a reviewer always asks: *who closes each channel, who waits for whom, and what happens to every goroutine when the work is done or cancelled?* You will build a fan-out/fan-in link-checker, prove it leaks nothing, and be ready for Week 4, where `context` becomes the cancellation backbone and the race detector becomes the proof of correctness.

## Learning objectives

By the end of this week, you will be able to:

- **Launch** goroutines with `go f()`, explain why they are cheaper than OS threads, and describe the GMP scheduler at a high level — and articulate why `main` returning kills every live goroutine. Cite <https://go.dev/tour/concurrency/1> and <https://go.dev/doc/effective_go#goroutines>.
- **Distinguish** unbuffered (rendezvous) from buffered (capacity-bounded) channels and state the blocking semantics of a send and a receive on each. Cite <https://go.dev/doc/effective_go#channels> and <https://go.dev/tour/concurrency/2>.
- **Apply** the "who closes" rule — the sender closes, exactly once, never the receiver — and explain why sending on or closing a closed channel panics. Cite <https://go.dev/tour/concurrency/4>.
- **Use** the `for v := range ch` pattern to drain a channel until it is closed, and the comma-ok receive (`v, ok := <-ch`) to distinguish a real value from a closed-and-drained channel. Cite <https://go.dev/doc/effective_go#channels>.
- **Write** a `select` that multiplexes channels, with a `default` for non-blocking operation, a `case <-time.After(d)` for a timeout, and the nil-channel trick to disable a branch. Cite <https://go.dev/tour/concurrency/5>.
- **Coordinate** goroutine completion with `sync.WaitGroup`, obeying the Add-before-`go` rule and passing it by pointer. Cite <https://pkg.go.dev/sync#WaitGroup>.
- **Diagnose** deadlocks (the runtime's "all goroutines are asleep - deadlock!" message) and goroutine leaks (a stuck `select` with no exit), counting goroutines with `runtime.NumGoroutine` and asserting zero leaks with `goleak`. Cite <https://pkg.go.dev/runtime#NumGoroutine> and <https://github.com/uber-go/goleak>.
- **Build** a fan-out/fan-in pipeline — a generator, N worker goroutines reading one channel, and a merge step folding their outputs back to one — and shut it down cleanly. Cite <https://go.dev/blog/pipelines>.
- **Choose** between a channel and a `sync.Mutex` for a given coordination problem, and defend the choice in review. Cite <https://go.dev/blog/codelab-share> and <https://pkg.go.dev/sync#Mutex>.

## Standards this week meets

| Bar | What this week is measured against |
| --- | --- |
| University | `CDA 4102` — Create concurrent units of execution and account for their cost, coordinate them by message passing with stated blocking semantics, multiplex several sources at one control point, and decompose work into a pipeline and a fan-out/fan-in. |
| Industry | Bound the work a service starts on somebody else's behalf: replace one-goroutine-per-item with an N-worker pool, and shut the pipeline down so nothing is left blocked on a channel no one will ever read. |
| Beyond the bar | A goroutine leak is planted on purpose, watched climbing in a live goroutine count, fixed by hand, and then asserted away in the test suite with `goleak` — a failure the compiler, `go vet` and a green happy-path suite all pass straight over — `challenges/challenge-01-prove-no-leaks.md` |

## Prerequisites

- **Weeks 1 and 2 of C30.** You can stand up a module, run the toolchain clean, write a small consumer-defined interface, return and wrap error values, and author a table-driven test. This week assumes all of that without re-teaching it.
- **The Go toolchain, 1.22 or newer.** Install from <https://go.dev/dl/>; verify with `go version`. The loop-variable scoping fix in 1.22 matters here: pre-1.22, `for _, x := range xs { go func(){ use(x) }() }` captured a shared `x` — a classic concurrency bug. On 1.22+ each iteration gets a fresh `x`. We rely on that.
- **`staticcheck`.** `go install honnef.co/go/tools/cmd/staticcheck@latest`. Its concurrency checks (a copied `WaitGroup`, a deferred `Unlock` on the wrong mutex) earn their keep this week.
- **(Optional) `goleak`.** `go get go.uber.org/goleak` inside your module when you reach the leak-detection challenge and the mini-project's leak proof. It is the only external dependency the week introduces, and it is test-only.

## Topics covered

- **Goroutines.** The `go` keyword; cheap, growable stacks; the GMP scheduler at a high level; `GOMAXPROCS`; why `go f()` returns immediately and why `main` returning kills the rest.
- **Channels.** `make(chan T)` (unbuffered) vs `make(chan T, n)` (buffered); send (`ch <- v`) and receive (`v := <-ch`) blocking semantics; channel direction types (`chan<- T` send-only, `<-chan T` receive-only) as documentation enforced by the compiler.
- **Closing channels.** `close(ch)`; the "who closes" rule (sender closes, once, never the receiver); the comma-ok receive (`v, ok := <-ch`); the panics — send on closed, close of closed, close of nil.
- **Ranging over a channel.** `for v := range ch` drains until closed; the producer/consumer shape this enables.
- **`select`.** Multiplexing multiple channel operations; random choice among ready cases; the `default` case (non-blocking); timeouts with `time.After`; the nil-channel trick to disable a branch.
- **`sync.WaitGroup`.** `Add`/`Done`/`Wait`; the Add-before-`go` rule; passing by pointer; `defer wg.Done()`.
- **Deadlocks.** The runtime's all-goroutines-asleep panic; the common causes (unbuffered send with no reader, everyone waiting on everyone).
- **Goroutine leaks.** The stuck-`select` leak; detecting it with `runtime.NumGoroutine` and `goleak`; closing the leak with a `done` channel.
- **Pipelines, fan-out, fan-in.** Generator → stage → sink; fan-out (N goroutines on one channel); fan-in (merge N channels into one); ordered shutdown.
- **Channel vs mutex.** "Share memory by communicating" and its limits; the decision matrix; `sync.Mutex` as the right answer for guarding small shared state.
- **A forward look.** `context` and `errgroup` are *next week* — this week's cancellation is a hand-rolled `done` channel, and that is on purpose, so you appreciate what `context` standardises.

## Weekly schedule

The schedule adds up to approximately **36 hours**. Treat it as a target, not a contract. Concurrency rewards drawing the goroutine/channel diagram before you write code; budget time for the whiteboard, not just the keyboard. Run `go test ./...` and, where relevant, `go test -race ./...` after every change — deep race coverage is Week 4, but the detector is cheap insurance starting now.

| Day       | Focus                                                              | Lectures | Exercises | Challenges | Quiz/Read | Homework | Mini-Project | Self-Study | Daily Total |
|-----------|-------------------------------------------------------------------|---------:|----------:|-----------:|----------:|---------:|-------------:|-----------:|------------:|
| Monday    | Goroutines, the scheduler, unbuffered/buffered channels, closing  |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Tuesday   | `select`, `WaitGroup`, deadlocks, the goroutine leak               |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Wednesday | Pipelines, fan-out/fan-in, channel-vs-mutex, the decision matrix   |    2h    |    1.5h   |     0h     |    0.5h   |   1h     |     0.5h     |    0.5h    |     6h      |
| Thursday  | Challenges — prove no leaks, bound an unbounded fan-out            |    0.5h  |    0h     |     2.5h   |    0.5h   |   1h     |     1.5h     |    0.5h    |     6.5h    |
| Friday    | Mini-project — build the concurrent link-checker                  |    0h    |    0h     |     0.5h   |    0.5h   |   0h     |     4h       |    0.5h    |     5.5h    |
| Saturday  | Mini-project polish — the leak proof, the `--workers` sweep        |    0h    |    0h     |     0h     |    0h     |   0h     |     2.5h     |    0h      |     2.5h    |
| Sunday    | Quiz, review, "when is a channel the wrong answer?"               |    0h    |    0h     |     0h     |    1h     |   0h     |     0.5h     |    0h      |     1.5h    |
| **Total** |                                                                   | **8.5h** | **7.5h**  | **5.5h**   | **4h**    | **5h**   | **9.5h**     | **3h**     | **36h**     |

## How to navigate this week

| File | What's inside |
|------|---------------|
| [README.md](./README.md) | This overview (you are here) |
| [resources.md](./resources.md) | A Tour of Go (concurrency), Effective Go, the spec, Go by Example, the Rob Pike concurrency-patterns talk, the pipelines blog, `sync`, `goleak` |
| [lecture-notes/01-goroutines-the-scheduler-and-channels.md](./lecture-notes/01-goroutines-the-scheduler-and-channels.md) | Goroutines and the GMP scheduler, unbuffered vs buffered channels, channel directions, range-over-channel, the "who closes" rule |
| [lecture-notes/02-select-waitgroup-and-the-leak.md](./lecture-notes/02-select-waitgroup-and-the-leak.md) | `select` (default, timeout, nil-channel), `sync.WaitGroup`, deadlocks, a worked goroutine-leak reproduction and its fix |
| [lecture-notes/03-pipelines-fan-out-fan-in-and-channels-vs-mutex.md](./lecture-notes/03-pipelines-fan-out-fan-in-and-channels-vs-mutex.md) | The pipeline pattern, fan-out/fan-in, "share memory by communicating" and its limits, the channel-vs-mutex decision matrix |
| [exercises/exercise-01-goroutines-and-channels.go](./exercises/exercise-01-goroutines-and-channels.go) | Launch goroutines, send/receive on unbuffered + buffered channels, range over a producer-closed channel, observe and fix a deadlock |
| [exercises/exercise-02-select-and-waitgroup.go](./exercises/exercise-02-select-and-waitgroup.go) | `select` with a timeout, `WaitGroup`-coordinated workers, and a deliberate goroutine leak you must spot |
| [exercises/exercise-03-fan-out-fan-in.go](./exercises/exercise-03-fan-out-fan-in.go) | A small fan-out/fan-in pipeline (square N numbers across W workers) with a table-test skeleton and a no-leak proof |
| [exercises/SOLUTIONS.md](./exercises/SOLUTIONS.md) | Annotated solutions for the three exercises, with the output transcripts and PREDICT answers |
| [challenges/challenge-01-prove-no-leaks.md](./challenges/challenge-01-prove-no-leaks.md) | Instrument a program, count goroutines before/after, add `goleak` in a `TestMain`, introduce a leak and fix it |
| [challenges/challenge-02-bounded-fan-out.md](./challenges/challenge-02-bounded-fan-out.md) | Convert an unbounded one-goroutine-per-item fan-out into a bounded N-worker pool and measure the difference |
| [quiz.md](./quiz.md) | 10 multiple-choice questions on goroutines, channel semantics, closing, `select`, `WaitGroup`, deadlocks, and channel-vs-mutex |
| [homework.md](./homework.md) | Six practice problems for the week |
| [mini-project/README.md](./mini-project/README.md) | Full spec for **Lab 03 — concurrent link-checker**: parse a `sitemap.xml`, fan out to N HTTP HEAD workers, fan results in, print a report, prove no leaks |

## The "no leaked goroutines" promise

C30 added one command to its clean-toolchain contract starting this week. Every concurrent artifact you ship — the exercises, the challenges, the mini-project — must satisfy all four:

```
$ go vet ./...
$ staticcheck ./...
$ go test ./...
ok      github.com/you/linkcheck       0.020s
$ go test -race ./...
ok      github.com/you/linkcheck       1.480s
```

To the Week 1 contract — `go vet`, `staticcheck`, and `go test` all clean — this week adds two concurrency-specific obligations. First, **every goroutine you start must have a guaranteed path to return.** A goroutine blocked forever on a channel is a leak, and "it works on my small input" is not a defence — you prove there is no leak, either by counting with `runtime.NumGoroutine()` around the work or by asserting zero leaks with `goleak` in a `TestMain`. Second, **`go test -race` runs clean.** Deep race-detector work is Week 4, but the detector is nearly free and it catches the data races that creep in the moment two goroutines touch the same variable without a channel or a mutex between them; run it now so the habit is automatic by the time it is graded.

The restated rule from Week 1 still holds verbatim: **a warning is a bug you have not fixed yet.** A `go vet` "copies lock value" finding on a `WaitGroup` passed by value, a `staticcheck` complaint about an unreachable `select` case, a `-race` report on a shared counter — each is the toolchain telling you about a concurrency bug that the compiler accepted. A pull request that adds concurrent code under any unaddressed finding, or that cannot demonstrate it leaks no goroutines, is by definition not ready for review.

> **Note on the toolchain.** Everything this week ships with the Go toolchain itself plus the two tools you already have from Week 1 (`gofmt`, `staticcheck`). The one new, *optional* dependency is `go.uber.org/goleak` (`go get go.uber.org/goleak`), used only in tests to assert "this code leaks no goroutines." It is the right tool for the job and worth installing — but note the deeper lesson, the same one as Week 1: the entire link-checker is built on the standard library (`net/http`, `encoding/xml`, `sync`, `time`), and reaching past it is a decision you justify, not a default.
