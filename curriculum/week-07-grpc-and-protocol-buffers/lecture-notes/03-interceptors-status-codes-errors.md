# Lecture 3 — Interceptors, Status Codes, and the Error Model: Observability and Correctness for gRPC in Go

Lecture 2 gave you a server and a client that *run*. This lecture gives you the machinery that makes them *correct under production conditions*: interceptors (the gRPC analogue of HTTP middleware) for logging, recovery, and metadata; the status-and-codes error model that replaces "return a string and hope"; rich error details for field-level validation feedback; and deadline propagation, the feature that keeps a fan-out of microservices from doing work nobody is waiting for. Every RPC you ship after this lecture is observable, returns a precise `codes.Code`, and respects its context deadline.

## 1. Interceptors are middleware

In Week 5 you wrapped your chi router in middleware: a function that takes the next handler and returns a handler, running code before and after. gRPC's equivalent is the **interceptor**. There are two server-side shapes — one for unary RPCs, one for streaming — with these signatures from `google.golang.org/grpc`:

```go
type UnaryServerInterceptor func(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo, // info.FullMethod = "/crunch.notes.v1.NotesService/GetNote"
	handler grpc.UnaryHandler,  // call this to invoke the next link / the RPC
) (resp any, err error)

type StreamServerInterceptor func(
	srv any,
	ss grpc.ServerStream,
	info *grpc.StreamServerInfo,
	handler grpc.StreamHandler,
) error
```

The `handler` is the "next" — call it to proceed, do work before and after the call. Register interceptors when you build the server, and chain them with `grpc.ChainUnaryInterceptor` / `grpc.ChainStreamInterceptor`:

```go
srv := grpc.NewServer(
	grpc.ChainUnaryInterceptor(recoveryUnary, requestIDUnary, loggingUnary),
	grpc.ChainStreamInterceptor(recoveryStream, loggingStream),
)
```

**Order matters and it is outside-in.** The first interceptor in the chain is outermost: its pre-handler code runs first and its post-handler code runs last. Put `recoveryUnary` first so it wraps everything — including the other interceptors — and can catch a panic from any of them. The chaining behavior is documented at <https://pkg.go.dev/google.golang.org/grpc#ChainUnaryInterceptor>.

## 2. A logging interceptor

This is the observability baseline: one structured log line per RPC, with method, duration, peer, and the resulting status code.

```go
import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

func loggingUnary(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req) // invoke the RPC
	code := status.Code(err)       // codes.OK when err == nil

	var addr string
	if p, ok := peer.FromContext(ctx); ok {
		addr = p.Addr.String()
	}

	slog.LogAttrs(ctx, levelFor(code), "grpc call",
		slog.String("method", info.FullMethod),
		slog.String("code", code.String()),
		slog.Duration("dur", time.Since(start)),
		slog.String("peer", addr),
	)
	return resp, err
}

func levelFor(c codes.Code) slog.Level {
	switch c {
	case codes.OK:
		return slog.LevelInfo
	case codes.Internal, codes.Unknown, codes.DataLoss:
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}
```

`status.Code(err)` extracts the `codes.Code` from any error a handler returns — `codes.OK` for `nil`, the embedded code for a `status.Error`, and `codes.Unknown` for a non-status error (which is itself a smell: see §5). `peer.FromContext` gives the client address. Mapping the code to a log level means a dashboard query for `level=error` surfaces exactly the failures that are *your* fault, not the client's `InvalidArgument`s.

## 3. A panic-recovery interceptor

An unrecovered panic in a handler crashes the whole server process. The recovery interceptor turns a panic into a clean `codes.Internal` and keeps the process alive:

```go
import "runtime/debug"

func recoveryUnary(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			slog.ErrorContext(ctx, "panic recovered",
				slog.String("method", info.FullMethod),
				slog.Any("panic", r),
				slog.String("stack", string(debug.Stack())),
			)
			err = status.Errorf(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}
```

The named return `err` is essential: the deferred closure assigns to it, so the caller sees the `codes.Internal` instead of a zero-value `(nil, nil)`. Note what the client gets — a bare `"internal error"`, never the panic value or the stack. The stack goes to *your* logs; the client gets nothing that leaks internals. This is the gRPC version of "never echo the exception to the user."

## 4. Metadata and request IDs

