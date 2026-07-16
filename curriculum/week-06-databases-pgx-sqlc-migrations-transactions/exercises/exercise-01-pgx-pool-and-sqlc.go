// Exercise 1 — pgx Pool + sqlc, behind the Week 5 Repository interface
//
// GOAL
//   Open a *pgxpool.Pool against a Postgres container, drive sqlc-generated
//   queries, thread ctx into every call, and translate pgx errors into the
//   service's sentinel errors (pgx.ErrNoRows -> ErrNotFound, 23505 ->
//   ErrConflict). The PgRepo must satisfy the same interface MemRepo did.
//
// SETUP (run these once)
//   docker run --name pg-ex01 -e POSTGRES_PASSWORD=devpass -e POSTGRES_DB=notes \
//     -p 5432:5432 -d postgres:16
//   export DATABASE_URL='postgres://postgres:devpass@localhost:5432/notes?sslmode=disable'
//   mkdir ex01 && cd ex01 && go mod init ex01
//   go get github.com/jackc/pgx/v5
//   # save migrations/000001_create_notes.up.sql, internal/db/query.sql, sqlc.yaml
//   # (their contents are in SOLUTIONS.md and the lecture)
//   migrate -path migrations -database "$DATABASE_URL" up
//   sqlc generate          # produces internal/db/*.go
//   go run .
//
// TASKS
//   1. Implement newPool: parse the DSN, set MaxConns, Ping at startup.
//   2. Implement Get: call q.GetNote, translate pgx.ErrNoRows -> ErrNotFound.
//   3. Implement Create: call q.CreateNote, translate a 23505 PgError ->
//      ErrConflict (use errors.As with *pgconn.PgError).
//   4. Confirm toDomain maps the generated db.Note row to the domain Note.
//
// NOTE
//   This file shows the repository SHAPE. The `db` package is sqlc-generated;
//   the import path and method names match the lecture's query.sql. Run sqlc
//   generate before `go build`.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	// "ex01/internal/db" // sqlc-generated; uncomment after `sqlc generate`
)

// ---- domain (the Week 5 contract) ----

type Note struct {
	ID        string
	Title     string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

var (
	ErrNotFound = errors.New("note not found")
	ErrConflict = errors.New("note already exists")
)

type Repository interface {
	Create(ctx context.Context, n Note) (Note, error)
	Get(ctx context.Context, id string) (Note, error)
}

// ---- pool ----

func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MinConns = 2
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// ---- repository ----
//
// PgRepo wraps the sqlc-generated *db.Queries. With the generated package in
// place you would write:
//
//   type PgRepo struct {
//       pool *pgxpool.Pool
//       q    *db.Queries
//   }
//   func NewPgRepo(pool *pgxpool.Pool) *PgRepo {
//       return &PgRepo{pool: pool, q: db.New(pool)}
//   }
//
//   func (r *PgRepo) Get(ctx context.Context, id string) (Note, error) {
//       row, err := r.q.GetNote(ctx, id)
//       if err != nil {
//           if errors.Is(err, pgx.ErrNoRows) {
//               return Note{}, ErrNotFound
//           }
//           return Note{}, err
//       }
//       return toDomain(row), nil
//   }
//
//   func (r *PgRepo) Create(ctx context.Context, n Note) (Note, error) {
//       row, err := r.q.CreateNote(ctx, db.CreateNoteParams{ID: n.ID, Title: n.Title, Body: n.Body})
//       if err != nil {
//           var pgErr *pgconn.PgError
//           if errors.As(err, &pgErr) && pgErr.Code == "23505" {
//               return Note{}, ErrConflict
//           }
//           return Note{}, err
//       }
//       return toDomain(row), nil
//   }
//
// toDomain maps db.Note -> Note:
//   func toDomain(r db.Note) Note {
//       return Note{ID: r.ID, Title: r.Title, Body: r.Body,
//           CreatedAt: r.CreatedAt.Time, UpdatedAt: r.UpdatedAt.Time}
//   }

// classifyConflict shows the error-translation logic in isolation, runnable
// without the generated package. It is the same logic Create uses.
func classifyConflict(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return ErrConflict
	}
	return err
}

func main() {
	ctx := context.Background()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		fmt.Fprintln(os.Stderr, "set DATABASE_URL")
		os.Exit(1)
	}
	pool, err := newPool(ctx, dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	fmt.Println("pool ready; wire in db.New(pool) after `sqlc generate`")

	// Demonstrate the translation logic.
	fmt.Println(classifyConflict(pgx.ErrNoRows))                          // note not found
	fmt.Println(classifyConflict(&pgconn.PgError{Code: "23505"}))         // note already exists
}
