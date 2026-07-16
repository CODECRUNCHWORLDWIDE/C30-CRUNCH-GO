// Exercise 2 — select, WaitGroup, and a Goroutine Leak
//
// GOAL
// ----
// Use select with a time.After timeout; coordinate workers with a WaitGroup
// (Add-before-go, by pointer); then find and fix a DELIBERATE goroutine leak
// by watching runtime.NumGoroutine() climb.
//
// This is a runnable program. Put it in its own module:
//
//   mkdir ex02 && cd ex02
//   go mod init github.com/you/ex02
//   # save this file as main.go
//   go run .
//   go vet ./...        # must print nothing
//   staticcheck ./...   # must print nothing
//
// Work the PREDICT comments before running. Then do the TODO in part 3 to fix
// the leak, and confirm the goroutine count stops climbing. Answers are in
// SOLUTIONS.md.
//
// ACCEPTANCE
// ----------
// You can point at the EXACT line where a goroutine blocks forever in the leaky
// version, predict roughly what runtime.NumGoroutine() reports before your fix,
// and explain why your fix lets that goroutine return.

package main

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
	"time"
)

func main() {
	selectWithTimeout()
	waitGroupWorkers()
	leakDemo()
}

// --- 1. select with a timeout ------------------------------------------------

// fetch sends a value after `delay`. Buffered by 1 so its send never blocks,
// even if nobody is listening (the no-leak idiom — see part 3).
func fetch(delay time.Duration) <-chan int {
	out := make(chan int, 1)
	go func() {
		time.Sleep(delay)
		out <- 42
	}()
	return out
}

func selectWithTimeout() {
	fmt.Println("--- 1. select + timeout ---")

	// This one beats the timeout.
	select {
	case v := <-fetch(50 * time.Millisecond):
		fmt.Println("fast: got", v)
	case <-time.After(200 * time.Millisecond):
		fmt.Println("fast: timed out")
	}

	// This one loses to the timeout.
	// PREDICT 1: which case wins here?
	select {
	case v := <-fetch(500 * time.Millisecond):
		fmt.Println("slow: got", v)
	case <-time.After(200 * time.Millisecond):
		fmt.Println("slow: timed out")
	}
}

// --- 2. WaitGroup-coordinated workers ----------------------------------------

func waitGroupWorkers() {
	fmt.Println("--- 2. WaitGroup ---")
	var wg sync.WaitGroup
	const n = 4

	for i := 1; i <= n; i++ {
		wg.Add(1) // Add BEFORE the go
		go func(id int) {
			defer wg.Done() // first line of the goroutine
			time.Sleep(time.Duration(id) * 10 * time.Millisecond)
		}(i)
	}

	wg.Wait() // returns exactly when the last worker calls Done — no sleep
	// PREDICT 2: can this line ever print before all 4 workers finish?
	fmt.Println("all", n, "workers finished")
}

// --- 3. A goroutine leak: spot it, then fix it -------------------------------

// leakyFetch starts a worker that sends on an UNBUFFERED channel after 100ms.
// If the caller times out at 20ms and returns, the worker wakes at 100ms, tries
// to send, and BLOCKS FOREVER — nobody is receiving. One leaked goroutine per
// timed-out call.
//
// TODO(you): fix the leak. Two ways, both in the lecture:
//   (a) make `result` buffered: make(chan int, 1) — the send always succeeds.
//   (b) add a `done := make(chan struct{})`, `defer close(done)`, and have the
//       worker `select` between `result <- 42` and `<-done`.
// Apply ONE fix, then rerun and watch the leaked count drop to ~0.
func leakyFetch() (int, error) {
	result := make(chan int) // TODO: change to make(chan int, 1) to fix (option a)

	go func() {
		time.Sleep(100 * time.Millisecond)
		result <- 42 // <-- LEAK: blocks forever if the caller already gave up
	}()

	select {
	case r := <-result:
		return r, nil
	case <-time.After(20 * time.Millisecond):
		return 0, errors.New("timeout")
	}
}

func leakDemo() {
	fmt.Println("--- 3. leak demo ---")
	before := runtime.NumGoroutine()

	const calls = 50
	for i := 0; i < calls; i++ {
		_, _ = leakyFetch() // every call times out (20ms < 100ms): every call leaks
	}

	// Give the slow workers time to wake and block on their send.
	time.Sleep(200 * time.Millisecond)
	runtime.GC() // a blocked goroutine is LIVE, not garbage: GC won't reclaim it

	after := runtime.NumGoroutine()
	// PREDICT 3: before the fix, roughly what is (after - before)?
	//            after the fix, what should it be?
	fmt.Printf("goroutines: before=%d after=%d leaked≈%d\n", before, after, after-before)
}
