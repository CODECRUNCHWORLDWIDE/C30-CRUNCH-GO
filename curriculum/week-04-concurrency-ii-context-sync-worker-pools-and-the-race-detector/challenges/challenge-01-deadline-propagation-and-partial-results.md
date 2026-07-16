# Challenge 1 — Deadline Propagation and Partial Results

> **Time:** 90 minutes. **Prerequisites:** Exercises 1 and 2. **Citations:** the `context` deadline docs at <https://pkg.go.dev/context#WithDeadline>, the "Pipelines and cancellation" post at <https://go.dev/blog/pipelines>, and the `errgroup` docs at <https://pkg.go.dev/golang.org/x/sync/errgroup>.

## The premise

A fan-out that returns "everything or an error" is the easy case. The hard, real-world case is: *you have a deadline, some workers finish before it, some do not, and you want the partial results plus an honest accounting of what did not complete.* A dashboard that queries six backends with a 200ms budget should render the four that answered and show "timed out" for the two that did not — not fail the whole page because one backend was slow.

You will build a `GatherWithin` function that runs N tasks under a deadline, collects every result that completed in time, and reports the rest as cancelled — without leaking a single goroutine.

## What you will build

```
src/gather/
  gather.go
  gather_test.go
```

```go
package gather

import "context"

type Outcome[T any] struct {
	Index int
	Value T
	Err   error // nil on success; context.DeadlineExceeded if it ran out of time
}

// GatherWithin runs each task with the deadline already on ctx. It returns one
// Outcome per task: a Value for tasks that finished in time, and an Err of
// context.DeadlineExceeded for tasks the deadline cut off. It must NOT leak
// goroutines: every started task either completes or observes cancellation and
// returns.
func GatherWithin[T any](
	ctx context.Context,
	tasks []func(context.Context) (T, error),
	limit int,
) []Outcome[T]
```

## Requirements

1. **The deadline lives on `ctx`.** The caller derives it with `context.WithTimeout` (or `WithDeadline`) before calling `GatherWithin`. Your function threads that `ctx` into every task.
2. **Tasks that finish in time get their value.** Tasks still running when the deadline fires get `Err: context.DeadlineExceeded`. Use `errors.Is` to classify, since a task may wrap the sentinel.
3. **Bound concurrency to `limit`.** Use `errgroup.SetLimit` or a semaphore channel.
4. **No goroutine leaks.** When the deadline fires, in-flight tasks must observe `ctx.Done()` and return promptly. Prove it: the test below starts a `goleak`-style check (or counts goroutines before/after).
5. **Deterministic result ordering.** `Outcome[i].Index == i`; write each result to a pre-sized slice at its own index (race-free, as Lecture 1 explained), do not append from goroutines.

## The test harness

```go
package gather

import (
	"context"
	"errors"
	"runtime"
	"testing"
	"time"
)

func TestPartialResults(t *testing.T) {
	tasks := make([]func(context.Context) (int, error), 6)
	for i := range tasks {
		i := i
		// Tasks 0-3 are fast (20ms); tasks 4-5 are slow (500ms).
		dur := 20 * time.Millisecond
		if i >= 4 {
			dur = 500 * time.Millisecond
		}
		tasks[i] = func(ctx context.Context) (int, error) {
			select {
			case <-time.After(dur):
				return i * 10, nil
			case <-ctx.Done():
				return 0, ctx.Err()
			}
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	before := runtime.NumGoroutine()
	out := GatherWithin(ctx, tasks, 8)

	// Four finished, two timed out.
	finished, timedOut := 0, 0
	for _, o := range out {
		switch {
		case o.Err == nil:
			finished++
		case errors.Is(o.Err, context.DeadlineExceeded):
			timedOut++
		default:
			t.Fatalf("task %d: unexpected error %v", o.Index, o.Err)
		}
	}
	if finished != 4 || timedOut != 2 {
		t.Errorf("got finished=%d timedOut=%d, want 4 and 2", finished, timedOut)
	}

	// Give stragglers a beat to unwind, then check for leaks.
	time.Sleep(50 * time.Millisecond)
	if after := runtime.NumGoroutine(); after > before {
		t.Errorf("goroutine leak: before=%d after=%d", before, after)
	}
}
```

## Acceptance criteria

1. `go test -race ./...` is green.
2. With four fast and two slow tasks under a 200ms deadline, exactly four `Outcome`s have `Err == nil` and exactly two have `context.DeadlineExceeded`.
3. The goroutine count after `GatherWithin` returns (plus a settle delay) is not higher than before — no leaks.
4. `Outcome[i].Index == i` for every `i`; results are written by index, never appended.
5. `go vet ./...` is clean (no lost cancel, no copied locks).

## Stretch goals

1. **A grace period for stragglers.** After the hard deadline, give in-flight tasks an extra 50ms to finish (a "soft" then "hard" deadline) before recording them as cancelled. Implement with a second derived context and document the difference between the soft and hard deadlines.
2. **Return the first hard error eagerly.** Add a variant `GatherOrFail` where a task returning a *non-deadline* error cancels the siblings (the standard `errgroup` first-error behaviour) — and contrast it, in a paragraph, with `GatherWithin`'s collect-everything behaviour. When does a dashboard want each?
3. **Deadline budgeting across a chain.** Wrap each task so it gets *half* the remaining budget at the moment it starts (read `ctx.Deadline()` and derive a tighter child). Explain when per-task budgeting beats a single shared deadline.

Cited references: <https://pkg.go.dev/context#WithDeadline>, <https://pkg.go.dev/context#WithTimeout>, <https://go.dev/blog/pipelines>, <https://pkg.go.dev/golang.org/x/sync/errgroup>.
