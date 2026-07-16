# Challenge 1 — Static Binary Autopsy: Build It Four Ways and Explain Every Byte

> **Time:** 1.5 hours. **Prerequisites:** Lecture 1, Exercise 1. **Citations:** the `go build` docs at <https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies>, the linker docs at <https://pkg.go.dev/cmd/link>, `runtime/debug.ReadBuildInfo` at <https://pkg.go.dev/runtime/debug#ReadBuildInfo>, and the "Customizing Go Binaries with Build Tags" / build-info material on <https://go.dev>.

## The premise

A Go binary is not a black box. It embeds the runtime, your code, the standard-library code you reach for, and a pile of metadata. In this challenge you build the *same tiny program* four different ways, measure each, and explain — with evidence, not guesses — where every megabyte goes and which dependencies the binary carries.

## The program

Use a minimal program so the binary's size is dominated by the runtime and the packages you import, not your own code:

```go
// main.go
package main

import (
	"fmt"
	"runtime/debug"
)

func main() {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("no build info")
		return
	}
	fmt.Println("go version:", info.GoVersion)
	fmt.Println("module path:", info.Main.Path)
	for _, s := range info.Settings {
		fmt.Printf("  %s = %s\n", s.Key, s.Value)
	}
}
```

`go mod init github.com/you/autopsy`, save the file, and confirm `go run .` prints the build settings.

## The four builds

Build all four into distinctly named files and tabulate their sizes with `ls -lh`:

```sh
# Build 1: the default.
go build -o bin-default

# Build 2: fully static, CGO off.
CGO_ENABLED=0 go build -o bin-cgo-off

# Build 3: static + stripped (no symbol table, no DWARF).
CGO_ENABLED=0 go build -ldflags="-s -w" -o bin-stripped

# Build 4: cross-compiled to Linux ARM64, static + stripped.
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -o bin-linux-arm64
```

## The autopsy — required evidence

For each build, gather and record:

1. **Size** — `ls -lh bin-*`.
2. **Dynamic dependencies** — `ldd bin-default` (Linux) / `otool -L bin-default` (macOS) / `file bin-*`. Which builds are "not a dynamic executable"? Explain why Build 1 might differ from Build 2 (hint: the `net` and `os/user` packages can pull in `cgo` for DNS / user lookups on some platforms; `CGO_ENABLED=0` forces the pure-Go resolver).
3. **Embedded build metadata** — `go version -m bin-default`. Record the `CGO_ENABLED`, `GOOS`, `GOARCH`, and (if you built from a clean Git checkout) the `vcs.revision`.
4. **Section sizes** — `go tool nm bin-default | wc -l` (symbol count) for the non-stripped builds vs the stripped one. The stripped build should report far fewer symbols (or fail/`no symbols` — note which).
5. **Run-anywhere proof** (Linux only, optional) — copy `bin-cgo-off` into a `FROM scratch` Docker image and run it:
   ```dockerfile
   FROM scratch
   COPY bin-cgo-off /app
   ENTRYPOINT ["/app"]
   ```
   `docker build -t autopsy . && docker run --rm autopsy`. A `scratch` image has *nothing* in it — no shell, no libc. If your static binary runs, you have proven it has zero external dependencies. (This is a direct preview of Week 10.)

## Acceptance criteria

1. A table of all four binary sizes with the deltas explained: how many bytes does stripping save (`-s -w`), and what did you give up (debug symbols, used by `pprof` in Week 4)?
2. A correct statement of which builds are fully static and why `CGO_ENABLED=0` matters for the `net` package specifically.
3. The `go version -m` output for at least one binary, with each `build` setting explained in one sentence.
4. (If you did the Docker step) a screenshot/transcript of the `scratch`-based binary running, proving zero dependencies.

## Stretch goals

1. **Find the runtime floor.** Replace the program with a literal `func main() {}` (empty). Build it `CGO_ENABLED=0 -ldflags="-s -w"`. How small can a Go binary get? That floor is the embedded runtime (GC + scheduler). Compare to the floor for a C `int main(){}` static binary — Go's floor is larger; explain the trade you are buying.
2. **Trace one import's cost.** Add `import _ "net/http"` (blank import, just to pull it in) and rebuild. How many bytes did the HTTP stack add? Now add `import _ "encoding/json"`. Build a small "cost of each standard-library package" table for the five packages you expect to use most this track.
3. **`-trimpath`.** Build with `-trimpath` and diff the `go version -m` output and any embedded file paths (`strings bin-default | grep $HOME`). Explain why a production build uses `-trimpath` (it removes absolute build-machine paths from the binary — a small privacy/reproducibility win).

Cited references: <https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies>, <https://pkg.go.dev/cmd/link>, <https://pkg.go.dev/runtime/debug#ReadBuildInfo>, the `net` package's pure-Go resolver note at <https://pkg.go.dev/net#hdr-Name_Resolution>.
