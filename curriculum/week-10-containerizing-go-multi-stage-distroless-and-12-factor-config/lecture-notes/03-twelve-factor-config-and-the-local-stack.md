# Lecture 3 — 12-Factor Config, Logs to Stdout, the Stateless Process, and the Full Local Stack

## Why this lecture exists

Lectures 1 and 2 produced a small, static, non-root, shell-less image. This lecture makes that image *portable* — the same bytes running in dev, in the compose stack, and (next week) in Kubernetes, with the only difference being a handful of environment variables. That property is what the Twelve-Factor App calls config in the environment, and it is the difference between an image you can deploy anywhere and an image welded to one machine's `config.yaml`.

Three jobs. First, move every setting `notes` has into a validated config struct loaded once from the environment — and understand why a baked-in or file-based config is a rotation and leak hazard. Second, confirm the disposability factors the cluster will lean on: logs to stdout, a stateless process, fast startup. Third, write the `compose.yaml` that brings up the whole stack — `notesd` + Postgres + Jaeger + Prometheus + Grafana — wired by environment variables, so the local topology mirrors the cloud one.

The reference is the Twelve-Factor App at <https://12factor.net/>, specifically factor III (config) at <https://12factor.net/config>, factor XI (logs) at <https://12factor.net/logs>, factor VI (processes) at <https://12factor.net/processes>, and factor IX (disposability) at <https://12factor.net/disposability>. Read all four — they are each two paragraphs and they are the spine of the week.

## Factor III: config in the environment

The Twelve-Factor rule is blunt: "store config in the environment." Config is everything that varies between deploys — the database URL, the OTLP collector endpoint, the log level, the listen addresses, feature flags, credentials. It does *not* include things that are the same everywhere (the routes, the business logic). The test the spec offers: *could you open-source the repo this minute without leaking any credentials?* If a password lives in a checked-in file, the answer is no, and the config is in the wrong place.

The three wrong places, and why:

```
+------------------------------+-------------------------------------------------------+
| Where config lives           | Why it is wrong                                       |
+------------------------------+-------------------------------------------------------+
| Compiled into the binary     | Cannot change a password without a recompile +        |
|   (const dbURL = "...")       | redeploy; the secret is in the binary forever         |
| Baked into an image layer    | Recoverable via `docker history` / layer tarball even |
|   (COPY config.prod.yaml)     | after "removal"; same image cannot serve two envs     |
| A file mounted at a fixed    | Closer, but still a file to manage, template, and     |
|   path read at startup        | keep out of Git; env is simpler and platform-native   |
+------------------------------+-------------------------------------------------------+
```

The right place is the process environment. In Kubernetes the values come from a ConfigMap (non-secret) and a Secret (credentials) projected as environment variables (Week 11). In the compose stack they come from the `environment:` block. On your laptop they come from `export` or a `.env` that is **gitignored and never copied into the image**. The same image reads `NOTES_DATABASE_URL` in all three; only the value differs. Citation: <https://12factor.net/config>.

## The validated config struct for `notes`

Load config once, at startup, from the environment; validate it; fail fast with a clear message if anything required is missing; and never read the environment again. A config struct is the single source of truth that the rest of the program is handed:

```go
// internal/config/config.go
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"time"
)

// Config holds every runtime setting, loaded once from the environment.
// Nothing in the program reads os.Getenv after Load returns.
type Config struct {
	HTTPAddr        string        // NOTES_HTTP_ADDR        e.g. ":8080"
	GRPCAddr        string        // NOTES_GRPC_ADDR        e.g. ":9090"
	MetricsAddr     string        // NOTES_METRICS_ADDR     e.g. ":2112"
	DatabaseURL     string        // NOTES_DATABASE_URL     (required, secret)
	OTLPEndpoint    string        // NOTES_OTLP_ENDPOINT    e.g. "jaeger:4318"
	LogLevel        slog.Level    // NOTES_LOG_LEVEL        debug|info|warn|error
	ShutdownTimeout time.Duration // NOTES_SHUTDOWN_TIMEOUT e.g. "15s"
	Environment     string        // NOTES_ENV             dev|staging|prod
}

// Load reads the environment, applies defaults, validates, and returns a
// Config or an error naming the first thing that is wrong. Call it once in main.
func Load() (Config, error) {
	cfg := Config{
		HTTPAddr:        getenv("NOTES_HTTP_ADDR", ":8080"),
		GRPCAddr:        getenv("NOTES_GRPC_ADDR", ":9090"),
		MetricsAddr:     getenv("NOTES_METRICS_ADDR", ":2112"),
		DatabaseURL:     os.Getenv("NOTES_DATABASE_URL"), // no default: required
		OTLPEndpoint:    getenv("NOTES_OTLP_ENDPOINT", ""),
		Environment:     getenv("NOTES_ENV", "dev"),
		LogLevel:        parseLevel(getenv("NOTES_LOG_LEVEL", "info")),
		ShutdownTimeout: parseDuration(getenv("NOTES_SHUTDOWN_TIMEOUT", "15s")),
	}

	// Validate: the database URL is the one truly required setting. Fail fast
	// and loud — a service that boots with no DB and only discovers it on the
	// first request is harder to diagnose than one that refuses to start.
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("NOTES_DATABASE_URL is required")
	}
	if cfg.Environment == "prod" && cfg.OTLPEndpoint == "" {
		return Config{}, fmt.Errorf("NOTES_OTLP_ENDPOINT is required in prod")
	}
	return cfg, nil
}

func getenv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func parseLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func parseDuration(s string) time.Duration {
	d, err := time.ParseDuration(s)
	if err != nil {
		return 15 * time.Second
	}
	return d
}

// (strconv is imported for callers that add integer settings, e.g. pool size.)
var _ = strconv.Atoi
```

And `main` loads it once and hands it down:

```go
// cmd/notesd/main.go
func main() {
	cfg, err := config.Load()
	if err != nil {
		// No logger configured yet, so write the fatal config error to stderr
		// in plain text and exit non-zero. A bad config must not boot.
		fmt.Fprintf(os.Stderr, "config error: %v\n", err)
		os.Exit(1)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	logger.Info("starting notesd",
		"env", cfg.Environment,
		"http_addr", cfg.HTTPAddr,
		"grpc_addr", cfg.GRPCAddr,
		// NEVER log cfg.DatabaseURL — it carries the password. Log that it is set.
		"database_configured", cfg.DatabaseURL != "",
	)
	// ... wire the pgx pool, the chi router, the gRPC server, the OTel exporter,
	//     all from cfg, then serve. (Graceful shutdown is Week 11.)
}
```

Three disciplines in that code. **Required values have no default and fail loud** — `NOTES_DATABASE_URL` missing is a startup error, not a `nil` pool discovered on the first request. **Defaults are sensible for dev** — the ports and log level default so a developer can `docker run` with only the database URL set. **Secrets are never logged** — the startup log line reports `database_configured: true`, not the URL, because the URL carries the password and logs are not a secret store. The Week 9 `slog` work is what makes the structured, secret-free startup line natural.

```mermaid
flowchart TD
  A["Load reads the environment"] --> B{"NOTES_DATABASE_URL set"}
  B -->|"No"| C["Return error fail fast"]
  B -->|"Yes"| D{"Environment is prod and OTLP endpoint empty"}
  D -->|"Yes"| C
  D -->|"No"| E["Return validated Config"]
```
*Config loading fails fast and loud on the first missing required setting rather than booting with a hidden gap.*

This is deliberately *not* a config-framework. Go's standard library and a 60-line struct cover it; reaching for Viper or a config DSL adds a dependency and a file format to keep config *out* of, which is the opposite of the goal. Twelve-factor config is environment variables and a struct. Citation: <https://12factor.net/config> and the `os.LookupEnv` doc at <https://pkg.go.dev/os#LookupEnv>.

## Factor XI: logs to stdout, as an event stream

