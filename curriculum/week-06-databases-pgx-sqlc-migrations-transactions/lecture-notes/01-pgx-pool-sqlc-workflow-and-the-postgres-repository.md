# Lecture 1 — The `pgx` Pool, the `sqlc` Workflow, and the Postgres Repository

> **Time:** 2 hours. Take the `pgx`-pool material in one sitting and the `sqlc`-workflow material in a second. **Prerequisites:** Week 5 (the `Repository` interface and the seam) and Week 4 (`context`). **Citations:** the `pgx` docs at <https://pkg.go.dev/github.com/jackc/pgx/v5>, the `pgxpool` docs at <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>, and the `sqlc` docs at <https://docs.sqlc.dev/>.

## 1. Why a typed query layer, not an ORM, and why this lecture first

Coming from Django's ORM or Rails' ActiveRecord, the instinct is to model rows as objects and let a library write the SQL. Go's senior default is the opposite: *write the SQL yourself, and let a tool make it type-safe.* The tool is `sqlc`. You write `SELECT id, title, body FROM notes WHERE id = $1`, annotate it, and `sqlc` generates a Go function `GetNote(ctx, id) (Note, error)` whose types match the schema exactly — checked when you run `sqlc generate`, not at runtime. The SQL in your repo is the SQL on the wire. There is no DSL, no lazy-loading N+1, no reflection. When a query is slow, you `EXPLAIN` the literal statement you wrote.

This matters because a service's relationship to its database is the part you least want hidden. An ORM that decides to issue a `JOIN`, or 101 `SELECT`s, or a different query than you expected, is a performance incident waiting to happen — and the abstraction that was supposed to save you time costs you a debugging afternoon when it surprises you. `sqlc` removes the surprise: you own the SQL, the tool owns the type-checking and the boilerplate. The one place an ORM still earns its keep is rapid prototyping of a CRUD admin where the queries are trivial and uniform; for the kind of service C30 builds — one you operate under load and reason about under concurrency — legible SQL wins.

This lecture is first because **the repository is where Week 5's seam meets the database.** Get the `PgRepo` right — same interface, `pgx` types translated to domain types at the boundary — and the service, handlers, and middleware from Week 5 do not change at all. That is the test of the seam, and the payoff of having drawn it.

## 2. The connection pool

You almost never open a single connection to Postgres. You open a *pool* — a concurrency-safe set of connections that hands one out per query and reclaims it after:

```go
package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func newPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse dsn: %w", err)
	}
	// Size the pool deliberately. Too small: requests queue for a connection.
	// Too large: you exhaust Postgres's max_connections (default 100).
	cfg.MaxConns = 10
	cfg.MinConns = 2
	cfg.MaxConnLifetime = time.Hour
	cfg.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	// Verify connectivity at startup; fail fast if the database is unreachable.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping: %w", err)
	}
	return pool, nil
}

// usage in main:
//   dsn := os.Getenv("DATABASE_URL")  // postgres://user:pass@host:5432/db?sslmode=disable
//   pool, err := newPool(ctx, dsn)
//   defer pool.Close()
```

Five things:

1. **`*pgxpool.Pool` is concurrency-safe.** You create one at startup, store it (often on your repository struct), and use it from every goroutine. You do not create a pool per request.
2. **`MaxConns` is a real decision.** Each pooled connection is a Postgres backend process; `MaxConns × number-of-service-instances` must stay under Postgres's `max_connections`. A common starting point is `MaxConns = 4 × CPU cores`, tuned by load testing. Too small starves requests; too large overwhelms the database.
3. **The DSN comes from the environment** (`DATABASE_URL`), never hard-coded — the 12-factor config rule Week 10 formalises, planted here.
4. **`pool.Ping(ctx)` at startup** fails fast if the database is unreachable, so a misconfigured DSN is a boot error, not a first-request 500.
5. **`defer pool.Close()`** at the top of `main` drains the pool on shutdown — part of the graceful-shutdown story from Week 5.

Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#ParseConfig>.

## 3. The `sqlc` workflow: schema → query → generate

`sqlc` reads two inputs — your **schema** and your **queries** — and emits type-safe Go. The config ties them together:

```yaml
# sqlc.yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"        # the up migrations ARE the schema
    queries: "internal/db/query.sql"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"   # generate code targeting pgx, not database/sql
        emit_json_tags: true
        emit_pointers_for_null_types: true
```

The schema (the same SQL your `up` migration applies — Lecture 2):

```sql
-- migrations/000001_create_notes.up.sql
CREATE TABLE notes (
    id         TEXT        PRIMARY KEY,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

The queries, each annotated with a name and a result cardinality:

```sql
-- internal/db/query.sql

-- name: CreateNote :one
INSERT INTO notes (id, title, body)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetNote :one
SELECT * FROM notes WHERE id = $1;

