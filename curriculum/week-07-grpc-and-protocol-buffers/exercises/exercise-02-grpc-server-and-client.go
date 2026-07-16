// =============================================================================
// Exercise 2 — A gRPC server and client for NotesService, in one runnable file
// C30 · Crunch Go — Week 7 (gRPC & Protocol Buffers)
// =============================================================================
//
// WHAT THIS FILE IS
//   A self-contained driver that (1) implements the NotesService gRPC server
//   over an in-memory store (so it runs with NO Postgres), (2) starts it on a
//   real localhost listener, and (3) drives a unary call and a server-streaming
//   call from a real gRPC client using the MODERN grpc.NewClient / grpc.NewServer
//   constructors — never the deprecated grpc.Dial.
//
// HOW TO RUN
//   This file imports the generated package produced by Exercise 1:
//     notesv1 "example.com/notes/gen/notes/v1"
//   So it compiles inside a module that has run `buf generate`. From a module
//   wired up like the mini-project:
//     go run ./exercises/exercise-02-grpc-server-and-client.go
//   Expected output is reproduced in exercises/SOLUTIONS.md.
//
// YOUR TASK
//   The server below implements CreateNote, GetNote, and WatchNotes fully.
//     (A) Implement ListNotes with cursor pagination over the in-memory slice
//         (page_size cap of 50, page_token = the index of the next item).
//     (B) Add a client call that creates three notes, lists them one page of
//         two at a time, and prints the next_page_token round-trip.
//     (C) Make WatchNotes emit a KEEPALIVE event every second so the stream
//         stays warm, and have the client print only non-keepalive events.
//   Everything you need is shown in the patterns below; no new concepts.
//
// Citations: grpc-go basics    https://grpc.io/docs/languages/go/basics/
//            grpc godoc        https://pkg.go.dev/google.golang.org/grpc
// =============================================================================

package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	notesv1 "example.com/notes/gen/notes/v1"
)

// -----------------------------------------------------------------------------
// In-memory store. Stands in for the Week-6 Postgres repository so this file
// runs anywhere. The mini-project swaps this for the real repository behind
// the SAME service interface.
// -----------------------------------------------------------------------------

type store struct {
	mu      sync.RWMutex
	byID    map[string]*notesv1.Note
	order   []string
	seq     int
	watcher chan *notesv1.NoteEvent
}

func newStore() *store {
	return &store{
		byID:    make(map[string]*notesv1.Note),
		watcher: make(chan *notesv1.NoteEvent, 64),
	}
}

func (s *store) create(title, body string, tags []string) *notesv1.Note {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seq++
	now := timestamppb.Now()
	n := &notesv1.Note{
		Id:        idFor(s.seq),
		Title:     title,
		Body:      body,
		Tags:      tags,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.byID[n.Id] = n
	s.order = append(s.order, n.Id)
	s.broadcast(&notesv1.NoteEvent{
		Kind: notesv1.NoteEventKind_NOTE_EVENT_KIND_CREATED,
		Note: n,
		At:   now,
	})
	return n
}

func (s *store) get(id string) (*notesv1.Note, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.byID[id]
	return n, ok
}

// broadcast is best-effort: a full buffer drops the event rather than blocking
// a write under the store lock. A production fan-out uses per-subscriber chans.
func (s *store) broadcast(ev *notesv1.NoteEvent) {
	select {
	case s.watcher <- ev:
	default:
	}
}

func idFor(seq int) string { return "n-" + time.Now().Format("150405") + "-" + itoa(seq) }

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}

// -----------------------------------------------------------------------------
// The gRPC server. Note the UnimplementedNotesServiceServer embed: it supplies
// defaults for any RPC not written here, so the server compiles across schema
// growth and returns codes.Unimplemented at runtime for the unwritten ones.
// -----------------------------------------------------------------------------

type server struct {
	notesv1.UnimplementedNotesServiceServer
	st *store
}

func (srv *server) CreateNote(ctx context.Context, req *notesv1.CreateNoteRequest) (*notesv1.CreateNoteResponse, error) {
	if req.GetTitle() == "" {
		return nil, status.Error(codes.InvalidArgument, "title must not be empty")
	}
	n := srv.st.create(req.GetTitle(), req.GetBody(), req.GetTags())
	return &notesv1.CreateNoteResponse{Note: n}, nil
}

