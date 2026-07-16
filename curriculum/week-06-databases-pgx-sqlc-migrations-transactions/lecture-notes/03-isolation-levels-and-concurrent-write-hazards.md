# Lecture 3 — Isolation Levels, Concurrent-Write Hazards, and Integration Testing Against a Real Postgres

> **Time:** 2 hours. Take the isolation-and-hazards material in one sitting and the integration-testing material in a second. **Prerequisites:** Lectures 1 and 2. **Citations:** the Postgres transaction-isolation manual at <https://www.postgresql.org/docs/current/transaction-iso.html>, the `SELECT ... FOR UPDATE` docs at <https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE>, and `testcontainers-go` at <https://golang.testcontainers.org/>.

## 1. Why isolation is your problem, not just the database's

A single statement against Postgres is atomic and isolated for free. The trouble starts when *two transactions touch the same data at the same time.* Postgres's default isolation level, `READ COMMITTED`, prevents some anomalies (you never read another transaction's *uncommitted* changes) but explicitly allows others — and those others are bugs that only appear under concurrency, which means they pass every single-threaded test and surface in production under load. The senior skill this lecture builds: *look at a use case and name the concurrent-write hazard it carries, then close it.* You do not need to memorise the SQL standard's full anomaly table; you need to recognise the lost update and write skew, and know the three cures.

Postgres offers three isolation levels you will actually use:

- **`READ COMMITTED`** (the default) — each statement sees data committed before *that statement* began. Prevents dirty reads; allows lost updates and write skew.
- **`REPEATABLE READ`** — the whole transaction sees a single consistent snapshot taken at its start. Prevents non-repeatable reads; in Postgres it also detects and aborts some write conflicts.
- **`SERIALIZABLE`** — the transaction behaves *as if* it ran with no other transaction concurrently; Postgres detects any serialization anomaly and aborts one of the conflicting transactions with a `40001` error you must retry.

Citation: <https://www.postgresql.org/docs/current/transaction-iso.html>.

## 2. The lost update — the most common hazard

The shape: two transactions read the same row, each computes a new value from what it read, and each writes it back. The second write overwrites the first — one update is *lost*.

