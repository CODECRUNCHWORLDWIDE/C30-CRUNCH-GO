# Challenge 1 — Integration Tests Against a Real Postgres with `testcontainers-go`

> **Estimated time:** 2 hours. **Prerequisite:** Week 6 complete (the `notes` Postgres repository, `golang-migrate` migrations) and Docker installed and running. **Citations:** the `testcontainers-go` documentation at <https://golang.testcontainers.org/>, the Postgres module at <https://golang.testcontainers.org/modules/postgres/>, and the build-constraints reference at <https://pkg.go.dev/cmd/go#hdr-Build_constraints>.

## The premise

A `fakeRepo` proves your *service logic* is correct. It cannot prove your *SQL* is correct, because there is no SQL in a fake. Your Week-6 `PostgresRepo` issues real queries, runs real migrations, and scans real rows — and every one of those is a place a bug can hide that a fake will never see: a misspelled column, a `NULL` you did not handle, a `pgx` scan into the wrong field, a `down` migration that does not undo its `up`. This challenge builds the integration suite that catches those bugs: a real Postgres in a container, your real migrations applied, your real repository exercised — all from inside `go test`, gated behind a build tag, skipping cleanly when Docker is absent.

## What you will build

- An integration test file `repo_integration_test.go` with `//go:build integration` as its first line.
- A `TestMain` that starts one `testcontainers-go` Postgres container, runs the Week-6 migrations once, runs the suite, and terminates the container after.
- Repository integration tests: create-and-get round-trips, list, delete, a `NOT NULL` / constraint violation surfaced as the right error, and a uniqueness or foreign-key check if your schema has one.
- A migrations test proving a clean `up`, `down`, `up` cycle.
- A `RESULTS.md` documenting the run, the container image and version, and the answers to the reflection questions.

## Setup

### 1. Confirm Docker

```bash
docker info            # must succeed without sudo (or use rootless / Colima)
docker pull postgres:16-alpine
```

