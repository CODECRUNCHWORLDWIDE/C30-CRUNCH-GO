// Exercise 3 — Fan-Out / Fan-In, and Proving No Leaks
//
// GOAL
// ----
// Build a small fan-out/fan-in pipeline that squares N numbers across W worker
// goroutines, merges their outputs (the WaitGroup + single-closer idiom), and
// returns the results. Then prove the pipeline leaks no goroutines by checking
// runtime.NumGoroutine() returns to baseline after it drains.
//
// This is a runnable program. Put it in its own module:
//
//   mkdir ex03 && cd ex03
//   go mod init github.com/you/ex03
//   # save this file as main.go
//   go run .
//   go vet ./...        # must print nothing
//   staticcheck ./...   # must print nothing
//
// Work the PREDICT comments. The TODO is to complete `merge`. Answers and the
// table-test you should write are in SOLUTIONS.md.
//
// ACCEPTANCE
// ----------
// SquareAll(in, W) returns the squares of `in` for any N>=0 and W>=1, and the
// goroutine count after it returns equals the count before (within scheduler
// noise). You can explain who closes each of the three kinds of channel.

package main

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"time"
)

// gen is the SOURCE: emit nums onto a channel and close it.
func gen(nums []int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out) // source owns out, source closes it
		for _, n := range nums {
			out <- n
		}
	}()
	return out
}

// square is one fan-out STAGE: receive ints, send their squares, close on exit.
func square(in <-chan int) <-chan int {
	out := make(chan int)
	go func() {
		defer close(out)
		for n := range in { // ends when in is closed and drained
			out <- n * n
		}
	}()
	return out
}

// merge fans in: forward every input channel onto one out channel, then close
// out exactly once after every forwarder has finished.
//
// TODO(you): complete this function. The shape is:
//   1. make the out channel
//   2. var wg sync.WaitGroup
//   3. for each input c: wg.Add(1); go a forwarder that ranges c into out, deferring wg.Done()
//   4. a closer goroutine: wg.Wait(); close(out)
//   5. return out
func merge(inputs ...<-chan int) <-chan int {
	out := make(chan int)
	var wg sync.WaitGroup

	forward := func(c <-chan int) {
		defer wg.Done()
		for v := range c { // TODO: drain c into out
			out <- v
		}
	}

	wg.Add(len(inputs)) // Add before the go statements
	for _, c := range inputs {
		go forward(c)
	}

	// TODO: the ONLY safe closer — close out after all forwarders return.
	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// SquareAll fans `in` out across `workers` square stages, fans the results back
// in, and returns them sorted (results arrive unordered). This is the pure
// logic — no I/O, no os.Exit — so it is trivially testable (see SOLUTIONS.md).
func SquareAll(in []int, workers int) []int {
	if workers < 1 {
		workers = 1
	}
	source := gen(in)

	// FAN OUT: `workers` square stages, all reading the same source channel.
	stages := make([]<-chan int, workers)
	for i := 0; i < workers; i++ {
		stages[i] = square(source)
	}

	// FAN IN: merge the stages and collect.
	var got []int
	for v := range merge(stages...) {
		got = append(got, v)
	}
	sort.Ints(got)
	return got
}

func main() {
	before := runtime.NumGoroutine()

	in := []int{1, 2, 3, 4, 5, 6, 7, 8}
	got := SquareAll(in, 4)
	// PREDICT 1: what is `got`? (squares of 1..8, sorted)
	fmt.Println("squares:", got)

	// Let any tail goroutines unwind, then re-measure.
	time.Sleep(20 * time.Millisecond)
	runtime.GC()
	after := runtime.NumGoroutine()

	// PREDICT 2: a leak-free pipeline returns the count to baseline. What is
	//            (after - before) expected to be?
	fmt.Printf("goroutines: before=%d after=%d delta=%d\n", before, after, after-before)

	// PREDICT 3: SquareAll(nil, 4) and SquareAll([]int{9}, 1) — what do they return?
	fmt.Println("empty:", SquareAll(nil, 4))
	fmt.Println("single:", SquareAll([]int{9}, 1))
}
