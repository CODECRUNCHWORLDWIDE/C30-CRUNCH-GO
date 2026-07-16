# Lecture 3 — Generics: Type Parameters and Constraints

> **Time:** 2 hours. Take type-parameters-and-constraints first, then generic types and the decision matrix. **Prerequisites:** Lecture 1 (interfaces, `comparable` intuition) and Lecture 2 (errors). **Citations:** the generics tutorial at <https://go.dev/doc/tutorial/generics>, the "Intro to generics" blog post at <https://go.dev/blog/intro-generics>, the "When to use generics" post at <https://go.dev/blog/when-generics>, and the `cmp` package at <https://pkg.go.dev/cmp>.

## 1. Why this lecture

Generics arrived in Go 1.18 and are now load-bearing across the standard library — `slices`, `maps`, and `cmp` are all generic. They solve a problem you have already felt: before generics, a function that worked for any element type took `interface{}`, returned `interface{}`, and forced a type assertion that could panic at runtime — you traded compile-time safety for flexibility. A type parameter buys back the safety: one `Map`, one `Set`, one `Cache` that works for *every* element type, type-checked at compile time, with no boxing and no runtime assertion. This lecture teaches the mechanics (type parameters, constraints, inference) and then the judgement that matters more — *when generics earn their keep and when an interface is the better tool.* Lab 02's cache is generic; its eviction policy and store are interfaces; by the end of this lecture you will know exactly why.

## 2. A generic function

A type parameter list goes in square brackets *before* the ordinary parameters. The simplest useful generic is `Map`:

```go
// Map applies f to every element of s, returning a new slice.
// T is the input element type, U the output element type; both are "any".
func Map[T, U any](s []T, f func(T) U) []U {
	out := make([]U, len(s))
	for i, v := range s {
		out[i] = f(v)
	}
	return out
}
```

```go
nums := []int{1, 2, 3}
strs := Map(nums, func(n int) string { return strconv.Itoa(n) }) // []string{"1","2","3"}
lens := Map([]string{"a", "bb"}, func(s string) int { return len(s) }) // []int{1, 2}
```

`[T, U any]` declares two type parameters, each constrained by `any` (no restriction — any type). Notice the call sites do *not* spell out the types: Go *infers* `T` and `U` from the arguments (`nums` is `[]int` ⇒ `T = int`; the function returns `string` ⇒ `U = string`). Before generics, `Map` was impossible to write once; you copied it per type or used `interface{}` and lost the types. Citation: <https://go.dev/blog/intro-generics>.

## 3. Constraints — what a type parameter is allowed to do

`any` means "no operations available" — inside the function body you can only pass a `T` around, store it, compare it to `nil` if it is an interface. To *do* something with a `T`, you constrain it. A **constraint** is an interface used as a type-parameter bound; it says "the type argument must support these operations." Three built-in constraints carry most code:

### 3.1 `any`

