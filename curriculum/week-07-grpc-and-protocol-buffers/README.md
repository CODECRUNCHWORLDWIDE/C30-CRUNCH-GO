# Week 7 — gRPC and Protocol Buffers in Go: Proto3, `buf`, the Four Call Types, Interceptors, Status Codes, and One Domain Over Two Transports

Welcome to **C30 · Crunch Go**, Week 7. Weeks 5 and 6 built the `notes` service the way the world reaches it: a `net/http` + `chi` REST surface (Week 5) over a service and repository layer, then a real Postgres backing with `pgx`, `sqlc`, and `golang-migrate` (Week 6). JSON over HTTP is how a browser, a `curl` one-liner, and a junior on another team all talk to you — verbose, schema-optional, easy to reach. This week you add the *second* front door, the one your services use to talk to *each other*: gRPC over Protocol Buffers. By Sunday you will define a service in a `.proto`, generate Go with `buf`, implement a gRPC server and client against the **same** service layer your REST handlers already call, instrument them with interceptors, and return precise status codes — one domain, two transports.

The first thing to internalize is that **gRPC is two pieces, not one**. The first piece is **Protocol Buffers** (proto3): a schema language and a compact binary wire format. A `.proto` file declares messages and services; a code generator reads it and emits Go structs, nil-safe getters, and client/server interfaces. The encoding is tagged-and-varint — typically 3–10x smaller than the equivalent JSON and far faster to parse, because the keys are integers, not strings. The second piece is **gRPC** itself: a remote-procedure-call protocol over **HTTP/2**, where each RPC is one multiplexed stream, the method is the `:path`, and the final status rides in HTTP/2 trailers. The two are separable in theory and inseparable in practice — nearly all gRPC is protobuf and nearly all new protobuf is gRPC. We treat them as a pair. Lecture 1 is protobuf and the `buf` toolchain; Lecture 2 is the gRPC server and client.

The second thing to internalize is **the four call types**, because they generate four different Go method shapes and the choice is a *design* decision, not a transport one. **Unary** is one request, one response: `func (s *server) GetNote(ctx, *Req) (*Resp, error)` — the shape you already know. **Server-streaming** is one request and a stream of responses, written with `stream.Send` and a `select` on `stream.Context().Done()`. **Client-streaming** is a stream of requests and one response, read with `stream.Recv` until `io.EOF` then `SendAndClose`. **Bidirectional** is both directions, independent, with concurrent `Recv` and `Send`. All four ride the same HTTP/2 framing, but picking the wrong shape early costs a refactor. Lecture 2 walks each one with real Go.

The third thing to internalize is the week's thesis: **one domain, two transports, sharing the service layer**. The gRPC handler is a *thin adapter* over the same `service.Service` your chi handler calls — exactly as the chi handler is a thin adapter over it. The handler's only real jobs are translating protobuf to and from domain types at the edge, and delegating. Validation, business rules, transactions, and persistence live in the service and repository, written once and exercised twice. A bug fix in `svc.Create` fixes both transports; a new rule lands once. This is *why* the cloud-native pattern is "REST to the world, gRPC between services" rather than two parallel implementations that drift apart. Challenge 1 and the mini-project make it concrete: a note written over `curl` is read back over `grpcurl`.

The fourth thing to internalize is that **gRPC reports failure as a `codes.Code`, not an error string**. A handler does not return a bare `errors.New(...)` — it returns `status.Error(codes.NotFound, ...)` or `status.Errorf(codes.InvalidArgument, ...)`, and the client decodes the same code back with `status.FromError`. There are sixteen codes, and choosing well is a senior skill: `Unauthenticated` ("who are you?") versus `PermissionDenied` ("you may not"); `InvalidArgument` ("malformed regardless of state") versus `FailedPrecondition` ("state forbids it"); `Internal` ("our bug, do not retry") versus `Unavailable` ("transient, retry with backoff"). Returning `Internal` for everything is the gRPC `catch (Exception)` — it eats the one bit the operator needs. Lecture 3 has the table and the decision matrix. The payoff of "one domain, two transports": your service returns *sentinel domain errors*, and each transport maps them — chi to HTTP codes, gRPC to `codes.Code` — in one `errors.Is` switch.

