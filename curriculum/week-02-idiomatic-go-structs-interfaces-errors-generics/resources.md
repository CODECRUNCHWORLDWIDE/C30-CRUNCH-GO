# Week 2 — Resources

Every resource on this page is **free**. The Go website (`go.dev`), the package documentation (`pkg.go.dev`), the Go blog, and the Go source on GitHub are all free and require no account. `staticcheck` is open-source (MIT). No paywalled material is linked.

## Required reading (work it into your week)

### Methods, receivers, and interfaces

- **Effective Go — Methods** — pointer vs value receivers and the rationale:
  <https://go.dev/doc/effective_go#methods>
- **Effective Go — Interfaces and other types** — small interfaces, implicit satisfaction, the idiomatic patterns:
  <https://go.dev/doc/effective_go#interfaces>
- **A Tour of Go — Methods and interfaces** — the interactive module; do "Methods," "Interfaces," "Type assertions," and "Type switches":
  <https://go.dev/tour/methods/1>
- **The spec — Method sets** — the precise rule that governs interface satisfaction (the value set excludes pointer-receiver methods):
  <https://go.dev/ref/spec#Method_sets>
- **The spec — Type assertions** — the comma-ok form vs the panicking form:
  <https://go.dev/ref/spec#Type_assertions>
- **Go Code Review Comments — Interfaces & Receiver Type** — the reviewer's instincts: keep interfaces small, define them at the consumer, choose receivers consistently:
  <https://go.dev/wiki/CodeReviewComments#interfaces> and <https://go.dev/wiki/CodeReviewComments#receiver-type>
- **Effective Go — Embedding** — composition, promoted methods, why it is not inheritance:
  <https://go.dev/doc/effective_go#embedding>

### Errors: wrapping, sentinel, typed

- **The `errors` package** — `New`, `Is`, `As`, `Unwrap`, `Join`; read every function:
  <https://pkg.go.dev/errors>
- **`fmt.Errorf`** — the `%w` verb and how it builds a chain:
  <https://pkg.go.dev/fmt#Errorf>
- **Working with Errors in Go 1.13** — the canonical post on `%w`, `errors.Is`, `errors.As`, and when to wrap vs annotate:
  <https://go.dev/blog/go1.13-errors>
- **Error handling and Go** — the foundational post on errors as values, sentinel vs typed, and error design:
  <https://go.dev/blog/error-handling-and-go>
- **Defer, Panic, and Recover** — the reminder that `panic` is not error handling:
  <https://go.dev/blog/defer-panic-and-recover>

### Generics

- **Tutorial: Getting started with generics** — the hands-on introduction to type parameters and constraints:
  <https://go.dev/doc/tutorial/generics>
- **An Introduction to Generics** — the blog post: type parameters, constraints, inference, the `~` token, runtime model:
  <https://go.dev/blog/intro-generics>
- **When To Use Generics** — the decision framework (same logic ⇒ type parameter; different behaviour per type ⇒ interface; neither ⇒ neither):
  <https://go.dev/blog/when-generics>
- **The spec — Type parameters & Type constraints** — the precise rules for constraints, union elements, and `~`:
  <https://go.dev/ref/spec#Type_parameter_declarations> and <https://go.dev/ref/spec#Type_constraints>
- **The spec — Type inference** — when you can omit type arguments and when you cannot:
  <https://go.dev/ref/spec#Type_inference>

### The generic standard library you will read this week

- **`cmp`** — `Ordered`, `Compare`, `Less` (the constraint and helpers for ordering):
  <https://pkg.go.dev/cmp>
- **`slices`** — generic `Sort`, `Contains`, `Index`, `Max`, `Equal` — the canonical generic algorithms:
  <https://pkg.go.dev/slices>
- **`maps`** — generic `Keys`, `Values`, `Clone`, `Equal`:
  <https://pkg.go.dev/maps>
- **`container/list`** — the doubly linked list you use for an O(1) LRU policy in the challenge and mini-project:
  <https://pkg.go.dev/container/list>
- **`encoding/json`** — used by the file-backed store in the mini-project:
  <https://pkg.go.dev/encoding/json>

## Recommended reading (after the required set)

- **Go by Example — Interfaces, Errors, Generics** — short runnable examples:
  <https://gobyexample.com/interfaces>, <https://gobyexample.com/errors>, <https://gobyexample.com/generics>
- **Go FAQ — "Why does Go not have...?"** — the designers on no inheritance, no `implements`, and the interface model:
  <https://go.dev/doc/faq#inheritance> and <https://go.dev/doc/faq#guarantee_satisfies_interface>
- **The Go Blog index** — the errors and generics posts above all live here:
  <https://go.dev/blog/>
- **"The Go Programming Language" (Donovan & Kernighan)** — Chapters 6 (methods) and 7 (interfaces) cover this week in depth. Not free, but the canonical text:
  <https://www.gopl.io/>

## Tools (already installed from Week 1)

- **The Go toolchain** (1.22+, generics require 1.18+; `cmp.Ordered` and `slices`/`maps` are standard as of 1.21): <https://go.dev/dl/>.
- **`staticcheck`**: `go install honnef.co/go/tools/cmd/staticcheck@latest`. It carries forward the week's clean-under-analysis contract and earns its keep on receivers (`%w` misuse, copied locks).
- **An editor with `gopls`**: generics-aware completion makes the constraint material far easier to learn by experiment: <https://pkg.go.dev/golang.org/x/tools/gopls>.

## Citations policy

This curriculum cites `go.dev` (Effective Go, the tour, the spec, the blog, the tutorials), `pkg.go.dev` (the package documentation and source), the Go GitHub source, and `staticcheck.dev` as the primary references. Every example in the lecture notes and exercises traces back to one of these. When a third-party reference (Go by Example, the GOPL book) is the clearer source, it is cited explicitly with a URL — never paraphrased without attribution. If a citation is missing from a section of these notes, treat it as a bug and open an issue against the C30 curriculum repository.