A 12-factor process does not manage its own log files, rotation, or routing. It writes its event stream to stdout (and stderr for fatal pre-logger errors) as a stream of events, and lets the execution environment capture and route it. Week 9 already did this: `slog.NewJSONHandler(os.Stdout, ...)` writes one JSON object per line to stdout. In a container, the runtime captures stdout; in Kubernetes, `kubectl logs` reads it and the cluster's log pipeline (Fluent Bit, Loki, whatever) ships it. The service knows nothing about any of that — it writes lines.

What this *forbids*: writing to `/var/log/notes.log` inside the container. That file lives on the container's ephemeral filesystem, disappears when the pod is rescheduled, and is invisible to `kubectl logs`. A service that logs to a file in a container is logging into a black hole. The fix is always the same — log to stdout, let the platform route. Citation: <https://12factor.net/logs>.

A related hardening that follows: with logs on stdout and config in the environment, the container's *root filesystem can be read-only* (`readOnlyRootFilesystem: true` in the Week 11 securityContext). The service writes nothing to disk except `/tmp`. A read-only root filesystem means a compromised process cannot drop a binary, modify the app, or persist — another cheap, high-value hardening that distroless + stdout logging makes possible.

## Factor VI & IX: stateless and disposable

**Factor VI — stateless processes.** All durable state lives in Postgres. The `notes` process holds nothing on local disk it cannot lose: no session files, no uploaded blobs on the container filesystem, no in-memory cache it would be wrong to lose. This is what lets Kubernetes run *N* replicas of the same image behind a Service and round-robin requests across them — any replica can serve any request because no request depends on state that lives only in one replica. If `notes` kept per-session state in memory, two replicas would disagree and the round-robin would break. State goes in Postgres (Week 6); the process is a stateless function of it. Citation: <https://12factor.net/processes>.

**Factor IX — disposability.** A 12-factor process starts fast and shuts down gracefully. Fast startup: a static Go binary on distroless starts in milliseconds — no runtime to load, no JIT, no framework boot — which is exactly why it suits a cluster that may start and stop replicas constantly (scaling, rescheduling, rolling deploys). Graceful shutdown: on `SIGTERM` the process should stop accepting new work, finish in-flight work, close its connections, and exit — *not* drop in-flight requests on the floor. This week we confirm the startup half (the binary is fast) and the stdout/stateless groundwork; **Week 11 makes the graceful-shutdown half rigorous** — it is the heart of next week's lecture, because graceful shutdown is `context` cancellation (Week 4) applied to the whole process under the cluster's termination grace period. Citation: <https://12factor.net/disposability>.

```
   the disposability arc across Weeks 10-11
   ----------------------------------------
   Week 10 (this week): fast startup (static binary), logs to stdout, stateless
   Week 11 (next week):  graceful shutdown on SIGTERM within the grace period
   -> together: a process the cluster can start, stop, and reschedule freely
```

## The full local stack — `compose.yaml`

The capstone deploys `notes` to Kubernetes against Postgres, Jaeger, Prometheus, and Grafana. Before trusting the cluster, prove the image composes with its dependencies locally. A `compose.yaml` at the repo root brings the lot up, wired by environment variables, so the same image you ship to `kind` next week runs against the same dependency *shape* it will meet there:

```mermaid
flowchart LR
  PG["Postgres 16"] -->|"health check gate"| N["notes service"]
  N -->|"trace export"| J["Jaeger OTLP receiver"]
  N -->|"metrics endpoint"| P["Prometheus scrape"]
  P -->|"queries"| G["Grafana dashboard"]
```
*The compose stack wires notes to its dependencies the same way the cluster will next week.*

