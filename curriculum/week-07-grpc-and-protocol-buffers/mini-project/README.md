# Mini-Project — Lab 07: `notes` Over gRPC and REST Against One Shared Service

> Build the `notes` service so it answers on two transports at once: gRPC for service-to-service calls and REST (your Week-5 chi surface) for the world — both adapters over a single `service.Service` instance, both backed by the same store. Define the contract in a `.proto`, generate Go with `buf`, implement the gRPC server with a logging and a recovery interceptor and precise status codes, stand up a gRPC client, keep the REST surface live, and prove that a write through one transport is visible through the other. By the end you have the exact skeleton every mature cloud-native Go service ships: a proto-first contract, a shared domain layer, two thin transport adapters, and tests that hold both honest.

This is the capstone of Phase II so far. Weeks 5 and 6 gave you REST, a service layer, and a Postgres repository. This week you add the second transport without duplicating a line of business logic. The shape is genuinely production-shaped, and an engineer who has shipped gRPC will recognize every directory in it.

**Estimated time:** ~8 hours (split across Thursday, Friday, Saturday, Sunday in the suggested schedule).

---

## What you will build

A single Go module, `notes`, with:

- `proto/notes/v1/notes.proto` — the contract (Exercise 1's schema), plus `buf.yaml` and `buf.gen.yaml`.
- `gen/notes/v1/` — `buf generate` output; never edited by hand.
- `internal/domain/` — `domain.Note` and the sentinel errors both transports map from.
- `internal/service/` — the **shared** service layer: validation, business rules, orchestration. Imports neither protobuf nor chi.
- `internal/repo/` — a repository interface with an in-memory implementation (runs without Postgres) and, optionally, the Week-6 pgx/sqlc implementation behind the same interface.
- `internal/grpcserver/` — the gRPC adapter: protobuf ↔ domain conversion, `mapError`, the four+ RPC methods, the interceptors.
- `internal/restserver/` — the Week-5 chi adapter, kept live and pointed at the same service.
- `cmd/server/` — constructs the service once and serves both transports.
- `cmd/client/` — a gRPC client CLI exercising unary and streaming RPCs.
- `*_test.go` — tests for validation→codes, deadline propagation, streaming, and cross-transport visibility.

---

## The schema

`proto/notes/v1/notes.proto`:

```proto
syntax = "proto3";

package crunch.notes.v1;
option go_package = "example.com/notes/gen/notes/v1;notesv1";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";
import "google/protobuf/field_mask.proto";

message Note {
  string id = 1;
  string title = 2;
  string body = 3;
  repeated string tags = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
  optional google.protobuf.Timestamp archived_at = 7;

  reserved 9, 10;
  reserved "color", "pinned";
}

message CreateNoteRequest { string title = 1; string body = 2; repeated string tags = 3; }
message CreateNoteResponse { Note note = 1; }

message GetNoteRequest { string id = 1; }
message GetNoteResponse { Note note = 1; }

message ListNotesRequest { int32 page_size = 1; string page_token = 2; }
message ListNotesResponse { repeated Note notes = 1; string next_page_token = 2; }

message UpdateNoteRequest { Note note = 1; google.protobuf.FieldMask update_mask = 2; }
message UpdateNoteResponse { Note note = 1; }

message DeleteNoteRequest { string id = 1; }

enum NoteEventKind {
  NOTE_EVENT_KIND_UNSPECIFIED = 0;
  NOTE_EVENT_KIND_CREATED = 1;
  NOTE_EVENT_KIND_UPDATED = 2;
  NOTE_EVENT_KIND_DELETED = 3;
  NOTE_EVENT_KIND_KEEPALIVE = 4;
}
message WatchNotesRequest { string tag_filter = 1; }
message NoteEvent {
  NoteEventKind kind = 1;
  Note note = 2;
  string note_id = 3;
  google.protobuf.Timestamp at = 4;
}

service NotesService {
  rpc CreateNote(CreateNoteRequest) returns (CreateNoteResponse);
  rpc GetNote(GetNoteRequest) returns (GetNoteResponse);
  rpc ListNotes(ListNotesRequest) returns (ListNotesResponse);
  rpc UpdateNote(UpdateNoteRequest) returns (UpdateNoteResponse);
  rpc DeleteNote(DeleteNoteRequest) returns (google.protobuf.Empty);
  rpc WatchNotes(WatchNotesRequest) returns (stream NoteEvent);
}
```

This `.proto` is the contract. The gRPC adapter, the gRPC client, and the tests all reference the single generated package.

---

## Rules

- **You may** read grpc.io and protobuf.dev documentation, the `buf` docs, the Week-7 lecture notes and exercises, the `grpc/grpc-go` source, and any free Go documentation, plus your own Week-5 and Week-6 code.
- **You may NOT** depend on third-party modules other than:
  - `google.golang.org/grpc` and `google.golang.org/protobuf` (always).
  - `github.com/go-chi/chi/v5` (the REST surface).
  - `github.com/jackc/pgx/v5` (only if you wire the optional Postgres repo).
  - `buf` is a build-time CLI for codegen, not a module dependency.
- Target **Go 1.22+**. Use `log/slog`, generics where they earn their place, and the modern `grpc.NewServer` / `grpc.NewClient` constructors — **never the deprecated `grpc.Dial`**.
- No `.Result`-style anti-patterns: no goroutine leaks, no ignored errors from `stream.Send`/`stream.Recv`, no swallowed `ctx` cancellation.
- **`context.Context` everywhere**: the first parameter of every service and repo method; passed unbroken from the gRPC/REST edge down to the store.
- **Every RPC validates its input and returns a precise `codes.Code`** via `status.Error`/`status.Errorf` — never a raw error.
- **Interceptors are required**: a logging interceptor (one structured line per RPC: method, code, duration, peer) and a panic-recovery interceptor (panic → `codes.Internal`, no stack leaked), chained recovery-outermost.
- **The service and repo layers import neither `notesv1` nor `chi`.** Only the adapters touch transport types.
- Every gRPC client call sets a deadline. The `ClientConn` is constructed once and reused.
- Server reflection (`reflection.Register`) is **optional** — enable it to make `grpcurl` work without the proto, and note the trade-off in your README.

---

## Project structure

```
notes/
├── go.mod
├── buf.yaml
├── buf.gen.yaml
├── proto/
│   └── notes/v1/notes.proto
├── gen/
│   └── notes/v1/{notes.pb.go, notes_grpc.pb.go}
├── internal/
│   ├── domain/
│   │   └── note.go                 # domain.Note + sentinel errors
│   ├── service/
│   │   └── service.go              # shared logic; no protobuf, no chi
│   ├── repo/
│   │   ├── repo.go                 # the Repository interface
│   │   ├── memory.go               # in-memory impl (default)
│   │   └── postgres.go             # optional Week-6 pgx/sqlc impl
│   ├── grpcserver/
│   │   ├── server.go               # the NotesServiceServer adapter
│   │   ├── convert.go              # toProto / toDomain
│   │   ├── maperr.go               # domain sentinel -> codes.Code
│   │   └── interceptors.go         # logging + recovery
│   └── restserver/
│       └── handler.go              # the Week-5 chi adapter
├── cmd/
│   ├── server/main.go              # serves BOTH transports over one svc
│   └── client/main.go              # gRPC client CLI
├── grpcserver_test.go              # validation -> codes, deadline, streaming
└── crosstransport_test.go          # write over one transport, read over the other
```

---

## Acceptance criteria

### Server / gRPC

- [ ] `buf lint` and `buf build` pass clean; `buf generate` produces `gen/notes/v1/`.
- [ ] The gRPC server type embeds `notesv1.UnimplementedNotesServiceServer`.
- [ ] `CreateNote` validates `title` (1–200 runes), `tags` (≤ 16), and rejects violations with `codes.InvalidArgument`.
- [ ] `GetNote`/`UpdateNote`/`DeleteNote` return `codes.NotFound` for an unknown id.
- [ ] `ListNotes` paginates: respects `page_size` (capped server-side), returns a `next_page_token`, and an empty token at the end.
- [ ] `UpdateNote` honors the `FieldMask` — only masked paths are written.
- [ ] `WatchNotes` emits a snapshot/CREATED event flow, emits a `KEEPALIVE` periodically, and exits within 100ms of `stream.Context()` cancellation.
- [ ] Domain errors map to codes via a single `errors.Is` switch; an unmapped error falls through to `codes.Internal` with the detail logged, not leaked.
- [ ] Logging and recovery interceptors are registered, recovery outermost.

### REST

- [ ] The Week-5 chi surface stays live on `:8080` against the *same* `service.Service`.
- [ ] The chi handler maps the *same* domain sentinels to HTTP status codes (404/409/400/500).
- [ ] `service.New` is called exactly once in `cmd/server/main.go`.

### Client

- [ ] `cmd/client` constructs **one** `ClientConn` with `grpc.NewClient` + `insecure.NewCredentials()` and reuses it.
- [ ] Every call sets a deadline.
- [ ] Subcommands: `create <title>`, `get <id>`, `list`, `watch [tag]`. `watch` iterates the server stream with `Recv` until `io.EOF` and prints non-keepalive events.

### Tests

- [ ] All tests pass (`go test ./...`).
- [ ] Validation: empty title → `InvalidArgument`; unknown id → `NotFound`; per documented rule, the expected code.
- [ ] Deadline: a deliberately slow handler (e.g. a `"slow"` title that sleeps 1s) called with a 200ms deadline yields `codes.DeadlineExceeded` client-side and the server loop exits within ~deadline, not the full sleep.
- [ ] Streaming: a `WatchNotes` subscription receives a CREATED event for a concurrently-created note and stops cleanly on cancel.
- [ ] Cross-transport: a note created over REST is retrievable over gRPC by id, and vice versa.

---

## Day-by-day plan

### Thursday afternoon (2h) — Contract and scaffolding

1. `go mod init example.com/notes`. Add `buf.yaml` and `buf.gen.yaml` (Lecture 1 §8). Place the `.proto` at `proto/notes/v1/`.
2. `go install` the two plugins; run `buf lint`, `buf build`, `buf generate`. Confirm `gen/notes/v1/` compiles.
3. Bring over `internal/domain`, `internal/service`, and `internal/restserver` from Weeks 5–6 unchanged. Add `internal/repo/memory.go`.
4. Stub `internal/grpcserver/server.go` embedding `UnimplementedNotesServiceServer`. `go build ./...` clean.

### Friday morning (3h) — gRPC server, interceptors, client

1. Implement `convert.go` (`toProto`/`toDomain`) and `maperr.go` (the `errors.Is` switch).
2. Implement `CreateNote`, `GetNote`, `ListNotes`, `UpdateNote`, `DeleteNote` as thin adapters over `svc`.
3. Implement `WatchNotes` with snapshot + keepalive + cancellation.
4. Implement `interceptors.go` (logging + recovery); wire both into `grpc.NewServer` in `cmd/server/main.go`, recovery outermost.
5. Implement `cmd/client` with the four subcommands. Run end-to-end: server in one terminal, `client create`/`get`/`watch` in another.

### Saturday (2.5h) — Tests and cross-transport proof

1. Write `grpcserver_test.go`: validation→codes, the deadline test, the streaming test. Start the server on `127.0.0.1:0` per test for isolation.
2. Write `crosstransport_test.go`: create over REST, read over gRPC, and the reverse.
3. Run `go test ./... -race`. Fix any data race the `-race` detector finds in the watch fan-out.

### Sunday (0.5h) — Polish

1. `go vet ./...`, `gofmt -l .` (should be empty), `buf lint`.
2. Update `README.md` with the actual run commands and the reflection on whether you enabled server reflection.
3. Tag the commit.

---

## What you will be graded on

| Area                                                                       | Weight |
|----------------------------------------------------------------------------|-------:|
| Schema correctness (proto3 hygiene, reserved fields, well-known types, buf clean) | 15% |
| gRPC server (validation, status codes, FieldMask update, pagination, streaming)   | 25% |
| Shared service (one `service.New`; service/repo free of transport imports)        | 15% |
| Client (single reused conn, deadlines, stream drained to `io.EOF`)                | 10% |
| Interceptors and error mapping (logging, recovery, `errors.Is` switch)            | 10% |
| Tests (validation→codes, deadline, streaming, cross-transport)                    | 20% |
| Build hygiene (`go vet`, `gofmt`, no ignored errors, `-race` clean)               |  5% |
| **Total**                                                                         | **100%** |

The passing bar is **80**. The "you would put this behind a service mesh" bar is **90**.

---

## A note on installing `buf` and the plugins

Some learners will run this in an environment without `buf` or the protoc Go plugins installed. Installing them is your responsibility and free:

- `buf`: <https://buf.build/docs/installation/> (a single binary; `brew install bufbuild/buf/buf` on macOS).
- The Go plugins (once): `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`. `buf` finds them on your `PATH`.
- Optional: `grpcurl` (<https://github.com/fullstorydev/grpcurl>) for smoke-testing without the client. Go 1.22+ from <https://go.dev/dl/>.

You do not need a separate `protoc` install — `buf` drives the plugins directly.

---

## Submission

Commit the full module (excluding any `bin/` artifacts) on your branch with the message:

```
mini-project: notes over gRPC + REST against one shared service
```

Push and open a PR against `main`. The PR description should include:

1. The output of `go test ./... -race` showing all tests green.
2. A short transcript: `client watch work` in one window, `client create "ship gRPC"` (tagged `work`) in another, the event log showing the CREATED event.
3. A `curl` create + `grpcurl` get (or the reverse) proving cross-transport visibility.
4. One sentence on whether you enabled server reflection and why.

If `go test ./...` is not green, the PR is not reviewable — fix the failures first.
