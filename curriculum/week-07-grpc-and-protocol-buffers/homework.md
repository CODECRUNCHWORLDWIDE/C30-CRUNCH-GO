# Week 7 — Homework

Six practice problems. Allocate roughly 1 hour per problem; the last two are longer and may need 90 minutes. Submit one archive of code plus a single `homework.md` write-up. Rubric at the bottom.

---

## Problem 1 — Map a domain to a `.proto` from scratch (60 min)

Take the following domain spec and produce a lint-clean `.proto`. Justify each design decision in a comment.

> **Domain:** a "reading list" service. A `Bookmark` has an id, a url, a title, a status (one of `UNREAD`, `READING`, `DONE`, `ARCHIVED`), a creation timestamp, an optional `read_at` timestamp, a list of `tags` (free-text strings), an optional `note` (free text), and an estimated `read_duration` (a `google.protobuf.Duration`). The service exposes one unary RPC `AddBookmark(AddBookmarkRequest)` returning `AddBookmarkResponse { Bookmark bookmark }`.

**Required of your `.proto`:**

- `syntax = "proto3";` and a versioned `package` (e.g. `crunch.reading.v1`).
- `option go_package = "...;readingv1";` with **both** halves.
- An `enum` for status with `*_UNSPECIFIED = 0`.
- `optional` on the two genuinely-optional fields (`read_at`, `note`).
- `repeated` for the tags.
- `google.protobuf.Timestamp` for the timestamps and `google.protobuf.Duration` for the estimate.
- A `reserved 7;` line with a comment explaining the hypothetical removed field it tombstones.
- Hot fields (the ones in every message) in numbers 1–15.

**Deliverable:** `reading.proto` that passes `buf lint`, plus a one-paragraph `NOTES.md` defending each choice and the `buf lint` exit code.

---

## Problem 2 — Predict the wire bytes by hand, verify with `proto.Size` (45 min)

Take the `Note` message from Exercise 1. Construct one in Go with these specific values:

- `id = "n-42"`
- `title = "ok"`
- `body = ""`
- `tags = ["urgent", "auth"]`
- `created_at` = a `Timestamp` you choose
- `updated_at` = the same `Timestamp`
- `archived_at` unset

**Predict the byte count by hand** in a markdown table, field by field — show the tag byte, the length byte (where applicable), and the payload bytes. Remember: `body = ""` is the default and emits zero bytes; `archived_at` unset emits zero bytes; each repeated `tags` element is its own length-delimited field. Then construct the message in Go and call `proto.Size(m)` and `len(proto.Marshal(m))` to verify.

**Deliverable:** a `wire-prediction.md` with the per-field breakdown and the confirmed actual size. If prediction and actual differ, find the bug and document the cause (the usual culprit: forgetting that each `Timestamp` is itself a length-delimited embedded message with its own `seconds`/`nanos` tags).

---

## Problem 3 — A unary RPC with proper status-code mapping (75 min)

Implement `AddBookmark` from Problem 1 in a gRPC server over an in-memory store. Validation rules:

- `url` must be non-empty and start with `http://` or `https://`.
- `title` may be empty but must be ≤ 200 runes.
- `status` must not be `UNSPECIFIED`.
- A duplicate `url` is a conflict.

For each violated rule, return `status.Error` with the *most specific* code. Defend each choice in a comment. Write a client that exercises:

1. A happy-path add.
2. An empty url — expect `InvalidArgument`.
3. A 250-rune title — expect `InvalidArgument`.
4. A `status = UNSPECIFIED` add — expect `InvalidArgument` (defend why this is the right code rather than `FailedPrecondition`).
5. A duplicate url — expect `AlreadyExists`.

Assert each code client-side with `status.FromError`.

**Deliverable:** the server, the client, and a `NOTES.md` defending each status-code choice in one sentence.

---

## Problem 4 — Server-streaming with proper context cancellation (75 min)

Implement a server-streaming RPC `WatchBookmarks(WatchRequest)` returning `stream BookmarkEvent`, where `WatchRequest` has a `status_filter` and `BookmarkEvent` has a `kind` (ADDED/UPDATED/READ) and a `Bookmark` payload.

Server requirements:

- An in-memory event source (a `chan BookmarkEvent` fed by a background producer).
- The RPC reads from the channel, filters by status, and writes to the stream with `stream.Send`.
- The RPC respects `stream.Context().Done()` — exits the loop the instant the context fires, returning `status.FromContextError`.
- The RPC emits a `KEEPALIVE` event every 2 seconds so the stream stays warm under HTTP/2 idle timeouts.

