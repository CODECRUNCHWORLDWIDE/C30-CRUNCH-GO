// Exercise 3 — Errors as Values and Table-Driven Tests
//
// GOAL
// ----
// Write functions that return (T, error), then give them a table-driven test
// suite that covers the happy path AND every error branch, runs clean under
// `go vet` and `staticcheck`, and reports a coverage number you can explain.
//
// LAYOUT
// ------
//   ex03/
//   ├── go.mod                  (go mod init github.com/you/ex03)
//   ├── parse.go                (this file — package parse)
//   └── parse_test.go           (the test file printed at the bottom; write it)
//
// RUN
// ---
//   go test ./...                          # green
//   go test -v ./...                       # see each named subtest
//   go test -run 'TestParseTopN/zero'      # run ONE case
//   go test -cover ./...                   # coverage percentage
//   go test -coverprofile=c.out ./... && go tool cover -html=c.out  # line view
//   go vet ./... && staticcheck ./...      # must print nothing
//
// ACCEPTANCE
// ----------
// A green `go test ./...`, plus a coverage number you can explain — including
// which lines the red bars in the HTML view point at. (See SOLUTIONS.md.)

package parse

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseTopN parses a --top flag value. It must be a positive integer.
// On failure it returns 0 and a wrapped error (we tighten the wrapping with
// errors.Is in Week 2; this week the goal is just "an error is returned").
func ParseTopN(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("--top %q: not an integer: %w", s, err)
	}
	if n <= 0 {
		return 0, fmt.Errorf("--top must be positive, got %d", n)
	}
	return n, nil
}

// NormalizeWord lowercases a token and strips leading/trailing punctuation so
// that "Cat," and "cat" count as the same word. It returns an error if the
// token has no word characters at all (e.g. "!!!"), because the caller should
// skip such tokens rather than count an empty string.
func NormalizeWord(token string) (string, error) {
	trimmed := strings.Trim(strings.ToLower(token), ".,!?;:\"'()[]{}")
	if trimmed == "" {
		return "", fmt.Errorf("token %q has no word characters", token)
	}
	return trimmed, nil
}

// ---------------------------------------------------------------------------
// TODO: create parse_test.go (package parse) with the following content, then
// extend the tables with at least two more cases each.
// ---------------------------------------------------------------------------
//
//   package parse
//
//   import "testing"
//
//   func TestParseTopN(t *testing.T) {
//       tests := []struct {
//           name    string
//           in      string
//           want    int
//           wantErr bool
//       }{
//           {"valid", "20", 20, false},
//           {"zero is invalid", "0", 0, true},
//           {"negative is invalid", "-3", 0, true},
//           {"non-numeric is invalid", "abc", 0, true},
//           // TODO: add "empty string" and "leading whitespace" cases.
//       }
//       for _, tc := range tests {
//           t.Run(tc.name, func(t *testing.T) {
//               got, err := ParseTopN(tc.in)
//               if (err != nil) != tc.wantErr {
//                   t.Fatalf("ParseTopN(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
//               }
//               if got != tc.want {
//                   t.Errorf("ParseTopN(%q) = %d, want %d", tc.in, got, tc.want)
//               }
//           })
//       }
//   }
//
//   func TestNormalizeWord(t *testing.T) {
//       tests := []struct {
//           name    string
//           in      string
//           want    string
//           wantErr bool
//       }{
//           {"already clean", "cat", "cat", false},
//           {"uppercase and comma", "Cat,", "cat", false},
//           {"surrounded by quotes", "\"go\"", "go", false},
//           {"punctuation only", "!!!", "", true},
//           // TODO: add "empty input" and "internal apostrophe (don't)" cases.
//       }
//       for _, tc := range tests {
//           t.Run(tc.name, func(t *testing.T) {
//               got, err := NormalizeWord(tc.in)
//               if (err != nil) != tc.wantErr {
//                   t.Fatalf("NormalizeWord(%q) error = %v, wantErr %v", tc.in, err, tc.wantErr)
//               }
//               if got != tc.want {
//                   t.Errorf("NormalizeWord(%q) = %q, want %q", tc.in, got, tc.want)
//               }
//           })
//       }
//   }
