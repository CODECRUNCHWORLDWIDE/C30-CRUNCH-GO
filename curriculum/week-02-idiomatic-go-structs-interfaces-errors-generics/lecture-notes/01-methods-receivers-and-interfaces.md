# Lecture 1 — Methods, Receivers, and Interfaces

> **Time:** 2 hours. Take methods-and-receivers in one sitting and interfaces-and-type-switches in a second. **Prerequisites:** Week 1 (the zero value, structs, functions). **Citations:** Effective Go's methods and interfaces sections at <https://go.dev/doc/effective_go#methods> and <https://go.dev/doc/effective_go#interfaces>, the Tour's methods module at <https://go.dev/tour/methods/1>, the spec's "Method sets" at <https://go.dev/ref/spec#Method_sets>, and the Go Code Review Comments at <https://go.dev/wiki/CodeReviewComments>.

## 1. Why this lecture

Last week you wrote functions and structs. This week you attach functions to types (methods), abstract over types by *behaviour* (interfaces), and learn the two rules that a Go reviewer checks in the first thirty seconds of reading your code: *is the receiver kind right and consistent*, and *is the interface small and defined where it is used*. Get these two right and your Go reads like the standard library. Get them wrong and it reads like ported Java — a class hierarchy wearing a Go costume.

The mental shift from a class-based language is the hard part, so name it up front. There are no classes. There is no inheritance. There is no `implements` keyword. A "type with methods" is just a named type with functions attached, and "this type fits that interface" is a fact the compiler *checks*, never a fact you *declare*. The whole object model is two small ideas — methods and interfaces — and the rest of this lecture is the precise rules around them.

## 2. Methods are functions with a receiver

A method is a function with an extra parameter — the *receiver* — written before the name:

```go
type Counter struct {
	n int
}

// Value has a VALUE receiver: c is a copy of the Counter.
func (c Counter) Value() int {
	return c.n
}

// Inc has a POINTER receiver: c points at the original Counter.
func (c *Counter) Inc() {
	c.n++
}
```

```go
var c Counter
c.Inc()           // n is now 1
c.Inc()           // n is now 2
fmt.Println(c.Value()) // 2
```

You can define a method on *any named type declared in your package*, not just structs:

```go
type Celsius float64

func (c Celsius) Fahrenheit() float64 {
	return float64(c)*9/5 + 32
}

fmt.Println(Celsius(100).Fahrenheit()) // 212
```

You cannot define a method on a type from another package (you cannot add a method to `int` or to `time.Time`) — methods belong to the package that declares the type. If you want to add behaviour to someone else's type, you wrap it in your own type. Citation: <https://go.dev/tour/methods/3>.

## 3. Value vs pointer receivers — the load-bearing choice

This is the decision a reviewer checks first, so internalize the rule:

- A **value receiver** (`func (c Counter) ...`) operates on a *copy*. Mutations to the receiver are lost when the method returns. Use it when the method only *reads*, and the type is small (a few machine words).
- A **pointer receiver** (`func (c *Counter) ...`) operates on the *original* through a pointer. Use it when the method *mutates* the receiver, **or** when the type is large enough that copying it on every call is wasteful, **or** when the type contains a `sync.Mutex` or other field that must not be copied.

```go
type Counter struct{ n int }

func (c Counter) IncWrong() { c.n++ }   // mutates the COPY; the caller sees nothing
func (c *Counter) IncRight() { c.n++ }  // mutates the original
```

```go
var c Counter
c.IncWrong()           // no effect
fmt.Println(c.n)       // 0
c.IncRight()
fmt.Println(c.n)       // 1
```

The single most important review rule on top of that: **keep the receiver kind consistent across a type's whole method set.** If any method needs a pointer receiver (because it mutates, or the type is large), give *all* the type's methods pointer receivers. Mixing `func (c Counter) A()` and `func (c *Counter) B()` on the same type is a code smell — it makes the method-set rules below bite in surprising ways, and `go vet` and reviewers will push back. Citation: the Go Code Review Comments "Receiver Type" guidance at <https://go.dev/wiki/CodeReviewComments#receiver-type>.

### 3.1 The auto-`&` and auto-`*` convenience

When you call a pointer-receiver method on an *addressable value*, Go automatically takes its address for you:

```go
var c Counter
c.IncRight()   // Go rewrites this as (&c).IncRight()
```

And calling a value-receiver method through a pointer auto-dereferences:

```go
p := &Counter{}
fmt.Println(p.Value()) // Go rewrites this as (*p).Value()
```

