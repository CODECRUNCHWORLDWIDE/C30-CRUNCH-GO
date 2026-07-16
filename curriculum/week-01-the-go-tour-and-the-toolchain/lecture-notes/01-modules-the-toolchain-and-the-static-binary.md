# Lecture 1 — Modules, the Toolchain, and the Static Binary

> **Time:** 2 hours. Take the module-and-package material in one sitting and the toolchain-and-binary material in a second sitting. **Prerequisites:** a typed language and a working shell. **Citations:** the modules reference at <https://go.dev/ref/mod>, the "Create a Go module" tutorial at <https://go.dev/doc/tutorial/create-module>, the `go` command docs at <https://pkg.go.dev/cmd/go>, and Effective Go at <https://go.dev/doc/effective_go>.

## 1. Why this lecture first

You already know how to program. What you do not yet know is how a Go *project* is shaped, how its build is configured, and what comes out the other end. Those three things — module, toolchain, binary — are the floor that every later week stands on. Week 5's HTTP service is a module. Week 10's container holds the binary this lecture teaches you to build. Week 11's Kubernetes deployment ships that same binary. If the module and the toolchain are muscle memory by Friday, the rest of the track is about Go the language; if they are not, every week fights the build system instead of the problem. So we start here, at the floor.

The good news is that the floor is small. Go's entire build configuration for a typical project is one file, `go.mod`, usually under ten lines. There is no separate build system to learn, no plugin ecosystem, no `Makefile` required. The `go` command is the build system, the test runner, the formatter driver, the dependency manager, and the documentation server, all in one binary that shipped with your install.

## 2. A module from nothing

Open an empty directory and create a module:

```sh
$ mkdir wordfreq && cd wordfreq
$ go mod init github.com/you/wordfreq
go: creating new go.mod: module github.com/you/wordfreq
$ cat go.mod
module github.com/you/wordfreq

go 1.22
```

Three things are worth naming:

1. **The module path** (`github.com/you/wordfreq`) is the import prefix for every package in this module. A package in the subdirectory `internal/count` is imported as `github.com/you/wordfreq/internal/count`. The path conventionally matches the repository URL so that `go get` can fetch it, but for a program you never publish, any unique string works. It is not a URL the toolchain fetches at build time; it is a name.
2. **The `go 1.22` line** declares the language version this module targets. It gates which language features the compiler accepts and which toolchain it will, if necessary, download. Set it to the version you have or newer.
3. **There is nothing else.** No list of source files, no compiler flags, no output paths. The `go` command discovers source files by walking the tree and reading `package` declarations.

Now write the smallest program:

```go
// main.go
package main

import "fmt"

func main() {
	fmt.Println("hello, crunch")
}
```

```sh
$ go run .
hello, crunch
```

`go run .` compiled `main.go` to a temporary binary, ran it, and deleted it. The `.` means "the package in the current directory." Citation: the run command at <https://pkg.go.dev/cmd/go#hdr-Compile_and_run_Go_program>.

## 3. Packages and visibility

A Go package is **a directory of `.go` files that all declare the same package name on their first line**. The compiler unit is the package; the file is just a way to split a package across multiple files. Two rules govern packages:

1. **`package main` with a `func main()` is an executable.** Any other package name is a library, importable but not runnable. A repository can hold one or many `main` packages (one per command), conventionally under `cmd/`.
2. **Capitalization is visibility.** An identifier whose name begins with an uppercase letter (`Count`, `WordFrequency`) is *exported* — visible to code in other packages that import this one. An identifier beginning with a lowercase letter (`count`, `wordFrequency`) is package-private. There is no `public`/`private`/`internal` keyword; the case of the first letter is the access modifier. This is unusual coming from Java or C# and it is load-bearing: you can tell a type's visibility from its name alone, with no declaration to hunt for.

Split the program into a library package and a `main`:

```go
// internal/count/count.go
package count

import (
	"sort"
	"strings"
)

// Pair is one word and its frequency. Exported because main reads it.
type Pair struct {
	Word  string
	Count int
}

// Top returns the n most frequent words in text, most-frequent first,
// ties broken alphabetically. Exported: it is the package's API.
func Top(text string, n int) []Pair {
	freq := make(map[string]int)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		freq[w]++
	}
	pairs := make([]Pair, 0, len(freq))
	for w, c := range freq {
		pairs = append(pairs, Pair{Word: w, Count: c})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Count != pairs[j].Count {
			return pairs[i].Count > pairs[j].Count // higher count first
		}
		return pairs[i].Word < pairs[j].Word // tie: alphabetical
	})
	if n > len(pairs) {
		n = len(pairs)
	}
	return pairs[:n]
}
```

```go
// main.go
package main

import (
	"fmt"

	"github.com/you/wordfreq/internal/count"
)

func main() {
	for _, p := range count.Top("the cat sat on the mat the cat", 3) {
		fmt.Printf("%-6s %d\n", p.Word, p.Count)
	}
}
```

```sh
$ go run .
the    3
cat    2
mat    1
```

Two conventions worth naming:

