# Mini-Project — Lab 08: Harden `notes`

> Bring the `notes` service you have built over Weeks 5–7 to a strong test posture. Write a layered test suite — table-driven unit tests for the service and validation, integration tests against a real Postgres container, a benchmark for the hot path with one measured optimization, a `pprof` capture you can read, and a fuzz target against the input-parsing/validation code that surfaces and fixes at least one real crash — and read a coverage report as a signal. By the end you have a `notes` service whose every change ships with a test, whose hot path is measured, whose input parsing cannot panic on garbage, and which is clean under `go vet` and `go test -race`. This is the difference between code that works on your laptop and code an operator will put behind a load balancer.

This is the canonical "test it like you mean it" exercise for a real Go service. The shape is production-shaped: a fast unit suite that runs on every save, an integration suite behind a build tag that runs against a real Postgres, a benchmark and a profile that turn "I think it's fast" into "here is the `benchstat` delta," and a fuzz target that found a bug you would never have typed. Every senior Go engineer who has shipped a service has built this layered suite around it. The mini-project is that experience in microcosm.

**Estimated time:** ~12.5 hours (split across Friday, Saturday, and Sunday in the suggested schedule).

---

## What you will build

You start from your existing `notes` service (handler/service/repository, `pgx`/`sqlc`/`golang-migrate`). You add, around it, a complete test posture:

- **Unit tests** — table-driven tests for the service and the validation/parsing code, using a hand-written in-memory fake repository (no mocking library), `go-cmp` for struct assertions, and a golden-file test for any rendered output.
- **Integration tests** — behind a `//go:build integration` tag, against a `testcontainers-go` Postgres, with the Week-6 migrations run in `TestMain`, exercising the real repository's SQL.
- **A benchmark + one measured optimization** — a `testing.B` benchmark for a hot path (the tag-index build, the feed render, or the list-response JSON encode), profiled with `pprof`, optimized once, and proven with a `benchstat` delta.
- **A `pprof` capture artifact** — a CPU profile (`cpu.out`) and your reading of it (`top`/`list` output) committed as evidence of the optimization.
- **A fuzz target** — against the input-parsing or validation code (a query-string parser, a tag-list parser, a request-body validator) that finds and fixes at least **one** real crash, with the crasher committed under `testdata/fuzz/`.
- **A coverage report** — `go tool cover -func` / `-html` output, read as a signal to find untested holes (not chased to 100%).

---

## Rules

- **You may** read the Go documentation, the `testing` godoc, the Go blog, the fuzzing docs, the `testcontainers-go` docs, the Week 8 lecture notes and exercises, and any free Go reference.
- **Allowed dependencies** beyond the standard library:
  - `github.com/google/go-cmp` (struct assertions in tests).
  - `github.com/testcontainers/testcontainers-go` and its `modules/postgres` (integration tests only).
  - `github.com/golang-migrate/migrate/v4` (you already have this from Week 6).
  - `github.com/jackc/pgx/v5` (you already have this from Week 6).
  - `golang.org/x/perf/cmd/benchstat` as a **tool** (installed with `go install`, not imported).
  - No mocking library. Doubles are hand-written fakes implementing your small consumer interfaces.
- **Go version:** 1.22 or later for every package.
- **Clean under `go vet ./...`** — zero findings.
- **Clean under `go test -race ./...`** — zero data races. Your fakes must be concurrency-safe if any parallel test touches them.
- **Integration tests behind `//go:build integration`** — `go test ./...` must pass and start no container; `go test -tags=integration ./...` runs the real-Postgres suite.
- **The fuzz crash is not optional.** Your fuzz target must find a real crash, you must fix it, and the minimized crasher must be committed under `testdata/fuzz/` as a regression test. "I ran the fuzzer and it found nothing" means your target has no useful invariant or your code is already hardened — in either case, plant or locate a genuine input-handling bug, fuzz it, and fix it. (If your code is genuinely crash-free, fuzz a deliberately-introduced bug on a branch, document the find and fix, then keep the crasher.)
- **Benchmarks have a sink** and use `b.ReportAllocs()`; optimization claims are backed by `benchstat` over `-count=10` with `p < 0.05`.

---

## Project structure

