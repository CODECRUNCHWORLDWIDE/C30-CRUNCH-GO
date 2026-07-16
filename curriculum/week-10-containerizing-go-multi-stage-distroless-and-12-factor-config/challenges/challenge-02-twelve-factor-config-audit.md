# Challenge 2 — Audit `notes` Against the 12-Factor Config, Logs, and Disposability Factors and Fix Every Violation

> **Time:** 2 hours. **Prerequisites:** Lecture 3; Exercise 2. A `notes` service with the env config struct. **Citations:** the Twelve-Factor App at <https://12factor.net/>, factor III (config) at <https://12factor.net/config>, factor VI (processes) at <https://12factor.net/processes>, factor XI (logs) at <https://12factor.net/logs>, and factor IX (disposability) at <https://12factor.net/disposability>.

## The premise

"12-factor" is easy to nod along to and hard to actually pass. This challenge makes you audit `notes` against four specific factors — config (III), processes (VI), logs (XI), and disposability (IX) — find every violation, fix it, and write a one-line justification per factor with a citation. The skill is not reciting the twelve factors; it is looking at *your* code and finding the line that violates one.

The audit matters because Kubernetes (next week) *assumes* a 12-factor process. It projects config as environment variables, captures stdout as logs, runs N stateless replicas, and sends `SIGTERM` to dispose of a pod. A service that violates any of these fights the platform: a file-config service needs a mounted ConfigMap volume and a reload mechanism; a file-logging service logs into a black hole; a stateful service breaks under N replicas; a service that ignores `SIGTERM` gets `SIGKILL`ed mid-request and drops it. Passing the factors is what makes the cluster's defaults Just Work.

## The audit checklist

For each factor, find every place the code touches it, state pass/fail, and fix the failures. Produce a table.

### Factor III — Config in the environment (<https://12factor.net/config>)

```text
[ ] Every setting that varies between deploys is read from an env var (not a const, not a file).
[ ] No credential is compiled into the binary or baked into an image layer.
[ ] grep -rn "os.Getenv" finds reads only in internal/config (loaded once, passed down).
[ ] No checked-in config.yaml / config.dev.yaml; the .dockerignore and .gitignore exclude *.env.
[ ] The repo could be open-sourced this minute without leaking a credential.
```

The tell of a violation: a `const dbHost = "..."`, a `COPY config.prod.yaml` in the Dockerfile, or an `os.Getenv` in a handler. Fix: move it into the `Config` struct and pass `cfg` down.

### Factor XI — Logs as event streams to stdout (<https://12factor.net/logs>)

```text
[ ] All logging goes to stdout (and stderr for fatal pre-logger errors) via slog.
[ ] No code opens a log file (no os.OpenFile("/var/log/...")), no log rotation in-process.
[ ] No secret is ever logged (no database_url, no bearer token, no password field).
[ ] The log format is structured JSON (machine-parseable downstream).
```

The tell: a `log.SetOutput(file)`, a `lumberjack` rotation dependency, or a `logger.Info("url", cfg.DatabaseURL)`. Fix: `slog.NewJSONHandler(os.Stdout, ...)`; delete the file logging; log `database_configured: true`.

### Factor VI — Stateless processes (<https://12factor.net/processes>)

```text
[ ] No request depends on state held only in one process (no in-memory session store,
    no local-disk uploads, no in-process cache that would be wrong to lose).
[ ] All durable state is in Postgres.
[ ] The service runs correctly with N replicas behind a load balancer (any replica
    can serve any request).
```

The tell: a `map[sessionID]Session` in a package-level variable, a file written to `./uploads/`, a counter incremented in memory that is reported as truth. Fix: move state to Postgres (Week 6) or accept that a cache is a cache (lossy, rebuildable), not state.

### Factor IX — Disposability (<https://12factor.net/disposability>)

```text
[ ] The process starts fast (a static Go binary: milliseconds — verify with `time docker run ... --version`).
[ ] The process does not depend on a long warm-up before it can serve.
[ ] (Week 11 completes this) The process handles SIGTERM gracefully — for now,
    confirm there is at least a signal handler stub and the shutdown timeout is in config.
```

The tell for startup: a service that builds a large in-memory index before serving (slow start). The tell for shutdown: `main` ending in `select{}` or `http.ListenAndServe` with no signal handling — a `SIGTERM` then becomes a `SIGKILL` that drops in-flight requests. Note this as a finding to *complete in Week 11*; this week, ensure the `ShutdownTimeout` is in config and there is a TODO at the signal-handling seam.

## Acceptance criteria

1. An `AUDIT-12FACTOR.md` with a table: factor | location(s) in the code | pass/fail before | fix applied | citation.
2. Every factor-III, factor-XI, and factor-VI violation is *fixed in the code* (the factor-IX graceful-shutdown item may be a documented Week-11 TODO, since it is next week's lecture).
3. The `grep -rn "os.Getenv" cmd/ internal/` output, showing reads only in `internal/config`.
4. A `grep` for log-file opens and secret logging, showing none remain.
5. A demonstration of statelessness: run two instances of the image against the same Postgres on different ports, round-robin a few requests by hand, and show both serve correctly (no request needs state the other lacks).
6. One sentence per factor justifying the pass, each with its 12factor.net citation.

## Stretch goals

1. **The full twelve.** Extend the audit to the remaining eight factors (codebase, dependencies, backing services, build/release/run, port binding, concurrency, dev/prod parity, admin processes) and note which `notes` already passes (it passes most for free, being a Go service) and which need a sentence of justification. Cite <https://12factor.net/>.
2. **Read-only root filesystem.** Run the image with `docker run --read-only --tmpfs /tmp notesd:local` and confirm it still serves. This proves the service writes nothing to disk except `/tmp` — the precondition for the Week 11 `readOnlyRootFilesystem: true` securityContext.
3. **Secret recovery proof.** Deliberately `COPY .env` into a throwaway image, then recover the password from it with `docker history --no-trunc` or by extracting the layer tarball. This proves *why* a baked-in secret is a leak even after "removal" — the layer that added it persists.

## Deliverable

`AUDIT-12FACTOR.md` in the `notes` repo: the four-factor table with before/after and fixes, the grep evidence, the two-replica statelessness demo, and the per-factor justification with citations. This backs the capstone's "cloud-native posture" axis. The line this challenge defends: *the same image runs in dev, in the compose stack, and in the cluster because it reads all config from the environment, logs to stdout, holds no state, and starts in milliseconds — the four factors the cluster assumes and the four this audit proves.*
