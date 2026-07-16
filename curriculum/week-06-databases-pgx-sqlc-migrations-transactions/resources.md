# Week 6 — Resources

Every resource on this page is **free**. The Go package docs on `pkg.go.dev` are free and account-free. The `sqlc` docs, the `golang-migrate` repo, and the `testcontainers-go` docs are public. The PostgreSQL manual is free. `pgx`, `sqlc`, `golang-migrate`, and `testcontainers-go` are all open source (MIT or similar). No paywalled material is linked.

## Required reading (work it into your week)

### `pgx` and the connection pool

- **`pgx/v5` package documentation** — the driver, `Conn`, `Tx`, `Batch`, `CopyFrom`, the query methods:
  <https://pkg.go.dev/github.com/jackc/pgx/v5>
- **`pgxpool` package** — `New`, `NewWithConfig`, `ParseConfig`, `Pool`, `MaxConns`, `Acquire`/`Release`:
  <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>
- **`pgconn.PgError`** — the structured Postgres error with the SQLSTATE `Code` field you classify on:
  <https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError>
- **The `pgx` repository and wiki** — the getting-started guide, the FAQ, the `database/sql`-vs-`pgx` discussion:
  <https://github.com/jackc/pgx>

### `sqlc`

- **`sqlc` documentation home** — the overview, the supported engines, the philosophy:
  <https://docs.sqlc.dev/>
- **Getting started with PostgreSQL (sqlc tutorial)** — the schema → query → generate loop, end to end:
  <https://docs.sqlc.dev/en/latest/tutorials/getting-started-postgresql.html>
- **Query annotations reference** — `:one`, `:many`, `:exec`, `:execrows`, `:batchexec`, `:copyfrom`:
  <https://docs.sqlc.dev/en/latest/reference/query-annotations.html>
- **Transactions with sqlc** — the `WithTx` pattern for running generated queries inside a transaction:
  <https://docs.sqlc.dev/en/latest/howto/transactions.html>
- **The `sqlc.yaml` configuration reference** — `engine`, `schema`, `queries`, the `gen.go` options, `sql_package: pgx/v5`:
  <https://docs.sqlc.dev/en/latest/reference/config.html>

### Migrations

- **`golang-migrate` repository** — the CLI, the library, the supported databases and sources:
  <https://github.com/golang-migrate/migrate>
- **Migration file format and best practices** — the `up`/`down` pair, the sequence numbering, the `schema_migrations` table:
  <https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md>
- **Using migrate in your Go project** — the library API (`migrate.New`, `m.Up`, `m.Down`), the source/database driver imports:
  <https://github.com/golang-migrate/migrate#use-in-your-go-project>

### Transactions and isolation

- **PostgreSQL: Transaction Isolation** — the authoritative reference for `READ COMMITTED`, `REPEATABLE READ`, `SERIALIZABLE`, and the anomalies each allows:
  <https://www.postgresql.org/docs/current/transaction-iso.html>
- **PostgreSQL: Serializable isolation** — the SSI implementation, the `40001` serialization failure, the retry requirement:
  <https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE>
- **PostgreSQL: Concurrency Control (MVCC) overview** — how Postgres manages concurrent access:
  <https://www.postgresql.org/docs/current/mvcc.html>
- **PostgreSQL: `SELECT ... FOR UPDATE`** — the row-locking clause for the pessimistic lost-update cure:
  <https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE>
- **PostgreSQL: error codes (SQLSTATE) appendix** — `23505` unique, `23503` foreign key, `40001` serialization failure:
  <https://www.postgresql.org/docs/current/errcodes-appendix.html>

### Reading and planning queries

- **PostgreSQL: Using `EXPLAIN`** — how to read a query plan; index scan vs sequential scan; `EXPLAIN ANALYZE`:
  <https://www.postgresql.org/docs/current/using-explain.html>
- **PostgreSQL: `COPY`** — the bulk-load command underneath `pgx.CopyFrom` (Challenge 2):
  <https://www.postgresql.org/docs/current/sql-copy.html>

### Integration testing

- **`testcontainers-go` documentation** — spin up ephemeral containers from a Go test:
  <https://golang.testcontainers.org/>
- **`testcontainers-go` Postgres module** — the `postgres.Run` helper, wait strategies, the connection string:
  <https://golang.testcontainers.org/modules/postgres/>

### Cancellation (recap from Weeks 4–5)

- **`context` package** — every `pgx`/`sqlc` query takes a `ctx`; a deadline cancels the query server-side:
  <https://pkg.go.dev/context>

## Recommended reading (after the required set)

- **PostgreSQL: Data Definition** — tables, constraints (`UNIQUE`, `CHECK`, foreign keys), indexes; the schema vocabulary your migrations use:
  <https://www.postgresql.org/docs/current/ddl.html>
- **Use the Index, Luke** — the canonical free book on database indexing and why a query is slow:
  <https://use-the-index-luke.com/>
- **The `pgx` performance and architecture wiki** — connection pooling, prepared statements, the binary protocol:
  <https://github.com/jackc/pgx/wiki>
- **Designing Data-Intensive Applications, ch. 7 (Transactions)** — the definitive treatment of isolation levels and write anomalies (the book is paid, but the chapter's concepts are summarised free in many talks):
  <https://www.postgresql.org/docs/current/transaction-iso.html> (the Postgres manual covers the same ground free)
- **`sqlc` playground** — try the schema → query → generate loop in the browser before installing anything:
  <https://play.sqlc.dev/>

## Tools you will install this week

- **`github.com/jackc/pgx/v5`** — added per-module: `go get github.com/jackc/pgx/v5@latest`. MIT-licensed. The driver, pool, and error types.
- **`sqlc`** — `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` (or Homebrew / the Docker image). Verify with `sqlc version`. Generates the typed query layer.
- **`golang-migrate`** — `go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest` (or Homebrew / a release binary). Verify with `migrate -version`. Applies migrations.
- **`github.com/testcontainers/testcontainers-go`** — added per-module for the integration tests: `go get github.com/testcontainers/testcontainers-go/modules/postgres@latest`. MIT-licensed.
- **Postgres via Docker** — `docker run -e POSTGRES_PASSWORD=devpass -p 5432:5432 -d postgres:16`. Verify with `docker exec <id> pg_isready`.
- **`psql`** (optional) — the Postgres CLI, for `\d`, `EXPLAIN`, and poking the database directly.

## Citations policy

This curriculum cites the Go package documentation on `pkg.go.dev`, the `sqlc` and `golang-migrate` project documentation, the `testcontainers-go` docs, and the PostgreSQL manual as the primary references. Every example in the lecture notes and exercises is traced back to one of these. When a third-party reference (Use the Index Luke, the pgx wiki) is the clearer source, it is cited explicitly with a URL — never paraphrased without attribution. If a citation is missing from a section of these notes, treat it as a bug and open an issue against the C30 curriculum repository.