Client requirements:

- Open the watch stream with a 60-second deadline.
- Iterate with `Recv` until `io.EOF`, printing each non-keepalive event.
- After 5 real events, cancel the call's context.
- Observe `codes.Canceled` and verify (via the server log) that the loop exited within ~100ms of cancellation.

**Deliverable:** server + client + a `NOTES.md` answering: how does `stream.Send` interact with HTTP/2 flow control, and what happens to the server if the client is slow to read — does the goroutine block?

---

## Problem 5 — Interceptors + metadata propagation across a 3-service chain (90 min)

Build:

1. A **client** unary interceptor `requestIDClient` that ensures every outbound call carries an `x-request-id` header, generating a fresh 8-hex-char id when the caller did not supply one (`metadata.NewOutgoingContext`).
2. A **server** unary interceptor `requestIDServer` that reads `x-request-id` (`metadata.FromIncomingContext`), stashes it in the context, and includes it in every `slog` line for that call.
3. A chain of three gRPC services — `ServiceA` → `ServiceB` → `ServiceC` — each its own server. `A`'s handler calls `B`; `B`'s handler calls `C`. The key move: when `A` *received* a request id, it **re-attaches the same id** to its outbound call to `B` (read it from the incoming context, put it on the outgoing context), and `B` does the same to `C`.

Verify by hand that a single `x-request-id` threads through every log line across all three services for one client call. Capture and align the three services' logs.

**Deliverable:** the three services, the two interceptors, the re-attachment plumbing, and a text snippet of the aligned logs showing the same `x-request-id` at A-in, A→B, B→C, and C-return.

---

## Problem 6 — A cross-language client: a Python `grpcio` client or a `grpcurl` drive (90 min, stretch)

Prove the contract is language-agnostic. Take your Problem 3 server (running on localhost, plaintext). Then **either**:

**Option A — Python.** Generate Python stubs from the same `.proto`:

```bash
python3 -m pip install grpcio grpcio-tools
python3 -m grpc_tools.protoc -I proto \
        --python_out=. --grpc_python_out=. proto/reading/v1/reading.proto
```

Write a `client.py` that adds a bookmark and reads it back, and that triggers the `InvalidArgument` and `AlreadyExists` cases, inspecting `e.code()` on the `grpc.RpcError`.

**Option B — `grpcurl`.** With server reflection enabled (`reflection.Register(g)`), drive every RPC from the shell:

```bash
grpcurl -plaintext localhost:50051 list
grpcurl -plaintext -d '{"url":"https://go.dev","title":"Go","status":"BOOKMARK_STATUS_UNREAD"}' \
        localhost:50051 crunch.reading.v1.ReadingService/AddBookmark
grpcurl -plaintext -d '{"url":""}' \
        localhost:50051 crunch.reading.v1.ReadingService/AddBookmark   # expect InvalidArgument
```

**Deliverable:** the Python client (with `requirements.txt`) **or** the captured `grpcurl` transcript, plus a `NOTES.md` comparing the cross-language ergonomics to Go's — which surface is the cleanest for reading the status code, and what does that say about why the proto is the contract rather than any one language's stubs.

---

## Rubric

For each problem (max 100 points):

| Tier | Points | Description |
|------|--------|-------------|
| Master | 90–100 | Builds and runs. Every requirement met. The `NOTES.md` shows reasoning beyond the literal answer — at least one observation the spec did not ask for. |
| Solid | 75–89 | Builds and runs. Every requirement met. The `NOTES.md` answers what was asked, no more. |
| Working | 60–74 | Builds. Most requirements met; one or two missed. |
| Partial | 40–59 | Builds in places but with significant gaps; the spec was not fully read. |
| Submitted | 0–39 | Submission exists; substantial parts are missing or broken. |

Total: **600 points** across the six problems. **480** is the C30-passing threshold for this week's homework. The mini-project is graded separately.

## Submission

Archive the six problem folders together as `week-07-homework-<your-name>.zip` (exclude `bin/` and any generated `gen/` you can regenerate; include the `.proto` and `buf` config). Include a top-level `homework.md` that links to each problem's `NOTES.md` and lists your self-assigned tier per problem.

Submit by Sunday 11:59 PM local time. Late submissions are accepted with a one-tier markdown per 24h past the deadline.
