# Week 1 — The Go Tour and the Toolchain: Modules, the Standard Toolchain, Idiomatic Basics, and Table-Driven Tests

Welcome to **C30 · Crunch Go**, Week 1. You arrive here already fluent in at least one typed language — Java, C#, TypeScript, Rust, modern C++, or Swift — and the job of this first week is not to teach you what a `for` loop is. It is to rewire the small set of instincts that a Go reviewer can spot in thirty seconds: the instinct to reach for a class hierarchy, the instinct to throw an exception, the instinct to reach for a framework before you have written ten lines of standard library. Go is a deliberately small language, and the smallness is the point. By Friday you will have a static binary that counts word frequencies, a table-driven test suite that runs clean under `go vet`, and the beginning of an answer to the question that runs through the whole track: *what does the toolchain actually do, and what does the binary actually contain?*

The first thing to internalize is that **a Go program is a set of packages, and a buildable unit is a module**. A module is a directory tree with a `go.mod` file at its root that names the module path (`module github.com/you/wordfreq`) and the Go version it targets (`go 1.22`). Every `.go` file declares the package it belongs to on its first line (`package main` for an executable, `package wordfreq` for a library). The compiler does not care about file names; it cares about package declarations and the import graph. There is no project file listing every source file, no `csproj`, no `pom.xml` — `go build ./...` finds every package under the current directory by walking the tree and reading the `package` clauses. The canonical reference is the modules reference at <https://go.dev/ref/mod> and the "Tutorial: Create a Go module" at <https://go.dev/doc/tutorial/create-module>. Read `go.mod` once and you have read the entire build configuration of a Go project; there is nothing hidden behind it.

