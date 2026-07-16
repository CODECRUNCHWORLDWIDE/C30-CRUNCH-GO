# Week 7 — Quiz

Ten multiple-choice questions. Take it with your lecture notes closed. Aim for 9/10 before moving to Week 8. Answer key at the bottom — do not peek.

---

**Q1.** In proto3, a field declared `int32 x = 1;` is left unset by the sender. What does the receiver observe, and how does the generated Go represent it?

- A) The parser returns a "required field missing" error.
- B) The receiver sees `x = 0`, the zero default for `int32`; the Go field is plain `int32`, and there is no way to distinguish "unset" from "set to zero" without the `optional` keyword.
- C) The receiver sees the Go zero value `nil`.
- D) Behavior is undefined; implementations disagree.

---

**Q2.** A `.proto` declares:

```proto
enum NoteEventKind {
  NOTE_EVENT_KIND_UNSPECIFIED = 0;
  NOTE_EVENT_KIND_CREATED = 1;
  NOTE_EVENT_KIND_UPDATED = 2;
}
```

A v2 server adds `NOTE_EVENT_KIND_DELETED = 3` and emits it. A v1 Go client receives a message whose `kind` field is `3`. What happens?

- A) The v1 client returns a deserialization error because `3` is unknown.
- B) The v1 client reads `kind` as `NoteEventKind(3)` — an unnamed value — with no error. Proto3 enums are open.
- C) The v1 client sees `NOTE_EVENT_KIND_UNSPECIFIED` (the zero default).
- D) The protobuf parser strips the unknown value from the wire bytes.

---

**Q3.** What does the proto3 wire encoding look like for `message X { int32 a = 1; string b = 2; }` with `a = 0` and `b = "ok"`?

- A) Two field entries: tag `0x08` value `0`, then tag `0x12` len `0x02` `"ok"`. Six bytes.
- B) One field entry for `b`: tag `0x12`, length `0x02`, payload `0x6F 0x6B`. Four bytes total. `a = 0` is the default and emits nothing.
- C) JSON `{"a":0,"b":"ok"}` wrapped in a length prefix.
- D) Three entries: `a`, `b`, and an end-of-message marker.

---

**Q4.** A gRPC server method has this Go signature:

```go
func (s *server) WatchNotes(req *notesv1.WatchNotesRequest,
    stream notesv1.NotesService_WatchNotesServer) error
```

What call type is this?

- A) Unary
- B) Server-streaming
- C) Client-streaming
- D) Bidirectional streaming

---

**Q5.** A Go client calls `client.GetNote(context.Background(), req)` with no deadline. The server is slow and takes 60 seconds. What happens?

- A) The client times out after grpc-go's default 30-second deadline.
- B) The client waits indefinitely. grpc-go has **no** default client deadline; the absence of one is the bug.
- C) The client fails after 5 seconds with `codes.DeadlineExceeded`.
- D) The HTTP/2 keepalive forces a `codes.Unavailable` at 20 seconds.

---

**Q6.** A handler needs to signal "the caller's authentication token is missing or invalid." Which `codes.Code` is correct?

- A) `codes.PermissionDenied`
- B) `codes.Unauthenticated`
- C) `codes.Unauthorized`
- D) `codes.Internal`

---

**Q7.** What is the principal reason to develop *proto-first* with `buf` rather than hand-writing JSON contracts per service?

- A) `buf` generates faster runtime code than `encoding/json`.
- B) One `.proto` is the single cross-language contract — it generates a Go server, a Go client, a Python client, and more, all wire-compatible — and `buf breaking` mechanically prevents wire-incompatible changes from merging.
- C) Proto-first is required by Go 1.22.
- D) JSON cannot express streaming and protobuf can.

---

**Q8.** A gRPC client sets `ctx, cancel := context.WithTimeout(ctx, 200*time.Millisecond)` and calls a server that ignores its context and sleeps 1 second. Which statement is **false**?

- A) The remaining budget is serialized into the `grpc-timeout` HTTP/2 header and the server's incoming `ctx` carries the same deadline.
- B) The client observes `codes.DeadlineExceeded` at ~200ms.
- C) Because the server ignored its `ctx`, it wastes ~800ms of work after the client gave up — capacity that shows up as tail latency on unrelated RPCs.
- D) Deadlines are guaranteed accurate to one millisecond regardless of network latency and scheduler jitter.

