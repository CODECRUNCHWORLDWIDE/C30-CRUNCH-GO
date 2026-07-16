// Exercise 2 — Migrations + a Multi-Step Transaction
//
// GOAL
//   Author an up/down migration pair for a two-table schema (notes + audit),
//   apply it, then wrap a two-table write in ONE transaction with
//   defer Rollback / explicit Commit. Force the second insert to fail and PROVE
//   atomicity: neither row is left behind.
//
// SETUP
//   docker run --name pg-ex02 -e POSTGRES_PASSWORD=devpass -e POSTGRES_DB=notes \
//     -p 5432:5432 -d postgres:16
//   export DATABASE_URL='postgres://postgres:devpass@localhost:5432/notes?sslmode=disable'
//   mkdir ex02 && cd ex02 && go mod init ex02 && go get github.com/jackc/pgx/v5
//   # save the migrations below, then:
//   migrate -path migrations -database "$DATABASE_URL" up
//   go run .
//   migrate -path migrations -database "$DATABASE_URL" down   # the test of your down files
//
// MIGRATIONS (save under migrations/)
//   000001_init.up.sql:
//     CREATE TABLE notes (id TEXT PRIMARY KEY, title TEXT NOT NULL);
//     CREATE TABLE audit (
//       id BIGSERIAL PRIMARY KEY,
//       note_id TEXT NOT NULL REFERENCES notes(id),
//       actor TEXT NOT NULL CHECK (length(actor) <= 64),  -- the CHECK we'll violate
//       action TEXT NOT NULL
//     );
//   000001_init.down.sql:
//     DROP TABLE audit;
//     DROP TABLE notes;
//
// TASKS
//   1. Implement createWithAudit: Begin, defer Rollback, insert the note, insert
//      the audit row, Commit. Use a single tx for both inserts.
//   2. Call it once with a short actor (succeeds) and once with a 100-char actor
//      (violates the CHECK -> the audit insert fails -> the whole tx rolls back).
//   3. After the failing call, query the notes table and confirm the note from
//      that call is ABSENT — proof the note insert rolled back with the audit.

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func createWithAudit(ctx context.Context, pool *pgxpool.Pool, id, title, actor string) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin: %w", err)
	}
	defer tx.Rollback(ctx) // no-op after a successful Commit; safety on every other path

	if _, err := tx.Exec(ctx, `INSERT INTO notes (id, title) VALUES ($1, $2)`, id, title); err != nil {
		return fmt.Errorf("insert note: %w", err)
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO audit (note_id, actor, action) VALUES ($1, $2, 'create')`, id, actor); err != nil {
		return fmt.Errorf("insert audit: %w", err) // CHECK violation rolls back the note too
	}
	return tx.Commit(ctx)
}

func noteExists(ctx context.Context, pool *pgxpool.Pool, id string) (bool, error) {
	var exists bool
	err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM notes WHERE id=$1)`, id).Scan(&exists)
	return exists, err
}

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer pool.Close()

	// Success path.
	if err := createWithAudit(ctx, pool, "ok-1", "good", "alice"); err != nil {
		fmt.Fprintln(os.Stderr, "unexpected:", err)
		os.Exit(1)
	}
	ok, _ := noteExists(ctx, pool, "ok-1")
	fmt.Println("ok-1 exists after success:", ok) // true

	// Failure path: a 100-char actor violates CHECK(length(actor) <= 64).
	longActor := strings.Repeat("a", 100)
	err = createWithAudit(ctx, pool, "bad-1", "should-roll-back", longActor)
	fmt.Println("failing call returned error:", err != nil) // true

	exists, _ := noteExists(ctx, pool, "bad-1")
	fmt.Println("bad-1 exists after rollback:", exists) // MUST be false
	if exists {
		fmt.Fprintln(os.Stderr, "ATOMICITY BROKEN: the note survived a failed transaction")
		os.Exit(1)
	}
}

/*
main_test.go — an integration test (run against the container):

package main

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestTransactionRollsBack(t *testing.T) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("set DATABASE_URL to a test Postgres")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	err = createWithAudit(ctx, pool, "tx-test", "x", strings.Repeat("a", 100))
	if err == nil {
		t.Fatal("expected the CHECK violation to fail the transaction")
	}
	exists, err := noteExists(ctx, pool, "tx-test")
	if err != nil {
		t.Fatal(err)
	}
	if exists {
		t.Error("note survived a rolled-back transaction: atomicity broken")
	}
}
*/
