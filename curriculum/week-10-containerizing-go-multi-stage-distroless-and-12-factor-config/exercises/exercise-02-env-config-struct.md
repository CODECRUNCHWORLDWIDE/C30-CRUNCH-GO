# Exercise 02 — The Validated Env-Only Config Struct

> **Time:** ~90 minutes. **Prerequisites:** Lecture 3. A `notes` service that currently reads config from flags or a file.

## Goal

Move every runtime setting `notes` has off command-line flags and config files into a single validated config struct loaded once from the environment at startup (12-factor, factor III). Fail fast and loud on a missing required setting; never log a secret.

## Steps

1. **Create `internal/config/config.go`** with a `Config` struct holding, at minimum: `HTTPAddr`, `GRPCAddr`, `MetricsAddr`, `DatabaseURL`, `OTLPEndpoint`, `LogLevel`, `ShutdownTimeout`, and `Environment`. Follow the Lecture 3 shape.

2. **Write `Load() (Config, error)`** that:
   - Reads each field from its `NOTES_*` environment variable.
   - Applies sensible dev defaults for everything except `NOTES_DATABASE_URL` (which has no default — it is required).
   - Returns an error naming the first missing required setting.
   - Requires `NOTES_OTLP_ENDPOINT` when `NOTES_ENV=prod`.

3. **Rewire `cmd/notesd/main.go`** to call `config.Load()` first, write a fatal config error to stderr and `os.Exit(1)` if it fails, then construct the logger, the pgx pool, the chi router, and the gRPC server from `cfg` — not from `os.Getenv` scattered through the code.

4. **Log a secret-free startup line**: report `database_configured: cfg.DatabaseURL != ""`, never the URL itself.

5. **Remove** any `flag.String` calls and any `config.yaml`/`config.dev.yaml` loading. Delete the files; add them to `.dockerignore` and `.gitignore` defensively.

6. **Write a table-driven test** `internal/config/config_test.go` that sets environment variables with `t.Setenv` and asserts: defaults apply, a missing `NOTES_DATABASE_URL` errors, an invalid `NOTES_LOG_LEVEL` falls back to info, and prod without an OTLP endpoint errors.

## Acceptance criteria

- `go test ./internal/config/...` is green.
- Running `notesd` with no `NOTES_DATABASE_URL` prints `config error: NOTES_DATABASE_URL is required` to stderr and exits 1.
- Running with the env set starts cleanly and the startup log line shows `database_configured: true`, never the URL.
- `grep -rn "os.Getenv" cmd/ internal/` shows reads only inside `internal/config` — nowhere else.
- No `flag.` calls and no checked-in config files remain.

## Stretch

Add a `Redacted() string` method on `Config` that returns the config as a log-safe string with the password in `DatabaseURL` masked (`postgres://notes:***@...`), and log it at debug level. Test that the password never appears in the output.
