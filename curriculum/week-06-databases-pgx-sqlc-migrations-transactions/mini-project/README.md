# Mini-Project — `notes-api` on Postgres: `pgx` + `sqlc`, Reversible Migrations, a Multi-Step Transaction, and Integration Tests

> **Time:** ~9.5 hours across Friday-Saturday-Sunday. **Prerequisites:** Exercises 1-3, ideally both challenges, and Week 5's `notes-api`. **Citations:** every package doc referenced in the three lecture notes, plus the Postgres transaction-isolation manual.

## The spec

You are taking **Week 5's `notes-api`** and replacing its in-memory `MemRepo` with a Postgres-backed `PgRepo` built on `pgx` + `sqlc` — *without changing anything above the repository.* The handler, the service, the middleware, the wire structs, the status-code map, and the handler/service tests against a fake all stay exactly as they were. The proof of the seam is `git diff` on the service package: it should be empty.

```
   (unchanged from Week 5)
   Handler -> Service -> Repository (interface)
                              |
                              +--- MemRepo   (Week 5, kept for fast service tests)
                              |
                              +--- PgRepo    (THIS WEEK)
                                      |
                                      +-- *pgxpool.Pool
                                      +-- *db.Queries  (sqlc-generated)
                                      |
                                      v
                              +---------------------+
                              |  Postgres 16         |
                              |  golang-migrate      |  up / down
                              |  multi-step tx       |  note + audit, atomic
                              +---------------------+
```

## Functional requirements

### F1 — the `pgx` pool

- A `*pgxpool.Pool` constructed at startup from `DATABASE_URL` (env, never hard-coded), with a deliberately-set `MaxConns`, and a `Ping` at boot that fails fast on an unreachable database.
- The pool is closed on graceful shutdown (the Week 5 shutdown wiring, extended).

### F2 — `sqlc` queries

- A `query.sql` with `CreateNote :one`, `GetNote :one`, `ListNotes :many` (paginated), `UpdateNote :one`, `DeleteNote :execrows`, plus an `InsertAudit :exec`.
- `sqlc.yaml` targeting `pgx/v5`; `sqlc generate` produces the typed `Queries`.
- The generated code is committed (so reviewers see it) and re-generable (`sqlc generate` is clean).

### F3 — the `PgRepo`

- Implements the *same* `notes.Repository` interface as `MemRepo`.
- Translates `pgx.ErrNoRows` → `notes.ErrNotFound`, a `23505` `PgError` → `notes.ErrConflict`, and a 0-row `DeleteNote` → `notes.ErrNotFound`.
- Maps the generated `db.Note` row to the domain `notes.Note` in a `toDomain` helper. No `pgx` type escapes the repository.

### F4 — reversible migrations

- A `migrations/` directory with at least two `up`/`down` pairs: create `notes`, then add an `audit` table (with a foreign key to `notes`).
- `migrate up` applies them; `migrate down` reverses them cleanly; a `migrate down` then re-`up` succeeds (the test of your `down` files).

### F5 — a multi-step transaction

- A `CreateWithAudit(ctx, note, actor)` repository method that inserts the note *and* an audit row in **one** transaction (`pool.Begin` + `defer Rollback` + `q.WithTx(tx)` + `Commit`, or `pgx.BeginFunc`).
- A forced mid-transaction failure (e.g. an audit `CHECK` violation) rolls back the note too — proven by a test that finds zero rows after.

### F6 — `context` deadlines

- Every repository method threads `ctx` into the query. A request budget (the Week 5 timeout middleware) cancels a slow query server-side.

### F7 — a concurrent-write hazard, handled

- The `UpdateNote` path (or a counter/view-count field) is exposed to a lost-update or write-skew hazard, and you close it — atomic in-place `UPDATE`, `SELECT ... FOR UPDATE`, an optimistic version column, or `SERIALIZABLE` with a `40001` retry. Document which hazard and which cure in `PERF.md`.

### F8 — the swap is invisible above the repository

- Switching `main` from `notes.NewService(notes.NewMemRepo())` to `notes.NewService(postgres.NewPgRepo(pool))` is the *only* change above the repository. `git diff internal/notes/service.go internal/http/` is empty.

## Non-functional requirements

### NF1 — integration tests against a real Postgres

- Repository tests run against a real Postgres via `testcontainers-go` (or a `docker compose` harness): apply migrations, run tests, tear down.
- Tests cover: CRUD round-trip, the `23505`→`ErrConflict` translation, the `CreateWithAudit` atomicity (forced failure leaves nothing), the chosen hazard cure under concurrency, and a `migrate down`/re-`up`.
- `go test -race ./...` is green (the concurrency tests must be race-free).

### NF2 — code quality

- File-scoped, small functions; the repository is the only package importing `pgx`.
- `sqlc generate && go vet ./... && staticcheck ./...` all clean.
- Every repository method takes `ctx` first.

### NF3 — citations

- Every non-obvious choice has a one-line comment citing the relevant doc.
- `README.md` lists `github.com/jackc/pgx/v5`, `sqlc`, `golang-migrate`, and `testcontainers-go` with versions and licenses.

## Suggested project layout