The fifth thing to internalize is **field evolution and backward compatibility**. The field *number* — not the name — is the wire contract. Rename a field and old bytes still decode; renumber it or change its type and you have a production outage the next time a v1 client meets a v2 server. The rules are small and non-negotiable: never reuse a number, never change a type incompatibly, always `reserve` a removed number (and ideally its name). Proto3's forward-compatibility guarantee — a server silently skips fields it does not recognize — is what makes "you may add new fields freely" safe. Challenge 2 evolves the schema and makes `buf breaking` the CI gate that mechanically refuses any wire-incompatible change.

The sixth thing to internalize is **`buf` as the modern toolchain over raw `protoc`**. The historical workflow is `protoc` with a fistful of `--plugin` and `-I` flags, no dependency management, and no linting. `buf` is a single binary that lints (`buf lint`), compiles (`buf build`), generates (`buf generate`), and detects breaking changes (`buf breaking`) from two small YAML files. It drives `protoc-gen-go` (messages) and `protoc-gen-go-grpc` (services) for you. Wiring `buf lint` and `buf breaking` into CI turns the proto3 evolution rules from hopes into gates.

## Learning objectives

By the end of this week, you will be able to:

- **Write** a proto3 `.proto` from scratch: versioned `package`, `option go_package`, messages, scalar fields, `enum` (with `*_UNSPECIFIED = 0`), `repeated`, `oneof`, `map`, nested types, the well-known types (`Timestamp`, `Duration`, `Empty`, `FieldMask`), `optional` for presence, and `reserved` for safe deletion.
- **Explain** the proto3 wire format well enough to predict a message's byte size: varints, `tag = (field_number << 3) | wire_type`, the four common wire types, why fields 1–15 cost one tag byte, and why default-valued scalars emit nothing — then **verify** with `proto.Size`.
- **Map** every proto3 scalar to its Go type and choose correctly between `int32`/`sint32`, `string`/`bytes`, and bare-versus-`optional` fields.
- **Configure** `buf.yaml` and `buf.gen.yaml`, install `protoc-gen-go`/`protoc-gen-go-grpc`, and run `buf lint`, `buf build`, `buf generate`, and `buf breaking`.
- **Read** the generated Go: the message struct, nil-safe getters, the `NotesServiceClient` interface, the `NotesServiceServer` interface, and the `UnimplementedNotesServiceServer` embed that keeps a server compiling across schema growth.
- **Implement** all four gRPC call types in Go with the correct generated signatures — unary, server-streaming, client-streaming, bidirectional.
- **Stand up** a server with `grpc.NewServer`, `RegisterNotesServiceServer`, `Serve`, and `GracefulStop`, and a client with the modern `grpc.NewClient` and `insecure.NewCredentials()` — never the deprecated `grpc.Dial`.
- **Serve** the same domain over both gRPC and REST from a single shared `service.Service`, keeping the service and repository layers free of any transport import.
- **Write** server and client interceptors: a logging interceptor (`slog`: method, code, duration, peer), a panic-recovery interceptor, and a request-id/metadata interceptor with `metadata.FromIncomingContext`/`metadata.NewOutgoingContext`, chained recovery-outermost.
- **Return** precise `codes.Code` values via `status.Error`/`status.Errorf`, decode them with `status.FromError`, attach rich details with `status.WithDetails` and `errdetails.BadRequest`, and map domain sentinel errors to codes.
- **Propagate** deadlines: set one on every client call, honor `ctx.Done()` in every handler, and trace the `grpc-timeout` header across a service hop.
- **Evolve** a schema while preserving wire compatibility, and **gate** it with `buf breaking` against the previous version.

## Prerequisites

- **Week 5 of C30 complete.** You can build a `net/http` + `chi` REST service with a handler/service/repository split, and you wrote sentinel domain errors. The gRPC handler is the same service layer with a different adapter on top.
- **Week 6 of C30 complete.** You have a `service.Service` and a `pgx`/`sqlc` repository behind it. This week reuses both unchanged; the mini-project can run on an in-memory repo or the real Postgres one behind the same interface.
- **Go 1.22+.** This week uses `log/slog`, the modern `grpc.NewServer`/`grpc.NewClient` constructors, and generics where they earn their place. Confirm with `go version` reporting `go1.22` or newer. Install from <https://go.dev/dl/> if needed.
- **`buf` installed.** A single binary; `brew install bufbuild/buf/buf` on macOS or see <https://buf.build/docs/installation/>.
- **`protoc-gen-go` and `protoc-gen-go-grpc` on your `PATH`.** Install once with `go install google.golang.org/protobuf/cmd/protoc-gen-go@latest` and `go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest`.
- **`grpcurl` recommended but optional.** `curl` for gRPC, useful for smoke tests. <https://github.com/fullstorydev/grpcurl>.

