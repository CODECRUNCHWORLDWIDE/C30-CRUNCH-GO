// Exercise 3 — Reproduce a Lost Update, Then Cure It Two Ways
//
// GOAL
//   Show the lost-update hazard with two concurrent transactions that read-then-
//   write the same counter under READ COMMITTED, observe the lost increment,
//   then fix it with SELECT ... FOR UPDATE and again with a version column.
//   Each cure must produce the correct count under the same concurrency.
//
// SETUP
//   docker run --name pg-ex03 -e POSTGRES_PASSWORD=devpass -e POSTGRES_DB=ctr \
//     -p 5432:5432 -d postgres:16
//   export DATABASE_URL='postgres://postgres:devpass@localhost:5432/ctr?sslmode=disable'
//   mkdir ex03 && cd ex03 && go mod init ex03 && go get github.com/jackc/pgx/v5
//   go run . racy        # demonstrates the lost update
//   go run . forupdate   # demonstrates the FOR UPDATE cure
//   go test -race ./...
//
// SCHEMA (this program creates it on startup)
//   CREATE TABLE counter (id INT PRIMARY KEY, n INT NOT NULL, version INT NOT NULL DEFAULT 0);
//
// TASKS
//   1. Read incRacy: read n, then write n+1, in a transaction. Run it from
//      `goroutines` workers concurrently and observe the final n < goroutines.
//   2. Read incForUpdate: SELECT ... FOR UPDATE locks the row so the second
//      reader blocks until the first commits. The final n == goroutines.
//   3. Implement incVersion: read (n, version), UPDATE ... WHERE version = old,
//      retry on a 0-row update. Also produces the correct count.

package main

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const goroutines = 50

func setup(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS counter (id INT PRIMARY KEY, n INT NOT NULL, version INT NOT NULL DEFAULT 0);
		INSERT INTO counter (id, n) VALUES (1, 0)
		ON CONFLICT (id) DO UPDATE SET n = 0, version = 0;`)
	return err
}

// incRacy: read-then-write with no locking. Loses updates under concurrency.
func incRacy(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var n int
	if err := tx.QueryRow(ctx, `SELECT n FROM counter WHERE id=1`).Scan(&n); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE counter SET n=$1 WHERE id=1`, n+1); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// incForUpdate: lock the row on read so concurrent increments serialise.
func incForUpdate(ctx context.Context, pool *pgxpool.Pool) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	var n int
	if err := tx.QueryRow(ctx, `SELECT n FROM counter WHERE id=1 FOR UPDATE`).Scan(&n); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE counter SET n=$1 WHERE id=1`, n+1); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// incVersion: optimistic concurrency. Retry when another writer won the race.
func incVersion(ctx context.Context, pool *pgxpool.Pool) error {
	for {
		var n, ver int
		if err := pool.QueryRow(ctx, `SELECT n, version FROM counter WHERE id=1`).Scan(&n, &ver); err != nil {
			return err
		}
		ct, err := pool.Exec(ctx,
			`UPDATE counter SET n=$1, version=$2 WHERE id=1 AND version=$3`, n+1, ver+1, ver)
		if err != nil {
			return err
		}
		if ct.RowsAffected() == 1 {
			return nil // we won
		}
		// 0 rows affected: someone else bumped version; re-read and retry.
	}
}

func runConcurrently(ctx context.Context, pool *pgxpool.Pool, fn func(context.Context, *pgxpool.Pool) error) error {
	var wg sync.WaitGroup
	errs := make(chan error, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := fn(ctx, pool); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		return err
	}
	return nil
}

func finalCount(ctx context.Context, pool *pgxpool.Pool) int {
	var n int
	_ = pool.QueryRow(ctx, `SELECT n FROM counter WHERE id=1`).Scan(&n)
	return n
}

func main() {
	ctx := context.Background()
	mode := "racy"
	if len(os.Args) > 1 {
		mode = os.Args[1]
	}
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := setup(ctx, pool); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var fn func(context.Context, *pgxpool.Pool) error
	switch mode {
	case "racy":
		fn = incRacy
	case "forupdate":
		fn = incForUpdate
	case "version":
		fn = incVersion
	default:
		fmt.Fprintln(os.Stderr, "mode must be racy | forupdate | version")
		os.Exit(1)
	}

	if err := runConcurrently(ctx, pool, fn); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	got := finalCount(ctx, pool)
	fmt.Printf("mode=%-10s want=%d got=%d  %s\n", mode, goroutines, got,
		map[bool]string{true: "OK", false: "LOST UPDATES"}[got == goroutines])
}

var _ = pgx.ErrNoRows // referenced to keep the pgx import meaningful in examples
