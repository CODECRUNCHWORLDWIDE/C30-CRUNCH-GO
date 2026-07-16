// Exercise 1 — Router and JSON, on the bare 1.22 ServeMux
//
// GOAL
//   Build a small REST resource with the Go 1.22 ServeMux (no chi yet): method-
//   and-path routing, safe JSON decode, JSON encode, a single status-code map,
//   and httptest-based handler tests. This is the standard-library foundation
//   the rest of the week builds on.
//
// HOW TO RUN
//   mkdir ex01 && cd ex01 && go mod init ex01   # needs Go 1.22+
//   # save this file as main.go and the test below as main_test.go
//   go run .                          # starts on :8080
//   curl -s localhost:8080/items -d '{"name":"widget"}' -H 'content-type: application/json' -i
//   curl -s localhost:8080/items/nope -i        # 404
//   go test ./... && go vet ./...
//
// TASKS
//   1. Read the routing: "POST /items", "GET /items/{id}". Confirm a GET to
//      /items with no GET route returns 405 automatically.
//   2. Implement createItem: decode with DisallowUnknownFields + MaxBytesReader,
//      validate name is non-empty (422), store it, return 201 with a Location.
//   3. Implement getItem: read r.PathValue("id"), return 200 or writeError 404.
//   4. Confirm writeError maps ErrNotFound->404, errValidation->422,
//      errBadRequest->400, everything else->500.

package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ---- domain + store ----

type Item struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

var (
	ErrNotFound   = errors.New("item not found")
	errValidation = errors.New("validation failed")
	errBadRequest = errors.New("bad request")
)

type store struct {
	mu    sync.RWMutex
	items map[string]Item
	seq   int
}

func newStore() *store { return &store{items: map[string]Item{}} }

func (s *store) create(name string) Item {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	it := Item{ID: fmt.Sprintf("%d", s.seq), Name: name, CreatedAt: time.Now().UTC()}
	s.items[it.ID] = it
	return it
}

func (s *store) get(id string) (Item, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	it, ok := s.items[id]
	if !ok {
		return Item{}, ErrNotFound
	}
	return it, nil
}

// ---- json + error helpers ----

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, err error) {
	status, code := http.StatusInternalServerError, "internal"
	switch {
	case errors.Is(err, ErrNotFound):
		status, code = http.StatusNotFound, "not_found"
	case errors.Is(err, errValidation):
		status, code = http.StatusUnprocessableEntity, "validation"
	case errors.Is(err, errBadRequest):
		status, code = http.StatusBadRequest, "bad_request"
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]string{"code": code, "message": err.Error()},
	})
}

// ---- handlers ----

type handler struct{ s *store }

func (h *handler) createItem(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	var req struct {
		Name string `json:"name"`
	}
	if err := dec.Decode(&req); err != nil {
		writeError(w, fmt.Errorf("%w: %v", errBadRequest, err))
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, fmt.Errorf("%w: name is required", errValidation))
		return
	}
	it := h.s.create(req.Name)
	w.Header().Set("Location", "/items/"+it.ID)
	writeJSON(w, http.StatusCreated, it)
}

func (h *handler) getItem(w http.ResponseWriter, r *http.Request) {
	it, err := h.s.get(r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, it)
}

func newMux(h *handler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /items", h.createItem)
	mux.HandleFunc("GET /items/{id}", h.getItem)
	return mux
}

func main() {
	h := &handler{s: newStore()}
	srv := &http.Server{Addr: ":8080", Handler: newMux(h), ReadHeaderTimeout: 5 * time.Second}
	_ = srv.ListenAndServe()
}

/*
main_test.go — save alongside this file:

package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestItemsAPI(t *testing.T) {
	h := &handler{s: newStore()}
	mux := newMux(h)

	// create -> 201 + Location
	req := httptest.NewRequest("POST", "/items", strings.NewReader(`{"name":"widget"}`))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create: got %d want 201", rec.Code)
	}
	if rec.Header().Get("Location") == "" {
		t.Error("create: missing Location")
	}

	// empty name -> 422
	req = httptest.NewRequest("POST", "/items", strings.NewReader(`{"name":""}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("empty name: got %d want 422", rec.Code)
	}

	// unknown field -> 400
	req = httptest.NewRequest("POST", "/items", strings.NewReader(`{"naem":"x"}`))
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Errorf("unknown field: got %d want 400", rec.Code)
	}

	// missing id -> 404
	req = httptest.NewRequest("GET", "/items/nope", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Errorf("missing: got %d want 404", rec.Code)
	}

	// wrong method -> 405 (the router emits this)
	req = httptest.NewRequest("DELETE", "/items", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("wrong method: got %d want 405", rec.Code)
	}
}
*/