-- name: ListNotes :many
SELECT * FROM notes
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: UpdateNote :one
UPDATE notes
SET title = $1, body = $2, updated_at = now()
WHERE id = $3
RETURNING *;

-- name: DeleteNote :execrows
DELETE FROM notes WHERE id = $1;
```

The annotations:

- **`:one`** — the query returns exactly one row; the generated method returns `(Note, error)` and yields `pgx.ErrNoRows` if there is none.
- **`:many`** — returns zero or more rows; the method returns `([]Note, error)`.
- **`:exec`** — returns no rows; the method returns just `error`.
- **`:execrows`** — returns the affected-row count as `(int64, error)` — used by `DELETE` to detect "deleted nothing" (which the repository maps to `ErrNotFound`).

Run `sqlc generate` and you get `internal/db/{models.go, query.sql.go, db.go}` containing a `Note` struct (matching the columns), a `Queries` type, and a method per query:

```go
// generated — do not edit
func (q *Queries) GetNote(ctx context.Context, id string) (Note, error) { ... }
func (q *Queries) ListNotes(ctx context.Context, arg ListNotesParams) ([]Note, error) { ... }
func (q *Queries) DeleteNote(ctx context.Context, id string) (int64, error) { ... }
```

The promise: **a column typo is a generate-time error.** Rename `body` to `content` in the schema but not the query, run `sqlc generate`, and it fails: `column "body" does not exist`. The gap between "valid SQL against this schema" and "compiling Go" is closed before you ever run the program. Citation: <https://docs.sqlc.dev/en/latest/reference/query-annotations.html>.

```mermaid
flowchart LR
  A["Schema up migrations"] --> C["sqlc generate"]
  B["Annotated query.sql"] --> C
  C --> D["models.go Note struct"]
  C --> E["query.sql.go methods"]
  D --> F["Type safe Go in the repository"]
  E --> F
```
*The sqlc workflow: schema plus annotated queries compile into type-safe Go.*

## 4. The `PgRepo` — same interface, translated types

The repository implements Week 5's `notes.Repository` interface, backed by the generated `Queries`:

```go
package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/you/notes-api/internal/db"     // sqlc-generated
	"github.com/you/notes-api/internal/notes"  // domain + interface
)

type PgRepo struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

func NewPgRepo(pool *pgxpool.Pool) *PgRepo {
	return &PgRepo{pool: pool, q: db.New(pool)}
}

func (r *PgRepo) Get(ctx context.Context, id string) (notes.Note, error) {
	row, err := r.q.GetNote(ctx, id)
	if err != nil {
		// Translate pgx's no-rows into the service's sentinel.
		if errors.Is(err, pgx.ErrNoRows) {
			return notes.Note{}, notes.ErrNotFound
		}
		return notes.Note{}, err
	}
	return toDomain(row), nil
}

func (r *PgRepo) Create(ctx context.Context, n notes.Note) (notes.Note, error) {
	row, err := r.q.CreateNote(ctx, db.CreateNoteParams{
		ID: n.ID, Title: n.Title, Body: n.Body,
	})
	if err != nil {
		// 23505 = unique_violation -> the service's conflict sentinel.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return notes.Note{}, notes.ErrConflict
		}
		return notes.Note{}, err
	}
	return toDomain(row), nil
}

// toDomain maps a generated db.Note row to the domain notes.Note. This is the
// boundary where pgx/sqlc types STOP and domain types begin.
func toDomain(r db.Note) notes.Note {
	return notes.Note{
		ID:        r.ID,
		Title:     r.Title,
		Body:      r.Body,
		CreatedAt: r.CreatedAt.Time, // pgtype.Timestamptz -> time.Time
		UpdatedAt: r.UpdatedAt.Time,
	}
}
```

Four things this layer does:

1. **Implements the exact same interface as `MemRepo`.** The service constructs `notes.NewService(postgres.NewPgRepo(pool))` instead of `notes.NewService(notes.NewMemRepo())` — one line in `main` changes; nothing else.
2. **Translates `pgx.ErrNoRows` → `notes.ErrNotFound`.** The service knows only its own sentinels; it never imports `pgx`. This is what keeps the service database-agnostic.
3. **Translates a unique-constraint violation → `notes.ErrConflict`** by classifying the error as a `*pgconn.PgError` (Week 2's `errors.As`) and checking the SQLSTATE code `23505`. The repository is the only layer that knows what `23505` means.
4. **Maps the generated `db.Note` to the domain `notes.Note`** in `toDomain`. The generated row types (with `pgtype.Timestamptz` fields) are an implementation detail of the repository; the service sees clean `time.Time`s.

This translation is the discipline that makes the seam real. A `PgRepo` that returned `pgx.ErrNoRows` to the service, or leaked a `db.Note`, would force the service to know about `pgx` — and then the seam is a lie. Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError> and the SQLSTATE list at <https://www.postgresql.org/docs/current/errcodes-appendix.html>.

```mermaid
flowchart LR
  A["PgRepo method call"] --> B["pgx ErrNoRows"]
  A --> C["pgconn PgError 23505"]
  A --> D["db Note row"]
  B --> E["notes ErrNotFound"]
  C --> F["notes ErrConflict"]
  D --> G["toDomain mapping"]
  E --> H["Service sees only domain types"]
  F --> H
  G --> H
