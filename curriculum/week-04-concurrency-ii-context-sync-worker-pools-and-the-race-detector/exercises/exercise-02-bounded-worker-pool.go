// Exercise 2 — Bounded Worker Pool, Two Ways
//
// GOAL
//   Build a worker pool that runs AT MOST `limit` goroutines concurrently,
//   two different ways:
//     (A) a buffered channel used as a counting semaphore, and
//     (B) golang.org/x/sync/errgroup with g.SetLimit(limit).
//   Instrument both with an atomic high-water mark and PROVE the bound held:
//   the peak concurrency must never exceed `limit`.
//
// HOW TO RUN
//   mkdir ex02 && cd ex02 && go mod init ex02
//   go get golang.org/x/sync/errgroup
//   # save this file as pool.go and the test below as pool_test.go
//   go test -race ./...           # MUST pass under the race detector
//   go vet ./...                  # MUST be clean
//
// TASKS
//   1. Read enter()/exit(): the peak is updated with a compare-and-swap retry
//      loop. Explain why a plain `if cur > peak { peak = cur }` would race.
//   2. Implement RunSemaphore using a `chan struct{}` of capacity `limit`.
//      Acquire a token before work, release it after. Respect ctx.Done() while
//      waiting for a token.
//   3. Implement RunErrgroup using errgroup.WithContext + g.SetLimit(limit).
//   4. In the test, assert m.Peak() <= limit for several limits. That assertion
//      is your proof the bound held.
//
// ACCEPTANCE
//   `go test -race` is green and the peak-never-exceeds-limit assertion passes
//   for limit in {1, 4, 16}. You can explain why writing results[i] from each
//   goroutine is race-free (distinct indices + Wait's happens-before edge).

package main

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"
)

// Metrics tracks live and peak concurrency with lock-free atomics.
type Metrics struct {
	current atomic.Int64
	peak    atomic.Int64
}

func (m *Metrics) enter() {
	cur := m.current.Add(1)
	for { // CAS retry loop: set peak = cur if cur is larger
		old := m.peak.Load()
		if cur <= old || m.peak.CompareAndSwap(old, cur) {
			return
		}
	}
}

func (m *Metrics) exit()       { m.current.Add(-1) }
func (m *Metrics) Peak() int64 { return m.peak.Load() }

// fakeWork simulates a short, cancellable unit of work.
func fakeWork(ctx context.Context) error {
	select {
	case <-time.After(2 * time.Millisecond):
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// RunSemaphore runs n tasks with at most `limit` in flight, using a buffered
// channel as a counting semaphore.
func RunSemaphore(ctx context.Context, n, limit int) *Metrics {
	m := &Metrics{}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup

	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case sem <- struct{}{}: // acquire a slot
			case <-ctx.Done():
				return
			}
			defer func() { <-sem }() // release the slot

			m.enter()
			defer m.exit()
			_ = fakeWork(ctx)
		}()
	}
	wg.Wait()
	return m
}

// RunErrgroup runs n tasks with at most `limit` in flight, using errgroup.
func RunErrgroup(ctx context.Context, n, limit int) (*Metrics, error) {
	m := &Metrics{}
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(limit)

	for i := 0; i < n; i++ {
		g.Go(func() error {
			m.enter()
			defer m.exit()
			return fakeWork(ctx)
		})
	}
	return m, g.Wait()
}

func main() {
	ctx := context.Background()
	m := RunSemaphore(ctx, 200, 16)
	println("semaphore peak:", m.Peak())
	m2, _ := RunErrgroup(ctx, 200, 16)
	println("errgroup  peak:", m2.Peak())
}

/*
pool_test.go — save alongside this file:

package main

import (
	"context"
	"testing"
)

func TestSemaphoreBound(t *testing.T) {
	for _, limit := range []int{1, 4, 16} {
		m := RunSemaphore(context.Background(), 200, limit)
		if got := m.Peak(); got > int64(limit) {
			t.Errorf("limit=%d: peak %d exceeded the bound", limit, got)
		}
	}
}

func TestErrgroupBound(t *testing.T) {
	for _, limit := range []int{1, 4, 16} {
		m, err := RunErrgroup(context.Background(), 200, limit)
		if err != nil {
			t.Fatalf("limit=%d: %v", limit, err)
		}
		if got := m.Peak(); got > int64(limit) {
			t.Errorf("limit=%d: peak %d exceeded the bound", limit, got)
		}
	}
}
*/