func (srv *server) GetNote(ctx context.Context, req *notesv1.GetNoteRequest) (*notesv1.GetNoteResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id must not be empty")
	}
	n, ok := srv.st.get(req.GetId())
	if !ok {
		return nil, status.Errorf(codes.NotFound, "note %q not found", req.GetId())
	}
	return &notesv1.GetNoteResponse{Note: n}, nil
}

func (srv *server) DeleteNote(ctx context.Context, req *notesv1.DeleteNoteRequest) (*emptypb.Empty, error) {
	return &emptypb.Empty{}, nil // implemented fully in the mini-project
}

func (srv *server) WatchNotes(req *notesv1.WatchNotesRequest, stream notesv1.NotesService_WatchNotesServer) error {
	ctx := stream.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected or deadline fired: surface the right code.
			return status.FromContextError(ctx.Err()).Err()
		case ev := <-srv.st.watcher:
			if f := req.GetTagFilter(); f != "" && !hasTag(ev.GetNote(), f) {
				continue
			}
			if err := stream.Send(ev); err != nil {
				return err // client went away mid-stream
			}
		}
	}
}

func hasTag(n *notesv1.Note, tag string) bool {
	for _, t := range n.GetTags() {
		if t == tag {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// main: start the server, then drive it from a real client.
// -----------------------------------------------------------------------------

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	st := newStore()

	lis, err := net.Listen("tcp", "127.0.0.1:0") // :0 = OS picks a free port
	if err != nil {
		slog.Error("listen failed", "err", err)
		os.Exit(1)
	}
	addr := lis.Addr().String()

	grpcSrv := grpc.NewServer()
	notesv1.RegisterNotesServiceServer(grpcSrv, &server{st: st})
	go func() {
		if err := grpcSrv.Serve(lis); err != nil {
			slog.Error("serve failed", "err", err)
		}
	}()
	defer grpcSrv.GracefulStop()

	// --- Client: ONE conn for the process, reused. Modern constructor. ---
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		slog.Error("dial failed", "err", err)
		os.Exit(1)
	}
	defer conn.Close()
	client := notesv1.NewNotesServiceClient(conn)

	// Open the watch stream BEFORE creating notes, so we see the events.
	watchCtx, watchCancel := context.WithCancel(context.Background())
	defer watchCancel()
	stream, err := client.WatchNotes(watchCtx, &notesv1.WatchNotesRequest{})
	if err != nil {
		slog.Error("WatchNotes failed", "err", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			ev, err := stream.Recv()
			if errors.Is(err, io.EOF) || status.Code(err) == codes.Canceled {
				return
			}
			if err != nil {
				slog.Error("stream recv failed", "err", err)
				return
			}
			slog.Info("watch event",
				"kind", ev.GetKind().String(),
				"id", ev.GetNote().GetId(),
				"title", ev.GetNote().GetTitle(),
			)
		}
	}()

	// --- Unary: create a note, then read it back. Deadline on every call. ---
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	created, err := client.CreateNote(ctx, &notesv1.CreateNoteRequest{
		Title: "Ship gRPC", Body: "wire the second transport", Tags: []string{"work"},
	})
	if err != nil {
		slog.Error("CreateNote failed", "err", err)
		os.Exit(1)
	}
	slog.Info("created", "id", created.GetNote().GetId())

	got, err := client.GetNote(ctx, &notesv1.GetNoteRequest{Id: created.GetNote().GetId()})
	if err != nil {
		slog.Error("GetNote failed", "err", err)
		os.Exit(1)
	}
	slog.Info("fetched", "title", got.GetNote().GetTitle())

	// A NotFound, to prove the status code round-trips.
	_, err = client.GetNote(ctx, &notesv1.GetNoteRequest{Id: "does-not-exist"})
	slog.Info("expected NotFound", "code", status.Code(err).String())

	// Give the watch goroutine a beat to print the CREATED event, then stop.
	time.Sleep(200 * time.Millisecond)
	watchCancel()
	wg.Wait()
}