- **`internal/`** is special to the toolchain. A package under any `internal/` directory may be imported only by code rooted at the parent of that `internal/`. So `internal/count` is importable by our `wordfreq` module but not by anyone who imports our module. It is Go's "package-private to this module" mechanism. Citation: <https://pkg.go.dev/cmd/go#hdr-Internal_Directories>.
- **The import path is the module path plus the directory.** No relative imports (`../count`) in modules; you always write the full path. The toolchain resolves it against `go.mod`.

## 4. The build commands

Five commands carry you through the week. Memorize the difference:

| Command | What it does | When you reach for it |
|---|---|---|
| `go run .` | compile + run + discard the binary | iterating on a program |
| `go build` | compile to a binary in the current directory | producing a deliverable |
| `go build ./...` | compile every package in the tree (checks it compiles; no output for libraries) | "does the whole thing build" |
| `go install` | compile + copy the binary to `$GOBIN` (default `~/go/bin`) | putting a CLI on your `PATH` |
| `go test ./...` | run every test in the tree | always, after every change |

`go build` with no arguments builds the package in the current directory; for a `main` package it writes an executable named after the directory (`wordfreq`):

```sh
$ go build
$ ls -lh wordfreq
-rwxr-xr-x  1 you  staff   1.9M  wordfreq
$ ./wordfreq
the    3
...
```

The `./...` pattern is "this directory and everything under it, recursively." `go build ./...` is the cheapest way to ask "does my whole module still compile" — it produces no output on success and lists compile errors on failure. Citation: <https://pkg.go.dev/cmd/go#hdr-Package_lists_and_patterns>.

### 4.1 Cross-compilation

Go cross-compiles by setting two environment variables — `GOOS` (target OS) and `GOARCH` (target CPU) — with no cross-toolchain to install:

```sh
$ GOOS=linux GOARCH=amd64 go build -o wordfreq-linux-amd64
$ GOOS=darwin GOARCH=arm64 go build -o wordfreq-darwin-arm64
$ file wordfreq-linux-amd64
wordfreq-linux-amd64: ELF 64-bit LSB executable, x86-64, statically linked
```

From a Mac you just built a Linux binary. This is the property that makes Go containers trivial: your CI on macOS or Windows produces the exact Linux ELF that runs in the cluster. Citation: the list of valid `GOOS`/`GOARCH` pairs is in the installation docs at <https://go.dev/doc/install/source#environment>.

## 5. `gofmt`, `go vet`, and `staticcheck`

Go ships one canonical source format and a tool that enforces it. `gofmt` (invoked for a whole package by `go fmt`) reformats source — tabs for indentation, one canonical brace style, sorted imports — so that no Go team ever debates formatting. Run it; do not fight it. Most editors run it on save via `gopls`.

```sh
$ go fmt ./...
internal/count/count.go
```

(It prints the files it changed; an already-formatted tree prints nothing.)

`go vet` is the next layer: it reports constructs that compile but are almost certainly mistakes. The classics:

```go
fmt.Printf("%d\n", "not a number") // vet: Printf format %d has arg of wrong type string
var mu sync.Mutex
locked := mu // vet: assignment copies lock value (a copied mutex is a bug)
```

```sh
$ go vet ./...
# github.com/you/wordfreq
./main.go:8:2: Printf format %d has arg "not a number" of wrong type string
```

