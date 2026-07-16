// Exercise 3 — chi Router, the Service Seam, and Graceful Shutdown
//
// GOAL
//   Wire a chi router with a versioned route group, a service behind a
//   Repository interface (in-memory impl), and graceful shutdown driven by
//   signal.NotifyContext. Prove that an in-flight request finishes during a
//   SIGTERM drain and that ListenAndServe's ErrServerClosed is treated as
//   success, not failure.
//
// HOW TO RUN
//   mkdir ex03 && cd ex03 && go mod init ex03
//   go get github.com/go-chi/chi/v5
//   # save this file as main.go and the test below as main_test.go
//   go test ./... && go vet ./...
//   go run .   # then in another shell:
//     curl -s localhost:8080/v1/notes -d '{"title":"hi","body":"yo"}' -H 'content-type: application/json' -i
//     curl -s localhost:8080/v1/notes/1 -i
//     kill -TERM <pid>   # watch it drain and exit cleanly
//
// TASKS
//   1. Read newRouter: a /healthz route plus a /v1/notes group with full CRUD.
//   2. Implement the Create and Get handlers against the Service (already done);
//      confirm they thread r.Context() into the service.
//   3. Implement runServer: start in a goroutine, select on the server-error
//      channel and the signal context, and on signal call srv.Shutdown with a
//      fresh 10s deadline. Treat http.ErrServerClosed as success.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
)

// ---- domain, repo, service ----

type Note struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

var ErrNotFound = errors.New("note not found")

type Repository interface {
	Create(ctx context.Context, n Note) (Note, error)
	Get(ctx context.Context, id string) (Note, error)
}

type memRepo struct {
	mu    sync.RWMutex
	notes map[string]Note
	seq   int
}

func newMemRepo() *memRepo { return &memRepo{notes: map[string]Note{}} }

func (r *memRepo) Create(ctx context.Context, n Note) (Note, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.seq++
	n.ID = fmt.Sprintf("%d", r.seq)
	r.notes[n.ID] = n
	return n, nil
}

func (r *memRepo) Get(ctx context.Context, id string) (Note, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.notes[id]
	if !ok {
		return Note{}, ErrNotFound
	}
	return n, nil
}

type Service struct{ repo Repository }

func (s *Service) Create(ctx context.Context, title, body string) (Note, error) {
	return s.repo.Create(ctx, Note{Title: title, Body: body})
}
func (s *Service) Get(ctx context.Context, id string) (Note, error) {
	return s.repo.Get(ctx, id)
}

// ---- handlers ----

type Handler struct{ svc *Service }

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var req struct{ Title, Body string }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad request"})
		return
	}
	n, err := h.svc.Create(r.Context(), req.Title, req.Body)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		return
	}
	w.Header().Set("Location", "/v1/notes/"+n.ID)
	writeJSON(w, http.StatusCreated, n)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	n, err := h.svc.Get(r.Context(), chi.URLParam(r, "id"))
	if errors.Is(err, ErrNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, n)
}

func newRouter(h *Handler) http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Route("/v1/notes", func(r chi.Router) {
		r.Post("/", h.Create)
		r.Get("/{id}", h.Get)
	})
	return r
}

// ---- server + graceful shutdown ----

func runServer(srv *http.Server) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.ListenAndServe() }()

	select {
	case err := <-serverErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func main() {
	h := &Handler{svc: &Service{repo: newMemRepo()}}
	srv := &http.Server{Addr: ":8080", Handler: newRouter(h), ReadHeaderTimeout: 5 * time.Second}
	if err := runServer(srv); err != nil {
		fmt.Fprintln(os.Stderr, "server:", err)
		os.Exit(1)
	}
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

func TestRouterCRUD(t *testing.T) {
	h := &Handler{svc: &Service{repo: newMemRepo()}}
	ts := httptest.NewServer(newRouter(h))
	defer ts.Close()

	resp, err := http.Post(ts.URL+"/v1/notes", "application/json",
		strings.NewReader(`{"Title":"hi","Body":"yo"}`))
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d want 201", resp.StatusCode)
	}
	if resp.Header.Get("Location") == "" {
		t.Error("create: missing Location")
	}

	resp2, _ := http.Get(ts.URL + "/v1/notes/1")
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("get: got %d want 200", resp2.StatusCode)
	}

	resp3, _ := http.Get(ts.URL + "/v1/notes/999")
	resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("get missing: got %d want 404", resp3.StatusCode)
	}
}
*/