```
notes/
├── go.mod
├── Makefile                     (test, test-integration, bench, fuzz, cover targets)
├── migrations/                  (your Week-6 golang-migrate files)
│   ├── 0001_init.up.sql
│   └── 0001_init.down.sql
├── internal/
│   └── notes/
│       ├── note.go              (the domain type, validation)
│       ├── service.go           (the service layer)
│       ├── repository.go        (the Repository interface + the Postgres impl)
│       ├── render.go            (rendering / hot path under benchmark)
│       ├── query.go             (the input parser under fuzz)
│       ├── note_test.go         (table-driven validation tests)
│       ├── service_test.go      (service tests with the fake repo)
│       ├── render_test.go       (golden-file test + RenderFeed benchmark)
│       ├── query_test.go        (FuzzParseQuery + table tests)
│       ├── fake_repo_test.go    (the hand-written in-memory fake)
│       ├── repo_integration_test.go   (//go:build integration)
│       └── testdata/
│           ├── render_feed.golden
│           └── fuzz/
│               └── FuzzParseQuery/
│                   └── <committed crasher>
└── cmd/
    └── notesd/
        └── main.go              (the server; optionally exposes net/http/pprof on localhost)
```

You ship one module. The unit tests live next to the code; the integration test is one file behind the build tag; the benchmark and fuzz target are `_test.go` siblings of the code they exercise.

---

## Acceptance criteria

### Unit tests

- [ ] `note_test.go` is a table-driven test for `ValidateNote` with a `name`/`in`/`wantErr`/`wantErrIs` table, a `t.Run` loop, `t.Parallel()`, and `errors.Is` for the specific sentinel per invalid case.
- [ ] The table includes boundary cases (exactly at the length limit, one over).
- [ ] `service_test.go` tests the service against a **hand-written** in-memory `fakeRepo` (no mocking library); the fake is concurrency-safe (a `sync.Mutex`).
- [ ] Service tests drive at least one error path by inducing a repository error through the fake.
- [ ] Struct comparisons use `cmp.Diff` with `cmpopts` (ignoring server-generated fields), not `reflect.DeepEqual`.
- [ ] `render_test.go` has a golden-file test for the rendered output with an `-update` flag.
- [ ] Assertion helpers call `t.Helper()`.

### Integration tests

- [ ] `repo_integration_test.go`'s first line is `//go:build integration`.
- [ ] `go test ./...` passes and starts **no** container; `go test -tags=integration ./...` runs the suite.
- [ ] The suite `t.Skip`s / `os.Exit(0)`s cleanly when Docker is unavailable.
- [ ] `TestMain` starts **one** `testcontainers-go` Postgres, runs the Week-6 migrations once, runs the suite, and terminates the container.
- [ ] At least one create-and-get round-trip through real SQL, compared with `cmp.Diff`.
- [ ] At least one test asserting a constraint/error path the fake could not catch.
- [ ] A migration `up`/`down`/`up` reversibility test.

### Benchmark and `pprof`

- [ ] A `testing.B` benchmark for the hot path, with a package-level sink and `b.ReportAllocs()`.
- [ ] Sub-benchmarks across at least two input sizes (`b.Run`).
- [ ] A committed `cpu.out` (or its `top`/`list` text) showing the dominant cost.
- [ ] One optimization shipped, with a correctness test proving identical output.
- [ ] A committed `benchstat` table (`old.txt` vs `new.txt`, `-count=10`) showing the win with `p < 0.05`.

### Fuzz

- [ ] A `FuzzXxx` target against the input parser/validator, with `f.Add` seeds covering the known branches.
- [ ] The fuzz body asserts a real invariant (never-panic plus round-trip or output-validity), not nothing.
- [ ] The target found a real crash; the fix is in the code; the minimized crasher is committed under `testdata/fuzz/`.
- [ ] A short note (in `RESULTS.md`) of the crasher input and the fix.

### Hygiene

- [ ] `go build ./...`, `go vet ./...`, and `go test -race ./...` are all clean.
- [ ] A coverage report (`go tool cover -func` output) is in `RESULTS.md`, with one sentence on the holes it revealed.

---

## Day-by-day plan

### Thursday evening (1.5h) — Inventory and the unit-test floor

