# Lecture 1 — `context` for Cancellation and Deadlines, and the Bounded Worker Pool

> **Time:** 2 hours. Take the `context` material in one sitting and the worker-pool material in a second sitting. **Prerequisites:** Week 3 (channels, `select`, the "who closes" rule) and Week 2 (`errors.Is`, wrapped errors). **Citations:** the `context` package docs at <https://pkg.go.dev/context>, the Go blog "Go Concurrency Patterns: Context" at <https://go.dev/blog/context>, the "Pipelines and cancellation" post at <https://go.dev/blog/pipelines>, and the `errgroup` docs at <https://pkg.go.dev/golang.org/x/sync/errgroup>.

## 1. Why `context`, and why this lecture first

Every goroutine you start in a real service is started *on behalf of something* — an HTTP request, a CLI invocation, a cron tick — and that something can end before the goroutine does. The request times out. The client hangs up. The operator presses Ctrl-C. When that happens, the work the goroutine is doing is no longer wanted, and continuing it is pure waste: a database query whose result nobody will read, an HTTP call to a service that will get a response nobody is waiting for, CPU spent computing an answer that will be discarded. Worse, a goroutine with no way to be told "stop" is a *leak*: it holds memory, file descriptors, and a socket until the process dies.

Week 3 gave you the channel as a way for goroutines to *talk*. This week gives you `context.Context` as the way for a caller to tell every goroutine below it *stop*. The two are the same mechanism underneath — a `Context`'s cancellation signal is a channel — but `context` standardises the pattern so that every package in the Go ecosystem, from `net/http` to `database/sql` to gRPC, speaks the same cancellation language. When you write `db.QueryContext(ctx, ...)` and the `ctx` is cancelled, the query is cancelled, all the way down to the wire. That only works because everyone agreed on `context`.

This lecture is first because **everything else in the week, and most of the rest of the track, threads a `Context`**. The worker pool below takes a `Context`. The HTTP middleware in Week 5 reads request-scoped values off a `Context`. The graceful shutdown in Week 11 is a `Context` cancelled on `SIGTERM`. We start at the cancellation signal and build the pool on top of it.

## 2. The shape of a `Context`

`context.Context` is a four-method interface:

```go
type Context interface {
	Deadline() (deadline time.Time, ok bool)
	Done() <-chan struct{}
	Err() error
	Value(key any) any
}
```

Read each method in terms of what a *worker* does with it:

1. **`Done() <-chan struct{}`** returns a channel that is closed when the context is cancelled. A worker selects on it: `case <-ctx.Done():` fires the moment cancellation happens. A closed channel returns the zero value immediately and forever, which is why every blocked `<-ctx.Done()` unblocks at once when `cancel()` is called.
2. **`Err() error`** returns `nil` while the context is live, and after cancellation returns one of two sentinels: `context.Canceled` (someone called `cancel()`) or `context.DeadlineExceeded` (a timeout or deadline passed). You compare with `errors.Is(err, context.DeadlineExceeded)`.
3. **`Deadline() (time.Time, bool)`** tells you whether a deadline is set and what it is. A worker can use it to decide whether a unit of work is even worth starting.
4. **`Value(key any) any`** retrieves a request-scoped value stored with `context.WithValue`. Use it sparingly — Section 9.

You never implement this interface yourself. You get a `Context` from `context.Background()` (the root) and derive children from it. Citation: <https://pkg.go.dev/context#Context>.

## 3. The four constructors

```go
ctx := context.Background()                       // the root: never cancelled, no deadline
ctx := context.TODO()                             // a placeholder you have not wired yet

ctx, cancel := context.WithCancel(parent)         // cancel() cancels it manually
ctx, cancel := context.WithTimeout(parent, 5*time.Second)   // cancels after 5s, or on cancel()
ctx, cancel := context.WithDeadline(parent, t)    // cancels at time t, or on cancel()
```

Five rules govern their use:

1. **`context.Background()` is the root of every tree.** Use it in `main`, in tests, and at the top of a request handler that does not already have one. `context.TODO()` is identical at runtime but signals "I have not figured out where the real context comes from yet" — `go vet` and linters treat it as a marker.
2. **Every `WithCancel` / `WithTimeout` / `WithDeadline` returns a `cancel` function, and you must call it.** The idiom is `defer cancel()` on the very next line. Calling `cancel()` releases the context's resources (a timer, the child entry in the parent's tree). Forgetting it leaks those resources until the parent is cancelled. `go vet`'s `lostcancel` analyzer flags the omission; treat its warning as an error.
3. **Cancellation propagates down, never up.** Cancelling a child does not cancel its parent. Cancelling a parent cancels every descendant. The tree is the mental model: a request's root context has children for each goroutine it spawns, and cancelling the root unwinds all of them.
4. **Deadlines are inherited as the *earliest* of any ancestor's.** If a parent has a 5-second deadline and you derive a child with a 30-second timeout, the child still fires at 5 seconds, because the parent's `Done()` closes first. You cannot extend a deadline by deriving a longer one — you can only shorten it.
5. **`context` is a request-scoped value, not a struct field.** Pass it as the first parameter of a function — `func Do(ctx context.Context, ...)` — never store it on a struct. Storing it ties the struct's lifetime to one request, which is a bug.