---

**Q9.** A server-streaming RPC is opened; the Go client iterates `stream.Recv()` and returns from `main` (or cancels its context) after the third event. What happens to the underlying HTTP/2 stream?

- A) It stays open and the server keeps sending events that are silently discarded forever.
- B) Canceling the call's context (or letting it go out of scope) cancels the client side; the server's `stream.Context().Done()` fires, and a handler that selects on it exits cleanly. A handler that ignores it keeps producing into a dead stream.
- C) The stream cannot close until the server sends every event; the client must drain to `io.EOF`.
- D) The entire HTTP/2 connection is torn down, breaking all other concurrent RPCs on it.

---

**Q10.** A server is built with `grpc.NewServer(grpc.ChainUnaryInterceptor(recoveryUnary, loggingUnary))`. In what order do they run around the handler?

- A) `recovery-before → logging-before → handler → logging-after → recovery-after`. The first-registered interceptor is outermost.
- B) `logging-before → recovery-before → handler → recovery-after → logging-after`. The last-registered is outermost.
- C) Both run concurrently around the handler.
- D) Only `recoveryUnary` runs; `loggingUnary` is dead code.

---

## Answer key (no peeking until you have answered all ten)

1. **B.** Without `optional`, proto3 cannot distinguish unset from default; the wire emits nothing for default-valued scalars, and the Go field is a plain `int32`. `optional` turns the field into a `*int32` and tracks presence. By design, proto3 traded the distinction for terser bytes.

2. **B.** Proto3 enums are *open*: an unknown integer round-trips as `NoteEventKind(3)` with no error. This is the basis of forward-compatible enum extension — a v2 server may add values and v1 clients tolerate them.

3. **B.** Default-valued scalars are not emitted. `a = 0` produces zero bytes. `b = "ok"` is tag `(2<<3)|2 = 0x12`, length `0x02`, payload `0x6F 0x6B` — four bytes total.

4. **B.** A method whose *response* side is a typed stream (`..._WatchNotesServer` with `Send`) and whose request is a single message is server-streaming. The absence of a `Recv`-bearing request stream and the presence of the response stream identify the shape.

5. **B.** grpc-go has no default client deadline. A call with no deadline waits forever (until the transport's keepalive eventually intervenes, which is not a gRPC-level deadline). Setting a deadline on every call is the discipline; its absence is the bug.

6. **B.** `Unauthenticated` is "who are you?" — missing or invalid credentials. `PermissionDenied` is "I know who you are, and you may not do this." `codes.Unauthorized` is not a real code. In HTTP terms, `Unauthenticated` is 401 and `PermissionDenied` is 403.

7. **B.** The single cross-language contract plus mechanical breaking-change detection is the value proposition. `buf breaking` makes "never reuse a field number, never change a type" a CI gate rather than a code-review hope.

8. **D.** Deadlines are *not* millisecond-accurate; they are accurate to whatever the clock, the scheduler, and the cancellation plumbing provide — typically tens of milliseconds. Designing for sub-50ms deadline precision is a mistake. The other three statements are true.

9. **B.** Canceling the client context propagates: the server's `stream.Context().Done()` fires, and a handler that `select`s on it returns immediately and the stream is torn down cleanly. The failure mode the question warns against is a handler that *ignores* the context and keeps `Send`-ing into a dead stream.

10. **A.** `ChainUnaryInterceptor` runs interceptors outside-in in registration order: the first-registered (`recoveryUnary`) is outermost — its pre-code runs first and its post-code runs last — so it wraps `loggingUnary` and every handler, which is exactly why recovery is registered first.

---

## Scoring

- **10/10**: You can teach this material. Move to Week 8 with confidence.
- **8–9**: Solid. Re-read the lecture sections matching the questions you missed, then move on.
- **6–7**: Re-read all three lectures and retake. The gRPC operational model is dense; do not skim it.
- **≤5**: Slow down. Spend an extra evening on the lectures and SOLUTIONS.md before attempting the mini-project.
