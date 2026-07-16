# Week 4 — Homework

Six practice problems that consolidate the week's material. They are sized to ~45 minutes each. Do them after the lectures and the exercises; do them before the mini-project. Cite the URLs you used while solving each one in the commit message of your homework branch.

## Problem 1 — The cancellation tree, drawn

Take a program with a root context, a `WithTimeout(root, 5s)` child, and two `WithCancel` grandchildren derived from the child. Draw the tree. Then answer, in a 200-word write-up:

1. If you call the *child's* `cancel()`, which contexts fire `Done()` and which do not? (Hint: cancellation flows down, never up.)
2. If the root's 5-second deadline fires, what happens to the grandchildren?
3. If a grandchild derives a `WithTimeout(grandchild, 30s)`, when does *it* actually fire — at 30s or sooner? Why?
4. Where does each `defer cancel()` go, and what does `go vet`'s `lostcancel` check catch if you omit one?

Cite: <https://pkg.go.dev/context> and <https://go.dev/blog/context>.

Deliverable: `homework/01-cancellation-tree.md` with the drawn tree (ASCII is fine) and the four answers.

## Problem 2 — Channel or mutex? The decision, six times

For each of the following six scenarios, declare "channel" or "mutex" (or "atomic") and justify it in one or two sentences using the "transfer ownership / signal an event → channel; protect shared state → mutex; single scalar → atomic" rule.

- **A:** Hand each parsed log line from a reader goroutine to a pool of processor goroutines.
- **B:** A request counter incremented by every HTTP handler and read by a `/metrics` endpoint.
- **C:** An in-memory cache (`map[string][]byte`) read by many handlers and written by a background refresh.
- **D:** Signal a set of worker goroutines that a shutdown has begun.
- **E:** A config struct loaded once at startup and read on every request, replaced wholesale on SIGHUP.
- **F:** Accumulate per-worker partial sums into a final total at the end of a fan-out.

Cite Bryan Mills' "Rethinking Classical Concurrency Patterns" and the `sync` package overview at <https://pkg.go.dev/sync>.

Deliverable: `homework/02-channel-or-mutex.md` with six declarations and justifications.

## Problem 3 — Read a race report you did not write

Here is a real ThreadSanitizer report (lightly anonymised). Annotate it:

```
WARNING: DATA RACE
Write at 0x00c000128048 by goroutine 21:
  main.(*Cache).Set()
      /app/cache.go:41 +0x64
  main.refresh()
      /app/refresh.go:18 +0x2c

Previous read at 0x00c000128048 by goroutine 7:
  main.(*Cache).Get()
      /app/cache.go:33 +0x44
  net/http.HandlerFunc.ServeHTTP()
      /usr/local/go/src/net/http/server.go:2166 +0x44

Goroutine 21 (running) created at:
  main.main()
      /app/main.go:52 +0x120
Goroutine 7 (running) created at:
  net/http.(*Server).Serve()
      ...
```

Answer:

1. What two operations race, and on which line of which file is each?
2. Which goroutine is the writer and which is the reader? What started each (the "created at" lines)?
3. What is the shared state (hint: a `Cache` field)?
4. What is the fix? Write the corrected `Get` and `Set` with the right primitive, and justify the primitive choice.
5. Would this race ever be caught *without* the detector — say by a passing unit test? Explain why a green test suite is not evidence of race-freedom.

Cite: <https://go.dev/doc/articles/race_detector> and <https://go.dev/ref/mem>.

Deliverable: `homework/03-read-a-race.md`.

## Problem 4 — Atomic vs mutex, measured on your machine

Write two benchmarks: a counter guarded by a `sync.Mutex` and the same counter as a `sync/atomic.Int64`, both under `b.RunParallel`. Run with `go test -bench Counter -benchmem -count=5` and feed the output to `benchstat`. Report:

1. The mean ns/op for each, with the `benchstat` variance.
2. The ratio (how many times faster is the atomic).
3. `allocs/op` for each (should be 0; if not, find out why).
4. At what point would you *not* use the atomic — give a concrete example where the shared state is a group of fields and a mutex is the only correct choice.

Then write 150 words on the rule "one scalar → atomic; a group that changes together → mutex," using your numbers to argue *why* the atomic is not always the answer despite being faster.

Cite: <https://pkg.go.dev/sync/atomic>, <https://pkg.go.dev/testing#hdr-Benchmarks>, and the `benchstat` docs at <https://pkg.go.dev/golang.org/x/perf/cmd/benchstat>.

Deliverable: `homework/04-atomic-vs-mutex.md` with the numbers and the analysis.

## Problem 5 — Graceful Ctrl-C, end to end

Build a small CLI that starts a bounded `errgroup` pool processing 50 simulated tasks (each a 100ms cancellable sleep), wired to `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`. Then:

1. Run it to completion and confirm all 50 finish.
2. Run it and press Ctrl-C after ~1 second. Report how many tasks completed, how many returned `context.Canceled`, and the wall-clock time from Ctrl-C to clean exit.
3. Add a `--timeout 2s` flag (a `WithTimeout` derived from the signal context) and report the same numbers when the deadline fires instead of Ctrl-C. Confirm the error is `context.DeadlineExceeded`, not `Canceled`.
4. Explain, in a paragraph, why this exact pattern is the foundation of the graceful HTTP-server shutdown you will build in Week 11.

Cite: <https://pkg.go.dev/os/signal#NotifyContext> and <https://pkg.go.dev/golang.org/x/sync/errgroup>.

Deliverable: `homework/05-graceful-ctrlc.md` with the code and the three timing reports.

## Problem 6 — The memory model, at interview depth

Answer the following five short questions, each with a one-paragraph answer and a citation to the Go memory model spec:

1. Define "happens-before" in your own words. Why does the memory model only *guarantee* visibility across goroutines when a happens-before edge exists?
2. Name three synchronisation operations that establish a happens-before edge, and state the edge each one creates (e.g. "a channel send happens-before the corresponding receive completes").
3. Two goroutines both only *read* a shared variable, never write it. Is that a data race? Why or why not?
4. A goroutine sets a `bool done = true` and another goroutine spins `for !done {}`. Is this correct? What can go wrong, and what is the minimal fix?
5. Explain why a data race is *undefined behaviour* and not merely "you might read a stale value." Give one concrete way a racy program can do something that looks impossible.

Cite the Go Memory Model at <https://go.dev/ref/mem> for every answer.

Deliverable: `homework/06-memory-model.md` with the five answers.

## Submission

Push the six deliverables on a branch named `week04-homework/<your-handle>` and open a PR against the C30 curriculum repository. The PR description should link to each of the six files and include a 100-word summary of what you learned.

The teaching staff reviews homework PRs within 5 business days. Reviews focus on whether you have read the citations and whether your reasoning holds together, not on perfect grammar. The single most common review comment is "where is your citation for this claim" — preempt it by linking the Go package doc, the blog post, or the memory model spec for every non-trivial assertion.

Cited references this homework draws from: <https://pkg.go.dev/context>, <https://go.dev/blog/context>, <https://pkg.go.dev/sync>, <https://pkg.go.dev/sync/atomic>, <https://pkg.go.dev/os/signal#NotifyContext>, <https://pkg.go.dev/golang.org/x/sync/errgroup>, <https://go.dev/ref/mem>, <https://go.dev/doc/articles/race_detector>, <https://pkg.go.dev/testing#hdr-Benchmarks>.
