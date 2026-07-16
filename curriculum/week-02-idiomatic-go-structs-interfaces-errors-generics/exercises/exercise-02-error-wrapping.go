// Exercise 2 — Error Wrapping: Sentinel, Typed, errors.Is and errors.As
//
// GOAL
// ----
// Build a two-layer error chain with %w, then inspect it WITHOUT string
// matching: a sentinel checked with errors.Is, a typed error extracted with
// errors.As (and its fields read). Predict the difference between == and
// errors.Is on a wrapped error.
//
// LAYOUT
// ------
//   ex02/
//   ├── go.mod                  (go mod init github.com/you/ex02)
//   └── main.go                 (this file — package main)
//
// RUN
// ---
//   go run .                              # see the predictions confirmed
//   go vet ./... && staticcheck ./...     # must print nothing
//
// STEPS
// -----
//   1. Read each PREDICT comment and write down your answer before running.
//   2. Run and compare; where wrong, re-read Lecture 2 §5–§6.
//   3. Do the TODO: implement Unwrap on QueryError and prove that
//      errors.Is(err, ErrNotFound) then finds a sentinel wrapped UNDER it.
//
// ACCEPTANCE
// ----------
// A program that detects both errors by identity (Is) and type (As), reads the
// typed error's fields, and NEVER branches on err.Error(). (See SOLUTIONS.md.)

package main

import (
	"errors"
	"fmt"
	"time"
)

// --- Sentinel error: identity only, checked with errors.Is. ---
var ErrNotFound = errors.New("store: key not found")

// --- Typed error: carries data, extracted with errors.As. ---
type ExpiredError struct {
	Key       string
	ExpiredAt time.Time
}

func (e *ExpiredError) Error() string {
	return fmt.Sprintf("store: key %q expired at %s", e.Key, e.ExpiredAt.Format(time.RFC3339))
}

// QueryError wraps a cause with the query that produced it.
// TODO: add an Unwrap() error method returning e.Err so that errors.Is and
// errors.As can see THROUGH a QueryError to whatever it wraps.
type QueryError struct {
	Query string
	Err   error
}

func (e *QueryError) Error() string { return e.Query + ": " + e.Err.Error() }

// TODO: implement this. (Delete this comment when done.)
// func (e *QueryError) Unwrap() error { return e.Err }

// store is the bottom layer. It returns wrapped sentinel / typed errors.
type entry struct {
	val      string
	deadline time.Time
}

var data = map[string]entry{
	"fresh": {val: "still good", deadline: time.Now().Add(time.Hour)},
	"stale": {val: "too old", deadline: time.Now().Add(-time.Hour)},
}

func lookup(key string) (string, error) {
	e, ok := data[key]
	if !ok {
		// Wrap the SENTINEL with context.
		return "", fmt.Errorf("lookup %q: %w", key, ErrNotFound)
	}
	if time.Now().After(e.deadline) {
		// Wrap the TYPED error with context.
		return "", fmt.Errorf("lookup %q: %w", key, &ExpiredError{Key: key, ExpiredAt: e.deadline})
	}
	return e.val, nil
}

// query is the upper layer. It wraps lookup's error in a QueryError.
func query(key string) (string, error) {
	v, err := lookup(key)
	if err != nil {
		return "", &QueryError{Query: "GET " + key, Err: err}
	}
	return v, nil
}

func main() {
	// --- The sentinel, one layer of wrapping (via lookup directly). ---
	_, err := lookup("ghost")

	// PREDICT 1: is `err == ErrNotFound` true or false? Why?
	fmt.Println("err == ErrNotFound:", err == ErrNotFound)

	// PREDICT 2: is `errors.Is(err, ErrNotFound)` true or false? Why does it
	// differ from PREDICT 1?
	fmt.Println("errors.Is(err, ErrNotFound):", errors.Is(err, ErrNotFound))

	// --- The typed error, read its fields with errors.As. ---
	_, err = lookup("stale")
	var ee *ExpiredError
	// PREDICT 3: does errors.As bind ee here? What does ee.Key print?
	if errors.As(err, &ee) {
		fmt.Printf("expired: key=%q expiredAt=%s\n", ee.Key, ee.ExpiredAt.Format(time.RFC3339))
	} else {
		fmt.Println("not an ExpiredError")
	}

	// --- TWO layers: query wraps lookup in a QueryError. ---
	_, err = query("ghost")
	fmt.Println("two-layer error:", err)

	// PREDICT 4: BEFORE you implement Unwrap on QueryError, is
	// errors.Is(err, ErrNotFound) true? AFTER you implement Unwrap, is it true?
	// Implement the TODO, then run again and watch this line change.
	fmt.Println("errors.Is(query err, ErrNotFound):", errors.Is(err, ErrNotFound))

	// PREDICT 5: same question for the typed error through two layers.
	var ee2 *ExpiredError
	_, err = query("stale")
	fmt.Println("errors.As(query err, &ExpiredError) bound:", errors.As(err, &ee2))
}