## Topics covered

- **The proto3 language.** `syntax`, versioned `package`, `option go_package` (both halves), `message`, `enum`, `oneof`, `map`, `repeated`, nested types, the scalar types and their Go mappings, the well-known types, `optional` and field presence, `reserved` for compatibility.
- **The proto3 wire format.** Tag + wire type, varint encoding, length-delimited fields, packed repeated scalars, the worked size estimate, verification with `proto.Size`.
- **The `buf` toolchain.** `buf.yaml`, `buf.gen.yaml` with `protoc-gen-go` + `protoc-gen-go-grpc`, `buf lint`, `buf build`, `buf generate`, `buf breaking`.
- **The generated Go shape.** Message struct, nil-safe getters, `NotesServiceClient`, `NotesServiceServer`, `UnimplementedNotesServiceServer`, `RegisterNotesServiceServer`.
- **gRPC over HTTP/2.** One RPC per stream, the `:path`, length-prefixed protobuf, status in trailers.
- **The four call types in Go.** Unary `(ctx, req) (resp, error)`; server-streaming with `stream.Send`; client-streaming with `Recv`/`SendAndClose`; bidirectional with concurrent `Recv`/`Send`. The client side: `Recv` until `io.EOF`, `CloseAndRecv`, `CloseSend`.
- **Server and client setup.** `net.Listen`, `grpc.NewServer`, `RegisterNotesServiceServer`, `Serve`, `GracefulStop`; `grpc.NewClient`, `insecure.NewCredentials()`, one reused `ClientConn`.
- **One domain, two transports.** The shared `service.Service`; the gRPC and chi handlers as thin adapters; the transport-free service/repository layers; protobuf↔domain conversion at the edge.
- **Interceptors.** `UnaryServerInterceptor`/`StreamServerInterceptor` signatures, `ChainUnaryInterceptor` ordering (outside-in), logging, panic recovery, request-id/metadata, client interceptors.
- **Status and errors.** `status.Error`/`status.Errorf`, the sixteen `codes.Code` values, the decision matrix, `status.FromError`, `status.WithDetails` + `errdetails.BadRequest_FieldViolation`, mapping `domain.ErrNotFound`-style sentinels with `errors.Is`.
- **Deadlines and cancellation.** Client deadline → `grpc-timeout` header → server `ctx.Done()`; propagation into downstream calls; `status.FromContextError`.

## Weekly schedule

The schedule adds up to approximately **36 hours**. Treat it as a target, not a contract. Schema work rewards a fresh mind; do not write your `.proto` files at 2am.

| Day       | Focus                                                                 | Lectures | Exercises | Challenges | Quiz/Read | Homework | Mini-Project | Self-Study | Daily Total |
|-----------|-----------------------------------------------------------------------|---------:|----------:|-----------:|----------:|---------:|-------------:|-----------:|------------:|
| Monday    | proto3 language, wire format, scalar/well-known types, `buf`          |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Tuesday   | gRPC over HTTP/2, the four call types, server + client setup          |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Wednesday | interceptors, status codes, error details, deadline propagation       |    2h    |    2h     |     0h     |    0.5h   |   1h     |     0h       |    0.5h    |     6h      |
| Thursday  | one domain / two transports; Challenge 1 + schema-evolution challenge  |    0.5h  |    0h     |     3h     |    0h     |   1h     |     1.5h     |    0h      |     6h      |
| Friday    | Mini-project — proto, gRPC server + interceptors, client              |    0h    |    0h     |     1h     |    0.5h   |   1h     |     3h       |    0.5h    |     6h      |
| Saturday  | Mini-project — tests, cross-transport proof, deadline tests           |    0h    |    0h     |     0h     |    0h     |   0h     |     2.5h     |    0h      |     2.5h    |
| Sunday    | Quiz, review, schema-evolution reflection                            |    0h    |    0h     |     0h     |    1h     |   0h     |     2.5h     |    0h      |     3.5h    |
| **Total** |                                                                       | **6.5h** | **8h**    | **4h**     | **3.5h**  | **5h**   | **10h**      | **2.5h**   | **36h**     |