```
notes-api/
├── go.mod
├── README.md            <-- build, migrate, run, the DSN
├── PERF.md              <-- the query-latency + hazard write-up
├── sqlc.yaml
├── docker-compose.yml   <-- Postgres for local dev
├── migrations/
│   ├── 000001_create_notes.up.sql
│   ├── 000001_create_notes.down.sql
│   ├── 000002_create_audit.up.sql
│   └── 000002_create_audit.down.sql
├── cmd/notesapi/
│   └── main.go          <-- pool, migrations check, server, shutdown
└── internal/
    ├── notes/           <-- UNCHANGED from Week 5 (domain, service, interface, MemRepo)
    ├── db/              <-- sqlc-generated (query.sql + generated *.go)
    │   ├── query.sql
    │   ├── db.go
    │   ├── models.go
    │   └── query.sql.go
    ├── postgres/        <-- the PgRepo + error translation + the transaction
    │   ├── repo.go
    │   └── repo_test.go  <-- testcontainers integration tests
    └── http/            <-- UNCHANGED from Week 5
```

## Starter

A starter scaffold is provided in `mini-project/starter/`. Copy it as your starting point:

- `internal/notes/` — the Week 5 service, interface, and `MemRepo`, complete and unchanged.
- `sqlc.yaml` and `internal/db/query.sql` — the config and the queries, complete; you run `sqlc generate`.
- `migrations/000001_*` — the notes migration pair, complete; you author `000002` (audit).
- `internal/postgres/repo.go` — the `PgRepo` struct and the `Get`/`Create` methods, complete; `Update`, `Delete`, `List`, and `CreateWithAudit` are stubs.
- `internal/postgres/repo_test.go` — the `testcontainers` harness, complete; the test bodies are stubs.

The starter compiles after `sqlc generate`; the stubbed methods return `errors.New("not implemented")` until you fill them in.

## The perf write-up (`PERF.md`)

Treat it as part of the deliverable.

### M1 — query latency

Against the container, time a `GetNote` by primary key and a `ListNotes` page. Paste the `EXPLAIN ANALYZE` for each (the generated SQL is in `query.sql.go`). Confirm `GetNote` is an index scan on the primary key. Report the latency.

### M2 — the transaction, proven atomic

Paste the test output showing `CreateWithAudit` with a forced failure leaves zero rows in both tables. State the SQL `CHECK` (or other constraint) you used to force the failure.

### M3 — migrate down and re-up

Paste the `migrate down` then `migrate up` output (or the test) showing both succeed. This is the test of your `down` files.

### M4 — the hazard and its cure

State which concurrent-write hazard your service is exposed to (lost update / write skew), reproduce it under concurrency (the bug), apply your cure, and show the corrected behaviour. Report the cure you chose and why it is the cheapest correct one for this case.

### M5 — pool saturation

Drive concurrent requests above `MaxConns` and observe queueing (requests wait for a connection). Report the latency at `MaxConns`, `2×MaxConns`, and `4×MaxConns` concurrent requests. One sentence: how would you size `MaxConns` for your expected load?

## Grading rubric

- **35 points: functional correctness.** F1-F8 implemented; the full CRUD surface works on Postgres; the swap above the repository is invisible (`git diff` empty).
- **20 points: the transaction + the hazard cure.** `CreateWithAudit` is atomic (proven by a test); the concurrent-write hazard is named and correctly cured.
- **15 points: migrations.** Reversible `up`/`down` pairs; the `down` is demonstrated; the FK-aware drop order is correct.
- **15 points: integration tests.** `testcontainers` (or compose) tests cover CRUD, conflict, atomicity, the hazard, and migrate down/up; `go test -race` green.
- **10 points: the perf write-up.** All five measurements (M1-M5) with real numbers and `EXPLAIN` output.
- **5 points: citations.** At least eight distinct citations in the source pointing at the package docs or the Postgres manual.

## Stretch goals

1. **`sqlc` with `pgx.Batch`.** Add a bulk-import endpoint (`POST /v1/notes:import`) that loads many notes in one batched transaction (Challenge 2). Report the throughput vs row-by-row.
2. **A read replica.** Configure a second pool pointed at a (simulated) read replica, route `GET`/`List` to it and writes to the primary, and discuss the read-your-writes consistency hazard that introduces.
3. **Optimistic concurrency end to end.** Add a `version` column to `notes`, return it as an `ETag` (Week 5's stretch), require `If-Match` on `PATCH`, and translate a 0-row conditional `UPDATE` into a 412 (Precondition Failed). This connects Week 5's HTTP concurrency control to Week 6's database concurrency control.

## Submission

Push the project on a branch named `week06-mini-project/<your-handle>` and open a PR against the C30 curriculum repository. The PR description must link to `PERF.md`, paste the green `go test -race ./...` line, and paste the empty `git diff internal/notes/` showing the service was untouched.

The teaching staff reviews mini-project PRs within 7 business days. Reviews focus on: (a) whether the eight functional requirements are met, (b) whether the seam genuinely held (the service must be byte-for-byte unchanged), (c) whether the transaction is atomic and the hazard is correctly cured, and (d) whether the integration tests run against a real Postgres.

## The Phase II checkpoint

This `notes-api`-on-Postgres is the foundation Week 7 (gRPC) and Week 8 (testing/fuzzing) build on, and the Phase II gate (Week 8) demos. Keep it clean: the gRPC twin in Week 7 will reach for the *same* service and repository you have here.

Cited references: every page referenced in the three lecture notes, plus <https://www.postgresql.org/docs/current/transaction-iso.html>, <https://pkg.go.dev/github.com/jackc/pgx/v5>, <https://docs.sqlc.dev/>, <https://github.com/golang-migrate/migrate>, <https://golang.testcontainers.org/>.
