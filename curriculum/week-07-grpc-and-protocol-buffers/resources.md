# Week 7 — Resources

Every resource on this page is **free**. grpc.io, protobuf.dev, the `buf` docs, `pkg.go.dev`, and the public GitHub repositories require no account and link no paywalled material.

## Required reading (work it into your week)

### gRPC in Go (grpc.io)

- **gRPC-Go documentation home** — the landing page for the Go stack:
  <https://grpc.io/docs/languages/go/>
- **gRPC-Go quickstart** — hello-world server and client end to end; do this Monday:
  <https://grpc.io/docs/languages/go/quickstart/>
- **gRPC-Go basics tutorial** — the four call types in Go, with the route-guide example:
  <https://grpc.io/docs/languages/go/basics/>
- **gRPC-Go generated-code reference** — exactly what `protoc-gen-go-grpc` emits, including the `Unimplemented` embed:
  <https://grpc.io/docs/languages/go/generated-code/>

### Protocol Buffers (protobuf.dev)

- **proto3 language guide** — the canonical reference; plan ~90 minutes over the week:
  <https://protobuf.dev/programming-guides/proto3/>
- **protobuf encoding (wire format)** — varints, tags, wire types, packed repeated; required before the Problem 2 byte prediction:
  <https://protobuf.dev/programming-guides/encoding/>
- **protobuf Go tutorial** — generating and using Go from a `.proto`:
  <https://protobuf.dev/getting-started/gotutorial/>
- **field presence semantics** — the `optional` keyword and what it changes in Go:
  <https://protobuf.dev/programming-guides/field_presence/>
- **well-known types reference** — `Timestamp`, `Duration`, `Empty`, `FieldMask`, the wrappers:
  <https://protobuf.dev/reference/protobuf/google.protobuf/>

### gRPC core concepts (grpc.io)

- **gRPC core concepts** — channels, the four call types, the gRPC-over-HTTP/2 mapping:
  <https://grpc.io/docs/what-is-grpc/core-concepts/>
- **gRPC error handling guide** — the `Status` model and choosing codes:
  <https://grpc.io/docs/guides/error/>
- **gRPC status codes guide** — the canonical code table and when each applies:
  <https://grpc.io/docs/guides/status-codes/>
- **gRPC deadlines guide** — propagation rules, what happens when a deadline expires:
  <https://grpc.io/docs/guides/deadlines/>

## Authoritative deep dives

- **grpc-go package godoc** — `grpc.NewServer`, `grpc.NewClient`, `ChainUnaryInterceptor`, the interceptor types:
  <https://pkg.go.dev/google.golang.org/grpc>
- **`status` package godoc** — `status.Error`, `status.Errorf`, `status.FromError`, `status.WithDetails`:
  <https://pkg.go.dev/google.golang.org/grpc/status>
- **`codes` package godoc** — every `codes.Code` with its documented meaning:
  <https://pkg.go.dev/google.golang.org/grpc/codes>
- **`metadata` package godoc** — `FromIncomingContext`, `NewOutgoingContext`, `Pairs`:
  <https://pkg.go.dev/google.golang.org/grpc/metadata>
- **`google.golang.org/protobuf` godoc** — `proto.Marshal`, `proto.Size`, the runtime API:
  <https://pkg.go.dev/google.golang.org/protobuf>

## Source you should read

The gRPC and protobuf Go libraries are open source under permissive licenses; source-link works. When a lecture says "the interceptor type is a few lines, go read it," it means literally that.

- **`grpc/grpc-go` — the canonical Go gRPC repository**:
  <https://github.com/grpc/grpc-go>
- **`protoc-gen-go` — the message-code generator**:
  <https://pkg.go.dev/google.golang.org/protobuf/cmd/protoc-gen-go>
- **`protoc-gen-go-grpc` — the service-code generator**:
  <https://pkg.go.dev/google.golang.org/grpc/cmd/protoc-gen-go-grpc>
- **`timestamppb` — the well-known Timestamp Go binding** (`New`, `AsTime`):
  <https://pkg.go.dev/google.golang.org/protobuf/types/known/timestamppb>
- **`errdetails` — the standard rich error-detail messages** (`BadRequest_FieldViolation`):
  <https://pkg.go.dev/google.golang.org/genproto/googleapis/rpc/errdetails>

## Tools (all free)

- **`buf`** — the modern protobuf toolchain (lint, build, generate, breaking) used all week:
  <https://buf.build/docs/>
- **`buf.gen.yaml` configuration reference** — the v2 plugin/output config:
  <https://buf.build/docs/configuration/v2/buf-gen-yaml>
- **`buf breaking` overview** — wire-compatibility detection against a previous version:
  <https://buf.build/docs/breaking/overview/>
- **`buf lint` overview** — the `STANDARD` rule group and the style it enforces:
  <https://buf.build/docs/lint/overview/>
- **`grpcurl`** — `curl` for gRPC; the smoke-test tool of choice (works with reflection):
  <https://github.com/fullstorydev/grpcurl>

## How to use this resource list

The lectures cite specific URLs from this page at decision points. The links to read end-to-end this week, in order:

1. **proto3 language guide** — the language reference. Plan ~90 minutes, spread across the week.
2. **gRPC-Go quickstart + basics** — the model and the four call types in Go. Plan ~60 minutes.
3. **protobuf encoding reference** — before the Problem 2 byte prediction. Plan ~30 minutes.
4. **gRPC error guide + status codes guide** — the two areas with the most subtle wording. Plan ~30 minutes.
5. **buf docs (lint + breaking)** — required before Challenge 2's evolution gate. Plan ~30 minutes.

The rest are reference material. Bookmark them and return when a specific question arises.

---

*Bookmarks decay. If a link rots, search the title — these are all canonical pieces and they reappear on the same authors' new homes.*