The second thing to internalize is **the standard toolchain is the whole development environment**. `go build` compiles, `go run` compiles-and-runs, `go install` compiles-and-installs to `$GOBIN`, `go test` runs tests, `go vet` reports likely mistakes the compiler accepts, and `gofmt` (run for you by `go fmt`) reformats source to the one canonical style so that no team ever argues about brace placement again. There is no separate build system, no separate test runner, no separate formatter to configure. `staticcheck` (a third-party linter at <https://staticcheck.dev>) adds a deeper layer of analysis on top of `go vet`, and it is the de facto standard for a serious Go codebase. The week's discipline is that every artifact you ship is clean under `go vet` and `staticcheck` — a warning is a bug you have not fixed yet.

The third thing to internalize is **the zero value, and why Go has no constructors**. Every type in Go has a zero value — `0` for numbers, `""` for strings, `nil` for pointers, slices, maps, channels, and interfaces, and a struct whose every field is its own zero value. A freshly declared variable is never "uninitialized garbage"; it is its zero value, guaranteed. Idiomatic Go leans on this hard: a `bytes.Buffer` is useful at its zero value with no `New`, a `sync.Mutex` is ready to lock at its zero value, a `var m map[string]int` is a readable (if not writable) nil map. The design instinct you are building is "make the zero value useful," and it is the opposite of the constructor-everywhere instinct from Java or C#. Citation: the "zero value" section of the Tour at <https://go.dev/tour/basics/12> and Effective Go's discussion of allocation at <https://go.dev/doc/effective_go#allocation_new>.

The fourth thing to internalize is **errors are values, not control flow**. Go has no exceptions. A function that can fail returns an `error` as its last return value, and the caller checks it explicitly: `f, err := os.Open(name); if err != nil { return err }`. This is more verbose than a `try`/`catch` and that verbosity is deliberate — the error path is in your face on every line that can fail, not hidden in a stack-unwinding mechanism three frames up. The `panic`/`recover` mechanism exists, but it is for *programmer mistakes* (a nil-pointer dereference, an out-of-bounds index) and for truly unrecoverable situations, not for ordinary failure. The mantra, which you will hear all week, is "don't panic; return an error." We cover error *values* in depth in Week 2; this week we just establish the reflex of checking `err` on every call that returns one. Citation: Effective Go's errors section at <https://go.dev/doc/effective_go#errors>.

The fifth thing to internalize is **slices and maps are the two workhorse data structures, and you must understand their reference semantics**. A slice is a small three-word header — a pointer to a backing array, a length, and a capacity — passed by value. That means passing a slice to a function copies the header (cheap) but shares the backing array (so the function can mutate your elements). `append` may or may not allocate a new backing array depending on capacity, which is the source of the single most common Go surprise for newcomers. A map is a reference to a hash table; passing it to a function shares the table. Both have a nil zero value, and the asymmetry between them — a nil slice is safe to `append` to and `range` over, a nil map is safe to read but panics on write — is exactly the kind of detail a Go reviewer probes. Citation: the Go blog's "Go Slices: usage and internals" at <https://go.dev/blog/slices-intro> and "Go maps in action" at <https://go.dev/blog/maps>.

The sixth thing to internalize is **`defer` is the cleanup primitive, and it runs LIFO at function return**. `defer f.Close()` immediately after `f, err := os.Open(...)` (and after the error check) guarantees the file is closed however the function returns — normal return, early return, or panic. Multiple `defer`s in a function run in last-in-first-out order. The arguments to a deferred call are evaluated at the point of the `defer` statement, not at the point it runs, which is a subtlety worth burning into memory now. `defer` is how Go does `try`/`finally` without the `try`; it co-locates the acquire and the release on adjacent lines, which is far easier to review than a `finally` block forty lines down. Citation: the "Defer, Panic, and Recover" blog post at <https://go.dev/blog/defer-panic-and-recover>.

The seventh thing to internalize is **table-driven tests are the default test shape in Go**. The `testing` package ships in the standard library; a test is a function `func TestXxx(t *testing.T)` in a file named `*_test.go`, and `go test` runs every such function. The idiomatic shape is not one test function per case but one test function holding a *table* — a slice of structs, each a `{name, input, want}` case — iterated with `t.Run(tc.name, func(t *testing.T) { ... })` so each case is a named subtest you can run in isolation with `go test -run TestFoo/specific_case`. There is no assertion framework in the standard library and you do not need one: a test fails by calling `t.Errorf("got %v, want %v", got, want)`. Citation: the `testing` package docs at <https://pkg.go.dev/testing> and the "Add a test" tutorial at <https://go.dev/doc/tutorial/add-a-test>. The Go wiki's "TableDrivenTests" page (<https://go.dev/wiki/TableDrivenTests>) is the canonical pattern reference.

The eighth thing to internalize is **what a Go binary actually is**. `go build` on a `package main` produces a single statically linked native executable. With `CGO_ENABLED=0`, that binary has no dynamic library dependencies at all — you can `scp` it to a bare Linux box with nothing installed and it runs. This is the property the whole cloud-native world is built on: a Go service ships as one file, into a `FROM scratch` container if you want, with no runtime to install. The binary embeds the Go runtime (the garbage collector and scheduler are part of every Go program), which is why a "hello world" is a few megabytes rather than a few kilobytes — a trade Go makes deliberately, and one we will revisit in Week 10 when we make the binary tiny and the container distroless. This week you build the binary, inspect its size with `ls -lh` and `go version -m`, and prove it has no dynamic dependencies with `ldd` (or `otool -L` on macOS). Citation: the `go build` command docs at <https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies>.

By the end of the week you will be the person who can take an empty directory, `go mod init`, write a small CLI with idiomatic slices and maps, give it a table-driven test suite, run the whole standard toolchain clean, and hand someone a single static binary that just works. That is the foundation every later week stands on.

## Learning objectives

By the end of this week, you will be able to:

- **Initialize** a Go module with `go mod init <path>`, lay out packages with correct `package` declarations and export visibility (capitalized = exported), and build the whole tree with `go build ./...`. Cite <https://go.dev/ref/mod> and <https://go.dev/doc/tutorial/create-module>.
- **Run** the standard toolchain — `go build`, `go run`, `go install`, `go test`, `go vet`, `gofmt`/`go fmt` — and add `staticcheck` for deeper analysis. Reach a state where `go vet ./...` and `staticcheck ./...` both report nothing. Cite <https://pkg.go.dev/cmd/go> and <https://staticcheck.dev>.
- **Declare** variables with `var` and `:=`, and explain when each is idiomatic; reason about the zero value of every built-in type and why "make the zero value useful" is a design instinct. Cite <https://go.dev/tour/basics/12> and <https://go.dev/doc/effective_go#allocation_new>.
- **Use** slices correctly — the length/capacity/backing-array model, `append`'s reallocation behaviour, the shared-backing-array aliasing trap, and the nil-slice-is-usable property. Cite <https://go.dev/blog/slices-intro>.
- **Use** maps correctly — the comma-ok read idiom (`v, ok := m[k]`), the nil-map read-vs-write asymmetry, and the unordered iteration guarantee (there is none). Cite <https://go.dev/blog/maps>.
- **Apply** `defer` for cleanup, predict LIFO ordering, and explain when the deferred call's arguments are evaluated. Cite <https://go.dev/blog/defer-panic-and-recover>.
- **Check** errors as values on every call that returns one, and articulate why Go has no exceptions and why `panic` is not error handling. Cite <https://go.dev/doc/effective_go#errors>.
- **Author** a table-driven test suite with `t.Run` subtests, failing via `t.Errorf` with a `got`/`want` message, and run a single case with `go test -run`. Cite <https://pkg.go.dev/testing> and <https://go.dev/wiki/TableDrivenTests>.
- **Read** standard-library source as the reference for idiomatic Go — walk a function from the `strings` package and explain why it is written the way it is. Cite <https://pkg.go.dev/strings>.
- **Build** a `CGO_ENABLED=0` static binary, inspect its size and embedded build metadata with `go version -m`, and prove it has no dynamic dependencies. Cite <https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies>.

## Standards this week meets

| Bar | What this week is measured against |
| --- | --- |
| University | `COP 4813` — Past the outcome set: neither ledger course examines a language toolchain in its own right, because both assume you already have one. Week 1 supplies that assumption — build, run, test, vet and format a module, and read the standard library as the reference for how the language is written. |
| Industry | Set up a service repository a stranger can clone and build: a module with its dependencies pinned in a lockfile, a formatter and two static analysers reporting nothing, a table-driven test suite, and one static binary that runs on a machine with no runtime installed. |
| Beyond the bar | The same trivial program is built four ways and weighed each time, and the build metadata is read back out of the shipped artifact with `go version -m` — so "where did 8 MB come from" is answered with evidence rather than a guess — `challenges/challenge-01-static-binary-autopsy.md` |

## Prerequisites

- **A typed language under your belt.** You can read and write functions, loops, conditionals, and basic data structures in at least one of Java, C#, TypeScript, Rust, C++, or Swift. C30 does not teach programming from zero; it teaches Go to programmers.
- **A Linux/macOS/WSL2 shell.** You can navigate directories, set an environment variable, and run a command. C14 (Crunch Linux) is the prerequisite if this is shaky.
- **The Go toolchain, 1.22 or newer.** Install from <https://go.dev/dl/>. Verify with `go version`; you should see `go version go1.22.x` or later. The track assumes generics, `slog`, and the 1.22 `net/http` routing, all of which require a current release.
- **`staticcheck`.** Install with `go install honeycomb.io/...` — no: `go install honnef.co/go/tools/cmd/staticcheck@latest`. Verify with `staticcheck --version`.
- **An editor with `gopls`.** VS Code with the Go extension, GoLand, or Neovim with `gopls` (the official language server, <https://pkg.go.dev/golang.org/x/tools/gopls>). You want format-on-save and inline `go vet` from day one.

## Topics covered

- **Modules.** `go mod init`, the `go.mod` file (module path, Go version, `require` directives), `go mod tidy`, the module cache, semantic import versioning at a high level.
- **Package layout and visibility.** One directory = one package; the `package main` / `func main()` entry point; capitalized identifiers are exported, lowercase are package-private; the `internal/` directory convention.
- **The build commands.** `go build` vs `go run` vs `go install`; `go build ./...` to build everything; cross-compilation with `GOOS`/`GOARCH`; `CGO_ENABLED=0` for a fully static binary.
- **Formatting and analysis.** `gofmt`/`go fmt` (non-negotiable, one canonical style), `go vet` (likely mistakes), `staticcheck` (deeper analysis), the "clean under all three" discipline.
- **Declarations and the zero value.** `var` vs `:=`, the zero value of every built-in type, "make the zero value useful," constants and `iota`.
- **Basic types.** Numbers (sized integers, `int`, `float64`), strings (immutable, UTF-8, `byte` vs `rune`), booleans; conversions are explicit (no implicit numeric coercion).
- **Slices.** The three-word header, `len`/`cap`, `make([]T, len, cap)`, `append` and reallocation, slicing (`s[1:3]`) and the shared backing array, the nil slice.
- **Maps.** `make(map[K]V)`, the comma-ok read, `delete`, iteration order is randomized, the nil-map write panic, maps are not safe for concurrent writes (a Week 3/4 callback).
- **Structs.** Struct literals, field access, embedded structs (composition, not inheritance), struct comparison, the empty struct `struct{}`.
- **Functions.** Multiple return values, named returns (and when not to use them), variadic functions, first-class functions and closures.
- **`defer`, `panic`, `recover`.** `defer` for cleanup, LIFO ordering, argument-evaluation timing; `panic`/`recover` for programmer errors only.
- **Errors as values.** The `error` interface, returning `error` as the last value, the `if err != nil` reflex, why no exceptions.
- **The `testing` package.** `func TestXxx(t *testing.T)`, `*_test.go` files, `t.Run` subtests, table-driven tests, `t.Errorf` vs `t.Fatalf`, `go test -v`, `go test -run`, `go test -cover`.
- **Reading the standard library.** Using `pkg.go.dev` and the source browser to read `strings`, `bufio`, and `os` as the reference for idiomatic Go.
- **The binary.** What `go build` produces, the embedded runtime, static linking, `go version -m`, inspecting size and dependencies.

## Weekly schedule

The schedule adds up to approximately **36 hours**. Treat it as a target, not a contract. The toolchain material rewards repetition; run `go test ./...` and `go vet ./...` after every change until the muscle memory is automatic.

| Day       | Focus                                                              | Lectures | Exercises | Challenges | Quiz/Read | Homework | Mini-Project | Self-Study | Daily Total |
|-----------|-------------------------------------------------------------------|---------:|----------:|-----------:|----------:|---------:|-------------:|-----------:|------------:|
| Monday    | Modules, the toolchain, packages, the static binary               |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Tuesday   | Zero value, declarations, slices, maps, structs, `defer`          |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Wednesday | Functions, errors-as-values, the `testing` package, table tests   |    2h    |    1.5h   |     0h     |    0.5h   |   1h     |     0.5h     |    0.5h    |     6h      |
| Thursday  | Challenges, reading the standard library, `go vet`/`staticcheck`  |    0.5h  |    0h     |     2.5h   |    0.5h   |   1h     |     1.5h     |    0.5h    |     6.5h    |
| Friday    | Mini-project — build the `wordfreq` CLI                            |    0h    |    0h     |     0.5h   |    0.5h   |   0h     |     4h       |    0.5h    |     5.5h    |
| Saturday  | Mini-project polish, binary inspection, test-coverage pass        |    0h    |    0h     |     0h     |    0h     |   0h     |     2.5h     |    0h      |     2.5h    |
| Sunday    | Quiz, review, "what maps from my old language and what does not"  |    0h    |    0h     |     0h     |    1h     |   0h     |     0.5h     |    0h      |     1.5h    |
| **Total** |                                                                   | **8.5h** | **7.5h**  | **5.5h**   | **4h**    | **5h**   | **9.5h**     | **3h**     | **36h**     |

## How to navigate this week

| File | What's inside |
|------|---------------|
| [README.md](./README.md) | This overview (you are here) |
| [resources.md](./resources.md) | A Tour of Go, Effective Go, the Go spec, Go by Example, the standard-library docs, the testing docs |
| [lecture-notes/01-modules-the-toolchain-and-the-static-binary.md](./lecture-notes/01-modules-the-toolchain-and-the-static-binary.md) | Modules, `go.mod`, packages and visibility, the build commands, `gofmt`/`vet`/`staticcheck`, the static binary |
| [lecture-notes/02-the-zero-value-slices-maps-structs-and-defer.md](./lecture-notes/02-the-zero-value-slices-maps-structs-and-defer.md) | The zero value, declarations, slices and their backing-array model, maps, structs, `defer` |
| [lecture-notes/03-functions-errors-as-values-and-table-driven-tests.md](./lecture-notes/03-functions-errors-as-values-and-table-driven-tests.md) | Functions and closures, errors-as-values, the `testing` package, the table-driven test pattern |
| [exercises/exercise-01-module-and-toolchain.go](./exercises/exercise-01-module-and-toolchain.go) | Stand up a module, build a static binary, run the whole toolchain clean |
| [exercises/exercise-02-slices-maps-structs.go](./exercises/exercise-02-slices-maps-structs.go) | Slice aliasing, the comma-ok map read, struct composition, `defer` ordering |
| [exercises/exercise-03-errors-and-table-tests.go](./exercises/exercise-03-errors-and-table-tests.go) | A function that returns errors, with a table-driven test suite that runs clean under `go vet` |
| [exercises/SOLUTIONS.md](./exercises/SOLUTIONS.md) | Annotated solutions for the three exercises, with the toolchain output you should reproduce |
| [challenges/challenge-01-static-binary-autopsy.md](./challenges/challenge-01-static-binary-autopsy.md) | Build the same program four ways and explain every byte of size difference |
| [challenges/challenge-02-stdlib-source-walk.md](./challenges/challenge-02-stdlib-source-walk.md) | Read `strings.Fields` and `bufio.Scanner` from source and write up why they are idiomatic |
| [quiz.md](./quiz.md) | 10 multiple-choice questions on modules, the toolchain, the zero value, slices, maps, errors, and tests |
| [homework.md](./homework.md) | Six practice problems for the week |
| [mini-project/README.md](./mini-project/README.md) | Full spec for **Lab 01 — `wordfreq` CLI**: word-frequency counter, top-20 Markdown table, stdin support, table tests, static binary |

## The "clean under the toolchain" promise

C30 treats three commands as a contract from Week 1 forward. Every artifact you ship in this week's exercises and the mini-project must produce empty output from all three:

```
$ go vet ./...
$ staticcheck ./...
$ go test ./...
ok      github.com/you/wordfreq        0.012s
```

A `go vet` warning is not a style nit; it is the toolchain telling you about a bug the compiler was willing to accept — a `Printf` format that does not match its arguments, a lock copied by value, an unreachable branch. A `staticcheck` warning is a deeper version of the same. The rule, restated every week of this track: **a warning is a bug you have not fixed yet.** A pull request that adds code under an unaddressed `go vet` or `staticcheck` finding is, by definition, a pull request that is not ready for review.

> **Note on the toolchain.** Everything this week ships with the Go toolchain itself (`go`, `gofmt`) plus one external tool, `staticcheck` (`go install honnef.co/go/tools/cmd/staticcheck@latest`). No module dependencies are required — the `wordfreq` CLI is built entirely on the standard library (`bufio`, `os`, `strings`, `sort`, `unicode`). That is itself a lesson: a surprising amount of real Go is standard-library-only, and reaching for a dependency is a decision you justify, not a default.