```yaml
# compose.yaml — the full local stack: notes + Postgres + observability.
name: notes-stack

services:
  postgres:
    image: postgres:16
    environment:
      POSTGRES_DB: notes
      POSTGRES_USER: notes
      POSTGRES_PASSWORD: devpass
    ports: ["5432:5432"]
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U notes -d notes"]
      interval: 5s
      timeout: 3s
      retries: 10
    volumes:
      - pgdata:/var/lib/postgresql/data

  jaeger:
    image: jaegertracing/all-in-one:1.57
    environment:
      COLLECTOR_OTLP_ENABLED: "true"
    ports:
      - "16686:16686"   # Jaeger UI
      - "4318:4318"     # OTLP/HTTP receiver (the OTel SDK exports here)

  prometheus:
    image: prom/prometheus:v2.52.0
    volumes:
      - ./deploy/observability/prometheus.yml:/etc/prometheus/prometheus.yml:ro
    ports: ["9091:9090"]   # host 9091 -> Prometheus 9090 (9090 is the gRPC port)

  grafana:
    image: grafana/grafana:11.0.0
    environment:
      GF_AUTH_ANONYMOUS_ENABLED: "true"
      GF_AUTH_ANONYMOUS_ORG_ROLE: Admin
    volumes:
      - ./deploy/observability/grafana/provisioning:/etc/grafana/provisioning:ro
      - ./deploy/observability/grafana/dashboards:/var/lib/grafana/dashboards:ro
    ports: ["3000:3000"]
    depends_on: [prometheus]

  notes:
    build:
      context: .
      dockerfile: Dockerfile
    depends_on:
      postgres:
        condition: service_healthy   # wait for the DB before the readiness probe polls
      jaeger:
        condition: service_started
    environment:
      NOTES_ENV: dev
      NOTES_HTTP_ADDR: ":8080"
      NOTES_GRPC_ADDR: ":9090"
      NOTES_METRICS_ADDR: ":2112"
      NOTES_DATABASE_URL: "postgres://notes:devpass@postgres:5432/notes?sslmode=disable"
      NOTES_OTLP_ENDPOINT: "jaeger:4318"
      NOTES_LOG_LEVEL: "info"
    ports:
      - "8080:8080"   # REST
      - "9090:9090"   # gRPC
      - "2112:2112"   # /metrics

volumes:
  pgdata:
```

`docker compose up --build` brings the lot online. The `depends_on ... condition: service_healthy` on Postgres makes `notes` wait for `pg_isready` to pass before it starts, so the readiness work (Week 11) finds a reachable database on first poll rather than racing the DB's boot. The `notes` service is configured **entirely through `environment:`** — the same image that runs here, with `NOTES_DATABASE_URL` pointing at the compose Postgres, will run in `kind` with `NOTES_DATABASE_URL` pointing at the cluster Postgres, no rebuild. Citation: <https://docs.docker.com/compose/> and the `depends_on` healthcheck reference at <https://docs.docker.com/reference/compose-file/services/#depends_on>.

Verify the stack:

```bash
docker compose up --build -d
# Wait for Postgres healthy, then notes starts and runs migrations.

curl -s http://localhost:8080/healthz     # ok          (liveness — no DB)
curl -s http://localhost:8080/readyz      # ok          (readiness — DB reachable)
curl -s -XPOST http://localhost:8080/notes -d '{"title":"first","body":"hello"}'
curl -s http://localhost:8080/notes       # the note, served from Postgres

# The trace shows up in Jaeger:
open http://localhost:16686               # search service "notes", see the span tree
# The metrics scrape and the dashboard:
open http://localhost:3000                # Grafana, the RED dashboard from Week 9
```

This compose file is the local mirror of the cloud topology — the same image, the same dependency shape, one `up` away. It is also the answer to "does my container actually work with its dependencies" *before* you spend an hour debugging it in `kind`.

## What we built

By the end of Lecture 3, the repo has:

- A validated, env-only config struct loaded once at startup, failing fast and loud on a missing required setting, and never logging a secret.
- A service that logs JSON to stdout (factor XI), holds no state on disk (factor VI), and starts fast (factor IX) — the disposability groundwork the cluster assumes, with graceful shutdown reserved for Week 11.
- A `compose.yaml` that brings up `notes` + Postgres + Jaeger + Prometheus + Grafana with one command, wired entirely by environment variables, mirroring the cloud topology.
- The proof that the *same image* runs in dev and in the stack, with only environment values changing — the 12-factor portability the capstone depends on.

The image is now small, static, non-root, shell-less, and configured entirely from the environment. It runs identically wherever you point its environment variables. Next week (Week 11) it goes to Kubernetes — and the disposability story finishes with a graceful shutdown that drains in-flight work on `SIGTERM` instead of dropping it.
