# Exercise Solutions — Week 7

These annotated solutions assume you have made a serious attempt at each exercise. Read your own attempt against the explanations below; do not copy without trying first. The three exercise files are the reference *starting* code — your tasks extend them.

---

## Exercise 1 — Design the notes proto

### Key correctness properties

- **The package is versioned (`crunch.notes.v1`) and `go_package` pins both halves** (`...;notesv1`). Drop the package-name half and the generated Go packages for every `vN` directory collide on the default name; `buf lint`'s `PACKAGE_VERSION_SUFFIX` and the import graph both punish you.
- **Every enum starts at `*_UNSPECIFIED = 0`.** `NoteEventKind_NOTE_EVENT_KIND_UNSPECIFIED` is the value a receiver sees when the field was never set; naming it `UNSPECIFIED` keeps "no information" unambiguous. `buf lint`'s `ENUM_ZERO_VALUE_SUFFIX` enforces this.
- **`archived_at` is `optional`** so the Go field is `*timestamppb.Timestamp` — the pointer is the presence bit. "Never archived" (`nil`) is distinguishable from "archived at the zero instant" (non-nil). The other timestamps are not `optional`: a note always has a `created_at`, so presence tracking buys nothing there.
- **`DeleteNote` returns `google.protobuf.Empty`,** not an invented `DeleteNoteResponse`. The well-known type documents "nothing meaningful comes back."
- **`UpdateNote` carries a `FieldMask`** so a partial update ("change only `title`") is expressible without a separate RPC per field.
- **The `reserved` block tombstones numbers 9, 10 and names `color`, `pinned`.** This is the schema-evolution safety device: the compiler now refuses to reuse those numbers or names.

### Expected toolchain output

```
$ buf lint
$ echo $?
0

$ buf build
$ buf generate
$ ls gen/notes/v1
notes.pb.go  notes_grpc.pb.go
```

`buf lint` printing nothing and exiting `0` is the pass condition. After Task (D), moving `body_format` (number 8) into the reserved list and re-running:

```
$ buf breaking --against '.git#branch=main'
$            # silent: removing a field and reserving its number is NOT a wire break

# now add `string something_else = 8;` and re-run:
$ buf breaking --against '.git#branch=main'
proto/notes/v1/notes.proto:NN:M: Field "8" with name "something_else" ... reuses a reserved number.
```

### Reflection answers

1. **Why `*_UNSPECIFIED = 0`?** Proto3 makes `0` the implicit default for any enum field, emitted as zero bytes on the wire. If `0` were a real state (`CREATED`), a sender who forgot to set the field would silently report that state. `UNSPECIFIED` makes absence-of-information a distinct, named value. Style guide: <https://protobuf.dev/programming-guides/style/>.

2. **Why is `optional` on `archived_at` but not `created_at`?** `archived_at` is genuinely nullable — most notes are never archived — and the server needs to tell "unset" from "the zero time." `created_at` always has a value, so explicit presence is pure overhead (a pointer and a wire byte) for no information gained.

3. **Why a `FieldMask` on `UpdateNote` rather than a full-replace?** A full-replace forces the client to read-modify-write the whole note, racing any concurrent change. A mask lets the client say "set only these paths," which is both safe under concurrency and cheap on the wire.

4. **Why does `WatchNotes` carry both `note` and `note_id` on `NoteEvent`?** A DELETED event may fire after the note is gone, so there is no `Note` to attach — but the subscriber still needs to know *which* note was deleted. The `note_id` string is always present; the `note` message is present only when there is one. Modeling both avoids a second RPC just to learn the id of a deleted note.

5. **Why is the `reserved` block better than a comment that says "do not reuse 9 and 10"?** A comment is advisory; the compiler ignores it, and the next engineer in a hurry adds `string foo = 9;` and ships a wire-incompatible change. `reserved 9, 10;` makes the compiler *refuse* that change, and `buf breaking` catches it in CI even if someone deletes the reservation. The safety lives in the toolchain, not in goodwill.

---

## Exercise 2 — gRPC server and client

### Key correctness properties of the reference code

- **`grpc.NewClient`, not `grpc.Dial`.** `Dial` is deprecated; `NewClient` is the modern lazily-connecting constructor. `insecure.NewCredentials()` is the explicit "no TLS, local only" — *not* the deprecated `grpc.WithInsecure()`.
- **`server` embeds `notesv1.UnimplementedNotesServiceServer`.** Without it the build fails with `missing method mustEmbedUnimplemented...`; with it, RPCs you have not written (`ListNotes`, `UpdateNote`) return `codes.Unimplemented` at runtime and the server still compiles.
- **`WatchNotes` selects on `stream.Context().Done()`** and returns `status.FromContextError(ctx.Err()).Err()`, so a client disconnect or deadline exits the loop immediately with the right code (`Canceled` / `DeadlineExceeded`).
- **`stream.Send`'s error is checked**, not discarded — a client that has gone away surfaces there.
- **One `ClientConn` for the whole program**, reused across calls. Every call sets a deadline via `context.WithTimeout`.

### Expected program output (illustrative)

```
level=INFO msg="created" id=n-...-1
level=INFO msg="watch event" kind=NOTE_EVENT_KIND_CREATED id=n-...-1 title="Ship gRPC"
level=INFO msg="fetched" title="Ship gRPC"
level=INFO msg="expected NotFound" code=NotFound
```

