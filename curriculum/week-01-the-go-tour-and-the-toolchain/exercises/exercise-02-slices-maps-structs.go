// Exercise 2 — Slices, Maps, Structs, and defer
//
// GOAL
// ----
// Reproduce the slice-aliasing trap and fix it; use the comma-ok map read to
// distinguish "absent" from "zero"; build a set with map[string]struct{};
// embed a struct and call a promoted method; and predict a defer LIFO sequence
// before you run it.
//
// This is a runnable program. Put it in its own module (go mod init
// github.com/you/ex02), save as main.go, then:
//
//   go run .
//   go vet ./...        # must print nothing
//   staticcheck ./...   # must print nothing
//
// Work through the five PREDICT comments: write down your prediction BEFORE
// running, then check against the output. The answers are in SOLUTIONS.md.

package main

import (
	"fmt"
	"sort"
)

// --- Structs and embedding ---------------------------------------------------

// Base carries an ID and knows how to describe itself.
type Base struct {
	ID string
}

// Describe is a method on Base. Embedding will PROMOTE it onto User.
func (b Base) Describe() string { return "id=" + b.ID }

// User embeds Base (no field name) — composition, not inheritance.
// User "has a" Base, and Base's fields/methods are promoted onto User.
type User struct {
	Base
	Name string
}

func main() {
	// --- 1. Slice aliasing -----------------------------------------------
	base := []int{1, 2, 3, 4, 5}
	window := base[1:3] // len 2, cap 4 — shares base's backing array
	window[0] = 99
	// PREDICT 1: what does base print here?
	fmt.Println("after window[0]=99:", base)

	// Appending into a sub-slice with spare capacity overwrites the parent.
	window = append(window, 777) // writes into base[3], because cap allowed it
	// PREDICT 2: what does base print now?
	fmt.Println("after append(window,777):", base)

	// The fix: a full-slice expression base[1:3:3] caps cap at len, so the next
	// append is forced to reallocate and CANNOT touch base.
	safe := base[1:3:3]
	safe = append(safe, 555) // reallocates — base is untouched
	// PREDICT 3: did base change this time?
	fmt.Println("after safe append:", base, "| safe:", safe)

	// --- 2. Maps: comma-ok and the set idiom -----------------------------
	freq := map[string]int{"cat": 2}
	v1 := freq["dog"]      // 0, but was "dog" present? ambiguous
	v2, ok := freq["dog"]  // comma-ok: v2==0, ok==false → absent
	v3, present := freq["cat"]
	// PREDICT 4: v1, v2/ok, v3/present?
	fmt.Printf("v1=%d  v2=%d ok=%t  v3=%d present=%t\n", v1, v2, ok, v3, present)

	// A set: map[string]struct{} stores zero bytes per value.
	seen := make(map[string]struct{})
	for _, w := range []string{"a", "b", "a", "c", "b"} {
		seen[w] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k) // map order is randomized; sort for stable output
	}
	sort.Strings(keys)
	fmt.Println("distinct, sorted:", keys)

	// --- 3. Embedding: promoted field and method -------------------------
	u := User{Base: Base{ID: "u1"}, Name: "alice"}
	fmt.Println("promoted field u.ID:", u.ID)            // not u.Base.ID
	fmt.Println("promoted method u.Describe():", u.Describe())

	// --- 4. defer LIFO and argument evaluation timing --------------------
	deferDemo()
}

func deferDemo() {
	i := 0
	defer fmt.Println("deferred print, i captured at defer-time =", i) // captures 0
	defer fmt.Println("this runs SECOND (LIFO)")
	defer fmt.Println("this runs FIRST  (LIFO)")
	i = 99
	// PREDICT 5: in what order do the three deferred lines print, and does the
	// first one show 0 or 99?
	fmt.Println("function body done; i is now", i)
}
