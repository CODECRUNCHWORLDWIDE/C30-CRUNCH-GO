# Lecture 2 — `golang-migrate` Schema Migrations and the Transaction Pattern

> **Time:** 2 hours. Take the migrations material in one sitting and the transactions material in a second. **Prerequisites:** Lecture 1 (the pool, `sqlc`, the repository). **Citations:** the `golang-migrate` docs at <https://github.com/golang-migrate/migrate>, the `pgx` transaction docs at <https://pkg.go.dev/github.com/jackc/pgx/v5#Tx>, and the `sqlc` transactions guide at <https://docs.sqlc.dev/en/latest/howto/transactions.html>.

## 1. Why migrations, and the up/down discipline

Your schema changes over the life of a service: you add a column, you create an index, you split a table. A *migration* is a versioned, repeatable description of one such change, so that any database — your laptop, CI, staging, production — can be brought to the same schema version by applying the same ordered set of changes. Without migrations, "the schema" is whatever ad-hoc `ALTER TABLE`s someone ran, undocumented and unreproducible. With migrations, the schema is code: in the repo, reviewed, versioned.

`golang-migrate` represents each migration as a *pair* of SQL files:

```
migrations/
  000001_create_notes.up.sql      -- forward: create the table
  000001_create_notes.down.sql    -- backward: drop it
  000002_add_notes_owner.up.sql    -- forward: add a column
  000002_add_notes_owner.down.sql  -- backward: drop the column
```

The `up` moves the schema forward to version N; the `down` moves it back to N−1. The discipline that the whole back half of the track depends on: **every migration has a `down`, and you have run it at least once.** A migration without a tested `down` is a migration you *cannot roll back*, and "we can't roll back the schema" is exactly the sentence you do not want to say at 3 AM when a deploy went wrong. The mini-project requires a clean `migrate down` and re-`up` — that demonstration is the test of your `down`. Citation: <https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md>.

## 2. Authoring and applying migrations

Create a migration pair with the CLI:

```bash
migrate create -ext sql -dir migrations -seq create_notes
# creates migrations/000001_create_notes.up.sql and .down.sql (empty)
```

Fill them in:

```sql
-- 000001_create_notes.up.sql
CREATE TABLE notes (
    id         TEXT        PRIMARY KEY,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

```sql
-- 000001_create_notes.down.sql
DROP TABLE notes;
```

Apply and roll back:

```bash
export DATABASE_URL='postgres://postgres:devpass@localhost:5432/notes?sslmode=disable'

