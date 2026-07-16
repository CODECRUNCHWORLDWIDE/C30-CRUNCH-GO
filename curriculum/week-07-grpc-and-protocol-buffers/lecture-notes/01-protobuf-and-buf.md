# Lecture 1 — Protocol Buffers and the `buf` Toolchain: Proto3 for Go, the Wire Format, and Code Generation

You spent Week 5 building a REST service with `net/http` and `chi`, and Week 6 putting a real Postgres layer behind it with `pgx`, `sqlc`, and `golang-migrate`. The `notes` service has a handler layer, a service layer, and a repository layer, and the three are cleanly separated. This week you give that same service a second front door. JSON over HTTP is how the world talks to you; it is verbose, schema-optional, and slow to parse, but a browser, a `curl` one-liner, and a junior on another team can all reach it without ceremony. **Protocol Buffers and gRPC are how your services talk to each other** — a compact, schema-mandatory, code-generated contract that two processes written in two languages on two machines can agree on with zero hand-written serialization code.

The first thing to internalize is that *gRPC is two pieces, not one*. The first piece — the subject of this lecture — is **Protocol Buffers** (proto3): a schema language and a binary wire format. A `.proto` file declares message types and service interfaces; a code generator reads that file and emits Go structs, getters, and client/server interfaces. The second piece — the subject of Lecture 2 — is gRPC itself, the call protocol over HTTP/2. They are separable in principle and inseparable in practice: nearly all gRPC traffic is protobuf-encoded, and nearly all new protobuf in a service mesh rides on gRPC. We treat them as a pair, and we start with the schema, because the schema is the contract and the contract comes first. This is *proto-first* development, and it is the discipline the whole week is built on.

## 1. The shape of a `.proto` file

Here is a complete, lint-clean proto3 file for a slice of the notes domain. Read it top to bottom; the sections that follow dissect each construct.

```proto
syntax = "proto3";

package crunch.notes.v1;

option go_package = "example.com/notes/gen/notes/v1;notesv1";

import "google/protobuf/timestamp.proto";
import "google/protobuf/empty.proto";

// A single note owned by a user.
message Note {
  string id = 1;
  string title = 2;
  string body = 3;
  repeated string tags = 4;
  google.protobuf.Timestamp created_at = 5;
  google.protobuf.Timestamp updated_at = 6;
  optional google.protobuf.Timestamp archived_at = 7;

  reserved 8, 9;
  reserved "color", "pinned";
}

message GetNoteRequest {
  string id = 1;
}

message GetNoteResponse {
  Note note = 1;
}

service NotesService {
  rpc GetNote(GetNoteRequest) returns (GetNoteResponse);
  rpc DeleteNote(DeleteNoteRequest) returns (google.protobuf.Empty);
}

message DeleteNoteRequest {
  string id = 1;
}
```

Four lines do administrative work before any type is declared:

- **`syntax = "proto3";`** — the language version. Always proto3 for new work; proto2 still exists but is legacy. The protobuf language guide is the authoritative reference for everything in this lecture: <https://protobuf.dev/programming-guides/proto3/>.
- **`package crunch.notes.v1;`** — the *protobuf* package, which namespaces the types on the wire and in fully-qualified names (`crunch.notes.v1.Note`). Note the `.v1` suffix: a version belongs in the package path so that v1 and v2 of a contract can coexist. This is doctrine, not taste.
- **`option go_package = "...";`** — where the generated Go lands. The value has two halves separated by a semicolon: the *import path* (`example.com/notes/gen/notes/v1`) and the *package name* (`notesv1`). Without the package-name half, the generated package defaults to the last path segment, which collides across `v1` directories. Always pin both halves. See the Go generated-code reference: <https://protobuf.dev/reference/go/go-generated/>.
- **`import "...";`** — pulls in the *well-known types* (more in §6). Imports are resolved against the include paths `buf` (or `protoc`) is given.

## 2. Messages, fields, and field numbers

A `message` is a record. Each field has a **type**, a **name**, and a **field number**. The field number is the load-bearing part: it — not the name — is what travels on the wire. `string title = 2;` means "field number 2 is a length-delimited string." Rename `title` to `subject` in a later version and existing wire bytes still decode correctly, because the wire knows only `2`. *Change* the number, and you have silently broken every serialized message in flight and at rest.

Three rules govern field numbers, and breaking any of them is a production incident:

