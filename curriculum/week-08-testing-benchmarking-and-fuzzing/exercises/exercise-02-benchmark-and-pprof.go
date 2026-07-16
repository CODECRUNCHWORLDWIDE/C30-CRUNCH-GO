// Exercise 02 — benchmark, profile, optimize.
//
// Task
// ----
// BuildTagIndexSlow and BuildTagIndexFast both build a tag -> []noteID index.
// The slow version allocates a fresh string with fmt.Sprintf on every (note,tag)
// pair and does not pre-size its maps; the fast version uses a reused
// strings.Builder, strconv.Itoa, and pre-sized maps.
//
// Your job (see exercise-02-benchmark-and-pprof_test.go and SOLUTIONS.md):
//
//  1. Run the benchmarks for both, with allocation reporting:
//     go test -bench='BuildTagIndex' -benchmem ./...
//
//  2. Capture a CPU and a heap profile from the slow benchmark:
//     go test -bench='BuildTagIndexSlow' -benchmem \
//             -cpuprofile=cpu.out -memprofile=mem.out ./...
//
//  3. Inspect with pprof and find the allocation hot path:
//     go tool pprof cpu.out      # then: top, list BuildTagIndexSlow
//     go tool pprof -alloc_space mem.out
//
//  4. Capture a benchstat A/B delta (slow as the "old", fast as the "new"):
//     go test -bench='BuildTagIndex/n=1000' -benchmem -count=10 ./... > runs.txt
//     # split into old/new, or run each separately, then:
//     benchstat old.txt new.txt
//
// Report the ns/op, B/op, allocs/op for both, the pprof finding, and the
// benchstat percentage deltas with their p-values.
//
// Real, compilable Go 1.22+. The slow/fast pair is the worked example from
// lecture-notes/02-benchmarking-and-pprof.md.
package notesx

import (
	"fmt"
	"strconv"
	"strings"
)

// BuildTagIndexSlow maps each tag to the IDs of the notes carrying it. It is the
// deliberately slow baseline: fmt.Sprintf allocates a fresh string on every inner
// iteration, and the maps/slices are never pre-sized, so they rehash and re-grow.
func BuildTagIndexSlow(notes []Note) map[string][]string {
	idx := map[string][]string{}
	for _, n := range notes {
		for _, tag := range n.Tags {
			// The hot path: one Sprintf allocation per (note, tag).
			key := fmt.Sprintf("%s:%d", tag, len(tag))
			idx[key] = append(idx[key], n.ID)
		}
	}
	return idx
}

// BuildTagIndexFast produces the identical result as BuildTagIndexSlow but avoids
// the per-iteration allocation: one reused strings.Builder, strconv.Itoa instead
// of Sprintf, and a pre-sized map. The two functions must agree for every input —
// the test asserts that with cmp.Diff, which is differential testing in miniature.
func BuildTagIndexFast(notes []Note) map[string][]string {
	idx := make(map[string][]string, len(notes))
	var b strings.Builder
	for _, n := range notes {
		for _, tag := range n.Tags {
			b.Reset()
			b.WriteString(tag)
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(len(tag)))
			key := b.String()
			idx[key] = append(idx[key], n.ID)
		}
	}
	return idx
}

// MakeNotes builds a deterministic slice of n notes, each with a small set of
// tags, for benchmarking. It is exported so the test file and any external
// driver can reuse it.
func MakeNotes(n int) []Note {
	notes := make([]Note, n)
	tagPool := []string{"go", "testing", "fuzz", "pprof", "bench", "postgres", "http", "grpc"}
	for i := range notes {
		tags := []string{
			tagPool[i%len(tagPool)],
			tagPool[(i+1)%len(tagPool)],
		}
		notes[i] = Note{
			ID:    "note-" + strconv.Itoa(i),
			Title: "Title " + strconv.Itoa(i),
			Body:  "Body of note " + strconv.Itoa(i),
			Tags:  tags,
		}
	}
	return notes
}