migrate -path migrations -database "$DATABASE_URL" up        # apply all pending
migrate -path migrations -database "$DATABASE_URL" down 1    # roll back the latest
migrate -path migrations -database "$DATABASE_URL" version   # current version
```

Three things:

1. **`golang-migrate` tracks applied migrations in a `schema_migrations` table** it creates in your database. So `up` run twice applies only the migrations not yet recorded — it is idempotent over the set. The table holds the current version and a `dirty` flag.
2. **The `dirty` flag** is set when a migration fails partway. A dirty database refuses further migrations until you fix the underlying issue and `migrate force <version>` to clear the flag — a deliberate safety brake so you do not stack migrations on a half-applied schema.
3. **A second migration that adds a column** shows the forward/back symmetry:

```sql
-- 000002_add_notes_owner.up.sql
ALTER TABLE notes ADD COLUMN owner TEXT NOT NULL DEFAULT '';
CREATE INDEX idx_notes_owner ON notes (owner);
```
```sql
-- 000002_add_notes_owner.down.sql
DROP INDEX idx_notes_owner;
ALTER TABLE notes DROP COLUMN owner;
```

The `down` undoes the `up` in reverse order (drop the index before the column it indexes). Citation: <https://github.com/golang-migrate/migrate/tree/master/cmd/migrate>.

## 3. Migrations in tests, and the production split

In **integration tests** (Lecture 3 of Week 8 and the mini-project), you apply migrations programmatically against the ephemeral container before running the tests, using the migrate library:

```go
import (
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func applyMigrations(dsn string) error {
	m, err := migrate.New("file://../../migrations", dsn)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
```

In **production**, the team decision is whether the service applies migrations at startup or whether a separate step does. The common senior choice for a service that may run multiple replicas: **do not auto-apply on every pod startup** (two pods racing to migrate is a hazard); instead run migrations as a distinct, gated step — a Kubernetes `Job`, a CI release stage, or an init container that runs once — before the new code rolls out. Week 11 returns to this when we discuss rollouts; for now, know that the `migrate` binary and the migrate library give you both options, and the gated-step option is the safer default. Citation: the deployment notes at <https://github.com/golang-migrate/migrate#use-in-your-go-project>.

## 4. The transaction: atomic, all-or-nothing

A transaction groups statements so they all commit or all roll back. The use case: "create a note *and* write an audit row" must be atomic — you never want the note without its audit entry or vice versa. In `pgx`:

```go
func (r *PgRepo) CreateWithAudit(ctx context.Context, n notes.Note, actor string) (notes.Note, error) {
	// 1. Begin a transaction.
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return notes.Note{}, fmt.Errorf("begin: %w", err)
	}
	// 2. Defer a rollback. A rollback AFTER a successful commit is a harmless
	//    no-op, so this guarantees we never leak an open transaction on an
	//    early return or a panic.
	defer tx.Rollback(ctx)

	// 3. Run queries on the transaction via q.WithTx(tx) — the sqlc idiom.
	qtx := r.q.WithTx(tx)

	row, err := qtx.CreateNote(ctx, db.CreateNoteParams{ID: n.ID, Title: n.Title, Body: n.Body})
	if err != nil {
		return notes.Note{}, err // defer rolls back
	}
	if err := qtx.InsertAudit(ctx, db.InsertAuditParams{
		NoteID: n.ID, Actor: actor, Action: "create",
	}); err != nil {
		return notes.Note{}, err // defer rolls back — the note insert is undone too
	}

	// 4. Commit. This is the ONLY thing that persists the work.
	if err := tx.Commit(ctx); err != nil {
		return notes.Note{}, fmt.Errorf("commit: %w", err)
	}
	return toDomain(row), nil
}
```

Five load-bearing details:

1. **`pool.Begin(ctx)`** starts a transaction and acquires a connection from the pool for its duration. All statements in the transaction run on that one connection.
2. **`defer tx.Rollback(ctx)`** immediately after `Begin` is the idiom. Because a rollback after a commit is a no-op, this single line correctly handles *every* exit: an error return, a panic, or the success path (where `Commit` has already happened and `Rollback` does nothing).
3. **`r.q.WithTx(tx)`** returns a new `*Queries` whose statements run inside the transaction. This is how `sqlc` queries participate in a transaction — the *same* generated methods, bound to `tx` instead of the pool. Without `WithTx`, a query would run on a *different* pooled connection, outside the transaction, defeating the atomicity.
4. **An error on any statement returns early**, and the deferred `Rollback` undoes *everything* — the note insert is rolled back when the audit insert fails. That is atomicity: all or nothing.
5. **`tx.Commit(ctx)` is the only thing that persists.** Reach the commit and both rows are durable; return before it and neither is.

Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5#Tx> and <https://docs.sqlc.dev/en/latest/howto/transactions.html>.

## 5. Mapping a transaction boundary to a use case

The hardest part of transactions is not the API; it is deciding *where the boundary goes*. The rule: **one transaction per use case.** A use case is a single unit of business intent that must succeed or fail as a whole.

- "Create a note with its audit row" → **one** transaction (both rows or neither).
- "Transfer money from account A to B" → **one** transaction (the debit and credit must both happen).
- "List all notes" → **no** transaction (a single read needs none; the implicit single-statement transaction Postgres gives every query is enough).
- "Import 1000 notes from a file" → a judgement call: one big transaction (all-or-nothing import, but a long-held lock and a big rollback on failure) vs one per note (each independent, partial progress survives) vs batches of 100 (a middle ground). The right answer depends on whether a partial import is acceptable.

A boundary drawn too *wide* holds locks too long and rolls back too much on failure; drawn too *narrow* breaks atomicity (the note without its audit row). The boundary is a design decision you defend in review, not a default. Citation: the use-case framing in the Postgres transaction docs at <https://www.postgresql.org/docs/current/tutorial-transactions.html>.

## 6. The wrapper helper — `BeginFunc`-style

The `Begin`/`defer Rollback`/`Commit` boilerplate repeats. `pgx` ships `pgx.BeginFunc` (and `pgx.BeginTxFunc` for an explicit isolation level) that wrap it: you pass a function that does the work, and the helper begins, runs it, and commits on a nil error or rolls back on a non-nil one:

```go
func (r *PgRepo) CreateWithAudit(ctx context.Context, n notes.Note, actor string) (notes.Note, error) {
	var out notes.Note
	err := pgx.BeginFunc(ctx, r.pool, func(tx pgx.Tx) error {
		qtx := r.q.WithTx(tx)
		row, err := qtx.CreateNote(ctx, db.CreateNoteParams{ID: n.ID, Title: n.Title, Body: n.Body})
		if err != nil {
			return err
		}
		if err := qtx.InsertAudit(ctx, db.InsertAuditParams{NoteID: n.ID, Actor: actor, Action: "create"}); err != nil {
			return err
		}
		out = toDomain(row)
		return nil
	})
	return out, err
}
```

`BeginFunc` commits if your function returns nil, rolls back if it returns an error or panics. It removes the chance of forgetting the `defer Rollback` or the `Commit`. Prefer it for the common case; drop to the explicit `Begin`/`Commit` when you need fine control (a savepoint, a conditional commit). Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5#BeginFunc>.

## 7. Savepoints — a nested rollback

Occasionally you want to roll back *part* of a transaction without aborting the whole thing — "try this insert; if it conflicts, skip it and continue." That is a **savepoint**: a named point you can roll back to, leaving earlier work intact. `pgx` exposes it as a nested `Begin` on a `tx`:

```go
sp, err := tx.Begin(ctx) // a savepoint (pgx implements nested Begin as SAVEPOINT)
if err != nil { return err }
if err := qtx.InsertAudit(ctx, ...); err != nil {
	_ = sp.Rollback(ctx) // roll back to the savepoint; the outer tx survives
} else {
	_ = sp.Commit(ctx)   // release the savepoint; keep the work
}
```

Savepoints are a niche tool — most use cases are all-or-nothing and need no savepoint — but when you need "best-effort sub-steps inside an atomic operation," they are the mechanism. Do not reach for them by default; reach for them when the use case genuinely has optional sub-steps. Citation: the Postgres `SAVEPOINT` docs at <https://www.postgresql.org/docs/current/sql-savepoint.html>.

## 8. Exercise pointer

Now do **Exercise 2 — Migrations and Transactions**. Author an `up`/`down` migration pair for a two-table schema (notes + audit), apply it, then write a `CreateWithAudit` that wraps both inserts in one transaction with `defer Rollback`/`Commit`. Force the audit insert to fail and prove — by querying the database afterward — that the note insert was rolled back too. The acceptance criterion is that a forced mid-transaction failure leaves *zero* rows in both tables, and a `migrate down` cleanly drops them.

## 9. Summary

- A migration is an `up`/`down` SQL pair; `golang-migrate` applies them in order and tracks them in `schema_migrations`. The `dirty` flag brakes on a half-applied migration.
- **Every migration has a `down`, and you have run it once.** A migration you cannot roll back is a 3 AM incident waiting to happen.
- Apply with `migrate up`; roll back with `migrate down 1`. In tests, apply programmatically before the run. In production, prefer a gated migration step over auto-apply-on-startup (replicas racing to migrate is a hazard).
- A transaction is atomic: `pool.Begin(ctx)`, `defer tx.Rollback(ctx)`, run queries via `q.WithTx(tx)`, then `tx.Commit(ctx)`. The deferred rollback handles every exit; the commit is the only thing that persists.
- One transaction per use case. Too wide holds locks too long; too narrow breaks atomicity. The boundary is a design decision.
- `pgx.BeginFunc` wraps the begin/rollback/commit boilerplate; prefer it for the common case.
- Savepoints (nested `tx.Begin`) roll back part of a transaction; a niche tool for use cases with optional sub-steps.

Cited references this lecture pulled from: <https://github.com/golang-migrate/migrate>, <https://pkg.go.dev/github.com/jackc/pgx/v5#Tx>, <https://pkg.go.dev/github.com/jackc/pgx/v5#BeginFunc>, <https://docs.sqlc.dev/en/latest/howto/transactions.html>, <https://www.postgresql.org/docs/current/sql-savepoint.html>.
