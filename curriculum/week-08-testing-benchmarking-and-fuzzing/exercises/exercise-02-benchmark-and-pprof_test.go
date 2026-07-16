// Exercise 02 — test/benchmark file: BenchmarkSlow / BenchmarkFast with a sink.
//
// Demonstrates: b.ReportAllocs(), b.ResetTimer() after setup, b.Run sub-benchmarks
// across input sizes, a package-level sink to defeat dead-code elimination, and a
// correctness test that the two implementations agree (differential testing).
//
// Run benchmarks:  go test -bench='BuildTagIndex' -benchmem ./...
// CPU+heap profile: go test -bench='BuildTagIndexSlow' -benchmem \
//                     -cpuprofile=cpu.out -memprofile=mem.out ./...
// A/B with stats:   go test -bench='BuildTagIndex/n=1000' -benchmem -count=10 ./...
package notesx

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// sinkIndex is the package-level sink. Assigning the benchmark result here stops
// the compiler from eliminating the call as dead code. Without it, a sufficiently
// clever build could report an impossibly low ns/op.
var sinkIndex map[string][]string

func BenchmarkBuildTagIndexSlow(b *testing.B) {
	for _, size := range []int{100, 1000, 10_000} {
		b.Run("n="+strconv.Itoa(size), func(b *testing.B) {
			notes := MakeNotes(size) // setup must not be timed
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkIndex = BuildTagIndexSlow(notes)
			}
		})
	}
}

func BenchmarkBuildTagIndexFast(b *testing.B) {
	for _, size := range []int{100, 1000, 10_000} {
		b.Run("n="+strconv.Itoa(size), func(b *testing.B) {
			notes := MakeNotes(size)
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				sinkIndex = BuildTagIndexFast(notes)
			}
		})
	}
}

// TestTagIndexImplementationsAgree is the differential test: the fast path is only
// a valid optimization if it produces exactly the same index as the slow path for
// every input. cmp.Diff gives a readable diff when they disagree.
func TestTagIndexImplementationsAgree(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 100, 1000} {
		notes := MakeNotes(size)
		slow := BuildTagIndexSlow(notes)
		fast := BuildTagIndexFast(notes)
		if diff := cmp.Diff(slow, fast); diff != "" {
			t.Errorf("n=%d: slow and fast disagree (-slow +fast):\n%s", size, diff)
		}
	}
}
