// Exercise 3 — Find the Race, Read the Report, Fix It Two Ways, Benchmark Both
//
// GOAL
//   Start from a program with a DELIBERATE data race. Find it with `go test
//   -race`, read the ThreadSanitizer report end to end (both stacks, the write
//   line, the read line, the "created at" lines), then fix it two ways — with a
//   sync.Mutex and with a sync/atomic.Int64 — and benchmark both to reproduce
//   the ~4x gap from Lecture 3.
//
// HOW TO RUN
//   mkdir ex03 && cd ex03 && go mod init ex03
//   # save this file as race.go and the test below as race_test.go
//   go test -race -run TestRacy ./...     # SHOULD report a data race (exit 66)
//   go test -race -run TestFixed ./...     # SHOULD be clean
//   go test -bench Counter -benchmem ./... # reproduce the mutex-vs-atomic gap
//
// TASKS
//   1. Run TestRacy under -race. Copy the report into SOLUTIONS notes and label
//      every section: the "Read at", the "Previous write at", the two stacks,
//      and the "Goroutine created at" line that names the `go` statement.
//   2. Confirm RacyCount returns a number a little under N (lost updates).
//   3. Implement MutexCount and AtomicCount so TestFixed passes under -race and
//      both return exactly N.
//   4. Run the benchmarks and record the ns/op for each. The atomic should be
//      meaningfully faster under RunParallel contention.
//
// ACCEPTANCE
//   TestRacy reports exactly one data race; TestFixed is -race-clean; both fixes
//   return exactly N; the benchmark shows atomic faster than mutex. You can
//   explain why counter++ is three operations and where the lost update happens.

package main

import (
	"sync"
	"sync/atomic"
)

// RacyCount is intentionally broken: counter++ is an unsynchronised
// read-modify-write shared across N goroutines. DO NOT "fix" it here — its
// job is to be caught by the race detector.
func RacyCount(n int) int {
	var counter int
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter++ // RACE
		}()
	}
	wg.Wait()
	return counter
}

// MutexCount: fix the race with a sync.Mutex. Should always return n.
func MutexCount(n int) int {
	var (
		mu      sync.Mutex
		counter int
	)
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}
	wg.Wait()
	return counter
}

// AtomicCount: fix the race with a sync/atomic.Int64. The idiomatic fix for a
// single scalar. Should always return n.
func AtomicCount(n int) int {
	var counter atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counter.Add(1)
		}()
	}
	wg.Wait()
	return int(counter.Load())
}

func main() {
	println("racy  :", RacyCount(1000), "(usually < 1000)")
	println("mutex :", MutexCount(1000))
	println("atomic:", AtomicCount(1000))
}

/*
race_test.go — save alongside this file:

package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

const N = 1000

// Run with: go test -race -run TestRacy   (expect a DATA RACE report)
func TestRacy(t *testing.T) {
	_ = RacyCount(N) // the -race build flags the race inside RacyCount
}

func TestFixed(t *testing.T) {
	if got := MutexCount(N); got != N {
		t.Errorf("MutexCount = %d, want %d", got, N)
	}
	if got := AtomicCount(N); got != N {
		t.Errorf("AtomicCount = %d, want %d", got, N)
	}
}

func BenchmarkCounterMutex(b *testing.B) {
	var mu sync.Mutex
	var n int
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			mu.Lock()
			n++
			mu.Unlock()
		}
	})
}

func BenchmarkCounterAtomic(b *testing.B) {
	var n atomic.Int64
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			n.Add(1)
		}
	})
}
*/