```
*Every pgx and sqlc type is translated to a domain sentinel or struct at the repository boundary.*

## 5. Threading `context` into every query

Every `sqlc`/`pgx` method takes `ctx` first, and you pass the request's context (or a tighter timeout) into it:

```go
func (r *PgRepo) List(ctx context.Context, limit, offset int32) ([]notes.Note, error) {
	rows, err := r.q.ListNotes(ctx, db.ListNotesParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, err
	}
	out := make([]notes.Note, len(rows))
	for i, row := range rows {
		out[i] = toDomain(row)
	}
	return out, nil
}
```

When `ctx` is cancelled — the client disconnected, the request budget expired, the server is shutting down — `pgx` sends a cancellation to Postgres (the wire-protocol `CancelRequest`), Postgres aborts the running query, and the connection returns to the pool. A query that ignored `ctx` would hold its connection until Postgres finished, and under a slow database that is how you exhaust the pool and stall the whole service. The Week 4 discipline — thread `ctx` everywhere — is what makes a slow query a *cancelled* query instead of a *stuck* one. Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool#Pool.Query>.

## 6. Reading the SQL `sqlc` generated

The wire-bytes discipline of the week: you can produce the exact SQL for any query. The generated `query.sql.go` contains it as a string constant:

```go
const getNote = `-- name: GetNote :one
SELECT id, title, body, created_at, updated_at FROM notes WHERE id = $1`
```

That is *exactly* what goes on the wire — `sqlc` expanded the `SELECT *` to the explicit column list at generate time (one reason `*` is safe with `sqlc`: it is resolved against the schema, not at runtime). To see the plan, paste it into `psql`:

```
notes=# EXPLAIN ANALYZE SELECT id, title, body, created_at, updated_at FROM notes WHERE id = '42';
 Index Scan using notes_pkey on notes  (cost=0.15..8.17 rows=1 width=...) (actual time=0.021..0.022 rows=1 loops=1)
   Index Cond: (id = '42'::text)
 Planning Time: 0.08 ms
 Execution Time: 0.04 ms
```

An index scan on the primary key — fast. When a query is slow, this is the loop: read the generated SQL, `EXPLAIN ANALYZE` it, find the sequential scan or the missing index, add the index in a migration. There is no ORM between you and the plan. Citation: the Postgres `EXPLAIN` docs at <https://www.postgresql.org/docs/current/using-explain.html>.

## 7. Exercise pointer

Now do **Exercise 1 — `pgx` Pool and `sqlc`**. Stand up a pool against a Postgres container, write the `notes` schema and queries, run `sqlc generate`, implement the `Get`/`Create` repository methods with `ctx` and the `ErrNoRows`/`23505` translation, and write a test that a missing note yields `ErrNotFound` and a duplicate yields `ErrConflict`. The acceptance criterion is that the repository satisfies the Week 5 `Repository` interface and the service runs against it unchanged.

## 8. Summary

- `sqlc` over an ORM for a service that values legible, reviewable SQL: you write the SQL, the tool makes it type-safe at generate time. No DSL, no reflection, no surprise N+1.
- `*pgxpool.Pool` is a concurrency-safe connection pool; create one at startup, size `MaxConns` deliberately against Postgres's `max_connections`, `Ping` at boot, `Close` on shutdown.
- The `sqlc` loop: schema (`.sql`) + queries (annotated `:one`/`:many`/`:exec`/`:execrows`) → `sqlc generate` → type-safe `Queries` methods. A column typo is a generate-time error.
- The `PgRepo` implements the same `Repository` interface as `MemRepo`; one line in `main` changes when you swap them.
- Translate at the boundary: `pgx.ErrNoRows` → `ErrNotFound`, `pgconn.PgError` code `23505` → `ErrConflict`, a `db.Note` → a domain `Note`. Keep `pgx` types out of the service.
- Thread `ctx` into every query so a request budget cancels a slow query server-side.
- Read the generated SQL and `EXPLAIN ANALYZE` it; there is no translation layer hiding the statement or the plan.

Cited references this lecture pulled from: <https://pkg.go.dev/github.com/jackc/pgx/v5>, <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>, <https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError>, <https://docs.sqlc.dev/>, <https://www.postgresql.org/docs/current/errcodes-appendix.html>, <https://www.postgresql.org/docs/current/using-explain.html>.
