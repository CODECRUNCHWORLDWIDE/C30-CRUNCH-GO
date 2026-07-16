// Exercise 03 — write a fuzz target that finds a real crash.
//
// Task
// ----
// ParseQuery turns a search string like `tag:go author:alice some words` into a
// Query. It has a REAL, fuzzer-findable bug: it indexes val[0] (checking for a
// disallowed leading '#') without first checking that val is non-empty, so the
// input "tag:" (a key with an empty value) panics with
// "index out of range [0] with length 0".
//
// Your job (see exercise-03-fuzz-target_test.go and SOLUTIONS.md):
//
//  1. Run the fuzzer and watch it find the crash:
//     go test -run='^$' -fuzz=FuzzParseQuery -fuzztime=30s ./...
//
//  2. Read the minimized crasher the engine writes to
//     testdata/fuzz/FuzzParseQuery/<hash> and the re-run command it prints.
//
//  3. Fix the bug (guard the index: `if len(val) > 0 && val[0] == '#'`, or
//     equivalently check `len(val) == 0` first and reject an empty value).
//
//  4. Re-run the single crasher, confirm it passes, then re-run the fuzzer and
//     confirm it finds nothing new. Commit the crasher file as a regression test.
//
// The bug is left IN PLACE on purpose — this is the exercise. The documented fix
// is in SOLUTIONS.md. The code is real and compiles against Go 1.22+.
package notesx

import (
	"fmt"
	"sort"
	"strings"
)

// Query is the parsed form of a search string.
type Query struct {
	Tags   []string // from `tag:foo` fields (a leading '#' on the value is rejected)
	Author string   // from an `author:foo` field (last one wins)
	Terms  []string // bare words with no `key:` prefix
}

// String serializes a Query back into a canonical query string. ParseQuery and
// String are meant to round-trip: parsing the output of String must yield an
// equivalent Query. The fuzz target asserts exactly that, which is how a
// non-round-tripping bug would be caught in addition to the panic.
func (q Query) String() string {
	var parts []string

	tags := append([]string(nil), q.Tags...)
	sort.Strings(tags)
	for _, t := range tags {
		parts = append(parts, "tag:"+t)
	}
	if q.Author != "" {
		parts = append(parts, "author:"+q.Author)
	}
	terms := append([]string(nil), q.Terms...)
	sort.Strings(terms)
	parts = append(parts, terms...)

	return strings.Join(parts, " ")
}

// ParseQuery parses a search string into a Query.
//
// BUG: in the "tag" branch it reads val[0] to check for a leading '#' without
// first checking len(val) > 0. The input `tag:` produces val == "", and val[0]
// panics. A human writing tests thinks of `tag:go` and `tag:#go`; the empty-value
// case `tag:` is exactly the kind of input a fuzzer reaches by deleting characters
// from a seed.
func ParseQuery(s string) (Query, error) {
	var q Query
	for _, field := range strings.Fields(s) {
		i := strings.IndexByte(field, ':')
		if i < 0 {
			q.Terms = append(q.Terms, field)
			continue
		}
		key := field[:i]
		val := field[i+1:]
		switch key {
		case "tag":
			// A leading '#' on a tag value is rejected (write `tag:go`, not
			// `tag:#go`). BUG IS HERE: val may be empty (input "tag:"), and reading
			// val[0] panics with "index out of range [0] with length 0".
			if val[0] == '#' {
				return Query{}, fmt.Errorf("tag value must not start with '#': %q", field)
			}
			q.Tags = append(q.Tags, val)
		case "author":
			if val == "" {
				return Query{}, fmt.Errorf("empty author value in %q", field)
			}
			q.Author = val
		default:
			return Query{}, fmt.Errorf("unknown field %q", key)
		}
	}
	return q, nil
}