## How to navigate this week

| File | What's inside |
|------|---------------|
| [README.md](./README.md) | This overview (you are here) |
| [resources.md](./resources.md) | grpc.io docs, protobuf.dev, the `buf` docs, the grpc-go/protobuf godoc and source |
| [lecture-notes/01-protobuf-and-buf.md](./lecture-notes/01-protobuf-and-buf.md) | proto3 language, scalar→Go mappings, enums/`oneof`/`map`/nested, well-known types, presence, `reserved`, the wire format with a worked size estimate, the `buf` toolchain, and the generated Go shape |
| [lecture-notes/02-grpc-server-and-client-and-streaming.md](./lecture-notes/02-grpc-server-and-client-and-streaming.md) | gRPC over HTTP/2; the four call types in Go; server setup with `GracefulStop`; the client with `grpc.NewClient`; the adapter layer; one domain over two transports |
| [lecture-notes/03-interceptors-status-codes-errors.md](./lecture-notes/03-interceptors-status-codes-errors.md) | logging/recovery/metadata interceptors and chaining order; the `status`/`codes` model and decision matrix; `errdetails`; mapping domain errors; deadline propagation |
| [exercises/exercise-01-design-the-notes-proto.proto](./exercises/exercise-01-design-the-notes-proto.proto) | A complete, lint-clean `notes` `.proto`: messages, `FieldMask` update, server-streaming `WatchNotes`, `reserved`, runnable through `buf generate` |
| [exercises/exercise-02-grpc-server-and-client.go](./exercises/exercise-02-grpc-server-and-client.go) | A self-contained gRPC server (in-memory store) plus a client driving unary and server-streaming calls, with `grpc.NewServer`/`grpc.NewClient` |
| [exercises/exercise-03-interceptors-and-status-codes.go](./exercises/exercise-03-interceptors-and-status-codes.go) | Chained logging + recovery interceptors, a validating handler, and a client asserting `InvalidArgument`/`NotFound`/`Internal` via `status.FromError` |
| [exercises/SOLUTIONS.md](./exercises/SOLUTIONS.md) | Annotated solutions, expected output, reflection answers, and a "Common mistakes" section |
| [challenges/challenge-01-grpc-rest-shared-logic.md](./challenges/challenge-01-grpc-rest-shared-logic.md) | Stand up the Week-5 chi REST surface against the *same* `service.Service` as the gRPC server; prove cross-transport visibility |
| [challenges/challenge-02-protobuf-schema-evolution.md](./challenges/challenge-02-protobuf-schema-evolution.md) | Evolve `notes.proto` v1→v2, run the four-cell client×server matrix, and gate it with `buf breaking` |
| [quiz.md](./quiz.md) | 10 multiple-choice questions on proto3, the wire format, the call types, deadlines, codes, and interceptor order |
| [homework.md](./homework.md) | Six practice problems for the week |
| [mini-project/README.md](./mini-project/README.md) | Lab 07 — `notes` over gRPC + REST against one shared service, with tests and a cross-transport proof |

## The gRPC contract — the week's promise

Week 5 gave you "every handler returns the right HTTP status and never leaks an internal error." Week 7 restates that for the network boundary. **Every RPC you ship this week respects its context deadline, returns a precise `codes.Code` (never a bare error string), and is observable through a logging interceptor that records method, code, and duration.** An RPC that ignores `ctx.Done()` is the network equivalent of a goroutine that keeps running after its caller has given up — it burns capacity nobody is waiting for, and the cost shows up as tail latency on unrelated calls. An RPC that returns `codes.Internal` for every failure is the network equivalent of `catch (err)` swallowing the type — it eats the one bit, "is retrying safe?", the operator most needs.

And a schema contract: **every `.proto` you ship reserves removed field numbers, never reuses a number, and never changes a field's type incompatibly** — and `buf breaking` proves it in CI. A wire-incompatible schema change is a production outage the next time your v1 client meets your v2 server. The rules are few; obeying them is non-negotiable.

> **Note on the toolchain.** Some learners will run this week in an environment without `buf`, the protoc Go plugins, or a current Go. Installing them is your responsibility and free: Go from <https://go.dev/dl/>, `buf` from <https://buf.build/docs/installation/>, and the two plugins via `go install`. You do not need a separate `protoc` — `buf` drives the plugins directly. Set this up before Monday's exercise.
