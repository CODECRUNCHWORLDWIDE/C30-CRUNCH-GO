// Exercise 1 — Module and Toolchain
//
// GOAL
// ----
// Stand up a Go module from nothing, split it into a `main` package and an
// `internal/` library package, build a fully static binary, prove it has no
// dynamic dependencies, and reach a clean `go vet` and `staticcheck`.
//
// This file is the `main` package for the exercise. Drop it into a fresh
// module laid out like this:
//
//   ex01/
//   ├── go.mod                 (go mod init github.com/you/ex01)
//   ├── main.go                (this file)
//   └── internal/
//       └── greet/
//           └── greet.go       (the library package — write it; see TODO 2)
//
// STEPS
// -----
//  1. mkdir ex01 && cd ex01 && go mod init github.com/you/ex01
//  2. Create internal/greet/greet.go with an EXPORTED function
//     `Salutation(name string) string` that returns "hello, <name>" (lowercased
//     name). Capitalize the function name so main can import it.
//  3. Save this file as main.go and fix the import path to match your module.
//  4. go run .                          # should print three greetings
//  5. go vet ./...                      # must print nothing
//  6. staticcheck ./...                 # must print nothing
//  7. CGO_ENABLED=0 go build -o ex01    # build a static binary
//  8. ldd ex01    (Linux)   # expect: "not a dynamic executable"
//     otool -L ex01 (macOS) # note the difference; macOS always links libSystem
//  9. go version -m ex01                # read the embedded build metadata
//
// ACCEPTANCE
// ----------
// You can recite, from memory, what `go build ./...` does that `go build`
// alone does not. (Answer in SOLUTIONS.md.)

package main

import (
	"fmt"
	"os"
	"strings"

	// TODO 1: fix this path to match the module name you chose in step 1.
	"github.com/you/ex01/internal/greet"
)

func main() {
	// Default names if none are given on the command line.
	names := os.Args[1:]
	if len(names) == 0 {
		names = []string{"Ada", "Grace", "Alan"}
	}

	// Build the output with a strings.Builder, not repeated string concatenation
	// in a loop — the latter reallocates every iteration and staticcheck flags it.
	var b strings.Builder
	for _, n := range names {
		b.WriteString(greet.Salutation(n))
		b.WriteByte('\n')
	}
	fmt.Print(b.String())
}

// TODO 2 (in internal/greet/greet.go):
//
//   package greet
//
//   import "strings"
//
//   // Salutation returns a lowercase greeting for name.
//   func Salutation(name string) string {
//       return "hello, " + strings.ToLower(name)
//   }
//
// Note: `greet` is lowercase (the package name); `Salutation` is uppercase
// (exported, so main can call it). That capitalization IS the access modifier.
