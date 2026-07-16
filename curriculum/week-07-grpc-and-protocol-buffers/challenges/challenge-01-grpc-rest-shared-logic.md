# Challenge 1 — One Domain, Two Transports: gRPC and REST Over the Same Service

> **Estimated time:** 2.5 hours. **Prerequisite:** Exercise 2 complete (a working gRPC NotesService); your Week-5 chi REST handler and Week-6 service/repository layers on hand. **Citations:** grpc-go basics <https://grpc.io/docs/languages/go/basics/>; the core-concepts page on transports <https://grpc.io/docs/what-is-grpc/core-concepts/>.

## The premise

This is the central claim of the week, made concrete: a cloud-native service speaks **gRPC to its peers and REST to the world**, and both front doors are thin adapters over *one* shared service layer. You already have the pieces. Week 5 gave you a chi router and HTTP handlers. Week 6 gave you a `service.Service` and a Postgres-backed repository. This week gave you a gRPC server. The challenge is to wire all three together so that a single `service.Service` instance — constructed once — is called by *both* a chi handler and a gRPC handler, and to prove that a write through one transport is visible through the other.

If at the end of this you can run `curl` to create a note and `grpcurl` to read it back (or vice versa), you have built the pattern that runs in every mature microservice you will ever touch.

## What you build

A single Go module, `notes`, with this layout:

```
notes/
├── go.mod
├── buf.yaml
├── buf.gen.yaml
├── proto/notes/v1/notes.proto         # Exercise 1's schema
├── gen/notes/v1/                       # buf generate output (do not edit)
├── internal/
│   ├── domain/note.go                  # domain.Note, sentinel errors (Week 5)
│   ├── service/service.go              # the SHARED layer (Weeks 5-6)
│   ├── repo/memory.go                  # in-memory repo so it runs w/o Postgres
│   ├── restserver/handler.go           # chi adapter (Week 5)
│   └── grpcserver/server.go            # gRPC adapter (this week)
└── cmd/server/main.go                  # starts BOTH transports over one svc
```

The load-bearing file is `cmd/server/main.go`: it constructs the service exactly once and hands the *same pointer* to both servers.

## Setup

### 1. The shared service and a memory repository

`internal/service/service.go` is unchanged from Week 6 — it takes a `repo`, validates input, returns domain types and sentinel errors. `internal/domain/note.go` declares the sentinels both transports map from:

```go
package domain

import "errors"

var (
	ErrNotFound      = errors.New("note not found")
	ErrInvalidInput  = errors.New("invalid note")
	ErrAlreadyExists = errors.New("note already exists")
)

type Note struct {
	ID         string
	Title      string
	Body       string
	Tags       []string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	ArchivedAt *time.Time
}
```

`internal/repo/memory.go` implements the repository interface over a `map` so the challenge runs without Postgres (swap in the Week-6 pgx repo by changing one line in `main.go`).

### 2. Both transports over one service

```go
// cmd/server/main.go
func main() {
	ctx := context.Background()
	repo := repo.NewMemory()
	svc := service.New(repo) // <-- constructed ONCE

	// Front door #1: REST over chi.
	rest := restserver.New(svc)
	go func() {
		slog.Info("REST listening", "addr", ":8080")
		if err := http.ListenAndServe(":8080", rest.Router()); err != nil {
			slog.Error("rest serve", "err", err)
		}
	}()

	// Front door #2: gRPC. SAME svc pointer.
	lis, err := net.Listen("tcp", "127.0.0.1:50051")
	if err != nil {
		slog.Error("listen", "err", err)
		os.Exit(1)
	}
	g := grpc.NewServer(grpc.ChainUnaryInterceptor(loggingUnary, recoveryUnary))
	notesv1.RegisterNotesServiceServer(g, grpcserver.New(svc))

	slog.Info("gRPC listening", "addr", ":50051")
	if err := g.Serve(lis); err != nil {
		slog.Error("grpc serve", "err", err)
	}
	_ = ctx
}
```

### 3. The two adapters, side by side

The chi handler (REST) and the gRPC handler each translate their transport's vocabulary to and from the domain, then delegate:

