// Exercise 1 — Goroutines and Channels
//
// GOAL
// ----
// Launch goroutines and coordinate them WITHOUT time.Sleep; feel the
// difference between an unbuffered channel (a rendezvous) and a buffered one
// (a bounded queue); range over a channel that a producer closes; observe a
// deadlock (commented out) and understand the fix.
//
// This is a runnable program. Put it in its own module:
//
//   mkdir ex01 && cd ex01
//   go mod init github.com/you/ex01
//   # save this file as main.go
//   go run .
//   go vet ./...        # must print nothing
//   staticcheck ./...   # must print nothing
//
// Work the five PREDICT comments: write down your prediction BEFORE running,
// then check against the output. Answers are in SOLUTIONS.md.
//
// ACCEPTANCE
// ----------
// You can state, for every channel here, WHO closes it and what would happen
// if nobody did. You can explain why part 2's send does not block but a third
// send would, and why the deadlock in part 5 is a deadlock.

package main

import (
	"fmt"
	"sort"
	"sync"
)

func main() {
	rendezvous()
	bufferedDoesNotBlock()
	rangeUntilClosed()
	fanOutCollect()
	// deadlockDemo() // <- part 5: leave commented; read it, then read SOLUTIONS.md
}

// --- 1. Unbuffered channel: a rendezvous, no sleep needed --------------------

func rendezvous() {
	fmt.Println("--- 1. rendezvous ---")
	ch := make(chan string) // unbuffered

	go func() {
		ch <- "done" // blocks until main receives
		fmt.Println("goroutine: ran AFTER the send completed")
	}()

	msg := <-ch // blocks until the goroutine sends; this IS the synchronisation
	// PREDICT 1: which prints first — "main got: done" or the goroutine's line?
	fmt.Println("main got:", msg)
}

// --- 2. Buffered channel: capacity lets sends proceed without a receiver -----

func bufferedDoesNotBlock() {
	fmt.Println("--- 2. buffered ---")
	ch := make(chan int, 2) // capacity 2

	ch <- 10 // does not block (1/2)
	ch <- 20 // does not block (2/2)
	// A third send here — ch <- 30 — WOULD block: buffer full, no receiver.

	// PREDICT 2: what do len(ch) and cap(ch) print here?
	fmt.Printf("len=%d cap=%d\n", len(ch), cap(ch))

	fmt.Println(<-ch) // 10 (FIFO)
	fmt.Println(<-ch) // 20
}

// --- 3. Producer closes; consumer ranges until closed ------------------------

// produce sends squares onto out and CLOSES out (the sender closes, once).
// out is send-only here (chan<- int) — the compiler forbids receiving on it.
func produce(out chan<- int, n int) {
	defer close(out) // who closes? the sender. Remove this and part 3 deadlocks.
	for i := 1; i <= n; i++ {
		out <- i * i
	}
}

func rangeUntilClosed() {
	fmt.Println("--- 3. range until closed ---")
	out := make(chan int)
	go produce(out, 4)

	// PREDICT 3: what values print, and what ends this loop?
	for v := range out { // ends when produce closes out
		fmt.Println("got", v)
	}
	fmt.Println("range ended (channel closed and drained)")
}

// --- 4. Fan out to W workers, collect results, no sleep ----------------------

// worker reads jobs (receive-only) and sends results (send-only).
func worker(jobs <-chan int, results chan<- int, wg *sync.WaitGroup) {
	defer wg.Done()
	for j := range jobs { // drains jobs until closed
		results <- j * 2
	}
}

func fanOutCollect() {
	fmt.Println("--- 4. fan-out / collect ---")
	const workers = 3
	jobs := make(chan int)
	results := make(chan int)

	// Feeder owns jobs, so the feeder closes it.
	go func() {
		defer close(jobs)
		for j := 1; j <= 6; j++ {
			jobs <- j
		}
	}()

	// Fan out: W workers all read the same jobs channel.
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)             // Add BEFORE the go statement
		go worker(jobs, results, &wg) // WaitGroup BY POINTER
	}

	// The only safe closer of results: a goroutine that waits for all workers.
	go func() {
		wg.Wait()
		close(results) // safe: every sender (worker) has returned
	}()

	// Fan in: collect until results is closed.
	var got []int
	for r := range results {
		got = append(got, r)
	}
	sort.Ints(got) // results arrive in nondeterministic order; sort for display
	// PREDICT 4: what is `got` after sorting?
	fmt.Println("doubled:", got)
}

// --- 5. A deadlock (leave commented) -----------------------------------------
//
// Uncomment the call to deadlockDemo() in main to SEE it. The runtime aborts
// with: fatal error: all goroutines are asleep - deadlock!
//
// PREDICT 5: why is this a deadlock, and what is the one-line fix?
//
//nolint:unused // intentionally unused; uncomment to demonstrate the deadlock
func deadlockDemo() {
	ch := make(chan int) // unbuffered
	ch <- 1              // blocks forever: no receiver, no other goroutine
	fmt.Println(<-ch)    // unreachable
}
