# Challenge 2 — Bulk Load 100k Rows Three Ways: Row-by-Row, Batch, and `CopyFrom`

> **Time:** 90 minutes. **Prerequisites:** Exercises 1 and 2. **Deliverable:** a program that loads 100,000 rows three ways, a benchmark of each, and a one-page write-up on when to reach for which.

## Statement of the problem

"Insert these 100,000 rows" has wildly different performance depending on how you send them. Naive row-by-row inserts make 100,000 network round trips; a batch makes one (or a few); `pgx.CopyFrom` uses Postgres's streaming `COPY` protocol, the fastest bulk path Postgres offers. The throughput gap between the slowest and fastest is often 50-100x. Your job is to measure it on your machine and learn when each is the right tool.

## What you will build

```
src/bulk/
  bulk.go        // the three load strategies
  bulk_test.go   // a benchmark of each
  BULK.md        // the write-up
```

The schema (a migration):

```sql
CREATE TABLE events (
    id      BIGINT      NOT NULL,
    kind    TEXT        NOT NULL,
    payload JSONB       NOT NULL,
    at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### Strategy 1 — row by row

```go
func loadRowByRow(ctx context.Context, pool *pgxpool.Pool, events []Event) error {
	for _, e := range events {
		if _, err := pool.Exec(ctx,
			`INSERT INTO events (id, kind, payload) VALUES ($1, $2, $3)`,
			e.ID, e.Kind, e.Payload); err != nil {
			return err
		}
	}
	return nil
}
```

100,000 statements, each a round trip. The baseline — and the trap a naive importer falls into.

### Strategy 2 — batched in a transaction with `pgx.Batch`

```go
func loadBatched(ctx context.Context, pool *pgxpool.Pool, events []Event) error {
	const chunk = 1000
	for i := 0; i < len(events); i += chunk {
		end := min(i+chunk, len(events))
		batch := &pgx.Batch{}
		for _, e := range events[i:end] {
			batch.Queue(`INSERT INTO events (id, kind, payload) VALUES ($1, $2, $3)`,
				e.ID, e.Kind, e.Payload)
		}
		br := pool.SendBatch(ctx, batch)
		if err := br.Close(); err != nil {
			return err
		}
	}
	return nil
}
```

`pgx.Batch` pipelines many statements in one round trip per chunk — far fewer round trips, much faster.

### Strategy 3 — `pgx.CopyFrom`

```go
func loadCopyFrom(ctx context.Context, pool *pgxpool.Pool, events []Event) error {
	rows := make([][]any, len(events))
	for i, e := range events {
		rows[i] = []any{e.ID, e.Kind, e.Payload}
	}
	_, err := pool.CopyFrom(ctx,
		pgx.Identifier{"events"},
		[]string{"id", "kind", "payload"},
		pgx.CopyFromRows(rows),
	)
	return err
}
```

`CopyFrom` uses Postgres's binary `COPY` protocol — the fastest bulk-insert path, streaming rows with minimal per-row overhead and no per-statement parsing.

## The measurement plan

### M1 — the three timings

Benchmark each strategy loading 100,000 rows (truncate the table in `[ResetTimer]`/setup between runs). Report wall-clock time and rows/sec for each. Expect roughly:

```
loadRowByRow    100000 rows in  18.4 s    (~5,400 rows/s)
loadBatched     100000 rows in   0.9 s    (~111,000 rows/s)
loadCopyFrom    100000 rows in   0.21 s   (~476,000 rows/s)
```

`CopyFrom` is typically ~80-100x faster than row-by-row and ~3-5x faster than batched. The exact numbers depend on your machine and whether Postgres is local or remote (the round-trip count is the dominant factor for row-by-row).

### M2 — the round-trip explanation

Explain *why* the gap exists in terms of network round trips and per-statement parsing:

- Row-by-row: 100,000 round trips, 100,000 parse/plan cycles.
- Batched (chunk 1000): 100 round trips, still 100,000 statements parsed (but pipelined).
- `CopyFrom`: 1 streaming operation, no per-row statement parsing.

If your Postgres is on `localhost`, run the row-by-row strategy again against a *remote* Postgres (or add an artificial latency) and show the gap *widens* — round trips dominate, and they cost more when each one crosses a network.

### M3 — when each is correct

Write the decision: row-by-row is fine for a handful of inserts in a request; batched is the right tool for "import a few thousand under a transaction with per-row error handling"; `CopyFrom` is for "load a large file, you control the data, you want maximum throughput and do not need per-row error feedback." Note `CopyFrom`'s limitation: it is all-or-mostly-nothing on errors and does not run per-row triggers the same way, so it is for trusted bulk data, not user-by-user inserts.

## Acceptance criteria

1. All three strategies load 100,000 rows correctly (verify `SELECT count(*) = 100000` after each).
2. `BULK.md` reports the three timings and rows/sec (M1).
3. The round-trip explanation (M2) is correct and, ideally, demonstrated with an added-latency run.
4. The decision matrix (M3) correctly characterises when each is the right tool, including `CopyFrom`'s limitations.
5. The benchmark truncates between strategies so the numbers are comparable; `go test -bench .` runs clean.

## A trap to watch for

Do not include the data *generation* in the timed region — generate the 100,000 `Event`s once in setup, `b.ResetTimer()`, then time only the load. A benchmark that re-generates the slice inside the timed loop measures your random-number generator, not your inserts.

## A second trap: the connection count

Row-by-row over a pool acquires and releases a connection per `Exec` (cheap, same connection reused via the pool), but if you accidentally spawn 100,000 goroutines each doing one insert, you will saturate `MaxConns` and queue. Keep the row-by-row strategy *sequential* for a fair comparison of the per-statement cost; the point is round trips, not concurrency.

## Submission

Submit the `bulk` package (runnable with `go test -bench .`) and `BULK.md` with the three timings, the round-trip explanation, and the decision matrix. A comment block in `bulk.go` links to the `pgx.CopyFrom` and `pgx.Batch` docs.

The rubric:

- (40%) All three strategies correct and benchmarked; the numbers show the expected ordering (CopyFrom < batched < row-by-row).
- (30%) The round-trip explanation is correct and connects the gap to network round trips and parsing.
- (20%) The decision matrix correctly places each tool, including `CopyFrom`'s trade-offs.
- (10%) Benchmark hygiene (setup outside the timed loop, truncate between runs); citations present.

Cited references: the `pgx.CopyFrom` docs at <https://pkg.go.dev/github.com/jackc/pgx/v5#Conn.CopyFrom>, the `pgx.Batch` docs at <https://pkg.go.dev/github.com/jackc/pgx/v5#Batch>, the Postgres `COPY` docs at <https://www.postgresql.org/docs/current/sql-copy.html>, and the testing-benchmarks docs from Week 4.
