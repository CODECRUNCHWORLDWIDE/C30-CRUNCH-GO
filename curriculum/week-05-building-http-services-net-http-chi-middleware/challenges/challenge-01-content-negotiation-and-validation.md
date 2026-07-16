# Challenge 1 — Field-Level Validation and Content Negotiation Behind One Handler

> **Time:** 90 minutes. **Prerequisites:** Exercises 1 and 2. **Citations:** the `encoding/json` docs at <https://pkg.go.dev/encoding/json>, RFC 9110 §12 (Content Negotiation) at <https://www.rfc-editor.org/rfc/rfc9110#name-content-negotiation>, and the `Accept`/`Content-Type` header semantics in RFC 9110 §8.

## The premise

A real API does two things this week's exercises skipped: it returns *field-level* validation errors (not just "validation failed" but "title is required, body must be under 10000 chars"), and it can serve more than one representation of a resource based on the client's `Accept` header. You will add both to the `notes` handlers — without changing the service layer, because validation and serialization are *handler* concerns.

## What you will build

```
src/notesapi/
  validate.go     // the validation framework + the notes validators
  negotiate.go    // Accept-header content negotiation
  handler.go      // handlers using both
  handler_test.go
```

### Part A — field-level validation (422 with details)

Define a small validation result that collects per-field errors, and return them as a 422 with a structured body:

```go
package notesapi

type FieldError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

type ValidationErrors []FieldError

func (v ValidationErrors) Error() string { return "validation failed" }

// validateCreate checks a create request and returns one FieldError per
// violated rule (not just the first).
func validateCreate(title, body string) ValidationErrors {
	var errs ValidationErrors
	if strings.TrimSpace(title) == "" {
		errs = append(errs, FieldError{"title", "is required"})
	}
	if len(title) > 200 {
		errs = append(errs, FieldError{"title", "must be at most 200 characters"})
	}
	if len(body) > 10000 {
		errs = append(errs, FieldError{"body", "must be at most 10000 characters"})
	}
	return errs
}
```

The handler maps a non-empty `ValidationErrors` to a 422 with the full list:

```go
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	req, err := decodeJSON[createRequest](w, r)
	if err != nil {
		writeError(w, fmt.Errorf("%w: %v", errBadRequest, err))
		return
	}
	if verrs := validateCreate(req.Title, req.Body); len(verrs) > 0 {
		// 422 with {"error":{"code":"validation","fields":[{field,message},...]}}
		respondValidation(w, verrs)
		return
	}
	// ... call h.svc.Create ...
}
```

**Requirement:** collect *all* violated rules, not just the first — a client filling a form wants every error at once, not one per round trip.

### Part B — content negotiation (JSON now, a second format)

Read the client's `Accept` header and serve the matching representation. Support `application/json` (default) and `text/plain` (a human-readable rendering):

```go
// negotiate returns the best representation the client accepts, defaulting to
// JSON when the Accept header is missing or "*/*".
func negotiate(accept string) string {
	switch {
	case strings.Contains(accept, "text/plain"):
		return "text/plain"
	default:
		return "application/json"
	}
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, err)
		return
	}
	switch negotiate(r.Header.Get("Accept")) {
	case "text/plain":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		fmt.Fprintf(w, "%s\n\n%s\n", n.Title, n.Body)
	default:
		writeJSON(w, http.StatusOK, toResponse(n))
	}
}
```

**Requirement:** the service layer is *unchanged*. Negotiation and validation are entirely in the handler — proof that the seam holds. If you found yourself editing the service for either feature, the seam is leaking.

## The test harness

```go
func TestValidationCollectsAllErrors(t *testing.T) {
	h := newTestHandler()
	longBody := strings.Repeat("x", 10001)
	req := httptest.NewRequest("POST", "/v1/notes",
		strings.NewReader(`{"title":"","body":"`+longBody+`"}`))
	rec := httptest.NewRecorder()
	h.Create(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("got %d want 422", rec.Code)
	}
	// Expect TWO field errors: empty title AND oversized body.
	var resp struct {
		Error struct {
			Fields []FieldError `json:"fields"`
		} `json:"error"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Error.Fields) != 2 {
		t.Errorf("got %d field errors, want 2", len(resp.Error.Fields))
	}
}

func TestContentNegotiation(t *testing.T) {
	h := newTestHandler()
	// seed a note with id "1" first...
	req := httptest.NewRequest("GET", "/v1/notes/1", nil)
	req.Header.Set("Accept", "text/plain")
	rec := httptest.NewRecorder()
	h.Get(rec, req)
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", ct)
	}
}
```

## Acceptance criteria

1. A create request that violates two rules returns a 422 with *both* field errors in the `error.fields` array.
2. A `GET` with `Accept: text/plain` returns a `text/plain` body; the same `GET` with no `Accept` (or `application/json`) returns JSON.
3. The service layer is byte-for-byte unchanged from the lecture version — `git diff` on the service file is empty.
4. `go test ./...`, `go vet ./...`, and `staticcheck ./...` are all clean.
5. The `Content-Type` response header always matches the body format actually written.

## Stretch goals

1. **Quality values.** Parse `Accept: text/plain;q=0.5, application/json;q=0.9` and honour the `q` weights, picking the highest-weighted supported type. Cite RFC 9110 §12.4.2 (Quality Values).
2. **A third format.** Add `text/csv` for the *list* endpoint, streaming rows with `encoding/csv` as they come, so a large list does not buffer the whole CSV in memory. Discuss why streaming the response matters for a 100k-row list.
3. **Validation as a reusable interface.** Define `interface { Validate() ValidationErrors }`, implement it on each request type, and write a generic `decodeAndValidate[T Validator]` helper that decodes and validates in one call. Discuss the trade-off vs per-handler validation calls.

Cited references: <https://pkg.go.dev/encoding/json>, <https://www.rfc-editor.org/rfc/rfc9110#name-content-negotiation>, <https://pkg.go.dev/encoding/csv>, and the status-code map from Lecture 1.