`staticcheck` (the third-party linter at <https://staticcheck.dev>, install with `go install honnef.co/go/tools/cmd/staticcheck@latest`) is a deeper analysis still — dead code, inefficient string operations, ignored errors, redundant conditions. It is the de facto standard quality gate for serious Go.

```sh
$ staticcheck ./...
internal/count/count.go:21:2: this value of err is never used (SA4006)
```

The week's contract is that all three produce nothing:

```sh
$ go fmt ./... && go vet ./... && staticcheck ./... && echo "clean"
clean
```

A finding is a bug you have not fixed yet. Citations: `go vet` at <https://pkg.go.dev/cmd/vet>, `staticcheck` checks catalogue at <https://staticcheck.dev/docs/checks/>.

## 6. Dependencies, when you have them

This week's mini-project has no dependencies — it is standard-library only. But you should know the shape for when you do. Adding an import of an external package and running `go mod tidy` records it:

```go
import "github.com/go-chi/chi/v5" // we use this in Week 5
```

```sh
$ go mod tidy
$ cat go.mod
module github.com/you/service

go 1.22

require github.com/go-chi/chi/v5 v5.0.12
```

`go mod tidy` adds the `require` directives for everything you import and removes the ones you no longer use. A second file, `go.sum`, records the cryptographic checksums of every dependency (and its dependencies) so a build is reproducible and tamper-evident. You commit both `go.mod` and `go.sum`. The downloaded modules live in a shared cache at `$GOPATH/pkg/mod`, not in your project — there is no `node_modules` per project. Citation: <https://go.dev/ref/mod#go-mod-tidy>.

## 7. The static binary — what `go build` actually produces

Run `go build` on a `main` package and you get **one statically linked native executable that embeds the Go runtime**. "Embeds the runtime" means the garbage collector, the goroutine scheduler, and the standard-library code you use are all compiled into the file. That is why a trivial program is a couple of megabytes rather than a couple of kilobytes — and it is a deliberate trade: in exchange for the size, you get a single file with no separate runtime to install on the target machine.

By default a Go binary may link against the system C library for things like DNS resolution (via `cgo`). Setting `CGO_ENABLED=0` forces the pure-Go implementations and produces a *fully* static binary with zero dynamic dependencies:

```sh
$ CGO_ENABLED=0 go build -o wordfreq
$ ldd wordfreq            # on Linux
        not a dynamic executable
```

`not a dynamic executable` is the property the cloud-native world is built on. You can put that file into a `FROM scratch` container — an image with *nothing* in it, not even a libc — and it runs. We do exactly that in Week 10.

Inspect the build metadata embedded in the binary with `go version -m`:

```sh
$ go version -m wordfreq
wordfreq: go1.22.4
        path    github.com/you/wordfreq
        mod     github.com/you/wordfreq        (devel)
        build   -buildmode=exe
        build   CGO_ENABLED=0
        build   GOOS=linux
        build   GOARCH=amd64
        build   vcs.revision=...
        build   vcs.time=...
```

Every binary carries its own provenance: the Go version it was built with, the module path, the build flags, and — if built from a clean Git checkout — the commit revision and timestamp. This is free supply-chain metadata; you can ask any Go binary in production "what commit are you?" Citation: <https://pkg.go.dev/cmd/go#hdr-Print_Go_environment_information> and the `runtime/debug.ReadBuildInfo` docs at <https://pkg.go.dev/runtime/debug#ReadBuildInfo>.

### 7.1 Trimming size (a preview of Week 10)

Two flags shrink the binary by stripping debug information:

```sh
$ go build -ldflags="-s -w" -o wordfreq-small
$ ls -lh wordfreq wordfreq-small
-rwxr-xr-x  1.9M  wordfreq
-rwxr-xr-x  1.4M  wordfreq-small
```

`-s` strips the symbol table; `-w` strips DWARF debug info. The binary still runs identically; you have only removed information a debugger would use. Do not strip in development (you want the symbols for `pprof` in Week 4); do strip for a production image where size matters. Citation: the linker flags at <https://pkg.go.dev/cmd/link>.

## 8. What maps from your old language, and what does not

A short table for the typed-language graduate, because naming the mismatch is half of un-learning it:

| Instinct from before | Go's answer |
|---|---|
| A project file lists sources (`.csproj`, `pom.xml`) | No — the tree + `package` clauses are the project |
| `public`/`private` keywords | Capitalization is visibility |
| A class is the unit of code | A package is the unit; structs + functions, no classes |
| `try`/`catch` for failure | `error` return values, checked explicitly |
| A constructor initializes fields | The zero value initializes fields; "make the zero value useful" |
| Inheritance for reuse | Composition (embedding) and interfaces |
| A package manager downloads to `./node_modules` | `go.mod` + a shared module cache; nothing in your project tree |
| A formatter you configure | `gofmt`, one style, no configuration |

We work through the language items (the zero value, errors, composition) in Lectures 2 and 3. This lecture's job was the project shape and the toolchain — and you now have both.

## 9. Exercise pointer

Now do **Exercise 1 — Module and Toolchain**. Stand up a module from nothing, split it into a `main` and an `internal/` library package, build a `CGO_ENABLED=0` static binary, prove it has no dynamic dependencies, and reach a clean `go vet` / `staticcheck`. The acceptance criterion is that you can recite, from memory, what `go build ./...` does that `go build` alone does not.

## 10. Summary

- A **module** is a directory tree with a `go.mod` at its root; `go.mod` names the module path and the Go version and is the entire build configuration.
- A **package** is a directory of `.go` files sharing a `package` clause; `package main` + `func main()` is an executable; everything else is a library.
- **Capitalization is visibility** — uppercase identifiers are exported, lowercase are package-private. `internal/` packages are importable only within the module.
- The **toolchain** is one command: `go build` / `go run` / `go install` / `go test` / `go vet`, plus `gofmt` and third-party `staticcheck`. `./...` means "this tree, recursively."
- **Cross-compilation** is two environment variables (`GOOS`, `GOARCH`) and no cross-toolchain.
- The week's contract: `go fmt`, `go vet`, and `staticcheck` all produce nothing. A finding is a bug.
- `go build` produces **one statically linked native binary embedding the Go runtime**; `CGO_ENABLED=0` makes it fully static (`ldd` reports "not a dynamic executable") — the property the cloud-native world is built on.
- `go version -m <binary>` prints the binary's own provenance: Go version, module path, build flags, and Git revision.

Cited references this lecture pulled from: <https://go.dev/ref/mod>, <https://go.dev/doc/tutorial/create-module>, <https://pkg.go.dev/cmd/go>, <https://go.dev/doc/effective_go>, <https://staticcheck.dev>, <https://pkg.go.dev/runtime/debug#ReadBuildInfo>.