This convenience is why receivers feel seamless in everyday code — *until* you put the value into an interface, where the convenience disappears and the method-set rule (next) takes over. That gap is the most-missed detail in Go.

## 4. The method-set rule — the most-missed detail in Go

Here is the rule, stated precisely, because it governs interface satisfaction:

- The method set of a value of type `T` contains **only the methods declared with a value receiver `(t T)`**.
- The method set of a pointer `*T` contains **both** the value-receiver methods `(t T)` **and** the pointer-receiver methods `(t *T)`.

Why does the value's method set exclude pointer-receiver methods? Because a pointer-receiver method can mutate, and Go cannot guarantee that an arbitrary value (e.g. one stored inside a map, which is not addressable) has a stable address to mutate. So the value's method set is the read-only subset.

The consequence is concrete and it is what gets asked in interviews:

```go
type Stringer interface {
	String() string
}

type Loud struct{ msg string }

func (l *Loud) String() string { // POINTER receiver
	return strings.ToUpper(l.msg)
}

func main() {
	var s Stringer

	s = &Loud{msg: "hi"} // OK: *Loud's method set includes String()
	fmt.Println(s.String())

	// s = Loud{msg: "hi"} // COMPILE ERROR:
	// Loud does not implement Stringer (String method has pointer receiver)
}
```

`&Loud{...}` satisfies `Stringer`; the *value* `Loud{...}` does not, because `String` has a pointer receiver and so is not in `Loud`'s method set. The auto-`&` from §3.1 does **not** rescue you here, because the value being assigned to the interface is a copy with no guaranteed address. This is why you so often see `var x Iface = &T{}` rather than `var x Iface = T{}`. Predict this in Exercise 1; it is the single highest-yield fact this week. Citation: <https://go.dev/ref/spec#Method_sets>.

## 5. Interfaces are small, implicit, and consumer-defined

An interface is a set of method signatures. A type satisfies it by *having* those methods — there is no declaration, no `implements`:

```go
type Stringer interface {
	String() string
}
```

Any type with a `String() string` method satisfies `Stringer` automatically, even if its author never heard of `Stringer`. This *structural*, implicit satisfaction is the heart of Go's flexibility: you can write an interface that an existing type already satisfies, retroactively. Citation: <https://go.dev/doc/effective_go#interfaces>.

Three rules carry idiomatic interface design:

1. **Keep them small.** The standard library's most-used interfaces are one method: `io.Reader` (`Read`), `io.Writer` (`Write`), `fmt.Stringer` (`String`), `sort.Interface` (three, and that is already on the large side). The Go proverb: *"the bigger the interface, the weaker the abstraction."* A one-method interface is satisfied by almost anything and composes with everything.

2. **Define them at the consumer, not the producer.** The package that *calls* a method should declare the interface it needs, sized to exactly what it uses. The package that *implements* the behaviour just exposes a concrete struct. This is the opposite of the Java instinct (define the interface next to the implementation). Example — a report writer that needs only "write bytes":

```go
package report

import "io"

// Render writes a report to w. It accepts io.Writer (defined in the
// io package, consumed here) so the caller can pass a file, a buffer,
// an HTTP response, or a test fixture — Render does not care which.
func Render(w io.Writer, rows []Row) error {
	for _, r := range rows {
		if _, err := fmt.Fprintf(w, "%s\t%d\n", r.Name, r.Count); err != nil {
			return fmt.Errorf("writing report: %w", err)
		}
	}
	return nil
}
```

`report.Render` never names `*os.File`. It names `io.Writer`, the narrowest thing it uses, and the caller supplies the concrete type. Citation: <https://go.dev/wiki/CodeReviewComments#interfaces>.

3. **Do not export an interface "just in case."** A premature interface with one implementation is speculative generality — it adds an indirection a reader must follow for no benefit. Write the concrete type; introduce the interface at the consumer the moment a *second* implementation or a *test double* actually needs it. The Go team's guidance: don't define interfaces before you use them. Citation: <https://go.dev/wiki/CodeReviewComments#interfaces>.

## 6. "Accept interfaces, return structs"

Put §3–§5 together and you get the single most useful Go design maxim:

> **Accept interfaces, return structs.**

- **Accept the narrowest interface you actually use** as a parameter. A function that takes `io.Reader` works with files, sockets, buffers, and `strings.NewReader("...")` in a test — for free, with no work by any caller.
- **Return the concrete struct**, not an interface. The caller then has the full, documented type with every method and field, can call methods you add *later* without an API break, and never has to type-assert their way back to capability.