```go
// internal/restserver/handler.go — the REST adapter (abridged)
func (h *Handler) createNote(w http.ResponseWriter, r *http.Request) {
	var in createReq
	_ = json.NewDecoder(r.Body).Decode(&in)
	n, err := h.svc.Create(r.Context(), domain.Note{Title: in.Title, Body: in.Body, Tags: in.Tags})
	if err != nil {
		writeHTTPError(w, err) // maps domain sentinels -> 400/404/409/500
		return
	}
	writeJSON(w, http.StatusCreated, toJSON(n))
}

// internal/grpcserver/server.go — the gRPC adapter (abridged)
func (s *Server) CreateNote(ctx context.Context, req *notesv1.CreateNoteRequest) (*notesv1.CreateNoteResponse, error) {
	n, err := s.svc.Create(ctx, domain.Note{Title: req.GetTitle(), Body: req.GetBody(), Tags: req.GetTags()})
	if err != nil {
		return nil, mapError(err) // maps domain sentinels -> codes.Code
	}
	return &notesv1.CreateNoteResponse{Note: toProto(n)}, nil
}
```

Both call `h.svc.Create(...)`/`s.svc.Create(...)` — the *same method on the same instance*. The only difference is the translation at the edges.

## Prove it works

Start the server, then exercise both transports against the same data:

```bash
# Create via REST
curl -s localhost:8080/notes -d '{"title":"shared","body":"one svc","tags":["work"]}'
# -> {"id":"n-1", ...}

# Read the SAME note via gRPC (grpcurl uses server reflection or the proto)
grpcurl -plaintext -d '{"id":"n-1"}' \
  localhost:50051 crunch.notes.v1.NotesService/GetNote
# -> { "note": { "id": "n-1", "title": "shared", ... } }
```

The note created over HTTP is readable over gRPC because both adapters wrote to and read from one `service.Service`. Reverse it — create over gRPC, read over REST — and the symmetry holds.

## Acceptance criteria

- [ ] One Go module builds clean (`go build ./...`, `go vet ./...`).
- [ ] `service.Service` is constructed exactly once in `main.go` and shared by both servers (grep proves there is a single `service.New`).
- [ ] Neither `internal/service` nor `internal/repo` imports the generated `notesv1` package. (The domain layer is transport-agnostic; only the adapters touch protobuf.)
- [ ] The chi handler maps domain sentinel errors to HTTP status codes; the gRPC handler maps the *same* sentinels to `codes.Code` via one `errors.Is` switch.
- [ ] A note created over REST is retrievable over gRPC by id, and vice versa (demonstrate both directions).
- [ ] The gRPC server registers a logging interceptor; every RPC produces one structured log line with method, code, and duration.
- [ ] Both servers honor `context`: a canceled client context aborts the in-flight call on each transport.

## Reflection questions (write into RESULTS.md)

1. **Where does validation live?** If `title` must be 1–200 characters, in which layer do you enforce it so that *both* transports get the rule for free? What goes wrong if you enforce it in the chi handler instead?
2. **Two error vocabularies, one source.** The service returns `domain.ErrNotFound`. REST renders it as HTTP 404, gRPC as `codes.NotFound`. Trace one sentinel through both `mapError` functions. What happens to an *unmapped* sentinel on each transport, and why is that the safe default?
3. **The import boundary.** Why is it a design smell for `internal/service` to import `notesv1`? What concrete future change does that boundary protect you from?
4. **Streaming asymmetry.** REST has no clean equivalent of `WatchNotes` (server-streaming). How would you expose that capability to a browser — Server-Sent Events? long-poll? — and why is gRPC's first-class streaming the reason services prefer gRPC to each other?

## Stretch goals (optional)

- **Add `grpc-gateway` or a hand-written REST-to-gRPC shim** so the REST surface is *generated* from the proto rather than hand-written, and compare the maintenance cost against the hand-written chi handler.
- **Enable server reflection** (`google.golang.org/grpc/reflection`'s `reflection.Register(g)`) so `grpcurl` works without passing the `.proto`, then explain the security trade-off of leaving reflection on in production.
- **Wire the Week-6 pgx repository** in place of the memory repo by changing the single `repo.New...` line in `main.go`, and confirm both transports now read and write Postgres.

## Submission

Place all artifacts under `challenges/challenge-01/`. Commit with:

```
challenge-01: notes over gRPC + REST against one shared service
```

Include `RESULTS.md` with the four reflection answers and the captured `curl` + `grpcurl` transcript proving cross-transport visibility in both directions.
