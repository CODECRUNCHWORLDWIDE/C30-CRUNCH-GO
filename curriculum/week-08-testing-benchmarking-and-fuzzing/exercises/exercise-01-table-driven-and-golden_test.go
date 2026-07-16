// Exercise 01 — test file: table-driven validation + a golden-file render test.
//
// Demonstrates: a []struct table, t.Run subtests, t.Parallel(), errors.Is for
// specific-sentinel assertions, cmp.Diff for readable failure output, and the
// -update golden-file pattern.
//
// Run:           go test ./...
// Update golden: go test -run TestRenderNoteMarkdown -update
package notesx

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// update controls whether the golden test rewrites its golden file instead of
// comparing against it. Defined once for the whole package's test binary.
var update = flag.Bool("update", false, "update golden files")

func TestValidateNote(t *testing.T) {
	t.Parallel()

	valid := Note{Title: "A reasonable title", Body: "Some body text.", Tags: []string{"go", "testing"}}

	tests := []struct {
		name      string
		in        Note
		wantErr   bool
		wantErrIs error
	}{
		{name: "valid", in: valid},
		{name: "empty title", in: Note{Title: "", Body: "b"}, wantErr: true, wantErrIs: ErrEmptyTitle},
		{name: "whitespace title", in: Note{Title: "   ", Body: "b"}, wantErr: true, wantErrIs: ErrEmptyTitle},
		{name: "title too long", in: Note{Title: strings.Repeat("x", MaxTitleLen+1)}, wantErr: true, wantErrIs: ErrTitleTooLong},
		{name: "body too long", in: Note{Title: "ok", Body: strings.Repeat("x", MaxBodyLen+1)}, wantErr: true, wantErrIs: ErrBodyTooLong},
		{name: "too many tags", in: Note{Title: "ok", Tags: makeTags(MaxTags + 1)}, wantErr: true, wantErrIs: ErrTooManyTags},
		{name: "empty tag", in: Note{Title: "ok", Tags: []string{"go", "  "}}, wantErr: true, wantErrIs: ErrEmptyTag},
		{name: "tag too long", in: Note{Title: "ok", Tags: []string{strings.Repeat("x", MaxTagLen+1)}}, wantErr: true, wantErrIs: ErrTagTooLong},
		{name: "invalid utf8 in title", in: Note{Title: "\xff\xfe", Body: "b"}, wantErr: true, wantErrIs: ErrInvalidUTF8},
		{name: "boundary title length ok", in: Note{Title: strings.Repeat("x", MaxTitleLen)}},
		{name: "boundary tag length ok", in: Note{Title: "ok", Tags: []string{strings.Repeat("x", MaxTagLen)}}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateNote(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateNote(%+v) error = %v, wantErr = %v", tc.in, err, tc.wantErr)
			}
			if tc.wantErrIs != nil && !errorIs(err, tc.wantErrIs) {
				t.Errorf("ValidateNote(%+v) error = %v, want errors.Is(_, %v)", tc.in, err, tc.wantErrIs)
			}
		})
	}
}

func TestRenderNoteMarkdown(t *testing.T) {
	note := Note{
		Title: "Release Notes",
		Body:  "We shipped **fuzzing** support.\n\nSee the [docs](https://go.dev).\n",
		Tags:  []string{"release", "go", "announce"},
	}

	got := RenderNoteMarkdown(note)
	golden := filepath.Join("testdata", "render_note.golden")

	if *update {
		if err := os.MkdirAll(filepath.Dir(golden), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
			t.Fatalf("writing golden: %v", err)
		}
		t.Logf("updated %s", golden)
	}

	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden (run with -update to create it): %v", err)
	}
	if diff := cmp.Diff(string(want), got); diff != "" {
		t.Errorf("RenderNoteMarkdown mismatch (-golden +got):\n%s", diff)
	}
}

// --- small test helpers ---

func makeTags(n int) []string {
	tags := make([]string, n)
	for i := range tags {
		tags[i] = "t" + string(rune('a'+i%26))
	}
	return tags
}

// errorIs is a thin wrapper so the table can stay import-light; it is just
// errors.Is. (In real code you would call errors.Is directly.)
func errorIs(err, target error) bool {
	for err != nil {
		if err == target {
			return true
		}
		u, ok := err.(interface{ Unwrap() error })
		if !ok {
			return false
		}
		err = u.Unwrap()
	}
	return false
}