```go
package buffer

// New returns a *Buffer — a concrete type. Callers get every method, and we
// can add methods later without breaking them.
func New() *Buffer { return &Buffer{} }

// Append accepts io.Reader — the narrowest thing it uses — so any source works.
func (b *Buffer) Append(r io.Reader) error { /* ... */ return nil }
```

Why not return an interface? Three reasons a reviewer will cite. First, it is *lossy*: the caller loses access to everything not in the interface. Second, it is *brittle for evolution*: adding a method to the interface breaks every implementer; adding a method to a concrete struct breaks no one. Third, it usually means an *unnecessary heap allocation*, because the concrete value gets boxed into the interface. There are exceptions — a factory that genuinely returns one of several implementations (`io.Pipe` returns the `*PipeReader`/`*PipeWriter` concretes, but a plugin loader might return an interface) — but "return structs" is the default you deviate from with a reason. Citation: <https://go.dev/wiki/CodeReviewComments#interfaces> and the FAQ at <https://go.dev/doc/faq#guarantee_satisfies_interface>.

### 6.1 The compile-time satisfaction assertion

When a type is *meant* to satisfy an interface but nothing in the package forces the check (because you return the concrete type), assert it at compile time with a blank-identifier declaration:

```go
// Compile-time proof that *FileStore satisfies Store. Costs nothing at runtime;
// breaks the build immediately if a method signature drifts.
var _ Store[string, []byte] = (*FileStore)(nil)
```

This is the idiom for "I promise this implements that," and it gives you the early-break safety of an `implements` keyword without coupling the type to the interface. You will use it in the mini-project. Citation: <https://go.dev/doc/faq#guarantee_satisfies_interface>.

## 7. The empty interface, `any`, and when you actually need it

The empty interface `interface{}` has *no* methods, so *every* type satisfies it. Go 1.18 gave it the alias `any`, and you should always write `any`:

```go
func describe(v any) {
	fmt.Printf("%v has type %T\n", v, v)
}
```

`any` is how you hold "a value of unknown type" — what `encoding/json` decodes into, what `fmt.Println` accepts. But reaching for `any` discards all type information, and getting it back requires a runtime check that can fail. In the generics era (Lecture 3) most former uses of `any`-for-genericity are better written with a type parameter, which keeps the type. Use `any` when the type is *genuinely* dynamic (JSON, a logging key-value bag); use a type parameter when the type *varies but is known at the call site*. Citation: <https://go.dev/ref/spec#Interface_types> and the Tour's empty-interface page at <https://go.dev/tour/methods/14>.

## 8. Type assertions and type switches

To get a concrete type back out of an interface value, you *assert*:

```go
var v any = "hello"

s := v.(string)        // s == "hello"; PANICS if v is not a string
s, ok := v.(string)    // s == "hello", ok == true; never panics
n, ok := v.(int)       // n == 0, ok == false (v held a string)
```

Always prefer the **two-value comma-ok form** outside of code you have just type-checked yourself — the one-value form panics on a mismatch, which turns a recoverable condition into a crash. Citation: <https://go.dev/ref/spec#Type_assertions>.

When you must branch on several possible dynamic types, use a **type switch**:

```go
func stringify(v any) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		return x // x is typed as string in this case
	case int:
		return strconv.Itoa(x) // x is typed as int here
	case fmt.Stringer:
		return x.String() // matches any type implementing Stringer
	default:
		return fmt.Sprintf("%v", x) // x is typed as any here
	}
}
```

Inside each `case`, `x` has the case's type — no further assertion needed. A type switch is the idiomatic way to handle a small, *closed* set of dynamic types (a token in a parser, a node in an AST, a value from `json.Unmarshal` into `any`). If you find yourself writing a long type switch over your *own* types, that is often a sign you wanted an interface method instead (let each type implement `String()` and call `v.String()`), or a generic function. Citation: <https://go.dev/tour/methods/16>.

## 9. Embedding — composition, not inheritance

Go has no inheritance. It has *embedding*: put a type into a struct (or an interface into an interface) without a field name, and its fields and methods are *promoted* to the outer type.

```go
type Logger struct{ prefix string }

func (l Logger) Log(msg string) { fmt.Println(l.prefix, msg) }

type Server struct {
	Logger        // embedded: no field name
	addr   string
}

func main() {
	s := Server{Logger: Logger{prefix: "[srv]"}, addr: ":8080"}
	s.Log("starting") // promoted: calls s.Logger.Log("starting") => "[srv] starting"
}
```

This *looks* like inheritance, but it is not, and the difference is examinable:

- There is **no subtyping**. A `Server` is **not** a `Logger`; you cannot pass a `Server` where a `Logger` is expected (you can pass `s.Logger`). Embedding promotes *methods*, it does not establish an *is-a* relationship.
- There is **no virtual dispatch / no overriding in the OO sense**. If `Server` also declares `Log`, it *shadows* the embedded one; the embedded `Logger.Log` does not magically call `Server.Log`. Method resolution is static.
- It is **"has-a" wired to read like "is-a"** for convenience: `Server` *has a* `Logger` and borrows its method set. This is composition with sugar.

Embedding interfaces composes them — this is exactly how `io.ReadWriter` is built:

```go
type Reader interface{ Read(p []byte) (int, error) }
type Writer interface{ Write(p []byte) (int, error) }

type ReadWriter interface {
	Reader // embedded interface
	Writer // embedded interface
}
```

A type satisfies `ReadWriter` exactly when it satisfies both `Reader` and `Writer`. Citation: Effective Go's "Embedding" at <https://go.dev/doc/effective_go#embedding>.

## 10. Putting it together — a small testable seam

Here is the shape you will reuse all track: a consumer-defined interface, a concrete implementation, a function that accepts the interface, and a compile-time assertion.

```go
package mailer

// Sender is what the welcome flow consumes — exactly one method.
// Defined here, at the consumer, sized to what we use.
type Sender interface {
	Send(to, body string) error
}

// SendWelcome accepts the interface, so a test can pass a fake Sender
// and production passes the real one.
func SendWelcome(s Sender, to string) error {
	if err := s.Send(to, "welcome to the crunch"); err != nil {
		return fmt.Errorf("welcome email to %s: %w", to, err)
	}
	return nil
}
```

```go
package smtp

// Client is a concrete struct returned by New (return structs!).
type Client struct{ host string }

func New(host string) *Client { return &Client{host: host} }

func (c *Client) Send(to, body string) error { /* real SMTP */ return nil }

// Compile-time proof *Client satisfies the consumer's interface.
var _ mailer.Sender = (*Client)(nil)
```

The test for `SendWelcome` passes a fake `Sender` whose `Send` records its arguments — no SMTP server, no network. That testability is the *payoff* of accepting an interface, and it is why we drill the rule now: every layer of the Week 5–8 service is wired exactly this way.

## 11. Exercise pointer

Now do **Exercise 1 — Receivers and Interfaces**. You will write a type with both value- and pointer-receiver methods, *predict* which form satisfies a small interface and which does not, define a consumer-side interface, and route values through a type switch. The acceptance criterion is that you can state, from memory, the method-set rule and predict — before compiling — whether `T` or only `*T` satisfies a given interface.

## 12. Summary

- A **method** is a function with a receiver; you can attach methods to any named type your package declares, not just structs, but never to a type from another package.
- **Value receivers** operate on a copy (read-only effect); **pointer receivers** operate on the original (mutation, large types, types holding a mutex). **Keep the receiver kind consistent across a type's whole method set.**
- **The method-set rule:** a value `T`'s method set has only its value-receiver methods; a pointer `*T`'s method set has *both*. So a pointer-receiver method means `*T` satisfies an interface but the value `T` does not — the auto-`&` does not rescue interface assignment.
- **Interfaces are small, implicit, and consumer-defined.** No `implements` keyword; satisfaction is structural. *The bigger the interface, the weaker the abstraction.* Define the interface in the package that calls the method; don't export interfaces "just in case."
- **Accept interfaces, return structs.** Take the narrowest interface you use; return the concrete type so callers keep the full surface, you can add methods without breaking them, and you avoid needless boxing. Prove satisfaction with `var _ Iface = (*T)(nil)`.
- **`any`** (the empty interface) holds any value but discards its type; prefer a type parameter when the type varies but is known at the call site. Recover the type with a **comma-ok type assertion** or a **type switch**; never the panicking one-value assertion on untrusted input.
- **Embedding is composition, not inheritance:** promoted methods, no subtyping, no virtual dispatch, static resolution. Embedding interfaces composes them (`io.ReadWriter` = `Reader` + `Writer`).

Cited references this lecture pulled from: <https://go.dev/doc/effective_go#methods>, <https://go.dev/doc/effective_go#interfaces>, <https://go.dev/doc/effective_go#embedding>, <https://go.dev/tour/methods/1>, <https://go.dev/ref/spec#Method_sets>, <https://go.dev/ref/spec#Type_assertions>, <https://go.dev/wiki/CodeReviewComments#interfaces>, <https://go.dev/doc/faq#guarantee_satisfies_interface>.
