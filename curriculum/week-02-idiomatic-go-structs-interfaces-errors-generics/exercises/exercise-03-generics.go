// Exercise 3 — Generics: a Set[T comparable] and Map/Filter/Reduce
//
// GOAL
// ----
// Write a generic container (Set[T comparable]) and three type-parametric
// algorithms (Map, Filter, Reduce), instantiate them at more than one element
// type, and give them a table-driven test suite. Predict which instantiations
// compile (the comparable ones) and which do not.
//
// LAYOUT
// ------
//   ex03/
//   ├── go.mod                  (go mod init github.com/you/ex03)
//   ├── gen.go                  (this file — package gen)
//   └── gen_test.go             (the test skeleton printed at the bottom; write it)
//
// RUN
// ---
//   go test ./...                          # green
//   go test -v ./...                       # see each named subtest
//   go test -run 'TestSet/dedupes'         # run ONE case
//   go vet ./... && staticcheck ./...      # must print nothing
//
// STEPS
// -----
//   1. Read the PREDICT comments and answer them before running.
//   2. Complete the Reduce function (marked TODO).
//   3. Write gen_test.go from the skeleton at the bottom and extend each table
//      with at least two more cases.
//
// ACCEPTANCE
// ----------
// A green `go test ./...`, clean go vet / staticcheck, and a one-sentence answer
// for each of Set/Map/Filter/Reduce to: "container, algorithm, or neither — and
// would an interface have been better?" (See SOLUTIONS.md.)

package gen

// --- A generic container: Set[T comparable]. ---
// T is constrained to comparable because elements are used as map keys.

type Set[T comparable] struct {
	m map[T]struct{} // struct{} is zero-byte: a set, not a map-with-values
}

// NewSet builds a Set from zero or more initial items.
func NewSet[T comparable](items ...T) *Set[T] {
	s := &Set[T]{m: make(map[T]struct{}, len(items))}
	for _, v := range items {
		s.Add(v)
	}
	return s
}

func (s *Set[T]) Add(v T)      { s.m[v] = struct{}{} }
func (s *Set[T]) Has(v T) bool { _, ok := s.m[v]; return ok }
func (s *Set[T]) Len() int     { return len(s.m) }

// PREDICT 1: NewSet("a", "b", "a") works (strings are comparable). Would
// NewSet([]int{1}, []int{2}) compile? Why or why not? (Hint: Lecture 3 §3.2.)

// --- Type-parametric algorithms. ---

// Map applies f to every element of s, returning a new slice.
func Map[T, U any](s []T, f func(T) U) []U {
	out := make([]U, len(s))
	for i, v := range s {
		out[i] = f(v)
	}
	return out
}

// Filter returns the elements of s for which keep returns true.
func Filter[T any](s []T, keep func(T) bool) []T {
	out := make([]T, 0, len(s))
	for _, v := range s {
		if keep(v) {
			out = append(out, v)
		}
	}
	return out
}

// Reduce folds s into a single accumulator value, left to right.
// TODO: implement. Start from init, apply f(acc, element) across s, return acc.
func Reduce[T, A any](s []T, init A, f func(A, T) A) A {
	// PREDICT 2: what is the zero value of A if a caller passes init = A's zero?
	// Replace the panic with the fold.
	panic("TODO: implement Reduce")
}

// PREDICT 3: Map needs no constraint beyond `any` on T and U — why can it get
// away with `any` where Set needs `comparable`?

// ---------------------------------------------------------------------------
// TODO: create gen_test.go (package gen) with the following content, then
// extend each table with at least two more cases. Note the tests instantiate
// the generics at MORE THAN ONE element type — that is the point of generics.
// ---------------------------------------------------------------------------
//
//   package gen
//
//   import (
//       "reflect"
//       "testing"
//   )
//
//   func TestSet(t *testing.T) {
//       tests := []struct {
//           name    string
//           input   []string
//           wantLen int
//           probe   string
//           wantHas bool
//       }{
//           {"dedupes", []string{"a", "b", "a"}, 2, "a", true},
//           {"empty", nil, 0, "x", false},
//           {"absent probe", []string{"a"}, 1, "z", false},
//           // TODO: add "single" and "all distinct" cases.
//       }
//       for _, tc := range tests {
//           t.Run(tc.name, func(t *testing.T) {
//               s := NewSet(tc.input...)
//               if s.Len() != tc.wantLen {
//                   t.Errorf("Len() = %d, want %d", s.Len(), tc.wantLen)
//               }
//               if got := s.Has(tc.probe); got != tc.wantHas {
//                   t.Errorf("Has(%q) = %v, want %v", tc.probe, got, tc.wantHas)
//               }
//           })
//       }
//
//       // Instantiate Set at a DIFFERENT element type to prove genericity.
//       ints := NewSet(1, 2, 2, 3)
//       if ints.Len() != 3 {
//           t.Errorf("int set Len() = %d, want 3", ints.Len())
//       }
//   }
//
//   func TestMap(t *testing.T) {
//       got := Map([]int{1, 2, 3}, func(n int) int { return n * 2 })
//       want := []int{2, 4, 6}
//       if !reflect.DeepEqual(got, want) {
//           t.Errorf("Map double = %v, want %v", got, want)
//       }
//       // Different output type: int -> bool.
//       evens := Map([]int{1, 2, 3, 4}, func(n int) bool { return n%2 == 0 })
//       if !reflect.DeepEqual(evens, []bool{false, true, false, true}) {
//           t.Errorf("Map isEven = %v", evens)
//       }
//       // TODO: add an int -> string case.
//   }
//
//   func TestFilter(t *testing.T) {
//       got := Filter([]int{1, 2, 3, 4, 5}, func(n int) bool { return n%2 == 1 })
//       want := []int{1, 3, 5}
//       if !reflect.DeepEqual(got, want) {
//           t.Errorf("Filter odd = %v, want %v", got, want)
//       }
//       // TODO: add a string case (keep non-empty strings).
//   }
//
//   func TestReduce(t *testing.T) {
//       sum := Reduce([]int{1, 2, 3, 4}, 0, func(acc, n int) int { return acc + n })
//       if sum != 10 {
//           t.Errorf("Reduce sum = %d, want 10", sum)
//       }
//       // Different accumulator type: fold ints into a concatenated string.
//       joined := Reduce([]int{1, 2, 3}, "", func(acc string, n int) string {
//           return acc + string(rune('0'+n))
//       })
//       if joined != "123" {
//           t.Errorf("Reduce join = %q, want \"123\"", joined)
//       }
//       // TODO: add an "empty slice returns init" case.
//   }
