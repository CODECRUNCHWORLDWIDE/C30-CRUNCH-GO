# Week 6 — Exercise Solutions and Annotations

These are the worked solutions for the three exercises. Read them after attempting the exercises, not before. Every SQL block has been run against `postgres:16`; the Go has been built `go vet`-clean. The transcripts are from real runs.

## Exercise 1 — pgx Pool + sqlc

### The files you reproduce

`sqlc.yaml`:

```yaml
version: "2"
sql:
  - engine: "postgresql"
    schema: "migrations"
    queries: "internal/db/query.sql"
    gen:
      go:
        package: "db"
        out: "internal/db"
        sql_package: "pgx/v5"
        emit_json_tags: true
```

`migrations/000001_create_notes.up.sql`:

```sql
CREATE TABLE notes (
    id         TEXT        PRIMARY KEY,
    title      TEXT        NOT NULL,
    body       TEXT        NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

`internal/db/query.sql`:

```sql
-- name: CreateNote :one
INSERT INTO notes (id, title, body) VALUES ($1, $2, $3) RETURNING *;

-- name: GetNote :one
SELECT * FROM notes WHERE id = $1;
```

### What success looks like

```
$ sqlc generate          # produces internal/db/{db.go, models.go, query.sql.go}
$ go run .
pool ready; wire in db.New(pool) after `sqlc generate`
note not found
note already exists
```

The last two lines are `classifyConflict` translating `pgx.ErrNoRows` and a `23505` `PgError` into the domain sentinels — the translation logic in isolation.

### Why the column-typo-is-a-generate-error claim holds

Rename `body` to `content` in the migration but leave `query.sql` using `body`, then `sqlc generate`:

```
$ sqlc generate
# package:
internal/db/query.sql:1:1: column "body" does not exist
```

`sqlc` type-checked the query against the schema and refused to generate. The bug is caught at generate time, before any Go compiles — the whole point of `sqlc` over a stringly-typed driver.

### Why translation lives in the repository

The service knows only `ErrNotFound` and `ErrConflict`; it never imports `pgx` or `pgconn`. The repository is the single place that knows `pgx.ErrNoRows` means "not found" and SQLSTATE `23505` means "conflict." Move that translation up into the service and the service is no longer database-agnostic — and the seam from Week 5 is a lie.

### Common pitfalls

1. **Using `==` to compare errors.** `err == pgx.ErrNoRows` works until a layer wraps the error; `errors.Is(err, pgx.ErrNoRows)` keeps working. Same for the `PgError` — use `errors.As(err, &pgErr)`, never a type assertion that ignores wrapping.
2. **Opening a connection per query.** Open one `*pgxpool.Pool` at startup and reuse it. The pool is concurrency-safe; a connection per query exhausts Postgres.
3. **Forgetting `defer pool.Close()`.** The pool holds connections; close it on shutdown so Postgres reclaims the backends.

## Exercise 2 — Migrations + a Multi-Step Transaction

### What success looks like

```
$ migrate -path migrations -database "$DATABASE_URL" up
$ go run .
ok-1 exists after success: true
failing call returned error: true
bad-1 exists after rollback: false       # <- the proof: the note rolled back with the audit
$ migrate -path migrations -database "$DATABASE_URL" down
# both tables dropped cleanly — your down files work
```

### Why `bad-1` must not exist

`createWithAudit` runs both inserts on the *same* transaction. When the audit insert hits the `CHECK (length(actor) <= 64)` violation (the 100-char actor), the function returns an error before `tx.Commit`. The deferred `tx.Rollback(ctx)` then undoes *everything* the transaction did — including the note insert that succeeded moments earlier. So `bad-1` never persists. That is atomicity: the note and the audit row commit together or not at all.

### Why `defer tx.Rollback(ctx)` is safe even on the success path

After `tx.Commit(ctx)` succeeds, the transaction is closed. The deferred `Rollback` then runs and finds nothing to roll back — `pgx` returns `pgx.ErrTxClosed`, which the deferred call ignores (we do not check its error). So the one `defer` line correctly handles the error path, the panic path, *and* the success path with no special-casing.

### Why both inserts must use the same `tx`

If the audit insert ran on `pool.Exec` instead of `tx.Exec`, it would acquire a *different* pooled connection and run *outside* the transaction — so a rolled-back note would leave an orphan audit row (or vice versa). Both statements must run on the transaction handle (`tx`, or `q.WithTx(tx)` for sqlc) for atomicity to hold.

### Common pitfalls

1. **Calling `Commit` inside the loop / forgetting it entirely.** No `Commit` means the deferred `Rollback` discards everything — the success path silently writes nothing. The explicit `Commit` is the only thing that persists.
2. **An empty `down` file.** `migrate down` then does nothing, the schema is not reset, and your "tested down" is a no-op. The `down` must actually reverse the `up`.
3. **`DROP TABLE notes` before `DROP TABLE audit` in the down.** `audit` has a foreign key to `notes`; drop the referencing table (`audit`) first. The down undoes the up in *reverse* dependency order.

## Exercise 3 — Lost Update and the Cure

### What success looks like

```
$ go run . racy
mode=racy       want=50 got=37  LOST UPDATES