Citation: <https://go.dev/blog/context> and the package overview at <https://pkg.go.dev/context>.

## 4. A worker that respects cancellation

Here is the canonical shape: a worker that does a unit of slow work, but bails the instant its context is cancelled.

```go
package work

import (
	"context"
	"fmt"
	"time"
)

// process does some slow, cancellable work for one item. It returns
// ctx.Err() if the context is cancelled before the work finishes.
func process(ctx context.Context, item string) (string, error) {
	// Simulate work that takes 200ms but is cancellable.
	select {
	case <-time.After(200 * time.Millisecond):
		return fmt.Sprintf("processed %s", item), nil
	case <-ctx.Done():
		// ctx.Err() is context.Canceled or context.DeadlineExceeded.
		return "", ctx.Err()
	}
}
```

Two things are load-bearing:

1. **The `select` watches both the work and `ctx.Done()`.** Whichever fires first wins. If the context is cancelled while we wait on `time.After`, the `<-ctx.Done()` case fires and we return `ctx.Err()` instead of the result. This is the entire pattern: *a cancellable operation selects on its work and on `ctx.Done()`.*
2. **We return `ctx.Err()`, not a hand-rolled error.** The caller can then `errors.Is(err, context.DeadlineExceeded)` to distinguish a timeout from a manual cancel. Returning the sentinel preserves that information.

For work that is *not* a single blocking operation but a loop, you poll instead of select:

```go
func crunch(ctx context.Context, items []int) (int, error) {
	sum := 0
	for i, n := range items {
		// Check for cancellation every iteration (or every N iterations for
		// a tight loop, to amortise the cost of the check).
		if i%1024 == 0 {
			if err := ctx.Err(); err != nil {
				return 0, err
			}
		}
		sum += expensive(n)
	}
	return sum, nil
}
```

The crucial point, restated: **cancellation is cooperative.** Calling `cancel()` closes the `Done()` channel; it does not reach into the goroutine and stop it. If `crunch` never checked `ctx.Err()`, it would run to completion no matter how many times the caller cancelled. The Go runtime has no `Thread.interrupt()` and no `kill -9` for a single goroutine — by design. A goroutine stops because it *chooses* to, by watching its context. Citation: <https://pkg.go.dev/context#pkg-overview>.

## 5. Deadlines and timeouts in practice

A timeout is the most common context derivation in a service. Here is a function that calls a slow dependency with a budget:

```go
func fetchWithBudget(parent context.Context, url string) (string, error) {
	// Derive a 2-second budget from the parent. If the parent is already
	// cancelled or has a tighter deadline, that wins.
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	defer cancel() // release the timer even on the success path

	result, err := process(ctx, url)
	if err != nil {
		// Distinguish the two cancellation reasons for the caller / logs.
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return "", fmt.Errorf("fetch %s timed out after 2s: %w", url, err)
		case errors.Is(err, context.Canceled):
			return "", fmt.Errorf("fetch %s cancelled by caller: %w", url, err)
		default:
			return "", fmt.Errorf("fetch %s failed: %w", url, err)
		}
	}
	return result, nil
}
```

Three observations:

1. **`defer cancel()` runs on every return path**, including the success path. Even when the work finished well under budget, calling `cancel()` stops the timer immediately rather than letting it fire 2 seconds later into a context nobody is watching. This is the resource-release the `lostcancel` analyzer cares about.
2. **The two sentinels carry different meaning.** `DeadlineExceeded` means "we ran out of time" — usually a retry-or-degrade decision. `Canceled` means "the caller above us gave up" — usually nothing to retry, the whole operation is abandoned. Logging them differently is the difference between a useful and a useless incident trail.
3. **We wrap the sentinel with `%w`** so the caller above us can *still* `errors.Is` it. The wrapping adds context (which URL, what budget) without erasing the machine-readable cause. This is Week 2's error-wrapping contract applied to cancellation.

Citation: <https://pkg.go.dev/context#WithTimeout> and the `errors` semantics from Week 2.

## 6. Graceful Ctrl-C with `signal.NotifyContext`

The link between a long-running CLI and `context` cancellation is `signal.NotifyContext`. It returns a context that is cancelled the first time the process receives one of the named signals:

```go
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// ctx is cancelled when the user presses Ctrl-C (SIGINT) or the
	// process receives SIGTERM (the polite shutdown signal a container
	// runtime sends). stop releases the signal handler.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx); err != nil {
		log.Fatalf("run: %v", err)
	}
}
```

`run(ctx)` threads that context into everything below it. When the user presses Ctrl-C, `ctx.Done()` closes, every worker's `select` fires, in-flight work either finishes or returns `context.Canceled`, and the program exits cleanly instead of being killed mid-write. A *second* Ctrl-C, after `stop()` has restored the default handler, terminates the process hard — which is the right escape hatch if a worker is wedged.

This is the seed of the entire graceful-shutdown story C30 builds toward: **shutdown is cancellation applied to the top of the call tree.** Week 11's Kubernetes graceful shutdown is exactly this pattern with `SIGTERM` and an HTTP server's `Shutdown(ctx)`. Citation: <https://pkg.go.dev/os/signal#NotifyContext>.

## 7. The bounded worker pool with `errgroup`

Now the centrepiece. We have N units of work and we want to run them concurrently — but *at most* `limit` at a time, so we do not open ten thousand sockets or melt the downstream. We want the first error to stop the rest, and we want a single error returned at the end. `golang.org/x/sync/errgroup` does exactly this.

```go
package linkcheck

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"
)

type Result struct {
	URL    string
	Status int
	Err    error
}

// CheckAll checks every URL concurrently, capped at `limit` in flight at
// once. It returns one result per URL. A per-URL failure is recorded in the
// Result; it does not abort the whole run (we want a full report).
func CheckAll(ctx context.Context, client *http.Client, urls []string, limit int) ([]Result, error) {
	results := make([]Result, len(urls))

	// errgroup.WithContext derives a child context that is cancelled the
	// moment any g.Go func returns a non-nil error, or when `ctx` is cancelled.
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit) // at most `limit` goroutines run concurrently

	for i, url := range urls {
		i, url := i, url // capture loop vars (unnecessary on Go 1.22+, harmless)
		g.Go(func() error {
			status, err := check(ctx, client, url)
			results[i] = Result{URL: url, Status: status, Err: err}
			// Return nil so a single bad URL does not cancel the whole run.
			// (If you DID want the first hard failure to stop everything,
			// you would return err here.)
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return results, fmt.Errorf("link check: %w", err)
	}
	return results, nil
}

func check(ctx context.Context, client *http.Client, url string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	return resp.StatusCode, nil
}
```

Six pieces are worth naming:

1. **`errgroup.WithContext(ctx)`** returns a `*Group` and a *derived* context. That derived context is cancelled when (a) any `g.Go` func returns a non-nil error, or (b) the parent `ctx` is cancelled. Workers thread the derived `ctx` into their I/O, so a failure or a Ctrl-C stops the in-flight requests.
2. **`g.SetLimit(limit)`** caps the number of goroutines running concurrently. `g.Go` blocks until a slot is free if `limit` are already running. This is the bound: no matter how many URLs, at most `limit` HTTP requests are in flight.
3. **`g.Go(func() error { ... })`** starts a unit of work. The func's return value is the error; the *first* non-nil one wins and is what `g.Wait` returns. Returning `nil` here means a single bad URL is recorded but does not abort the run — the design choice for a *report* tool.
4. **Writing `results[i]` from each goroutine is race-free** because each goroutine writes a *distinct* index `i`, and `g.Wait()` establishes a happens-before edge between every goroutine's write and the `return results` that follows. No mutex needed. (We will prove this is race-free under `-race` in Lecture 3.)
5. **`http.NewRequestWithContext(ctx, ...)`** is how cancellation reaches the network. When `ctx` is cancelled, the in-flight HTTP request is aborted and `client.Do` returns promptly with a context error. A request built with `http.NewRequest` (no context) ignores cancellation — never use it in a service.
6. **`g.Wait()`** blocks until every started goroutine has returned, then returns the first error. It is the join point; after it, all the goroutines are done and `results` is safe to read.

Citation: <https://pkg.go.dev/golang.org/x/sync/errgroup> and `http.NewRequestWithContext` at <https://pkg.go.dev/net/http#NewRequestWithContext>.

## 8. The semaphore-channel variant

`errgroup` is the production answer, but you should be able to build the bound by hand, because the pattern shows up where `errgroup` does not fit. The tool is a *buffered channel used as a counting semaphore*:

```go
func CheckAllSem(ctx context.Context, client *http.Client, urls []string, limit int) []Result {
	results := make([]Result, len(urls))
	sem := make(chan struct{}, limit) // capacity = max concurrency
	var wg sync.WaitGroup

	for i, url := range urls {
		i, url := i, url
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Acquire a slot. Blocks if `limit` tokens are already taken.
			// Respect cancellation while waiting for a slot.
			select {
			case sem <- struct{}{}:
			case <-ctx.Done():
				results[i] = Result{URL: url, Err: ctx.Err()}
				return
			}
			defer func() { <-sem }() // release the slot

			status, err := check(ctx, client, url)
			results[i] = Result{URL: url, Status: status, Err: err}
		}()
	}

	wg.Wait()
	return results
}
```

Three points:

1. **`sem := make(chan struct{}, limit)`** is a channel whose *buffer capacity* is the concurrency cap. Sending a `struct{}{}` is "acquire a token"; receiving is "release." When the buffer is full, the send blocks — which is exactly the bound. `struct{}` is the zero-byte type; the channel carries no data, only the count.
2. **The acquire `select` also watches `ctx.Done()`**, so a goroutine waiting for a slot during a cancellation does not block forever. Without that case, cancelling while all slots are busy and the queue is deep would leave goroutines stuck on the send.
3. **`defer func() { <-sem }()`** releases the slot on every exit path, including a panic. The `WaitGroup` joins the goroutines exactly as in Week 3.

When to prefer which: **`errgroup` when you want error collection and sibling-cancellation for free; the raw semaphore when you need finer control** — for example, a dynamic limit that changes mid-run, or releasing the slot at a different point than function return. Citation: the Go blog "Pipelines and cancellation" at <https://go.dev/blog/pipelines> walks the bounded-fan-out pattern in depth.

## 9. Request-scoped values — `context.WithValue`, used sparingly

A `Context` can carry request-scoped *metadata* — a request ID, an authenticated user, a trace span — down the call tree without threading it through every function signature:

```go
type ctxKey int // unexported type prevents collisions with other packages

const requestIDKey ctxKey = 0

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}

func RequestID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(requestIDKey).(string)
	return id, ok
}
```

Two rules keep this from becoming an anti-pattern:

1. **The key must be an unexported custom type, not a string.** Two packages that both store a value under the string `"id"` would collide. An unexported `ctxKey` type is unique to your package and cannot be forged from outside it. Citation: <https://pkg.go.dev/context#WithValue> — the docs spell out this convention explicitly.
2. **Values are for request-scoped metadata, not for passing function parameters.** If a function *needs* a value to do its job, that value belongs in its parameter list where the compiler can check it — not hidden in a context where a missing value is a runtime `nil`. The litmus test: a request ID is metadata (every layer might want it for logging, none *requires* it); a database handle is a dependency (pass it explicitly). The Go team's guidance: "Use context Values only for request-scoped data that transits processes and APIs, not for passing optional parameters to functions."

We use `context.WithValue` for exactly one thing in C30: the request ID that Week 5's logging middleware injects and every log line reads. That is the canonical use.

## 10. Exercise pointer

Now do **Exercise 1 — Context and Cancellation**. Thread a context through a worker, cancel it three ways — manual `cancel()`, a `WithTimeout`, and a simulated Ctrl-C via `signal.NotifyContext` — and verify you get `context.Canceled` from the first and `context.DeadlineExceeded` from the second. The acceptance criterion is that you can explain, without looking, which sentinel each path produces and why.

## 11. Summary

- `context.Context` carries a cancellation signal (`Done()`), an optional deadline, an error (`Err()`), and request-scoped values.
- Derive children with `WithCancel` / `WithTimeout` / `WithDeadline`; **always `defer cancel()`**, even on success. `go vet`'s `lostcancel` enforces it.
- Cancellation is **cooperative**: `cancel()` closes a channel; a goroutine stops only if it watches `ctx.Done()` or polls `ctx.Err()`.
- `ctx.Err()` returns `context.Canceled` (manual) or `context.DeadlineExceeded` (timeout); distinguish them with `errors.Is`.
- `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)` turns Ctrl-C into context cancellation — the seed of graceful shutdown.
- A bounded worker pool runs at most N goroutines: `errgroup.WithContext` + `g.SetLimit(n)` for error-collection and sibling-cancellation, or a buffered `chan struct{}` semaphore by hand.
- Thread `ctx` into I/O via `http.NewRequestWithContext`, `db.QueryContext`, etc., so cancellation reaches the wire.
- Use `context.WithValue` only for request-scoped metadata, with an unexported key type — never for passing required parameters.

Cited references this lecture pulled from: <https://pkg.go.dev/context>, <https://go.dev/blog/context>, <https://go.dev/blog/pipelines>, <https://pkg.go.dev/golang.org/x/sync/errgroup>, <https://pkg.go.dev/os/signal#NotifyContext>, <https://pkg.go.dev/net/http#NewRequestWithContext>.