The CREATED watch event and the unary fetch both show the same note because both the streaming handler and the unary handler read the *same* in-memory store — the in-process version of "one domain, two call shapes." The `NotFound` line proves the status code round-trips from server to client unchanged.

### Reflection answers

1. **Why open the watch stream before creating notes?** The store's `broadcast` is best-effort with a buffered channel; events produced before any subscriber exists can be dropped. Opening the stream first guarantees the subscriber is draining when the CREATED event fires. (A production fan-out keeps a per-subscriber channel and a snapshot-on-subscribe, exactly what the mini-project's `Subscribe` does.)

2. **What does `io.EOF` from `stream.Recv()` mean on the client?** The server ended the stream cleanly by returning `nil`. It is the normal terminator, not a failure — which is why the client `break`s on `errors.Is(err, io.EOF)` and only logs on any *other* error.

3. **Why `127.0.0.1:0` for the listener?** Port `0` asks the OS for any free port, read back from `lis.Addr()`. It makes the example runnable on a machine where `50051` is already taken, and is the same trick the test suite uses to run servers in parallel without port collisions.

---

## Exercise 3 — Interceptors and status codes

### Key correctness properties of the reference code

- **`recoveryUnary` is registered OUTERMOST** (`ChainUnaryInterceptor(recoveryUnary, loggingUnary)`), so it wraps the logging interceptor *and* every handler and can catch a panic from any of them.
- **The recovery interceptor uses a NAMED return `err`** and assigns to it from inside `defer`. A panic therefore becomes `status.Error(codes.Internal, "internal error")` — and the client sees exactly that string, never the panic value and never the stack. The stack goes to `slog.ErrorContext`.
- **Validation maps to `InvalidArgument` with a field-and-rule message.** Empty title, over-length title (counted in runes via `utf8.RuneCountInString`, not bytes), too many tags — each is the client's fault and each says which field and which rule.
- **`status.FromError` decodes the code client-side**, and `assertCode` compares it to the expected `codes.Code`.

### Expected program output (illustrative)

```
level=INFO msg="grpc call" method=/crunch.notes.v1.NotesService/CreateNote code=OK dur=... peer=127.0.0.1:...
level=INFO msg="create ok"
level=WARN msg="grpc call" method=/crunch.notes.v1.NotesService/CreateNote code=InvalidArgument dur=...
level=INFO msg="code as expected" case="empty title" code=InvalidArgument
level=WARN msg="grpc call" method=/crunch.notes.v1.NotesService/GetNote code=NotFound dur=...
level=INFO msg="code as expected" case="missing note" code=NotFound
level=ERROR msg="panic recovered" method=/crunch.notes.v1.NotesService/CreateNote panic="simulated handler bug" stack=...
level=ERROR msg="grpc call" method=/crunch.notes.v1.NotesService/CreateNote code=Internal dur=...
level=INFO msg="code as expected" case="panicking handler" code=Internal
level=INFO msg="panic message to client" msg="internal error"
```

The log level tracks the code: `OK` is INFO, `InvalidArgument`/`NotFound` are WARN (the client's fault), `Internal` is ERROR (our bug). The final line proves the panic detail never crossed the wire.

### Reflection answers

1. **Why is the recovery interceptor outermost rather than innermost?** Innermost, it would only wrap the handler — a panic in the logging interceptor itself would crash the process. Outermost, it wraps logging too. The general rule: cross-cutting safety nets go outside; cross-cutting observation goes just inside them.

2. **Why count title length with `utf8.RuneCountInString`, not `len`?** `len(string)` counts bytes; a title of 200 emoji is 800 bytes. "At most 200 characters" is a rune count. Validating in bytes would reject valid multibyte input and mis-state the limit in the error message.

3. **Why `InvalidArgument` for an empty title rather than `FailedPrecondition`?** `InvalidArgument` means "this request is malformed regardless of system state" — an empty title is wrong no matter what is in the database. `FailedPrecondition` would imply the request is well-formed but the system state forbids it right now, which is not the case here.

---

## Common mistakes across the three exercises

- **Using the deprecated `grpc.Dial` instead of `grpc.NewClient`.** `Dial` is on its way out; `NewClient` is the current constructor and connects lazily on first RPC. Pretend `Dial` does not exist.
- **Forgetting the `UnimplementedNotesServiceServer` embed.** The build fails with `missing method mustEmbedUnimplemented...`. The fix is always to embed the generated `Unimplemented...Server` struct in your server type — it is the forward-compatibility shim that keeps the server compiling when the schema gains an RPC.
- **Returning raw errors from handlers instead of `status.Error`.** A bare `errors.New("not found")` reaches the client as `codes.Unknown` with the message — the worst of both worlds: no machine-readable classification and your internal string leaked. Always wrap in `status.Error`/`status.Errorf` with a deliberate code.
- **Not propagating context.** A handler that ignores `ctx`/`stream.Context()` keeps working after the client has given up. Pass the context into every downstream call and `select` on `Done()` in every stream loop.
- **Constructing a fresh `ClientConn` per call.** The conn owns the HTTP/2 connection pool and is goroutine-safe. One per server, reused for the process lifetime — the gRPC analogue of reusing your `*sql.DB`.
- **Ignoring `stream.Send`'s error.** When the client disconnects mid-stream, `Send` returns the error that tells you so; discarding it means the loop spins writing into a dead stream.

Next: the challenges. Challenge 1 stands up the Week-5 REST surface over the *same* service the gRPC server uses; Challenge 2 evolves the schema while `buf breaking` guards wire compatibility.
