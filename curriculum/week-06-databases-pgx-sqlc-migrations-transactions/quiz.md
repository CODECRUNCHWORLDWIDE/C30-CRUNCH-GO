# Week 6 — Quiz

Ten multiple-choice questions covering `pgx`, `sqlc`, `golang-migrate`, transactions, isolation levels, concurrent-write hazards, and error translation. Treat the quiz as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — `sqlc` type checking

When does `sqlc` catch a query that references a column the schema does not have?

- (A) At runtime, on the first query execution.
- (B) At `sqlc generate` time — it type-checks every query against the schema and refuses to generate.
- (C) Never; `sqlc` trusts the query.
- (D) At `go build` time, via a struct tag.

## Question 2 — The connection pool

How should you use a `*pgxpool.Pool` in a service?

- (A) Create a new pool for each request.
- (B) Create one pool at startup, store it, and use it from every goroutine (it is concurrency-safe); size `MaxConns` against Postgres's `max_connections`.
- (C) Create one connection (not a pool) and share it with a mutex.
- (D) Create a pool per goroutine.

## Question 3 — Query annotations

A `sqlc` query annotated `-- name: GetNote :one` generates a method that, when no row matches, returns:

- (A) `(Note{}, nil)` — a zero note, no error.
- (B) `(Note{}, pgx.ErrNoRows)` — which the repository translates to a domain `ErrNotFound`.
- (C) `nil` — the method returns no error.
- (D) A panic.

## Question 4 — Migrations

What does `golang-migrate` track in the `schema_migrations` table?

- (A) Every row ever inserted.
- (B) The current schema version and a `dirty` flag, so `up` applies only un-applied migrations and a half-applied migration brakes further changes.
- (C) The connection string.
- (D) Nothing; it re-runs every migration every time.

## Question 5 — The down file rule

Why does C30 require every migration to ship with a `down` you have run at least once?

- (A) `golang-migrate` refuses to apply an `up` without a `down`.
- (B) Because a migration you cannot roll back is a migration you cannot safely undo when a deploy goes wrong — "we can't roll back" is a 3 AM incident.
- (C) The `down` makes the `up` faster.
- (D) It is only a convention, with no operational consequence.

## Question 6 — The transaction idiom

In `tx, _ := pool.Begin(ctx); defer tx.Rollback(ctx); ...; tx.Commit(ctx)`, why is the `defer tx.Rollback(ctx)` safe even on the success path?

- (A) It is not safe; you must remove it before `Commit`.
- (B) A `Rollback` after a successful `Commit` is a harmless no-op, so the one `defer` correctly handles the error path, the panic path, and the success path.
- (C) `defer` does not run after a `return`.
- (D) `Commit` cancels the deferred `Rollback`.

## Question 7 — `q.WithTx(tx)`

Why must `sqlc` queries inside a transaction use `q.WithTx(tx)`?

- (A) For type safety.
- (B) So the queries run on the transaction's connection; without it, a query acquires a *different* pooled connection and runs outside the transaction, defeating atomicity.
- (C) `WithTx` is optional sugar with no behavioural effect.
- (D) To enable `SERIALIZABLE`.

## Question 8 — The lost update

Two transactions read a counter (value 10), each compute 11, each write 11 under `READ COMMITTED`. The result is:

- (A) 12, because both increments applied.
- (B) 11 — one increment was lost, because both read 10 and the second write clobbered the first. The cheapest cure is an atomic `UPDATE counter SET n = n + 1`.
- (C) An error; Postgres rejects the second write.
- (D) Undefined; Postgres may return either 11 or 12.

## Question 9 — Write skew

Two transactions each check "at least one doctor remains on call" (both see 2), then each set a *different* doctor off call. The on-call count ends at zero. Which cure fixes this in general?

- (A) `SELECT ... FOR UPDATE` on the row each transaction writes.
- (B) An atomic in-place `UPDATE`.
- (C) `SERIALIZABLE` isolation with a `40001` serialization-failure retry loop, because the writes touch different rows and the hazard spans the read set.
- (D) A higher `MaxConns`.

## Question 10 — Error translation boundary

Where should `pgx.ErrNoRows` be translated into the domain's `ErrNotFound`?

- (A) In the HTTP handler.
- (B) In the service.
- (C) In the repository — the only layer that imports `pgx`; the service sees only its own sentinels, keeping it database-agnostic.
- (D) Nowhere; the service should import `pgx` and check `ErrNoRows` directly.

---

## Answer key

- **Q1: (B).** `sqlc generate` type-checks every query against the schema. A column typo fails generation with `column "x" does not exist` — caught before any Go compiles. Citation: <https://docs.sqlc.dev/>.
- **Q2: (B).** One concurrency-safe pool at startup, shared across goroutines; size `MaxConns` so `MaxConns × instances < max_connections`. A pool per request or a single shared connection are both wrong. Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>.
- **Q3: (B).** `:one` returns `pgx.ErrNoRows` when nothing matches; the repository translates it to `ErrNotFound`. Citation: <https://docs.sqlc.dev/en/latest/reference/query-annotations.html>.
- **Q4: (B).** `schema_migrations` holds the version and a `dirty` flag. `up` applies only un-applied migrations; a half-applied (`dirty`) migration brakes further changes until forced. Citation: <https://github.com/golang-migrate/migrate>.
- **Q5: (B).** A migration without a tested `down` cannot be safely rolled back. `golang-migrate` does *not* refuse an empty down (A is wrong) — the rule is operational discipline, not tool-enforced. Citation: <https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md>.
- **Q6: (B).** A rollback after a successful commit is a no-op (`pgx.ErrTxClosed`, ignored), so the single `defer` handles error, panic, and success paths. Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5#Tx>.
- **Q7: (B).** `WithTx` binds the queries to the transaction's connection. Without it, a query runs on a different pooled connection, outside the transaction — so a rollback would not undo it. Citation: <https://docs.sqlc.dev/en/latest/howto/transactions.html>.
- **Q8: (B).** Both read 10, both write 11; one increment is lost. The cheapest cure is the atomic in-place `UPDATE n = n + 1` (no read-write gap). Citation: <https://www.postgresql.org/docs/current/transaction-iso.html>.
- **Q9: (C).** Write skew spans different rows, so a row lock on each write does not catch it. `SERIALIZABLE` detects the dependency and aborts one with `40001`, which you must retry. Citation: <https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE>.
- **Q10: (C).** The repository is the only layer that imports `pgx`; it translates `pgx.ErrNoRows`/`23505`/`40001` into domain sentinels. The service stays database-agnostic — the Week 5 seam. Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError>.

## Self-assessment

- 9-10: you can port `notes-api` to Postgres and defend every transaction boundary and hazard cure.
- 7-8: re-read the lecture notes on the questions you missed; the citations point to the exact docs.
- 5-6: re-read all three lecture notes and redo the exercises, especially Exercise 3 (the lost update).
- 0-4: rewind to Lecture 1. The mini-project depends on the `sqlc` workflow, the transaction idiom, and the hazard taxonomy being second nature.
