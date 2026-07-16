// Exercise 03 — test/fuzz file: a FuzzParseQuery target with seeds and invariants.
//
// Demonstrates: f.Add seed corpus, f.Fuzz with a typed string input, a never-panic
// invariant (any panic in ParseQuery fails the fuzz target automatically), and a
// round-trip / idempotency invariant on accepted inputs.
//
// Seeds run on every `go test` (regression behaviour). To actually fuzz:
//
//	go test -run='^$' -fuzz=FuzzParseQuery -fuzztime=30s ./...
//
// With the planted bug, the engine finds the crash (input "tag:") within seconds
// and writes it to testdata/fuzz/FuzzParseQuery/<hash>. After the fix, re-running
// finds nothing new and the committed crasher passes forever.
package notesx

import (
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseQueryExamples(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want Query
	}{
		{name: "bare terms", in: "hello world", want: Query{Terms: []string{"hello", "world"}}},
		{name: "tag", in: "tag:go", want: Query{Tags: []string{"go"}}},
		{name: "author", in: "author:alice", want: Query{Author: "alice"}},
		{name: "mixed", in: "tag:go author:alice text", want: Query{Tags: []string{"go"}, Author: "alice", Terms: []string{"text"}}},
		{name: "empty", in: "", want: Query{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseQuery(tc.in)
			if err != nil {
				t.Fatalf("ParseQuery(%q) error = %v", tc.in, err)
			}
			if diff := cmp.Diff(tc.want, got); diff != "" {
				t.Errorf("ParseQuery(%q) mismatch (-want +got):\n%s", tc.in, diff)
			}
		})
	}
}

func FuzzParseQuery(f *testing.F) {
	// Seed corpus: cover the known branches so the engine has good starting points.
	f.Add("tag:go")
	f.Add("tag:#go")
	f.Add("author:alice")
	f.Add("tag:go author:alice some text")
	f.Add("")
	f.Add(":")
	f.Add("plain words only")

	f.Fuzz(func(t *testing.T, in string) {
		// Invariant 1: never panic. A panic on any input fails the target with no
		// explicit assertion needed — that is what catches the planted bug ("tag:").
		q, err := ParseQuery(in)
		if err != nil {
			return // a rejected input is fine; only assert on accepted ones.
		}

		// Invariant 2: output validity — accepted queries never carry empty tags.
		for _, tag := range q.Tags {
			if tag == "" {
				t.Errorf("ParseQuery(%q) produced an empty tag in %#v", in, q)
			}
		}

		// Invariant 3: idempotent round-trip. Serializing and re-parsing an accepted
		// query must yield an equivalent query. String() normalizes ordering, so we
		// compare the second parse against the first (parse-of-serialize == parse),
		// which is robust to that normalization.
		out := q.String()
		q2, err := ParseQuery(out)
		if err != nil {
			t.Fatalf("re-parsing serialized output failed:\n in:  %q -> %#v\n out: %q -> error %v", in, q, out, err)
		}
		if !reflect.DeepEqual(q.canonical(), q2.canonical()) {
			t.Errorf("round-trip mismatch:\n in:  %q -> %#v\n out: %q -> %#v", in, q, out, q2)
		}
	})
}

// canonical returns a normalized copy of the Query for round-trip comparison:
// tags and terms sorted, nil and empty slices treated alike. This isolates the
// round-trip invariant from the (irrelevant) ordering String() imposes.
func (q Query) canonical() Query {
	c := Query{Author: q.Author}
	if len(q.Tags) > 0 {
		c.Tags = append([]string(nil), q.Tags...)
		sortStrings(c.Tags)
	}
	if len(q.Terms) > 0 {
		c.Terms = append([]string(nil), q.Terms...)
		sortStrings(c.Terms)
	}
	return c
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}