1. **Never reuse a number.** Once `8` meant `color`, `8` means `color` forever, even after you delete the field. To delete safely, `reserved` the number (and ideally the name) so the compiler refuses to let anyone reassign it.
2. **Never change a field's type** in a wire-incompatible way. `int32 → int64` is safe (same varint wire type); `int32 → string` is a wire break.
3. **Prefer numbers 1–15 for hot fields.** The tag for fields 1–15 fits in one byte; 16–2047 take two. Put the fields that appear in every message in the cheap range. We return to *why* in §7.

The `Note` message above ends with `reserved 8, 9;` and `reserved "color", "pinned";`. That is the tombstone for two fields a hypothetical earlier draft had at numbers 8 and 9. The reservation makes the protobuf compiler reject any future attempt to declare a field at 8 or 9 or named `color`/`pinned`. The reserved mechanism is the single most important schema-evolution safety device; the language guide covers it under "updating message types": <https://protobuf.dev/programming-guides/proto3/#updating>.

## 3. Scalar types and their Go mappings

Proto3 has fifteen scalar types. Each maps to a specific Go type in the generated code. The full table from the Go generated-code reference (<https://protobuf.dev/reference/go/go-generated/#scalar>):

| Proto type | Go type     | Wire type      | Notes |
|------------|-------------|----------------|-------|
| `double`   | `float64`   | I64 (fixed 8)  | |
| `float`    | `float32`   | I32 (fixed 4)  | |
| `int32`    | `int32`     | VARINT         | Inefficient for negatives (always 10 bytes if < 0) |
| `int64`    | `int64`     | VARINT         | Same negative-number caveat |
| `uint32`   | `uint32`    | VARINT         | |
| `uint64`   | `uint64`    | VARINT         | |
| `sint32`   | `int32`     | VARINT         | ZigZag-encoded; use this for fields that go negative |
| `sint64`   | `int64`     | VARINT         | ZigZag-encoded |
| `fixed32`  | `uint32`    | I32 (fixed 4)  | Cheaper than varint when values are usually large |
| `fixed64`  | `uint64`    | I64 (fixed 8)  | |
| `sfixed32` | `int32`     | I32 (fixed 4)  | |
| `sfixed64` | `int64`     | I64 (fixed 8)  | |
| `bool`     | `bool`      | VARINT         | One byte (0 or 1) |
| `string`   | `string`    | LEN            | Must be valid UTF-8 |
| `bytes`    | `[]byte`    | LEN            | Arbitrary octets |

Two senior-level facts. First, `int32`/`int64` encode negative numbers as ten-byte varints, because the sign bit forces the high bits on; if a field is routinely negative, use `sint32`/`sint64`, which ZigZag-encode so that small-magnitude negatives stay small. Second, for an `id` you store as text, `string` is correct; for a monetary amount, `int64` of cents beats `double` because IEEE-754 cannot represent `0.10` exactly. These are the same modeling decisions you would make for a Postgres column type, and they should agree: the proto `int64 amount_cents` and the SQL `bigint amount_cents` are the same field seen from two transports.

## 4. Default values and presence

Proto3 has no `required` and no per-field defaults you can declare. Every scalar field has an *implicit zero default*: numbers default to `0`, `bool` to `false`, `string` to `""`, `bytes` to empty, enums to their `0` value, and message-typed fields to `nil`. **A field set to its zero value is not emitted on the wire.** The receiver reconstructs the zero from the field's absence.

The consequence trips up everyone once: by default you *cannot distinguish "unset" from "set to zero."* A `delta = 0` and an omitted `delta` look identical to the receiver. When the distinction matters — a nullable column, a tri-state flag, a "user explicitly cleared this" signal — use the `optional` keyword, as `archived_at` does above. `optional` turns on *explicit presence tracking*: the generated Go gains an `Archived_at` field that is a pointer (`*timestamppb.Timestamp`), and a `GetArchivedAt()` getter, and the wire carries the field even when it holds the zero value. The presence semantics are documented at <https://protobuf.dev/programming-guides/field_presence/>.

In Go specifically: a plain `string title = 2` generates `Title string`; an `optional string title = 2` generates `Title *string`. The pointer *is* the presence bit. This matches how you would map a `text NOT NULL` column versus a nullable `text` column in your repository layer — and getting the two transports to agree on nullability is exactly the discipline this week teaches.

## 5. Enums, repeated, oneof, maps, nested

**Enums.** Proto3 enums are *open*: an unknown numeric value does not fail to parse, it round-trips as its integer. The first enumerator *must* be `0` and convention names it `*_UNSPECIFIED`, because `0` is the implicit default and a default of `UNSPECIFIED` is unambiguous (a default of, say, `ACTIVE` would silently misreport every unset field).

```proto
enum NoteEventKind {
  NOTE_EVENT_KIND_UNSPECIFIED = 0;
  NOTE_EVENT_KIND_CREATED = 1;
  NOTE_EVENT_KIND_UPDATED = 2;
  NOTE_EVENT_KIND_DELETED = 3;
}
```

Enum value names are prefixed with the enum name because protobuf enum values share the *enclosing* namespace, not the enum's — two enums in one file cannot both declare `CREATED`. The generated Go is `NoteEventKind_NOTE_EVENT_KIND_CREATED`, an `int32`-based named type with a `String()` method. Openness is the basis of forward-compatible enum extension: a v2 server can emit `NOTE_EVENT_KIND_ARCHIVED = 4` and a v1 client reads it as `NoteEventKind(4)` without crashing.

**`repeated`** is a list. `repeated string tags = 4;` generates `Tags []string`. Repeated scalars of a numeric type are *packed* by default in proto3 — the whole list rides in one length-delimited field rather than one tag per element.

**`oneof`** is a tagged union: at most one of its members is set, and setting one clears the others. It is the right tool when a message can carry exactly one of several alternatives.

```proto
message SearchResult {
  oneof payload {
    Note note = 1;
    string error_message = 2;
  }
}
```

In Go a `oneof` generates an interface field (`Payload isSearchResult_Payload`) and one wrapper struct per member; you type-switch to read it. Field numbers inside the `oneof` share the message's number space.

**`map<K, V>`** is sugar for `repeated` of a key/value entry message:

```proto
message NoteAnnotations {
  map<string, string> labels = 1;
}
```

This generates `Labels map[string]string`. Keys must be integral or string; values may be any type except another map.

**Nested messages and enums** are declared inside a message for namespacing:

```proto
message Note {
  enum Visibility {
    VISIBILITY_UNSPECIFIED = 0;
    VISIBILITY_PRIVATE = 1;
    VISIBILITY_SHARED = 2;
  }
  Visibility visibility = 10;
}
```

referenced from Go as `notesv1.Note_VISIBILITY_PRIVATE`.

## 6. Well-known types

Google ships a standard library of `.proto` messages — the *well-known types* — and you `import` them rather than redefining time, duration, or "nothing." The three you will use this week, and their Go runtime packages:

| Proto                       | Import                              | Go package                                                       | Go type |
|-----------------------------|-------------------------------------|------------------------------------------------------------------|---------|
| `google.protobuf.Timestamp` | `google/protobuf/timestamp.proto`   | `google.golang.org/protobuf/types/known/timestamppb`            | `*timestamppb.Timestamp` |
| `google.protobuf.Duration`  | `google/protobuf/duration.proto`    | `google.golang.org/protobuf/types/known/durationpb`             | `*durationpb.Duration` |
| `google.protobuf.Empty`     | `google/protobuf/empty.proto`       | `google.golang.org/protobuf/types/known/emptypb`                | `*emptypb.Empty` |
| `google.protobuf.FieldMask` | `google/protobuf/field_mask.proto`  | `google.golang.org/protobuf/types/known/fieldmaskpb`            | `*fieldmaskpb.FieldMask` |

`Timestamp` is the canonical absolute-time type; convert with `timestamppb.New(t time.Time)` and read back with `ts.AsTime()`. Never model time as an `int64` of "epoch millis, probably" — use `Timestamp` and the ambiguity disappears. `Empty` is the idiomatic "this RPC returns nothing meaningful," used by `DeleteNote` above; it is better than inventing an empty `DeleteNoteResponse`, because it documents intent. `FieldMask` carries a set of field paths and is the standard way to express a partial update — "update only `title` and `tags`" — which the mini-project's `UpdateNote` uses. The well-known types are catalogued at <https://protobuf.dev/reference/protobuf/google.protobuf/>.

## 7. The wire format

You do not normally read protobuf bytes by hand, but you must be able to, because "predict the size of this message" is a real interview question and "why did this field disappear" is a real debugging session. The encoding reference is the authority: <https://protobuf.dev/programming-guides/encoding/>.

A protobuf message is a flat sequence of **(tag, value)** records. The tag is a single varint:

```
tag = (field_number << 3) | wire_type
```

The low three bits are the **wire type**; the rest is the field number. There are six wire types but four matter:

| Wire type | Number | Used for |
|-----------|--------|----------|
| `VARINT`  | 0      | `int32/64`, `uint32/64`, `sint32/64`, `bool`, enums |
| `I64`     | 1      | `fixed64`, `sfixed64`, `double` |
| `LEN`     | 2      | `string`, `bytes`, embedded messages, packed repeated |
| `I32`     | 5      | `fixed32`, `sfixed32`, `float` |

A **varint** is a little-endian base-128 integer: seven payload bits per byte, the high bit a continuation flag. Values 0–127 take one byte; 128–16383 take two; and so on. That is why field numbers 1–15 are cheap — the tag `(field << 3) | type` for field ≤ 15 with a 3-bit type still fits in seven bits, one byte.

**A `LEN` field** is tag, then a varint *length*, then that many payload bytes. A string `"ok"` at field 2 is: tag `(2<<3)|2 = 0x12`, length `0x02`, payload `0x6F 0x6B` — four bytes.

### Worked size estimate

Take this message and these values:

```proto
message Sample {
  int32  a = 1;     // = 0
  int32  b = 2;     // = 7
  string c = 3;     // = "ok"
  bool   d = 4;     // = false
}
```

Walk it field by field:

| Field | Value | On the wire | Bytes |
|-------|-------|-------------|------:|
| `a=1` | `0`   | default → *omitted* | 0 |
| `b=2` | `7`   | tag `0x10`, varint `0x07` | 2 |
| `c=3` | `"ok"`| tag `0x1A`, len `0x02`, `0x6F6B` | 4 |
| `d=4` | `false` | default → *omitted* | 0 |
| **Total** | | | **6** |

Six bytes. The equivalent JSON, `{"b":7,"c":"ok"}` (dropping the defaults), is 16 bytes — and a naive JSON that emits the defaults is 30. Now verify in Go:

```go
m := &samplev1.Sample{B: 7, C: "ok"}
fmt.Println(proto.Size(m)) // 6
b, _ := proto.Marshal(m)
fmt.Printf("% x\n", b)     // 10 07 1a 02 6f 6b
```

`proto.Size` (from `google.golang.org/protobuf/proto`) returns the exact serialized length without allocating the buffer; it must agree with `len(proto.Marshal(...))` to the byte. Homework Problem 2 makes you do this prediction for the full `Note` message and verify it.

## 8. The `buf` toolchain

The historical way to run protobuf code generation is raw `protoc` with a fistful of `--plugin` and `-I` flags, no dependency management, and no linting. **`buf` is the modern replacement** — a single binary that lints, builds, generates, and detects breaking changes, with declarative config instead of shell archaeology. Install it from <https://buf.build/docs/installation/>. Two config files drive it.

**`buf.yaml`** declares the module, its lint rules, and its breaking-change rules:

```yaml
version: v2
modules:
  - path: proto
lint:
  use:
    - STANDARD
breaking:
  use:
    - FILE
```

The `STANDARD` lint group enforces the protobuf style guide: `*_UNSPECIFIED` zero enumerators, lower_snake_case fields, versioned package suffixes, and more. The lint overview is at <https://buf.build/docs/lint/overview/>.

**`buf.gen.yaml`** declares which code generators to run and where their output goes:

```yaml
version: v2
plugins:
  - local: protoc-gen-go
    out: gen
    opt:
      - paths=source_relative
  - local: protoc-gen-go-grpc
    out: gen
    opt:
      - paths=source_relative
```

Two plugins, two code generators. **`protoc-gen-go`** (from `google.golang.org/protobuf/cmd/protoc-gen-go`) emits the message structs and their methods. **`protoc-gen-go-grpc`** (from `google.golang.org/grpc/cmd/protoc-gen-go-grpc`) emits the service client and server interfaces. Install both with `go install`:

```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

`buf` finds them on your `PATH`. The `buf.gen.yaml` schema is documented at <https://buf.build/docs/configuration/v2/buf-gen-yaml/>. Now the four commands you run all week:

```bash
buf lint      # style + correctness checks against buf.yaml
buf build     # compile every .proto to an in-memory image; catches errors
buf generate  # run the plugins; write Go into gen/
buf breaking --against '.git#branch=main'  # diff against main, fail on wire-breaks
```

`buf breaking` is the safety net Challenge 2 builds on: it reads the *previous* version of your protos (from a git ref, a tarball, or another directory) and refuses any change that would break the wire — a reused field number, a changed type, a deleted-without-reserving field. Its overview is at <https://buf.build/docs/breaking/overview/>. Wiring this into CI means a wire-incompatible schema change cannot merge.

## 9. The generated Go shape

After `buf generate`, `gen/notes/v1/` holds `notes.pb.go` (messages) and `notes_grpc.pb.go` (service). You never edit these. Here is the *shape* — not byte-for-byte, but the surface you program against — for the `Note` message and `NotesService`.

The message becomes a struct with unexported state and exported fields plus getters:

```go
// notes.pb.go (generated — shape, abridged)
type Note struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	Id         string                 `protobuf:"bytes,1,opt,name=id,proto3" json:"id,omitempty"`
	Title      string                 `protobuf:"bytes,2,opt,name=title,proto3" json:"title,omitempty"`
	Body       string                 `protobuf:"bytes,3,opt,name=body,proto3" json:"body,omitempty"`
	Tags       []string               `protobuf:"bytes,4,rep,name=tags,proto3" json:"tags,omitempty"`
	CreatedAt  *timestamppb.Timestamp `protobuf:"bytes,5,opt,name=created_at,json=createdAt,proto3" json:"created_at,omitempty"`
	UpdatedAt  *timestamppb.Timestamp `protobuf:"bytes,6,opt,name=updated_at,json=updatedAt,proto3" json:"updated_at,omitempty"`
	ArchivedAt *timestamppb.Timestamp `protobuf:"bytes,7,opt,name=archived_at,json=archivedAt,proto3,oneof" json:"archived_at,omitempty"`
}

