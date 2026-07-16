// Exercise 1 — Context and Cancellation
//
// GOAL
//   Thread a context.Context through a worker and observe the three ways a
//   context ends: a manual cancel(), a deadline (WithTimeout), and an OS
//   signal (signal.NotifyContext). Prove to yourself which sentinel error each
//   path produces — context.Canceled vs context.DeadlineExceeded — and why.
//
// HOW TO RUN
//   Put this file in a module:
//     mkdir ex01 && cd ex01 && go mod init ex01
//     # save this file as main.go
//     go run .                 # runs all three scenarios
//     go run . signal          # runs the signal scenario; press Ctrl-C
//     go vet ./...             # MUST be clean (watch for the lostcancel check)
//
// TASKS
//   1. Implement worker() so it returns ctx.Err() the moment the context is
//      cancelled, and a real result otherwise. It already selects on
//      time.After and ctx.Done(); read why both cases matter.
//   2. In scenarioManualCancel, call cancel() after 100ms and confirm the
//      worker returns context.Canceled (use errors.Is to classify it).
//   3. In scenarioTimeout, derive a 100ms WithTimeout and confirm the worker
//      (which "works" for 500ms) returns context.DeadlineExceeded.
//   4. In scenarioSignal, wire signal.NotifyContext and confirm pressing
//      Ctrl-C cancels the worker with context.Canceled.
//   5. Make sure EVERY derived context has a matching `defer cancel()` (or
//      `defer stop()` for the signal context). Run `go vet` and confirm the
//      lostcancel analyzer says nothing.
//
// ACCEPTANCE
//   You can state, without running it, which sentinel each scenario prints and
//   why: manual cancel -> Canceled; timeout -> DeadlineExceeded; Ctrl-C ->
//   Canceled (because NotifyContext cancels, it does not set a deadline).

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// worker simulates a unit of work that takes `dur` to complete but is
// cancellable: if ctx ends first, it returns ctx.Err().
func worker(ctx context.Context, dur time.Duration) (string, error) {
	select {
	case <-time.After(dur):
		return "work complete", nil
	case <-ctx.Done():
		// ctx.Err() is context.Canceled or context.DeadlineExceeded.
		return "", ctx.Err()
	}
}

func classify(err error) string {
	switch {
	case err == nil:
		return "no error"
	case errors.Is(err, context.DeadlineExceeded):
		return "context.DeadlineExceeded (we ran out of time)"
	case errors.Is(err, context.Canceled):
		return "context.Canceled (someone gave up on us)"
	default:
		return fmt.Sprintf("other: %v", err)
	}
}

func scenarioManualCancel() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel from another goroutine after 100ms; the worker "needs" 500ms.
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := worker(ctx, 500*time.Millisecond)
	fmt.Printf("manual cancel -> %s\n", classify(err))
}

func scenarioTimeout() {
	// Budget 100ms; the worker "needs" 500ms, so the deadline fires first.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel() // release the timer even though the deadline will fire

	_, err := worker(ctx, 500*time.Millisecond)
	fmt.Printf("timeout       -> %s\n", classify(err))
}

func scenarioSignal() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Println("signal        -> waiting; press Ctrl-C to cancel...")
	// Give the worker a long duration so the signal is what ends it.
	_, err := worker(ctx, 1*time.Hour)
	fmt.Printf("signal        -> %s\n", classify(err))
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "signal" {
		scenarioSignal()
		return
	}
	scenarioManualCancel()
	scenarioTimeout()
	fmt.Println("signal        -> run `go run . signal` and press Ctrl-C")
}