$ go run . forupdate
mode=forupdate  want=50 got=50  OK

$ go run . version
mode=version    want=50 got=50  OK
```

The `racy` run loses ~13 increments (the exact number varies) because 50 goroutines read-then-write with no lock. Both cures recover the correct count of 50.

### Why `racy` loses updates

Each goroutine: `SELECT n` (reads, say, 10), then `UPDATE SET n = 11`. Under `READ COMMITTED`, many goroutines read the same `n` and all write `n+1` to the same value — the increments collapse. This is the SQL twin of Week 4's `counter++` data race: a read-modify-write with no synchronisation.

### Why `FOR UPDATE` fixes it

`SELECT n FROM counter WHERE id=1 FOR UPDATE` takes a row-level write lock. The second goroutine's `SELECT ... FOR UPDATE` *blocks* until the first commits, then reads the already-incremented value. The increments serialise; none is lost. The cost: goroutines queue on the lock, so throughput drops under heavy contention — the trade for correctness-by-blocking.

### Why the version column fixes it

`UPDATE ... WHERE id=1 AND version=$old` succeeds only if no one changed `version` since the read. If another goroutine won the race, the `WHERE version=$old` matches zero rows, `RowsAffected()` is 0, and the loop re-reads and retries with the new version. No locks are held across the read-think-write gap — the optimistic approach. The cost: retries under contention (here, frequent, because all 50 hammer one row; in a real low-contention workload, retries are rare).

### Which cure to ship

For a bare counter where you do not need the value in Go, the cheapest correct cure is neither of these — it is the atomic in-place `UPDATE counter SET n = n + 1 WHERE id=1` (no separate SELECT, no gap). Use `FOR UPDATE` when you must read-and-decide in Go and prefer to block; use the version column when contention is low and you cannot hold a lock across a long gap (a user editing a form).

### Common pitfalls

1. **Expecting `racy` to lose updates *every* run.** On a slow machine the goroutines may serialise by accident and produce 50. Run it a few times; under real concurrency it loses updates most runs. The hazard is *possible*, which is enough to be a bug.
2. **Forgetting the retry loop in the version cure.** A single conditional `UPDATE` without the retry just *fails silently* (0 rows) when it loses the race — you must loop and re-read.
3. **Holding `FOR UPDATE` across a slow operation.** A `FOR UPDATE` lock is held until commit; if you do slow work (an HTTP call) between the locked `SELECT` and the `COMMIT`, every other writer blocks behind you. Lock, write, commit — fast.

## Cross-cutting notes

- **Thread `ctx` into every query.** A query without a deadline can hang a pool connection forever when the database is slow.
- **The repository translates; the service stays clean.** `pgx.ErrNoRows`/`23505`/`40001` and the `db.Note` row type all stop at the repository boundary.
- **Every migration has a tested `down`.** The `migrate down` and re-`up` is the test.
- **Name the hazard, then pick the cheapest cure.** Atomic `UPDATE` < `FOR UPDATE` < version column < `SERIALIZABLE`. Reach up the ladder only when the hazard demands it.

Cited references: <https://pkg.go.dev/github.com/jackc/pgx/v5>, <https://docs.sqlc.dev/>, <https://github.com/golang-migrate/migrate>, <https://www.postgresql.org/docs/current/transaction-iso.html>, <https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE>.
