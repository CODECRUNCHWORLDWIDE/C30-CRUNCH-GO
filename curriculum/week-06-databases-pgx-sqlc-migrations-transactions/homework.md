# Week 6 — Homework

Six practice problems that consolidate the week's material. They are sized to ~45 minutes each. Do them after the lectures and the exercises; do them before the mini-project. Cite the URLs you used while solving each one in the commit message of your homework branch.

## Problem 1 — `sqlc` vs an ORM, defended

Write a 300-word post arguing the `sqlc`-over-ORM choice for a service you operate under load, then steel-man the other side. Cover:

1. What `sqlc` gives you that a query-builder ORM does not (legible SQL, generate-time type checking, no surprise N+1, no reflection).
2. What you give up (you write SQL by hand; no automatic association loading).
3. The one scenario where an ORM's productivity genuinely wins (rapid CRUD admin with trivial, uniform queries).
4. How you would `EXPLAIN` a slow `sqlc` query — and why that is harder with an ORM that generates the SQL for you.

Cite the `sqlc` docs at <https://docs.sqlc.dev/> and the Postgres `EXPLAIN` docs at <https://www.postgresql.org/docs/current/using-explain.html>.

Deliverable: `homework/01-sqlc-vs-orm.md`.

## Problem 2 — Author a migration pair, and break it

Write an `up`/`down` migration pair that (a) creates a `users` table, then a second pair that (b) adds a `users.email` column with a `UNIQUE` constraint and an index. Then:

1. Apply both with `migrate up`. Confirm with `\d users` in `psql`.
2. Roll back the second with `migrate down 1`. Confirm the column and index are gone.
3. Deliberately write a *bad* `down` for the second migration (one that does not drop the index). Apply up, then down, and observe what `migrate up` does next — does it cleanly re-apply, or does the leftover index cause a conflict? Document what you saw.
4. Fix the `down` and confirm a clean down/up cycle.

Write 150 words on why "every migration has a `down` you have run at least once" is a hard rule, not a nicety.

Cite the `golang-migrate` migrations guide at <https://github.com/golang-migrate/migrate/blob/master/MIGRATIONS.md>.

Deliverable: `homework/02-migration-pair.md` with the four SQL files and the observations.

## Problem 3 — Trace a transaction's atomicity

Take the `CreateWithAudit` pattern. In a small program, instrument it to print after each step (begin, note insert, audit insert, commit/rollback). Run it twice:

1. Success path: both inserts succeed, commit. Query the database and confirm both rows exist.
2. Failure path: force the audit insert to fail. Query the database and confirm *neither* row exists.

Then answer:

- Why does `defer tx.Rollback(ctx)` correctly handle both the error path and the success path?
- What would happen if the audit insert ran on `pool.Exec` instead of `tx.Exec` (or `q.WithTx(tx)`)? Draw the failure.
- Where would you put the transaction boundary for "import 1000 notes from a file," and what is the trade-off of one-big-tx vs one-per-note?

Cite <https://pkg.go.dev/github.com/jackc/pgx/v5#Tx> and the transaction tutorial at <https://www.postgresql.org/docs/current/tutorial-transactions.html>.

Deliverable: `homework/03-transaction-trace.md`.

## Problem 4 — Name the hazard

For each of the following use cases, name the concurrent-write hazard (lost update, write skew, or none) and the cheapest correct cure (atomic `UPDATE`, `FOR UPDATE`, version column, or `SERIALIZABLE`):

- **A:** Incrementing a post's view counter on every page load.
- **B:** "Withdraw $X if the balance is sufficient" — two withdrawals racing on one account.
- **C:** A doctor-rota rule "at least one doctor must remain on call" — two doctors leaving concurrently.
- **D:** Reserving the last seat on a flight — two bookings racing.
- **E:** Appending a row to an audit log (no read, just an insert).
- **F:** "Set the user's display name to X" — two updates from the same user racing.

For each, one sentence naming the hazard and one naming the cure. Then explain in a paragraph why write skew (C) needs `SERIALIZABLE` while the others can be cured more cheaply.

Cite the Postgres transaction-isolation manual at <https://www.postgresql.org/docs/current/transaction-iso.html>.

Deliverable: `homework/04-name-the-hazard.md`.

## Problem 5 — Error translation, exhaustively

Build a small repository against a real Postgres with a table that has a primary key, a `UNIQUE` constraint, and a foreign key. Deliberately trigger each of the following errors and report the SQLSTATE code and the domain error you would translate it to:

1. Selecting a row that does not exist (`pgx.ErrNoRows`).
2. Inserting a duplicate primary key or unique value (`23505`).
3. Inserting a row that violates a foreign key (`23503`).
4. A `SERIALIZABLE` serialization failure (`40001`) — run two conflicting transactions.

For each, paste the error `pgx` returns, the SQLSTATE you extracted with `errors.As`/`pgconn.PgError`, and the domain sentinel you mapped it to. Then write 150 words on why this translation belongs in the repository and not the service.

Cite <https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError> and the SQLSTATE list at <https://www.postgresql.org/docs/current/errcodes-appendix.html>.

Deliverable: `homework/05-error-translation.md`.

## Problem 6 — Integration test with `testcontainers-go`

Write a `testcontainers-go` integration test for a small repository: spin up an ephemeral Postgres, apply your migrations, run a CRUD round-trip and a conflict case, and tear down. Then:

1. Time the test (it will be seconds, not microseconds). Report the container-startup cost.
2. Explain why this test belongs at the *top* of the testing pyramid (few, slow) and what the service/handler tests below it use instead (a fake — no database).
3. Show that the test is hermetic: run it twice and confirm no leftover state (the container is fresh each run).

Write 150 words on the trade-off between `testcontainers` (a container per test run, hermetic, slower) and a shared `docker compose` Postgres (faster, but you must reset state yourself).

Cite `testcontainers-go` at <https://golang.testcontainers.org/modules/postgres/>.

Deliverable: `homework/06-testcontainers.md` with the test and the timing.

## Submission

Push the six deliverables on a branch named `week06-homework/<your-handle>` and open a PR against the C30 curriculum repository. The PR description should link to each of the six files and include a 100-word summary of what you learned.

The teaching staff reviews homework PRs within 5 business days. Reviews focus on whether you have read the citations and whether your reasoning holds together. The single most common review comment is "where is your citation for this claim" — preempt it by linking the package doc or the Postgres manual section for every non-trivial assertion.

Cited references this homework draws from: <https://docs.sqlc.dev/>, <https://github.com/golang-migrate/migrate>, <https://pkg.go.dev/github.com/jackc/pgx/v5>, <https://pkg.go.dev/github.com/jackc/pgx/v5/pgconn#PgError>, <https://www.postgresql.org/docs/current/transaction-iso.html>, <https://www.postgresql.org/docs/current/errcodes-appendix.html>, <https://golang.testcontainers.org/>.