func (x *Note) GetId() string                       { /* nil-safe */ return "" }
func (x *Note) GetTags() []string                   { /* nil-safe */ return nil }
func (x *Note) GetArchivedAt() *timestamppb.Timestamp { /* nil-safe */ return nil }
```

Two habits the getters teach. First, **always call the getter, never the field**, when the receiver might be `nil`: `req.GetNote().GetId()` returns `""` on a nil chain instead of panicking. Second, the `optional archived_at` is the only field whose getter can meaningfully return `nil` versus a zero `Timestamp` — that pointer is the presence bit from §4.

The service generates three things in `notes_grpc.pb.go`:

```go
// The client interface and its constructor.
type NotesServiceClient interface {
	GetNote(ctx context.Context, in *GetNoteRequest, opts ...grpc.CallOption) (*GetNoteResponse, error)
	DeleteNote(ctx context.Context, in *DeleteNoteRequest, opts ...grpc.CallOption) (*emptypb.Empty, error)
}
func NewNotesServiceClient(cc grpc.ClientConnInterface) NotesServiceClient { /* ... */ }

// The server interface you implement.
type NotesServiceServer interface {
	GetNote(context.Context, *GetNoteRequest) (*GetNoteResponse, error)
	DeleteNote(context.Context, *DeleteNoteRequest) (*emptypb.Empty, error)
	mustEmbedUnimplementedNotesServiceServer()
}

