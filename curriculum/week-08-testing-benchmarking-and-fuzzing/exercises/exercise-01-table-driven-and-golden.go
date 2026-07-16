// Exercise 01 — Table-driven tests and a golden file.
//
// Task
// ----
// This file holds two production functions for the `notes` domain:
//
//	ValidateNote(Note) error      — enforces the note invariants.
//	RenderNoteMarkdown(Note) string — renders a note as a Markdown document.
//
// Your job (see exercise-01-table-driven-and-golden_test.go for the scaffold,
// and SOLUTIONS.md for the worked answer) is to write:
//
//  1. A table-driven test for ValidateNote: a []struct{name; in Note; wantErr bool;
//     wantErrIs error}, a t.Run loop, t.Parallel() on the subtests, and errors.Is
//     to assert the *specific* sentinel error for each invalid case.
//
//  2. A golden-file test for RenderNoteMarkdown: render a fixed note, compare the
//     output against testdata/render_note.golden using cmp.Diff, and support an
//     -update flag (var update = flag.Bool("update", ...)) that rewrites the golden
//     file with os.WriteFile. First run: `go test -run TestRenderNoteMarkdown -update`,
//     read the golden file, confirm it is correct, commit it.
//
// Run:  go test ./...
// Update golden: go test -run TestRenderNoteMarkdown -update
//
// All code in this package is real and compiles against Go 1.22+ with the single
// dependency github.com/google/go-cmp (used by the test file, not here).
package notesx

import (
	"errors"
	"sort"
	"strings"
	"unicode/utf8"
)

// Note is the core domain value for the `notes` service. It mirrors the Week-5
// model closely enough to test the rules without pulling in the full service.
type Note struct {
	ID    string
	Title string
	Body  string
	Tags  []string
}

// Validation limits. These are the invariants ValidateNote enforces.
const (
	MaxTitleLen = 200
	MaxBodyLen  = 10_000
	MaxTags     = 10
	MaxTagLen   = 32
)

// Sentinel errors. Tests assert these with errors.Is, which is why each rule has
// its own distinct value rather than a single generic "invalid note" error.
var (
	ErrEmptyTitle   = errors.New("notes: title must not be empty")
	ErrTitleTooLong = errors.New("notes: title exceeds max length")
	ErrBodyTooLong  = errors.New("notes: body exceeds max length")
	ErrTooManyTags  = errors.New("notes: too many tags")
	ErrEmptyTag     = errors.New("notes: tag must not be empty")
	ErrTagTooLong   = errors.New("notes: tag exceeds max length")
	ErrInvalidUTF8  = errors.New("notes: field contains invalid UTF-8")
)

// ValidateNote checks a Note against the domain invariants and returns the first
// violated rule as a sentinel error (wrapped with context). It returns nil for a
// valid note. Title is trimmed before the empty check so that a whitespace-only
// title is treated as empty.
func ValidateNote(n Note) error {
	if !utf8.ValidString(n.Title) || !utf8.ValidString(n.Body) {
		return ErrInvalidUTF8
	}

	title := strings.TrimSpace(n.Title)
	if title == "" {
		return ErrEmptyTitle
	}
	if utf8.RuneCountInString(title) > MaxTitleLen {
		return ErrTitleTooLong
	}
	if utf8.RuneCountInString(n.Body) > MaxBodyLen {
		return ErrBodyTooLong
	}
	if len(n.Tags) > MaxTags {
		return ErrTooManyTags
	}
	for _, tag := range n.Tags {
		t := strings.TrimSpace(tag)
		if t == "" {
			return ErrEmptyTag
		}
		if utf8.RuneCountInString(t) > MaxTagLen {
			return ErrTagTooLong
		}
	}
	return nil
}

// RenderNoteMarkdown renders a Note as a small, deterministic Markdown document.
// It is deterministic on purpose: the tag list is sorted, so the same Note always
// renders to the same bytes, which is what makes a golden-file test meaningful.
//
// Layout:
//
//	# <Title>
//	<blank>
//	<Body>
//	<blank>
//	**Tags:** `tag1`, `tag2`, ...      (omitted entirely when there are no tags)
func RenderNoteMarkdown(n Note) string {
	var b strings.Builder

	b.WriteString("# ")
	b.WriteString(strings.TrimSpace(n.Title))
	b.WriteString("\n\n")

	body := strings.TrimRight(n.Body, "\n")
	b.WriteString(body)

	if len(n.Tags) > 0 {
		tags := append([]string(nil), n.Tags...)
		for i := range tags {
			tags[i] = strings.TrimSpace(tags[i])
		}
		sort.Strings(tags)

		b.WriteString("\n\n**Tags:** ")
		for i, t := range tags {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteByte('`')
			b.WriteString(t)
			b.WriteByte('`')
		}
	}

	b.WriteByte('\n')
	return b.String()
}
