# Challenge 1 — Write Skew Under `SERIALIZABLE` and the `40001` Retry Loop

> **Time:** 2 hours. **Prerequisites:** Exercise 3 and Lecture 3 (isolation, write skew). **Citations:** the Postgres serializable-isolation docs at <https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE>, the `pgx.TxOptions` docs at <https://pkg.go.dev/github.com/jackc/pgx/v5#TxOptions>, and the SQLSTATE list at <https://www.postgresql.org/docs/current/errcodes-appendix.html>.

## The premise

The lost-update cures from Exercise 3 — row locks and version columns — do not catch **write skew**, where two transactions write *different* rows and the combination violates an invariant. The general cure is `SERIALIZABLE` isolation, but `SERIALIZABLE` is correct *only if* you retry the serialization-failure (`40001`) aborts it produces. You will reproduce a write-skew bug, watch `SERIALIZABLE` reject it, and build the retry loop that makes the use case correct.

## The scenario: the on-call invariant

```sql
CREATE TABLE oncall (
    name    TEXT PRIMARY KEY,
    on_call BOOLEAN NOT NULL
);
INSERT INTO oncall (name, on_call) VALUES ('alice', true), ('bob', true);
-- Invariant: at least one person must remain on_call = true.
```

The use case "go off call" reads the count of on-call people, and only writes `on_call = false` if at least one *other* person would remain:

```sql
-- inside one transaction:
SELECT count(*) FROM oncall WHERE on_call = true;  -- check the invariant
-- if count > 1, safe to leave:
UPDATE oncall SET on_call = false WHERE name = $1;
```

Run two such transactions concurrently (Alice and Bob both leaving) under `READ COMMITTED` and both see count = 2, both decide it is safe, both write — leaving **zero** on call. The invariant is violated.

## What you will build

```
src/oncall/
  oncall.go        // the use case under SERIALIZABLE + the retry loop
  oncall_test.go   // a concurrency test proving the invariant holds
  WRITEUP.md
```

### Part A — reproduce the write skew under `READ COMMITTED`

Write `goOffCallReadCommitted(ctx, pool, name)` that runs the check-then-write under the default isolation. In the test, run it concurrently for both `alice` and `bob` and assert (it will *fail*) that at least one remains on call. Document the violation in `WRITEUP.md`.

### Part B — the same under `SERIALIZABLE`, with the retry loop

```go
func goOffCall(ctx context.Context, pool *pgxpool.Pool, name string) error {
	return withSerializableRetry(ctx, pool, func(tx pgx.Tx) error {
		var onCall int
		if err := tx.QueryRow(ctx,
			`SELECT count(*) FROM oncall WHERE on_call = true`).Scan(&onCall); err != nil {
			return err
		}
		if onCall <= 1 {
			return errLastOnCall // refuse: you are the last one
		}
		_, err := tx.Exec(ctx, `UPDATE oncall SET on_call = false WHERE name = $1`, name)
		return err
	})
}

func withSerializableRetry(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	const maxAttempts = 10
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
			return nil
		}
		_ = tx.Rollback(ctx)

		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "40001" && attempt < maxAttempts {
			time.Sleep(backoffWithJitter(attempt)) // Week 4: jitter to avoid re-collision
			continue
		}
		return err
	}
}
```

Under `SERIALIZABLE`, when Alice and Bob's transactions conflict, Postgres aborts one with `40001`; the retry loop re-runs it, and the re-run now reads count = 1 (Alice already committed her departure) and refuses with `errLastOnCall`. The invariant holds. **At least one person always remains on call.**

## The measurement plan

### M1 — the violation under `READ COMMITTED`

Run the Part A concurrency test 100 times (or with 20 concurrent leavers). Report how often the invariant is violated (zero on call). It should violate frequently.

### M2 — the fix under `SERIALIZABLE`

Run the Part B test the same way. Report: the invariant is *never* violated, and count how many transactions hit a `40001` and retried. Report the retry rate (retries / total attempts).

### M3 — the cost of `SERIALIZABLE`

Benchmark the throughput (operations/sec) of the `READ COMMITTED` version vs the `SERIALIZABLE`-with-retry version under the same concurrency. Report the slowdown. `SERIALIZABLE` is correct but costs the dependency-tracking overhead plus the retries — quantify it.

## Acceptance criteria

1. Part A demonstrates the write skew under `READ COMMITTED` (the invariant is violated under concurrency).
2. Part B's `SERIALIZABLE` + retry version *never* violates the invariant across 100 concurrent runs.
3. The retry loop triggers *only* on `40001` (verified by `errors.As`/`pgconn.PgError`), has a cap, and uses jittered backoff.
4. `WRITEUP.md` reports M1 (violation rate), M2 (retry rate), and M3 (the throughput cost), with one sentence each.
5. `go test -race ./...` is green.

## Stretch goals

1. **A predicate the row-lock cure cannot reach.** Show explicitly that `SELECT ... FOR UPDATE` on the row each transaction *writes* does not fix the write skew (because they write different rows), and explain why only `SERIALIZABLE` (or a materialised lock on a summary row) catches it.
2. **`SELECT ... FOR UPDATE` on a guard row.** Add a single `oncall_summary` row that every transaction locks `FOR UPDATE` first; show this *also* fixes the skew (by serialising on the guard) and compare its throughput to `SERIALIZABLE`. Discuss the trade.
3. **Retry budget under a request deadline.** Wire the retry loop to respect a `context.WithTimeout` so a contended transaction does not retry past the request's budget. Report what happens when the budget expires mid-retry (`context.DeadlineExceeded`, not `40001`).

Cited references: <https://www.postgresql.org/docs/current/transaction-iso.html#XACT-SERIALIZABLE>, <https://pkg.go.dev/github.com/jackc/pgx/v5#TxOptions>, <https://www.postgresql.org/docs/current/errcodes-appendix.html>, and the jitter discussion from Week 4.