1. Take stock of your Week-7 `notes` service. List the pieces that have *no* test today.
2. Write the table-driven `ValidateNote` test first — it is the cheapest win and the template for everything else.
3. Write the hand-written `fakeRepo` and one service test against it. Confirm `go test ./...` is green.

### Friday (4h) — Unit suite, golden file, the fake-backed service tests

1. Round out the service tests: happy path, every validation error path, one induced repository error.
2. Add the golden-file render test; run `-update`, read the golden, commit it.
3. Add `httptest` handler tests if your handler layer has logic worth testing (status codes, content negotiation).
4. Run `go test -cover ./...`; open the HTML view; note the holes. Fill the ones that matter.

### Saturday (4h) — Benchmark, profile, optimize; fuzz

1. Pick the hot path (tag index, feed render, or list JSON). Write the benchmark with a sink and `ReportAllocs`.
2. Capture `cpu.out` and `mem.out`; read them with `pprof`; record the dominant cost.
3. Ship one optimization; add a correctness test for identical output; capture the `benchstat` delta.
4. Write the fuzz target against your input parser. Run `go test -fuzz=FuzzXxx -fuzztime=60s`. When it finds a crash, read the minimized input, fix the bug, re-run the crasher, commit it.

### Sunday (3h) — Integration suite, polish, write-up

1. Add the `testcontainers-go` integration suite behind the build tag; run `go test -tags=integration ./...`.
2. Add the migration reversibility test.
3. Run `go vet ./...` and `go test -race ./...`; fix anything.
4. Write `RESULTS.md`: the coverage summary, the `benchstat` table, the `pprof` finding, the fuzz crasher and fix.

---

## What you will be graded on

| Area                                                                 | Weight |
|----------------------------------------------------------------------|-------:|
| Unit tests (table-driven, fake repo, go-cmp, golden file, boundaries) |  25% |
| Integration tests (testcontainers Postgres, migrations, build tag, skip guard) |  20% |
| Benchmark + pprof + one measured optimization (sink, benchstat p<0.05, profile read) |  20% |
| Fuzz target (real invariant, found + fixed crash, committed crasher)  |  20% |
| Hygiene (clean `go vet`, `go test -race`, project structure)          |  10% |
| Coverage read as a signal (RESULTS.md write-up, not 100%-chasing)     |   5% |
| **Total**                                                             | **100%** |

The passing bar is **80**. The "you would put this behind a load balancer" bar is **90**. A submission with no found-and-fixed fuzz crash cannot exceed 80 — the fuzz find is the load-bearing skill of the week.

---

## A note on installing Docker and benchstat

Some learners will run this on a machine without Docker, or in a CI image without a daemon. The integration tests are written to skip cleanly in that case, so the rest of the lab runs unaffected — but you cannot *pass* the integration portion without Docker. Install Docker Desktop (<https://docs.docker.com/get-docker/>) or Colima (`brew install colima && colima start`, a lighter free alternative `testcontainers-go` drives identically). `benchstat` is a one-line install: `go install golang.org/x/perf/cmd/benchstat@latest`. `graphviz` (`brew install graphviz`) is only needed for the graphical flame graph (`go tool pprof -http`); `top` and `list` work without it. The unit tests, the benchmark, the `pprof` `top`/`list` reading, and the fuzzing all run with no Docker and no extra tooling at all.

---

## Submission

Zip the entire `notes/` tree (excluding `bin/`, build caches, and any `*.out` profile you do not want to ship — but **do** ship `cpu.out` or its text, `old.txt`/`new.txt`, and the committed crasher) as `notes-harden-<your-name>.zip`. Commit to your branch with the message:

```
mini-project: harden notes — layered test suite, measured optimization, fuzz crash fix
```

Push and open a PR against `main`. The PR description should include:

1. The output of `go test ./...` (green) and `go test -tags=integration ./...` (green, container started).
2. The `benchstat old.txt new.txt` table showing the optimization with `p < 0.05`.
3. The fuzz crasher input (e.g. `string("tag:")`) and a one-line description of the bug and the fix.
4. The `go tool cover -func` summary and one sentence on what the coverage holes told you.

If `go test ./...` is not green, or `go test -race ./...` reports a race, the PR is not reviewable — fix it first.