// The forward-compatibility embed.
type UnimplementedNotesServiceServer struct{}
func (UnimplementedNotesServiceServer) GetNote(context.Context, *GetNoteRequest) (*GetNoteResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method GetNote not implemented")
}

// The registration helper, called at server startup.
func RegisterNotesServiceServer(s grpc.ServiceRegistrar, srv NotesServiceServer) { /* ... */ }
```

The `mustEmbedUnimplementedNotesServiceServer()` method and the `UnimplementedNotesServiceServer` struct are the heart of forward compatibility on the *server* side. Your implementation embeds `UnimplementedNotesServiceServer`:

```go
type server struct {
	notesv1.UnimplementedNotesServiceServer
	svc *service.Service // the SAME service layer from Week 5/6
}
```

so that when the schema gains a new RPC, your server *still compiles* — the embedded type supplies a default `Unimplemented` method for the RPC you have not written yet, which returns `codes.Unimplemented` at runtime instead of failing the build. Skipping the embed is the most common beginner mistake; the build fails with "missing method mustEmbedUnimplemented…" and the fix is always "embed the Unimplemented struct." The generated-code contract is specified at <https://grpc.io/docs/languages/go/generated-code/>.

## 10. What you now know

You can write a proto3 file with messages, enums, `oneof`, `map`, `repeated`, nested types, well-known types, `optional` for presence, and `reserved` for safe deletion. You understand that the field number — not the name — is the wire contract, and the three rules that keep it backward-compatible. You can predict the byte size of a message and verify it with `proto.Size`. You can configure `buf.yaml` and `buf.gen.yaml`, run `buf lint`/`build`/`generate`/`breaking`, and read the Go that comes out — the message struct, the nil-safe getters, the `NotesServiceClient`, the `NotesServiceServer` interface, and the `UnimplementedNotesServiceServer` embed that keeps your server compiling across schema growth. Lecture 2 takes that generated server interface and stands a real gRPC server behind it — over the *same* service layer your REST handlers already call.