The alias for the empty interface — the unconstrained constraint. Use it for containers and pass-through algorithms that never inspect the value (`Map`'s `U`, a `Stack[T any]`).

### 3.2 `comparable`

`comparable` is the built-in constraint for types that support `==` and `!=` — exactly the types usable as **map keys**. You need it whenever a type parameter is used as a map key or compared with `==`:

```go
// index builds a value-to-position map; K must be comparable to be a map key.
func index[K comparable](s []K) map[K]int {
	m := make(map[K]int, len(s))
	for i, v := range s {
		m[v] = i
	}
	return m
}
```

`index([]string{...})` works (strings are comparable); `index([][]int{...})` is a *compile error* (slices are not comparable). This is why the cache is `Cache[K comparable, V any]`: keys go into a map, so `K` must be `comparable`; values are just stored, so `V` can be `any`. Citation: <https://go.dev/ref/spec#Comparison_operators> and the generics tutorial at <https://go.dev/doc/tutorial/generics>.

### 3.3 `cmp.Ordered` — for `<`, `<=`, `>`, `>=`

`comparable` gives you `==`, not `<`. For ordering you need `cmp.Ordered` (standard library since Go 1.21), the constraint of all types that support the ordering operators — integers, floats, and strings:

```go
import "cmp"

// Max returns the larger of a and b. T must be ordered to use `>`.
func Max[T cmp.Ordered](a, b T) T {
	if a > b {
		return a
	}
	return b
}

Max(3, 7)          // 7    (T = int)
Max("go", "rust")  // "rust" (T = string, lexicographic)
Max(2.5, 1.5)      // 2.5  (T = float64)
```

Before 1.21 you imported `constraints.Ordered` from `golang.org/x/exp/constraints`; with 1.21+ use the standard-library `cmp.Ordered` and need no external dependency. The `cmp` package also gives you `cmp.Compare` and `cmp.Less` for writing comparators, and the `slices` package uses `cmp.Ordered` for `slices.Sort`, `slices.Max`, and friends. Citation: <https://pkg.go.dev/cmp> and <https://pkg.go.dev/slices>.

### 3.4 Custom constraints and union elements

A constraint is just an interface, so you can write your own. A constraint may list *methods* (any type with those methods qualifies) and/or *type elements* — a union of allowed underlying types, joined with `|`:

```go
// Number is a custom constraint: any of these underlying numeric types.
type Number interface {
	~int | ~int64 | ~float64
}

func Sum[T Number](xs []T) T {
	var total T // zero value of T
	for _, x := range xs {
		total += x // allowed: every type in Number supports +
	}
	return total
}
```

The `~` token means "any type whose *underlying* type is this" — so `~int` admits `type Celsius int`, not just `int`. Without `~`, a constraint of `int` would reject `Celsius`. Use `~` whenever you want named types built on the listed primitives to qualify, which is almost always. Citation: <https://go.dev/ref/spec#Type_constraints> and <https://go.dev/blog/intro-generics>.

## 4. Generic types

You can parameterise a *type*, not just a function. The constraint goes in the same bracket position, and every method on the type repeats the type parameters in its receiver.

### 4.1 A generic `Stack[T any]`

```go
// Stack is a LIFO stack of any element type.
type Stack[T any] struct {
	items []T
}

func (s *Stack[T]) Push(v T) { s.items = append(s.items, v) }

func (s *Stack[T]) Pop() (T, bool) {
	var zero T // the zero value of T, used for the empty case
	if len(s.items) == 0 {
		return zero, false
	}
	v := s.items[len(s.items)-1]
	s.items = s.items[:len(s.items)-1]
	return v, true
}

func (s *Stack[T]) Len() int { return len(s.items) }
```

```go
var s Stack[int]   // instantiate with int; the zero value is a usable empty stack
s.Push(1)
s.Push(2)
v, ok := s.Pop()   // v == 2, ok == true
```

Two idioms to note. `var zero T` is how you produce "the zero value of the unknown type `T`" — you cannot write `nil` or `0` because `T` could be either; `var zero T` works for all of them. And `var s Stack[int]` follows Week 1's "make the zero value useful": the zero `Stack` is an empty, ready stack with no constructor. Citation: <https://go.dev/doc/tutorial/generics>.

### 4.2 A generic `Set[T comparable]`

A set needs its elements as map keys, so it constrains `T` to `comparable`:

```go
// Set is an unordered collection of unique comparable elements.
type Set[T comparable] struct {
	m map[T]struct{} // struct{}{} is a zero-byte value: a set, not a map-with-values
}

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
```

```go
s := NewSet("a", "b", "a") // *Set[string]; "a" stored once
s.Has("a")                  // true
s.Len()                     // 2
```

`map[T]struct{}` is the idiomatic Go set: `struct{}` is a zero-byte type, so the map stores keys with no value overhead. The constraint `comparable` is mandatory — without it, `map[T]struct{}` would not compile, because `T` might not be usable as a key. Citation: <https://go.dev/doc/effective_go#maps> and <https://go.dev/ref/spec#Comparison_operators>.

## 5. Instantiation and type inference

*Instantiation* is supplying the type arguments to get a concrete function or type. You can do it explicitly or — usually — let Go infer it:

```go
Map[int, string](nums, itoa) // explicit: T=int, U=string
Map(nums, itoa)              // inferred from the arguments — preferred when it works

var s Stack[int]             // type instantiation is ALWAYS explicit for a type literal
```

Inference works when the type parameters appear in the *ordinary* parameter types (so the compiler can read them off the arguments). It *fails*, and you must instantiate explicitly, when a type parameter appears only in the *return* type or only inside a constraint:

```go
// Zero returns the zero value of T. T is not in any parameter — inference
// has nothing to read it from, so the caller MUST instantiate explicitly.
func Zero[T any]() T {
	var z T
	return z
}

n := Zero[int]()    // explicit required; Zero() alone is a compile error
```

The rule of thumb: if every type parameter shows up in a value argument, omit the brackets; otherwise spell them out. Type literals (`Stack[int]`, `Set[string]`) are always explicit. Citation: <https://go.dev/blog/intro-generics> and the type-inference section of the spec at <https://go.dev/ref/spec#Type_inference>.

## 6. The decision: generics vs interfaces vs neither

This is the part that separates "knows the syntax" from "uses generics well." The Go team's own framing, which you cite in review, is three questions:

> **Use a type parameter when the function body treats every type the same.**
> **Use an interface when the function must behave differently per type.**
> **Use neither when it does neither.**

Mapped onto the kinds of code you write:

| You are writing... | Use | Why |
|---|---|---|
| A **container** holding any element type — `Cache[K,V]`, `Set[T]`, `Stack[T]`, a linked list, a tree | **Generics** | The container logic is identical regardless of element type; a type parameter keeps the element type *and* avoids boxing. |
| A **type-parametric algorithm** — `Map`, `Filter`, `Reduce`, `Min`, `Sort`, `Keys(m)` | **Generics** | The algorithm is the same for every element type; the only thing that varies is the type, which is exactly what a type parameter abstracts. |
| **Polymorphism over behaviour** — many concrete types with the *same method* but *different bodies*: an `io.Writer` to a file vs a socket, a `Store` with in-memory vs file-backed impls, a `Notifier` over email vs Slack | **Interfaces** | The behaviour differs per type; the call site dispatches dynamically to the right body. That is what interfaces are for, and a type parameter cannot express "run different code per type." |
| A function that calls one method on its argument and that method's *body* differs per type | **Interfaces** | Same as above — it must behave differently per type. |
| A function with one concrete type, no abstraction needed | **Neither** | Don't add a type parameter or an interface speculatively. |

The cache in Lab 02 is the textbook illustration of using *both*: the cache *container* is generic (`Cache[K comparable, V any]`) because storing and retrieving values is identical for every `K`/`V`; the *eviction policy* and the *store* are interfaces (`EvictionPolicy`, `Store[K, V]`) because LRU vs FIFO, and in-memory vs file-backed, are genuinely *different behaviour behind the same method set*. Container ⇒ generic; pluggable behaviour ⇒ interface. Citation: <https://go.dev/blog/when-generics>.

## 7. When *not* to use generics

A few concrete anti-patterns the "when generics" post and reviewers call out:

1. **A type parameter used by exactly one type.** `func process[T MyOnlyType](x T)` is generics theatre — write `func process(x MyOnlyType)`. Generics earn their keep across *multiple* type arguments.
2. **A type parameter where an interface method would do.** If your generic function's body does `switch any(v).(type) { ... }` to behave differently per type, you wanted an interface — let each type implement a method and call it. A type switch inside a generic is a smell.
3. **Reaching for generics before there is duplication.** Like interfaces, generics are an abstraction with a reading cost. Write the concrete version; generalise when a *second* element type actually appears. The standard `slices` and `maps` packages already cover `Sort`, `Contains`, `Keys`, `Index`, and most of what a beginner is tempted to re-derive — reach for them before writing your own.
4. **Method type parameters.** Go does *not* allow type parameters on *methods* (only on the type or on free functions). If you find yourself wanting `func (s *Stack[T]) Map[U any](...)`, that is not expressible — make it a free function `Map(s, f)`. Knowing this limitation prevents an hour of fighting the compiler. Citation: <https://go.dev/blog/when-generics> and the proposal note on method type parameters in the generics design.

## 8. A note on runtime cost

Generics are *not* free C++-style monomorphisation and *not* boxed-`interface{}` either — Go uses a hybrid ("GC shape stenciling"): the compiler generates one copy of the code per distinct memory layout (one for all pointer-shaped types, separate ones for distinct value layouts), sharing implementations where it can. The practical upshot for this week: generics give you *type safety* and avoid the *runtime type assertion* of the `interface{}` approach, and you should reach for them for containers and algorithms without worrying about a performance penalty at the beginner level. We measure real costs with benchmarks in Week 8; for now, "generic container, no assertion, type-safe" is the win. Citation: <https://go.dev/blog/intro-generics>.

## 9. Exercise pointer

Now do **Exercise 3 — Generics**. You will complete a generic `Set[T comparable]` and a small `Map`/`Filter`/`Reduce` trio, then fill in a table-test skeleton that instantiates them at two different element types. PREDICT comments ask which calls compile (the `comparable` ones) and which fail. The acceptance criterion is a green `go test ./...`, clean `go vet`/`staticcheck`, and a one-sentence answer to "is each of these a container, an algorithm, or neither — and would an interface have been better?"

## 10. Summary

- A **type parameter** goes in `[ ]` before the ordinary parameters: `func Map[T, U any](...)`, `type Stack[T any] struct{...}`. Methods on a generic type repeat the parameters in the receiver: `func (s *Stack[T]) Push(v T)`.
- A **constraint** is an interface used as a bound; it says what operations the type argument supports. `any` = no operations; **`comparable`** = `==`/`!=` and map-key usable; **`cmp.Ordered`** (1.21+) = the ordering operators; custom constraints list methods and/or a `|`-union of underlying types with `~`.
- **`var zero T`** produces the zero value of an unknown type; use it for empty/missing cases in generic code.
- **Inference** lets you omit type arguments when every type parameter appears in a value argument; instantiate explicitly when a parameter is only in the return type or in a type literal (`Stack[int]`).
- **The decision:** *generics* for containers and type-parametric algorithms (same logic, varying type); *interfaces* for polymorphism over behaviour (same method, different bodies per type); *neither* when no abstraction is needed. Lab 02's cache is generic; its policy and store are interfaces.
- **Don't** parameterise for one type, don't type-switch inside a generic (use an interface), don't generalise before duplication appears, and remember Go has no *method* type parameters.
- Generics are type-safe and assertion-free with no beginner-level performance penalty; reach for `slices`/`maps`/`cmp` before re-deriving the basics.

Cited references this lecture pulled from: <https://go.dev/doc/tutorial/generics>, <https://go.dev/blog/intro-generics>, <https://go.dev/blog/when-generics>, <https://pkg.go.dev/cmp>, <https://pkg.go.dev/slices>, <https://go.dev/ref/spec#Type_constraints>, <https://go.dev/ref/spec#Type_inference>.
