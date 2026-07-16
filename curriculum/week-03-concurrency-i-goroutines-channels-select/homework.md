# Week 3 — Homework

Six practice problems, roughly **45 minutes each**, reinforcing this week's concurrency primitives: goroutines, channel semantics, the "who closes" rule, `select` with timeouts, goroutine leaks, and the channel-vs-mutex decision. Do them after the lecture notes and exercises. Put your deliverables under a `homework/` directory in your week-03 working tree — one subdirectory per problem (`homework/p1`, `homework/p2`, …), each its own module (`go mod init github.com/you/hw-p1`). Every runnable answer must be clean under `go vet ./...` and `staticcheck ./...`, and the concurrent ones clean under `go test -race ./...`.

## Problem 1 — Predict the channel/deadlock output (≈45 min)

For each of the following snippets, **write down your prediction first** (the output, or "deadlock", or "leak"), then run it and explain any surprise.

```go
// (a)
ch := make(chan int)
ch <- 1
fmt.Println(<-ch)

// (b)
ch := make(chan int, 1)
ch <- 1
fmt.Println(<-ch)

// (c)
ch := make(chan int)
go func() { ch <- 1 }()
fmt.Println(<-ch)

// (d)
ch := make(chan int)
close(ch)
v, ok := <-ch
fmt.Println(v, ok)

// (e)
ch := make(chan int)
go func() { ch <- 1 }()
go func() { ch <- 2 }()
fmt.Println(<-ch) // only one receive
```

**Deliverable:** `homework/p1/answers.md` with your prediction, the actual output, and a one-sentence explanation for each — in particular, *why* (a) deadlocks but (c) does not, and what happens to the second goroutine in (e). Cite <https://go.dev/tour/concurrency/2> and <https://go.dev/doc/effective_go#channels>.

## Problem 2 — Unbuffered vs buffered semantics (≈45 min)

Write a single program that demonstrates, with print statements proving the ordering, the difference between an unbuffered and a buffered channel:

1. With an **unbuffered** channel, show that a send does not complete until a receive happens (print "before send", do the send in a goroutine, print "after send" inside the goroutine, and a "received" in `main` — the ordering proves the rendezvous).
2. With a **buffered** channel of capacity 2, show that two sends complete with no receiver present, and that a third send blocks (start the third send in a goroutine and show that the program finishes the first two before it).

**Deliverable:** `homework/p2/main.go` plus a `README.md` paragraph stating the rule in your own words: when does a send block on an unbuffered channel, and when on a buffered one? Cite <https://go.dev/doc/effective_go#channels>.

## Problem 3 — The "who closes" rule (≈45 min)

You are given this buggy program (reconstruct it):

```go
func main() {
	ch := make(chan int)
	go consume(ch)
	for i := 0; i < 3; i++ {
		ch <- i
	}
	close(ch) // (1)
}

func consume(ch chan int) {
	for v := range ch {
		fmt.Println(v)
		close(ch) // (2)  <-- BUG
	}
}
```

1. Identify the bug at `(2)` and predict the exact runtime panic it produces.
2. Fix the program so the **sender** (and only the sender) closes, exactly once, and the consumer ranges to completion.
3. Then write a *correct* program with **two** sender goroutines feeding one channel, where neither sender may close it — use the `WaitGroup` + single closer-goroutine idiom from Lecture 1 §6.1.

**Deliverable:** `homework/p3/` with the fixed single-sender program and the two-sender program, plus `answers.md` naming the panic at `(2)` and stating the rule. Cite <https://go.dev/tour/concurrency/4>.

## Problem 4 — A `select` with a timeout (≈45 min)

Write `fetchWithTimeout(d time.Duration) (string, error)` that:

1. Starts a goroutine that "fetches" (a `time.Sleep` of a random duration up to 300ms, then sends a result string on a channel).
2. Uses a `select` to return the result if it arrives within `d`, or `(", errors.New("timeout"))` if `time.After(d)` fires first.
3. **Leaks no goroutine** even when it times out — choose the buffered-channel-of-one fix or the `done`-channel fix and justify which you picked.

Add a `TestFetchWithTimeout` (table-driven: a generous timeout that succeeds, a tiny timeout that fails) and a `goleak.VerifyTestMain` (or a `runtime.NumGoroutine` check) proving no leak across many calls.

**Deliverable:** `homework/p4/` with the function, the test, and the leak proof. Cite <https://go.dev/tour/concurrency/5> and <https://pkg.go.dev/time#After>.

## Problem 5 — Spot and fix a goroutine leak (≈45 min)

You are given this function (reconstruct it):

```go
// worker pool that "forgets" to let workers exit
func ProcessForever(items []int) []int {
	jobs := make(chan int)
	results := make(chan int, len(items))

	for i := 0; i < 4; i++ {
		go func() {
			for j := range jobs { // never ends: jobs is never closed
				results <- j * j
			}
		}()
	}

	for _, it := range items {
		jobs <- it
	}
	// BUG: jobs is never closed, so the 4 worker goroutines range forever.

	out := make([]int, 0, len(items))
	for range items {
		out = append(out, <-results)
	}
	return out
}
```

1. Explain why this *returns the right answer* but still leaks four goroutines per call. (Hint: the workers are stuck on `for j := range jobs` because `jobs` is never closed.)
2. Write a `runtime.NumGoroutine()` test that calls `ProcessForever` 20 times and shows the count climbing.
3. Fix it (close `jobs` after feeding) and show the count returns to baseline. Add a `goleak.VerifyTestMain` that catches the leak before your fix and passes after.

**Deliverable:** `homework/p5/` with the leaky version, the counting test, the fix, and the `goleak` proof. Cite <https://pkg.go.dev/runtime#NumGoroutine> and <https://github.com/uber-go/goleak>.

## Problem 6 — Channel vs mutex: a decision essay (≈45 min)

For **each** of the following coordination problems, decide whether a **channel**, a **`sync.Mutex`**, or **neither** (a plain function call) is the right primitive, and justify it in 3–5 sentences using the decision matrix from Lecture 3:

1. A web crawler that processes 10,000 fetched pages in parallel and collects the extracted links.
2. A request counter incremented by every HTTP handler goroutine, read once per minute by a metrics endpoint.
3. A function that sorts a slice and returns it, called from one goroutine.
4. Telling 50 worker goroutines to stop because a deadline passed.
5. An in-memory LRU cache shared by all request handlers, with frequent reads and occasional writes.

Then implement **two** of them — one you decided is a channel and one you decided is a mutex — as small runnable programs, and write one paragraph on why swapping the primitives (mutex where you chose a channel, and vice versa) would be worse.

**Deliverable:** `homework/p6/decisions.md` (the five judgements) plus two runnable implementations (`channel_example.go`, `mutex_example.go`) in their own modules. Cite <https://go.dev/blog/codelab-share> and <https://pkg.go.dev/sync#Mutex>.

## Submission

Commit your `homework/` directory on your week-03 branch. Each problem's deliverable must run (where it is code) and be clean under `go vet`, `staticcheck`, and — for the concurrent problems (1, 4, 5) — `go test -race`. The teaching staff spot-checks two of the six per submission; the goroutine-leak proofs in Problems 4 and 5 are always checked.

Cited references: <https://go.dev/tour/concurrency/2>, <https://go.dev/tour/concurrency/4>, <https://go.dev/tour/concurrency/5>, <https://go.dev/doc/effective_go#channels>, <https://pkg.go.dev/time#After>, <https://pkg.go.dev/runtime#NumGoroutine>, <https://github.com/uber-go/goleak>, <https://go.dev/blog/codelab-share>, <https://pkg.go.dev/sync#Mutex>.
