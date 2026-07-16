// =============================================================================
// Exercise 3 — Interceptors and status codes
// C30 · Crunch Go — Week 7 (gRPC & Protocol Buffers)
// =============================================================================
//
// WHAT THIS FILE IS
//   A runnable driver that chains a logging interceptor (slog) and a panic-
//   recovery interceptor onto a grpc.NewServer, implements a NotesService.
//   CreateNote that VALIDATES input and returns precise status.Error codes,
//   and a client that deliberately triggers an InvalidArgument and a NotFound,
//   then asserts the codes with status.FromError.
//
// HOW TO RUN
//   Same module setup as Exercise 2 (it imports the generated notesv1 package):
//     go run ./exercises/exercise-03-interceptors-and-status-codes.go
//   Expected output is in exercises/SOLUTIONS.md.
//
// YOUR TASK
//   The interceptors and two RPCs are complete and correct.
//     (A) Add a requestIDUnary server interceptor that reads "x-request-id"
//         from incoming metadata (metadata.FromIncomingContext), defaults a
//         fresh value when absent, and includes it in every log line. Chain it
//         BETWEEN recovery and logging.
//     (B) Add a client unary interceptor that injects "x-request-id" on every
//         outbound call when the caller did not set one.
//     (C) Add one RPC, GetNote, that returns codes.NotFound for an unknown id,
//         and add a client call asserting that code via status.FromError.
//     (D) Add a handler that panics on a magic title ("BOOM") and prove the
//         recovery interceptor turns it into codes.Internal with NO stack
//         leaked to the client.
//
// Citations: interceptor chaining  https://pkg.go.dev/google.golang.org/grpc#ChainUnaryInterceptor
//            status / codes        https://pkg.go.dev/google.golang.org/grpc/status
//            metadata              https://pkg.go.dev/google.golang.org/grpc/metadata
// =============================================================================

package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"runtime/debug"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	notesv1 "example.com/notes/gen/notes/v1"
)

// -----------------------------------------------------------------------------
// Interceptors
// -----------------------------------------------------------------------------

// loggingUnary emits one structured line per RPC with method, code, duration,
// and peer. The code drives the log level: server faults are errors, client
// faults are warnings, success is info.
func loggingUnary(
	ctx context.Context,
	req any,
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (any, error) {
	start := time.Now()
	resp, err := handler(ctx, req)
	code := status.Code(err)

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

// recoveryUnary turns a panic into a clean codes.Internal. The named return
// `err` lets the deferred closure overwrite the response with the status. The
// stack goes to the log; the CLIENT gets only "internal error".
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
			err = status.Error(codes.Internal, "internal error")
		}
	}()
	return handler(ctx, req)
}

// -----------------------------------------------------------------------------
// Server with validating handlers
// -----------------------------------------------------------------------------

type server struct {
	notesv1.UnimplementedNotesServiceServer
}

const maxTitleLen = 200

func (srv *server) CreateNote(ctx context.Context, req *notesv1.CreateNoteRequest) (*notesv1.CreateNoteResponse, error) {
	// Validate -> precise InvalidArgument. The message names the field and the
	// rule, so the caller can fix it without reading our source.
	title := req.GetTitle()
	switch {
	case title == "":
		return nil, status.Error(codes.InvalidArgument, "title must not be empty")
	case utf8.RuneCountInString(title) > maxTitleLen:
		return nil, status.Errorf(codes.InvalidArgument,
			"title must be at most %d characters, got %d", maxTitleLen, utf8.RuneCountInString(title))
	case len(req.GetTags()) > 16:
		return nil, status.Error(codes.InvalidArgument, "at most 16 tags allowed")
	}

	if title == "BOOM" { // Task (D): prove the recovery interceptor catches this.
		panic("simulated handler bug")
	}

	return &notesv1.CreateNoteResponse{
		Note: &notesv1.Note{Id: "n-1", Title: title, Body: req.GetBody(), Tags: req.GetTags()},
	}, nil
}

func (srv *server) GetNote(ctx context.Context, req *notesv1.GetNoteRequest) (*notesv1.GetNoteResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id must not be empty")
	}
	// Nothing is stored here, so every lookup is a NotFound — the point is the
	// code, not the data.
	return nil, status.Errorf(codes.NotFound, "note %q not found", req.GetId())
}

// -----------------------------------------------------------------------------
// main
// -----------------------------------------------------------------------------

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}
	addr := lis.Addr().String()

	// recovery OUTERMOST so it wraps logging and every handler.
	grpcSrv := grpc.NewServer(grpc.ChainUnaryInterceptor(recoveryUnary, loggingUnary))
	notesv1.RegisterNotesServiceServer(grpcSrv, &server{})
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			slog.Error("serve failed", "err", err)
		}
	}()
	defer grpcSrv.GracefulStop()

	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("dial failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()
	client := notesv1.NewNotesServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// 1) Happy path.
	if _, err := client.CreateNote(ctx, &notesv1.CreateNoteRequest{Title: "hello"}); err != nil {
		slog.Error("unexpected create failure", "err", err)
	} else {
		slog.Info("create ok")
	}

	// 2) Empty title -> InvalidArgument. Assert the code.
	_, err = client.CreateNote(ctx, &notesv1.CreateNoteRequest{Title: ""})
	assertCode("empty title", err, codes.InvalidArgument)

	// 3) Missing note -> NotFound. Assert the code.
	_, err = client.GetNote(ctx, &notesv1.GetNoteRequest{Id: "ghost"})
	assertCode("missing note", err, codes.NotFound)

	// 4) Panic -> Internal, no stack leaked to the client.
	_, err = client.CreateNote(ctx, &notesv1.CreateNoteRequest{Title: "BOOM"})
	assertCode("panicking handler", err, codes.Internal)
	if st, ok := status.FromError(err); ok {
		slog.Info("panic message to client", "msg", st.Message()) // "internal error", no stack
	}
}

// assertCode decodes the status and reports whether the observed code matches.
func assertCode(label string, err error, want codes.Code) {
	st, _ := status.FromError(err)
	got := st.Code()
	if got == want {
		slog.Info("code as expected", "case", label, "code", got.String())
	} else {
		slog.Error("WRONG code", "case", label, "want", want.String(), "got", got.String(), "msg", st.Message())
	}
}