If `docker info` fails, install Docker Desktop (<https://docs.docker.com/get-docker/>) or Colima (`brew install colima && colima start`). The suite is written to skip — not fail — when the daemon is unreachable, so a teammate without Docker can still run `go test ./...`.

### 2. Add the dependencies

```bash
go get github.com/testcontainers/testcontainers-go
go get github.com/testcontainers/testcontainers-go/modules/postgres
go get github.com/golang-migrate/migrate/v4
go get github.com/jackc/pgx/v5/pgxpool
```

### 3. The build tag

The first line of the file, before `package`, with a blank line after:

```go
//go:build integration

package notes_test
```

`go test ./...` ignores this file entirely. Only `go test -tags=integration ./...` compiles and runs it. Verify both:

```bash
go test ./...                      # integration file not even compiled
go test -tags=integration ./...    # integration file runs (needs Docker)
```

### 4. `TestMain`: one container, migrations once

```go
//go:build integration

package notes_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testDSN string

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Skip the whole suite cleanly if Docker is not available.
	if _, err := testcontainers.NewDockerProvider(); err != nil {
		println("integration: Docker unavailable, skipping suite:", err.Error())
		os.Exit(0) // exit 0 so `go test ./...` stays green
	}

	pg, err := tcpostgres.Run(ctx,
		"postgres:16-alpine",
		tcpostgres.WithDatabase("notes"),
		tcpostgres.WithUsername("notes"),
		tcpostgres.WithPassword("notes"),
		testcontainers.WithWaitStrategy(
			wait.ForListeningPort("5432/tcp").WithStartupTimeout(60*time.Second),
		),
	)
	if err != nil {
		println("integration: start postgres:", err.Error())
		os.Exit(1)
	}

	testDSN, err = pg.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		println("integration: dsn:", err.Error())
		os.Exit(1)
	}
	if err := runMigrations(testDSN); err != nil {
		println("integration: migrate:", err.Error())
		os.Exit(1)
	}

	code := m.Run()
	_ = pg.Terminate(ctx)
	os.Exit(code)
}

func runMigrations(dsn string) error {
	m, err := migrate.New("file://../../migrations", dsn) // path to your Week-6 migrations
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}
```

### 5. A repository integration test

```go
//go:build integration

func TestPostgresRepo_CreateGetDelete(t *testing.T) {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, testDSN)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)

	repo := notes.NewPostgresRepo(pool)

	created, err := repo.Create(ctx, notes.Note{Title: "Hi", Body: "there", Tags: []string{"go"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	got, err := repo.GetByID(ctx, created.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if diff := cmp.Diff(created, got, cmpopts.IgnoreFields(notes.Note{}, "CreatedAt")); diff != "" {
		t.Errorf("round-trip mismatch (-created +got):\n%s", diff)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := repo.GetByID(ctx, created.ID); !errors.Is(err, notes.ErrNotFound) {
		t.Errorf("after delete, GetByID error = %v, want ErrNotFound", err)
	}
}
```

### 6. The migration up/down/up test

```go
//go:build integration

func TestMigrationsReversible(t *testing.T) {
	m, err := migrate.New("file://../../migrations", testDSN)
	if err != nil {
		t.Fatalf("migrate.New: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })

	if err := m.Down(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("Down: %v", err)
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("Up after Down: %v", err)
	}
}
```

If `Up after Down` fails, your `down` migration did not fully undo your `up` — a real bug that ships silently until someone needs to roll back in production. Run this test serially (no `t.Parallel()`); it mutates the shared schema.

## Acceptance criteria

- [ ] The integration file's first line is `//go:build integration`.
- [ ] `go test ./...` (no tags) passes and does **not** start a container.
- [ ] `go test -tags=integration ./...` starts one Postgres container, runs migrations, and runs the suite.
- [ ] On a machine without Docker, `go test -tags=integration ./...` skips cleanly (`os.Exit(0)`), not fails.
- [ ] `TestMain` starts exactly one container for the whole suite (not one per test).
- [ ] The container uses a real wait strategy (`wait.ForListeningPort`, `wait.ForSQL`, or the module default), not a `time.Sleep`.
- [ ] A create-and-get test round-trips a `Note` through real SQL, compared with `cmp.Diff` (ignoring server-generated fields).
- [ ] A delete test proves `GetByID` returns `ErrNotFound` afterward.
- [ ] At least one test asserts a constraint violation surfaces as the expected application error (e.g. an empty required column → an error, not a panic).
- [ ] A migration test proves a clean `up`, `down`, `up` cycle.
- [ ] `RESULTS.md` records the image, the run output, and the reflection answers.

## Reflection (write into `RESULTS.md`)

1. **The wait strategy.** What is the difference between a container being "running" and a Postgres being "ready to accept connections"? What flake do you get if you connect too early, and why does `wait.ForSQL` (which executes a `SELECT 1`) close that gap more reliably than `wait.ForListeningPort`?

2. **One container vs. one per test.** `TestMain` starts a single container for the whole package. What does that buy you in wall-clock time? What does it cost you in isolation, and how do you recover the isolation (transaction rollback or schema-per-test) without paying the container-start cost again?

3. **The build tag boundary.** Why a build tag rather than checking an environment variable inside each test? What is the difference at *compile* time — which approach keeps the `testcontainers-go` dependency out of your fast `go test ./...` build entirely?

4. **The fake vs. this.** Name one bug your Week-6 fake repository could *not* have caught that this integration suite can. (Column typo? `NULL` handling? Scan order? Constraint behaviour?) Be specific to your schema.

## Stretch goals (optional)

- **Parallel isolation.** Make two repository tests `t.Parallel()`-safe with transaction rollback: each test runs inside a `pgx.Tx` rolled back in `t.Cleanup`, so neither sees the other's rows. Prove they pass under `-race`.
- **The compose alternative.** Add a `docker-compose.yml` with a Postgres service and a second code path that reads `TEST_DATABASE_URL` from the environment and `t.Skip`s when it is unset. Document when you would prefer compose (CI with a service container) over `testcontainers-go` (a developer's laptop).
- **`wait.ForSQL`.** Swap the wait strategy for `wait.ForSQL("5432/tcp", "pgx", func(host string, port nat.Port) string { ... })` that runs an actual query, and observe whether it removes a flake you saw with the port-only strategy.

## Submission

Place under your repo:

- `repo_integration_test.go` (and any helpers) with the `//go:build integration` tag.
- `RESULTS.md` with the run output, the image/version, and the four reflection answers.

Commit with the message:

```
challenge-01: integration tests against testcontainers postgres
```

Push and open a PR. The PR description should include the output of both `go test ./...` (green, no container) and `go test -tags=integration ./...` (green, container started), so the reviewer sees the build-tag boundary working.