gRPC carries key/value headers and trailers as **metadata** (`google.golang.org/grpc/metadata`, <https://pkg.go.dev/google.golang.org/grpc/metadata>). Metadata is how a request id, a trace context, or an auth token rides alongside the message. On the server, read incoming metadata with `metadata.FromIncomingContext`:

```go
func requestIDUnary(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	id := "unknown"
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if vals := md.Get("x-request-id"); len(vals) > 0 {
			id = vals[0]
		}
	}
	// Stash it for downstream handlers and downstream calls.
	ctx = context.WithValue(ctx, requestIDKey{}, id)
	return handler(ctx, req)
}
```

On the *client* side, you attach metadata to outgoing calls with `metadata.NewOutgoingContext`:

```go
ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-request-id", reqID))
resp, err := client.GetNote(ctx, req)
```

A *client* interceptor (`grpc.UnaryClientInterceptor`) is the place to inject `x-request-id` automatically on every outbound call, so application code never forgets it:

```go
func requestIDClient(
	ctx context.Context, method string, req, reply any,
	cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption,
) error {
	if _, ok := metadata.FromOutgoingContext(ctx); !ok {
		ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs("x-request-id", newID()))
	}
	return invoker(ctx, method, req, reply, cc, opts...)
}
// wired at dial time:
//   conn, _ := grpc.NewClient(addr, grpc.WithUnaryInterceptor(requestIDClient), ...)
```

In a service mesh, a server that *received* an `x-request-id` re-attaches it to its own outbound calls — that single id then threads through every log line across the whole call graph. Homework Problem 5 builds exactly this across a three-service chain.

## 5. The status-and-codes error model

A gRPC method does **not** return a bare error string. It returns a `status.Error` (or `status.Errorf`) carrying a `codes.Code` — a machine-readable classification the client decodes back into the same code. The packages are `google.golang.org/grpc/status` (<https://pkg.go.dev/google.golang.org/grpc/status>) and `google.golang.org/grpc/codes` (<https://pkg.go.dev/google.golang.org/grpc/codes>).

```go
return nil, status.Error(codes.NotFound, "note not found")
return nil, status.Errorf(codes.InvalidArgument, "title must be 1-200 chars, got %d", len(req.GetTitle()))
```

There are sixteen codes. One is the right answer for almost every failure you will hit. The full table, from the codes godoc and the gRPC status-codes guide (<https://grpc.io/docs/guides/status-codes/>):

| Code | Meaning | Typical trigger |
|------|---------|-----------------|
| `OK` | success | the happy path |
| `Canceled` | client canceled the call | client `ctx` canceled |
| `InvalidArgument` | client sent a malformed argument | empty title, bad email shape |
| `DeadlineExceeded` | the deadline passed before completion | slow handler, client gave up |
| `NotFound` | the requested entity does not exist | `GetNote` on a missing id |
| `AlreadyExists` | entity the client tried to create exists | duplicate unique key |
| `PermissionDenied` | authenticated, but not authorized | user X cannot touch user Y's note |
| `ResourceExhausted` | quota/rate limit hit | per-user request budget exceeded |
| `FailedPrecondition` | system state forbids the operation | "delete non-empty bucket" |
| `Aborted` | concurrency conflict; retry the txn | optimistic-lock collision |
| `OutOfRange` | value crossed a valid range | page beyond end of list |
| `Unimplemented` | the RPC is not implemented | unset method on `Unimplemented` embed |
| `Internal` | a bug on the server side | panic, invariant broken |
| `Unavailable` | transient; safe to retry with backoff | downstream is down |
| `DataLoss` | unrecoverable data corruption | checksum mismatch |
| `Unauthenticated` | no/invalid credentials | missing or bad token |

### The decision matrix that matters

Three pairs cause most mistakes:

- **`Unauthenticated` vs `PermissionDenied`.** `Unauthenticated` is "who are you?" — no token, expired token, garbage token. `PermissionDenied` is "I know who you are, and you cannot do this." 401 vs 403, in HTTP terms.
- **`InvalidArgument` vs `FailedPrecondition`.** `InvalidArgument` is "the request is wrong regardless of system state" (empty title). `FailedPrecondition` is "the request is well-formed, but the system is in a state that forbids it right now" (delete a non-empty folder).
- **`Internal` vs `Unavailable`.** `Internal` is *your bug* — do not retry, it will fail again. `Unavailable` is *transient* — a retry with backoff may succeed. Returning `Internal` for everything is the gRPC equivalent of `catch (Exception)`: it eats the one bit of information — "is retrying safe?" — that the operator most needs. Choosing the code well is a senior skill; the gRPC error guide is the reference: <https://grpc.io/docs/guides/error/>.

### Reading the code on the client

The client decodes the status with `status.FromError`:

```go
resp, err := client.GetNote(ctx, &notesv1.GetNoteRequest{Id: "missing"})
if err != nil {
	st, _ := status.FromError(err) // st is *status.Status
	switch st.Code() {
	case codes.NotFound:
		// expected; render an empty state
	case codes.Unavailable:
		// retry with backoff
	default:
		slog.Error("GetNote failed", "code", st.Code(), "msg", st.Message())
	}
}
```

`status.FromError` always returns a `*status.Status`; `st.Code()` and `st.Message()` give you the classification and the human string.

## 6. Mapping domain errors to codes

The payoff of "one domain, two transports": your service layer returns *sentinel domain errors*, and each transport maps them to its own vocabulary. The gRPC adapter's `mapError` is one `errors.Is` switch:

```go
import "errors"

func mapError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, domain.ErrNotFound):
		return status.Error(codes.NotFound, "note not found")
	case errors.Is(err, domain.ErrAlreadyExists):
		return status.Error(codes.AlreadyExists, "note already exists")
	case errors.Is(err, domain.ErrInvalidInput):
		return status.Error(codes.InvalidArgument, err.Error())
	case errors.Is(err, domain.ErrConflict):
		return status.Error(codes.Aborted, "write conflict; retry")
	default:
		// Unknown error: log the detail, return a bare Internal.
		slog.Error("unmapped domain error", "err", err)
		return status.Error(codes.Internal, "internal error")
	}
}
```

The `default` arm is the safety floor: an error you forgot to map becomes `Internal` with no detail leaked — and the `slog.Error` ensures you *see* the gap in your logs and add a case. The exact same domain sentinels, in your chi handler, map to HTTP 404/409/400/409/500. The service layer authored them once; the two adapters translate.

## 7. Rich error details with `errdetails`

`codes.InvalidArgument` plus a string is fine for a human. For a *client that wants to render per-field errors*, attach structured details with `status.WithDetails` and the standard `google.rpc.errdetails` messages (`google.golang.org/genproto/googleapis/rpc/errdetails`):

```go
import (
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func invalidNote(violations []*errdetails.BadRequest_FieldViolation) error {
	st := status.New(codes.InvalidArgument, "note failed validation")
	br := &errdetails.BadRequest{FieldViolations: violations}
	st, err := st.WithDetails(br)
	if err != nil {
		return status.Error(codes.Internal, "failed to attach details")
	}
	return st.Err()
}

// usage:
return invalidNote([]*errdetails.BadRequest_FieldViolation{
	{Field: "title", Description: "must be 1-200 characters"},
	{Field: "tags",  Description: "at most 16 tags"},
})
```

The client reads them back from `st.Details()` and type-asserts to `*errdetails.BadRequest`. This is the structured, machine-consumable form of "which fields were wrong, and why" — the gRPC analogue of a JSON `{ "errors": { "title": "...", "tags": "..." } }` body, but typed and cross-language.

## 8. Deadlines and cancellation propagation

This is the feature that justifies gRPC's complexity in a service mesh. When a client sets a deadline:

```go
ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
defer cancel()
resp, err := client.GetNote(ctx, req)
```

grpc-go serializes the remaining budget into the **`grpc-timeout`** HTTP/2 header. The server reconstructs it: the handler's incoming `ctx` already has that deadline, so `ctx.Done()` fires when the budget is spent, *regardless of what the handler is doing*. If the handler makes its own outbound gRPC call and passes that same `ctx` along, the deadline propagates again — the whole call graph becomes deadline-respecting with zero per-hop bookkeeping. The deadlines guide is the reference: <https://grpc.io/docs/guides/deadlines/>.

Two disciplines follow:

1. **Always set a deadline on the client.** grpc-go has *no default client deadline* — a call with no deadline waits forever. The absence of a deadline is the bug, not a convenience.
2. **Always honor the context in the handler.** Pass `ctx` into every downstream call (`s.svc.Get(ctx, ...)`, which passes it into pgx). A handler that ignores `ctx.Done()` keeps burning CPU and a database connection after the client has given up — wasted capacity that shows up as elevated tail latency on *unrelated* RPCs.

When the deadline fires, map it cleanly. A canceled or expired context surfaces through `status.FromContextError`:

```go
select {
case <-ctx.Done():
	return status.FromContextError(ctx.Err()).Err() // -> Canceled or DeadlineExceeded
case res := <-work:
	return res, nil
}
```

`status.FromContextError` turns `context.Canceled` into `codes.Canceled` and `context.DeadlineExceeded` into `codes.DeadlineExceeded` — the correct codes, automatically.

## 9. What you now know

You can write server and client interceptors and chain them outside-in: a logging interceptor that emits one structured `slog` line per RPC with method, duration, peer, and code; a recovery interceptor that turns a panic into a clean `codes.Internal` without leaking the stack; a request-id interceptor that threads metadata through the call graph with `metadata.FromIncomingContext` and `metadata.NewOutgoingContext`. You can return precise `codes.Code` values via `status.Error`/`status.Errorf`, decode them client-side with `status.FromError`, and attach per-field details with `status.WithDetails` and `errdetails.BadRequest`. You can map your Week-5 sentinel domain errors to codes in one `errors.Is` switch — the same errors your chi handler maps to HTTP codes. And you understand deadline propagation: set one on every client call, honor `ctx.Done()` in every handler, and let the `grpc-timeout` header carry the budget across the mesh. The mini-project puts all of this behind the `notes` service over both gRPC and REST.
