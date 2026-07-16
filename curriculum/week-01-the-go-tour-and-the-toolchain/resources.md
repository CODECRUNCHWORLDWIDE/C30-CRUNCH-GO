# Week 1 — Resources

Every resource on this page is **free**. The Go website (`go.dev`), the package documentation (`pkg.go.dev`), and the Go source on GitHub are all free and require no account. `staticcheck` is open-source (MIT). "Go by Example" is free. No paywalled material is linked.

## Required reading (work it into your week)

### The tour and the language basics

- **A Tour of Go** — the interactive introduction; do the "Basics," "Methods and interfaces" (skim; Week 2), and "Concurrency" (skim; Week 3) modules. Run the snippets in the embedded playground:
  <https://go.dev/tour/>
- **A Tour of Go — the zero value** — the specific page on zero values:
  <https://go.dev/tour/basics/12>
- **Effective Go** — the canonical idiomatic-Go document; read "Names," "Control structures," "Functions," "Data," "Errors," and "Embedding" this week:
  <https://go.dev/doc/effective_go>
- **The Go Programming Language Specification** — the precise definition; you do not read it cover to cover, but you *do* consult it for exact semantics (slice expressions, `iota`, the `range` statement):
  <https://go.dev/ref/spec>

### Modules and the toolchain

- **Tutorial: Create a Go module** — `go mod init`, packages, calling code from another module:
  <https://go.dev/doc/tutorial/create-module>
- **Go Modules Reference** — `go.mod`, `go.sum`, `go mod tidy`, the module cache, versioning:
  <https://go.dev/ref/mod>
- **The `go` command documentation** — every subcommand (`build`, `run`, `install`, `test`, `vet`, `mod`) and the package patterns (`./...`):
  <https://pkg.go.dev/cmd/go>
- **`go vet` documentation** — what the vet analyzers catch:
  <https://pkg.go.dev/cmd/vet>
- **Staticcheck** — the de facto third-party linter; install with `go install honnef.co/go/tools/cmd/staticcheck@latest`. Read the checks catalogue:
  <https://staticcheck.dev> and <https://staticcheck.dev/docs/checks/>
- **`gofmt`** — the canonical formatter:
  <https://pkg.go.dev/cmd/gofmt>
- **`gopls`** — the official language server (powers editor format-on-save and inline vet):
  <https://pkg.go.dev/golang.org/x/tools/gopls>

### The data structures (read the blog posts; they are short and definitive)

- **Go Slices: usage and internals** — the three-word header, `append`, reslicing, the aliasing trap:
  <https://go.dev/blog/slices-intro>
- **Arrays, slices (and strings): The mechanics of 'append'** — the deeper companion piece:
  <https://go.dev/blog/slices>
- **Go maps in action** — the comma-ok read, the nil-map asymmetry, iteration order, concurrency:
  <https://go.dev/blog/maps>
- **Strings, bytes, runes and characters in Go** — UTF-8, `byte` vs `rune`, ranging over a string:
  <https://go.dev/blog/strings>

### `defer`, `panic`, errors

- **Defer, Panic, and Recover** — the canonical post on `defer` LIFO ordering, argument-evaluation timing, and the legitimate uses of `panic`/`recover`:
  <https://go.dev/blog/defer-panic-and-recover>
- **Effective Go — Errors** — the errors-as-values model:
  <https://go.dev/doc/effective_go#errors>
- **The `errors` package** — `errors.New`; `errors.Is` / `errors.As` / `%w` are previewed here and covered in Week 2:
  <https://pkg.go.dev/errors>

### Testing

- **The `testing` package** — `T`, `t.Run`, `t.Errorf` vs `t.Fatalf`, `t.Helper`, `t.Cleanup`, `t.Parallel`:
  <https://pkg.go.dev/testing>
- **Tutorial: Add a test** — writing your first Go test:
  <https://go.dev/doc/tutorial/add-a-test>
- **Go Wiki: TableDrivenTests** — the canonical table-driven-test pattern reference:
  <https://go.dev/wiki/TableDrivenTests>
- **The cover tool** — `go test -cover`, `-coverprofile`, `go tool cover -html`:
  <https://go.dev/blog/cover>

### The standard library you will read this week

- **`pkg.go.dev` — the package index** — every standard-library (and published) package's docs and source:
  <https://pkg.go.dev/std>
- **`strings`** — `Fields`, `ToLower`, `Trim`, `Builder`, `Split`:
  <https://pkg.go.dev/strings>
- **`bufio`** — `Scanner`, `NewScanner`, `ScanWords`, `ScanLines`, the `SplitFunc` contract:
  <https://pkg.go.dev/bufio>
- **`os`** — `Open`, `Args`, `Exit`, `Stdin`, `Stderr`:
  <https://pkg.go.dev/os>
- **`sort`** — `Slice`, `Strings` (and the newer generic `slices.Sort`):
  <https://pkg.go.dev/sort> and <https://pkg.go.dev/slices>
- **`flag`** — the standard-library command-line flag parser:
  <https://pkg.go.dev/flag>
- **`unicode`** — `IsSpace`, `IsLetter` (used by `strings.Fields`):
  <https://pkg.go.dev/unicode>
- **`runtime/debug` — `ReadBuildInfo`** — the embedded build metadata you read in Challenge 1:
  <https://pkg.go.dev/runtime/debug#ReadBuildInfo>

## Recommended reading (after the required set)

- **Go by Example** — short, runnable examples for every basic construct; the best "show me the syntax" reference:
  <https://gobyexample.com/>
- **Go Code Review Comments** — the Go team's list of common review feedback; read it to internalize the reviewer's instincts early:
  <https://go.dev/wiki/CodeReviewComments>
- **Frequently Asked Questions (FAQ)** — the language designers' own answers to "why no exceptions," "why no inheritance," "why the zero value":
  <https://go.dev/doc/faq>
- **The Go Blog index** — the team's writing; the slices, maps, strings, and defer posts above all live here:
  <https://go.dev/blog/>
- **"The Go Programming Language" (Donovan & Kernighan)** — the book; Chapters 1–4 cover this week's material in depth. Not free, but the canonical text:
  <https://www.gopl.io/>

## Tools you will install this week

- **The Go toolchain** (1.22 or newer): download from <https://go.dev/dl/>. Verify with `go version` (expect `go1.22.x` or later).
- **`staticcheck`**: `go install honnef.co/go/tools/cmd/staticcheck@latest`. Verify with `staticcheck --version`. (It installs into `$GOBIN`, default `~/go/bin`; make sure that is on your `PATH`.)
- **An editor with `gopls`**: VS Code + the Go extension, GoLand, or Neovim with `gopls`. Enable format-on-save and inline `go vet`. The official extension setup is at <https://github.com/golang/vscode-go>.
- **(Optional) Docker**: only needed if you attempt the `FROM scratch` step in Challenge 1. Not required for any graded Week 1 work; install from <https://docs.docker.com/get-docker/>.

## Citations policy

This curriculum cites `go.dev` (the tour, Effective Go, the spec, the blog, the tutorials), `pkg.go.dev` (the package documentation and source), the Go GitHub source, and `staticcheck.dev` as the primary references. Every example in the lecture notes and exercises traces back to one of these. When a third-party reference (Go by Example, the GOPL book, a Go team member's talk) is the clearer source, it is cited explicitly with a URL — never paraphrased without attribution. If a citation is missing from a section of these notes, treat it as a bug and open an issue against the C30 curriculum repository.