A concrete example: incrementing a counter with read-then-write (the SQL analogue of Week 4's `counter++` race):

```sql
-- Transaction A                          -- Transaction B
BEGIN;                                     BEGIN;
SELECT views FROM posts WHERE id=1;        SELECT views FROM posts WHERE id=1;
-- both read views = 10                    -- both read views = 10
UPDATE posts SET views = 11 WHERE id=1;    UPDATE posts SET views = 11 WHERE id=1;
COMMIT;                                    COMMIT;
-- final views = 11, but TWO increments happened. One was lost.
```

Under `READ COMMITTED`, both transactions read 10, both write 11, and the second `UPDATE` silently clobbers the first. The view count should be 12; it is 11. This is the lost update, and it is the single most common data-corruption-under-concurrency bug. The three cures:

### Cure 1 — `SELECT ... FOR UPDATE` (pessimistic row lock)

Take a row lock on the read, so the second transaction *waits* until the first commits, then reads the already-incremented value:

```sql
BEGIN;
SELECT views FROM posts WHERE id=1 FOR UPDATE;  -- locks the row
-- transaction B's SELECT ... FOR UPDATE now BLOCKS until A commits
UPDATE posts SET views = views + 1 WHERE id=1;
COMMIT;
```

The `FOR UPDATE` makes the second reader block until the first commits, serialising the two increments. Correct, and the right tool when contention is moderate and you want to *block* rather than *retry*.

### Cure 2 — the atomic in-place update

Avoid read-then-write entirely: compute the new value *in the UPDATE* so there is no gap:

```sql
UPDATE posts SET views = views + 1 WHERE id=1;  -- no separate SELECT
```

`views = views + 1` is computed by Postgres under the row's lock during the `UPDATE` itself — there is no window between read and write for another transaction to slip into. This is the cleanest cure when the new value is a function of the old one (a counter, a balance adjustment) and you do not need to *see* the value in Go first.

### Cure 3 — the optimistic version column

Add a `version` column, read it, and make the write *conditional* on the version not having changed:

```sql
-- read: SELECT views, version FROM posts WHERE id=1;  -> views=10, version=7
UPDATE posts SET views=11, version=8 WHERE id=1 AND version=7;
-- if another transaction already bumped version to 8, this UPDATE affects 0 rows.
```

If the `UPDATE` affects **zero** rows, someone else won the race — you re-read and retry. This is the database analogue of Week 5's `If-Match`/`ETag` optimistic-concurrency stretch goal. It is the right cure when contention is *low* (retries are rare) and you want no locks held across the read-think-write gap (e.g. the user is editing a form). Citation: <https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE>.

## 3. Write skew — the subtler hazard

Write skew is nastier because each transaction's write is individually fine; it is the *combination* that violates an invariant. The classic example: an on-call rota that must always have at least one doctor on call.

```
Invariant: at least one doctor must remain on call.
Currently: Alice and Bob are both on call.

-- Transaction A (Alice going off)        -- Transaction B (Bob going off)
SELECT count(*) FROM oncall              SELECT count(*) FROM oncall
  WHERE on_call = true;  -- sees 2         WHERE on_call = true;  -- sees 2
-- 2 >= 1, safe to leave                  -- 2 >= 1, safe to leave
UPDATE oncall SET on_call=false           UPDATE oncall SET on_call=false
  WHERE name='alice';                       WHERE name='bob';
COMMIT;                                   COMMIT;
-- Now ZERO doctors on call. The invariant is violated.
```

Each transaction read "2 on call, safe to leave," and each wrote a *different* row — so no row-level lock catches the conflict. `SELECT ... FOR UPDATE` on the rows each transaction *writes* does not help, because they write different rows. The cure that handles write skew in general is **`SERIALIZABLE` isolation**:

```sql
BEGIN ISOLATION LEVEL SERIALIZABLE;
SELECT count(*) FROM oncall WHERE on_call = true;
UPDATE oncall SET on_call=false WHERE name='alice';
COMMIT;  -- one of the two concurrent SERIALIZABLE transactions gets a 40001 error
```

Under `SERIALIZABLE`, Postgres tracks the read/write dependencies and detects that the two transactions cannot be serialised; it aborts one with SQLSTATE `40001` (`serialization_failure`). The aborted transaction must be **retried** — and that retry loop is mandatory under `SERIALIZABLE`, because the level guarantees correctness *only if* you retry the aborts. Challenge 1 builds exactly this retry loop. Citation: the serializable-isolation section at <https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE>.

## 4. The serialization-failure retry loop

`SERIALIZABLE` is only correct if you retry `40001` aborts. The pattern in Go:

```go
func withSerializableRetry(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	const maxAttempts = 5
	for attempt := 1; ; attempt++ {
		tx, err := pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.Serializable})
		if err != nil {
			return err
		}
		err = fn(tx)
		if err == nil {
			err = tx.Commit(ctx)
		}
		if err == nil {
			return nil // success
		}
		_ = tx.Rollback(ctx)

		// Retry only on a serialization failure (40001), up to a cap.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" && attempt < maxAttempts {
			// brief backoff with jitter (Week 4) before retrying
			time.Sleep(backoffWithJitter(attempt))
			continue
		}
		return err // a non-serialization error, or attempts exhausted
	}
}
```

Three points: **(1)** the retry triggers *only* on SQLSTATE `40001`, classified with `errors.As` and `pgconn.PgError` (Week 2 + Lecture 1). **(2)** the loop has a cap (`maxAttempts`) so a pathologically contended transaction does not spin forever. **(3)** the backoff has jitter (Week 4's thundering-herd lesson) so retrying transactions do not all collide again at the same instant. This loop is the price of `SERIALIZABLE`, and it is a price worth paying when the use case has a write-skew hazard that the row-lock cures cannot reach. Citation: <https://pkg.go.dev/github.com/jackc/pgx/v5#TxOptions>.

## 5. Choosing a cure — the decision

| Hazard | Cure | When |
|--------|------|------|
| Lost update | atomic in-place `UPDATE` (`x = x + 1`) | the new value is a function of the old; you do not need it in Go first |
| Lost update | `SELECT ... FOR UPDATE` | you must read-then-decide in Go; moderate contention; you prefer to block |
| Lost update | optimistic version column | low contention; no locks across a long read-think-write gap (form edits) |
| Write skew | `SERIALIZABLE` + `40001` retry loop | the invariant spans rows that different transactions write; row locks cannot catch it |

The skill is reading a use case and reaching for the *cheapest correct* cure. An atomic `UPDATE` is cheapest; `SERIALIZABLE` is the heaviest (every transaction pays the dependency-tracking cost and may need a retry). Reach for `SERIALIZABLE` when the hazard genuinely needs it, not by default. Citation: the Postgres concurrency-control overview at <https://www.postgresql.org/docs/current/mvcc.html>.

## 6. Integration testing against a real Postgres

You cannot test any of this against a fake — a fake has no isolation levels, no row locks, no `40001`. So the repository's tests run against a *real* Postgres in a container, spun up from the test with `testcontainers-go`:

```go
package postgres_test

import (
	"context"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/jackc/pgx/v5/pgxpool"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	ctx := context.Background()

	// Spin up an ephemeral Postgres container for this test run.
	pgC, err := postgres.Run(ctx, "postgres:16",
		postgres.WithDatabase("notes"),
		postgres.WithUsername("test"),
		postgres.WithPassword("test"),
		testcontainers.WithWaitStrategyAndDeadline(/* wait for "ready" */),
	)
	if err != nil {
		t.Fatalf("start postgres: %v", err)
	}
	t.Cleanup(func() { _ = pgC.Terminate(ctx) }) // tear down when the test ends

	dsn, err := pgC.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	if err := applyMigrations(dsn); err != nil { // from Lecture 2
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func TestPgRepo_CreateGet(t *testing.T) {
	repo := postgres.NewPgRepo(newTestPool(t))
	ctx := context.Background()

	created, err := repo.Create(ctx, notes.Note{ID: "1", Title: "hi"})
	if err != nil {
		t.Fatal(err)
	}
	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "hi" {
		t.Errorf("title = %q, want hi", got.Title)
	}

	// A duplicate id is a conflict (the 23505 translation from Lecture 1).
	_, err = repo.Create(ctx, notes.Note{ID: "1", Title: "dup"})
	if !errors.Is(err, notes.ErrConflict) {
		t.Errorf("duplicate: got %v, want ErrConflict", err)
	}
}
```

Four points:

1. **`testcontainers-go` starts a real Postgres** from inside the test, applies your migrations, and `t.Cleanup` tears it down. No shared test database, no leftover state between runs — each test run is hermetic.
2. **These tests are slow** — container startup is a few seconds. That is why the testing pyramid (Week 8) keeps few integration tests at the top and many fast unit tests below. The service and handler tests (Week 5) use a fake and run in microseconds; only the *repository* needs the real database.
3. **This is the only layer that needs a database**, which is the seam paying off again: the `23505`→`ErrConflict` translation, the transaction atomicity, the lost-update cure — all live in the repository, so all are tested here, and nowhere above needs a database to test.
4. **A compose harness is the alternative** — a `docker compose up` you run before `go test` — when you prefer a long-lived local database or `testcontainers`'s Docker-in-test does not fit your CI. Both apply migrations first.

Citation: <https://golang.testcontainers.org/modules/postgres/>.

## 7. The transaction and migration tests

Two integration tests the mini-project requires, beyond CRUD:

```go
func TestCreateWithAudit_Atomicity(t *testing.T) {
	repo := postgres.NewPgRepo(newTestPool(t))
	ctx := context.Background()

	// Force the audit insert to fail (e.g. a too-long actor violating a CHECK),
	// then assert NEITHER row exists — the note insert was rolled back too.
	_, err := repo.CreateWithAudit(ctx, notes.Note{ID: "1", Title: "x"}, strings.Repeat("a", 10_000))
	if err == nil {
		t.Fatal("expected the audit insert to fail")
	}
	if _, err := repo.Get(ctx, "1"); !errors.Is(err, notes.ErrNotFound) {
		t.Error("note should not exist: the transaction did not roll back")
	}
}

func TestMigrateDownAndReUp(t *testing.T) {
	dsn := startContainer(t)
	m, _ := migrate.New("file://../../migrations", dsn)
	if err := m.Up(); err != nil { t.Fatal(err) }
	if err := m.Down(); err != nil { t.Fatal(err) }   // the test of your down files
	if err := m.Up(); err != nil { t.Fatal(err) }      // re-up must work cleanly
}
```

The atomicity test proves the transaction rolls back on failure; the migrate-down-and-re-up test proves your `down` files are correct — the demonstration the "every migration has a tested down" rule demands.

## 8. Exercise pointer

Now do **Exercise 3 — Lost Update and the Cure**. Reproduce a lost update with two concurrent transactions (two goroutines, each read-then-write the same counter under `READ COMMITTED`), observe the lost increment, then fix it two ways — `SELECT ... FOR UPDATE` and a version column — and prove each fix produces the correct count under the same concurrency. The acceptance criterion is a `-race`-clean test that demonstrates the bug and both cures against a real Postgres container.

## 9. Summary

- Postgres defaults to `READ COMMITTED`, which allows lost updates and write skew — bugs that pass single-threaded tests and surface under concurrency.
- The lost update (read-then-write, second write clobbers first) has three cures: atomic in-place `UPDATE` (cheapest), `SELECT ... FOR UPDATE` (block), optimistic version column (low-contention, lock-free across the gap).
- Write skew (each write individually fine, the combination violates an invariant across different rows) needs `SERIALIZABLE` isolation.
- `SERIALIZABLE` is correct only with a `40001` serialization-failure retry loop (capped, with jittered backoff). Classify the abort with `errors.As`/`pgconn.PgError`.
- Choose the cheapest correct cure; `SERIALIZABLE` is the heaviest, not the default.
- Test the repository against a real Postgres with `testcontainers-go` (or a compose harness): apply migrations, run, tear down. These are slow; keep them few (the testing pyramid).
- The repository is the only layer needing a database — the seam paying off. Test atomicity (forced mid-transaction failure leaves nothing) and `migrate down`/re-`up` (the test of your `down` files).

Cited references this lecture pulled from: <https://www.postgresql.org/docs/current/transaction-iso.html>, <https://www.postgresql.org/docs/current/sql-select.html#SQL-FOR-UPDATE-SHARE>, <https://www.postgresql.org/docs/current/mvcc.html>, <https://pkg.go.dev/github.com/jackc/pgx/v5#TxOptions>, <https://golang.testcontainers.org/modules/postgres/>.
