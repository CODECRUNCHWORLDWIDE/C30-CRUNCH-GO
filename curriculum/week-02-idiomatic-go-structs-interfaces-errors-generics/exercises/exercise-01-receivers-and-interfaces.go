// Exercise 1 — Receivers, Method Sets, and a Consumer-Defined Interface
//
// GOAL
// ----
// Internalize the method-set rule (a value's method set excludes its
// pointer-receiver methods), design a small consumer-defined interface, and
// route values through a type switch. Predict — before running — which form
// (T vs *T) satisfies the interface.
//
// LAYOUT
// ------
//   ex01/
//   ├── go.mod                  (go mod init github.com/you/ex01)
//   └── main.go                 (this file — package main)
//
// RUN
// ---
//   go run .                              # see the predictions confirmed
//   go vet ./... && staticcheck ./...     # must print nothing
//
// STEPS
// -----
//   1. Read each PREDICT comment and WRITE DOWN your answer before running.
//   2. Run the program and compare. Where you were wrong, re-read Lecture 1 §4.
//   3. Uncomment the line marked "UNCOMMENT TO SEE THE COMPILE ERROR" and run
//      `go build`. Read the exact error, then re-comment it.
//   4. Add one more case to the type switch in describe (a new type of your
//      choosing) and confirm it routes correctly.
//
// ACCEPTANCE
// ----------
// You can state the method-set rule from memory and predict, before compiling,
// whether T or only *T satisfies a given interface. (See SOLUTIONS.md.)

package main

import "fmt"

// ---------------------------------------------------------------------------
// A type with a POINTER-receiver method and a VALUE-receiver method.
// ---------------------------------------------------------------------------

// Counter counts up. Inc mutates, so it takes a pointer receiver; for
// consistency a real type would make Value a pointer receiver too, but we keep
// it a value receiver here precisely to expose the method-set rule.
type Counter struct {
	n int
}

// Inc has a POINTER receiver (it mutates n).
func (c *Counter) Inc() { c.n++ }

// Value has a VALUE receiver (it only reads).
func (c Counter) Value() int { return c.n }

// Incrementer is a small, consumer-defined interface: exactly the one method
// this code needs. Anything that can Inc satisfies it.
type Incrementer interface {
	Inc()
}

func bumpThrice(in Incrementer) {
	in.Inc()
	in.Inc()
	in.Inc()
}

// ---------------------------------------------------------------------------
// A Stringer-style consumer interface + a type switch.
// ---------------------------------------------------------------------------

// Named is anything that can report a display name. Defined at the consumer
// (this file), sized to the single method we call.
type Named interface {
	Name() string
}

type User struct{ Handle string }

func (u User) Name() string { return "@" + u.Handle } // value receiver

type Robot struct{ ID int }

func (r Robot) Name() string { return fmt.Sprintf("bot-%04d", r.ID) } // value receiver

// describe routes an arbitrary value by its dynamic type. A type switch is the
// idiomatic way to handle a small, closed set of dynamic types.
func describe(v any) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return "string: " + x
	case int:
		return fmt.Sprintf("int: %d", x)
	case Named: // matches ANY type implementing Name()
		return "named: " + x.Name()
	default:
		return fmt.Sprintf("unknown %T: %v", x, x)
	}
}

func main() {
	// --- Method sets and interface satisfaction ---

	var c Counter
	bumpThrice(&c) // we pass &c, a *Counter
	// PREDICT 1: what does c.Value() print after bumpThrice(&c)?
	fmt.Println("counter after bumpThrice(&c):", c.Value())

	// PREDICT 2: *Counter satisfies Incrementer (its method set includes the
	// pointer-receiver Inc). Does the VALUE Counter satisfy Incrementer?
	// The next line tries to store a *Counter in the interface — does it compile?
	var inc Incrementer = &c
	inc.Inc()
	fmt.Println("counter after one more inc via interface:", c.Value())

	// PREDICT 3: uncomment the next line. Does it compile? If not, what is the
	// exact compiler error, and which rule from Lecture 1 §4 explains it?
	// UNCOMMENT TO SEE THE COMPILE ERROR:
	// var inc2 Incrementer = c // <-- predict before uncommenting

	// PREDICT 4: c.Inc() is called on an ADDRESSABLE value (a local variable).
	// Go inserts the &c for you. Does this compile and mutate c?
	c.Inc()
	fmt.Println("counter after c.Inc() (auto-&):", c.Value())

	// --- Type switch routing ---
	// PREDICT 5: for each value below, which case of describe() matches, and
	// what string comes out? Write all four down before running.
	vals := []any{
		"hello",
		42,
		User{Handle: "ada"},
		Robot{ID: 7},
	}
	for _, v := range vals {
		fmt.Println(describe(v))
	}

	// A small demonstration that User satisfies Named with a VALUE receiver,
	// so the value (not just the pointer) is enough here — contrast with Counter.
	var n Named = User{Handle: "grace"} // compiles: Name() has a value receiver
	fmt.Println("named via value:", n.Name())
}
