# Week 10 Exercise Solutions

Worked solutions to the three exercises: the canonical implementation, the verification output the grader looks for, and the most common ways each gets done wrong. Read your own solution first; check it against the canonical one second. The point is not to copy — it is to surface the patterns and the failure modes so you recognize them when your own build goes red.

---

## Exercise 01 — The multi-stage distroless Dockerfile

The canonical file, in full, so you can diff yours line for line:

```dockerfile
# syntax=docker/dockerfile:1
# ---- build stage -----------------------------------------------------------
FROM golang:1.22 AS build
WORKDIR /src

# Module download cached on a layer keyed only on go.mod/go.sum.
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Source copied after the download — editing a .go file does not bust the
# download layer.
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" -o /out/notesd ./cmd/notesd

# ---- runtime stage ---------------------------------------------------------
FROM gcr.io/distroless/static-debian12:nonroot AS final
COPY --from=build /out/notesd /notesd
USER 65532:65532
EXPOSE 8080 9090 2112
ENTRYPOINT ["/notesd"]
```

And the `.dockerignore`:

```
.git/
.github/
*.env
.env*
**/testdata/
compose.yaml
compose.*.yaml
Dockerfile
.dockerignore
deploy/
*.md
bin/
tmp/
```

`CGO_ENABLED=0` is the load-bearing line: it makes the binary fully static so it runs on distroless/static. `-ldflags="-s -w"` drops the symbol table and DWARF info (~22 MB → ~16 MB). `-trimpath` removes build-machine paths for reproducibility.

### Verification output

1. `docker build -t notesd:local .` succeeds; the build log shows a small "transferring context" (a few hundred KB) thanks to `.dockerignore`.
2. A second build after editing a `.go` file shows `CACHED` on the `go mod download` step.
3. `file /tmp/notesd` says `statically linked`; `ldd /tmp/notesd` says `not a dynamic executable`.
4. `docker images notesd:local` prints ~16–20 MB.
5. `docker inspect notesd:local --format '{{.Config.User}}'` prints `65532:65532`.
6. `curl http://localhost:8080/healthz` returns `ok`.
7. `docker history notesd:local` shows the `COPY /out/notesd` layer as the only owned layer, with no SDK-sized layer above the base.

```text
docker images:  ~18 MB
docker history notesd:local --format "{{.Size}}\t{{.CreatedBy}}":
  0B      ENTRYPOINT ["/notesd"]
  0B      USER 65532:65532
  0B      EXPOSE 8080 9090 2112
  16.2MB  COPY /out/notesd /notesd # buildkit     <- yours
  2.4MB   (distroless base)
```

### Common stumbles

The "image is ~850 MB" mistake: the final `FROM` is `golang:1.22`, not a runtime base — you wrote a single-stage build, or forgot the second `FROM`. The final `FROM` must be `distroless/static` (or another runtime base).

The "container exits immediately with 'no such file or directory'": you built with cgo on (omitted `CGO_ENABLED=0`), so the binary is dynamically linked against libc, and distroless/static has no libc and no dynamic linker. The error names the binary but means its *interpreter* is missing. Fix: `CGO_ENABLED=0`.

The "TLS handshake fails to the OTLP endpoint" on a `scratch` build: `scratch` has no CA certs. Either copy `/etc/ssl/certs/ca-certificates.crt` in, or use distroless/static (which ships them). This is the whole reason we default to distroless, not `scratch`.

The "second build is just as slow as the first": you wrote `COPY . .` before `go mod download`, so any source edit busts the download layer. Or your `.dockerignore` is missing and `bin/`/`tmp/` churn busts the `COPY . .` cache. Copy the module files and download first; keep the context clean.

The "permission denied writing /tmp" at runtime: only if you set `readOnlyRootFilesystem` without mounting a writable `/tmp`. Distroless ships a writable `/tmp`; the issue surfaces in Week 11's securityContext and is fixed with an `emptyDir` mount there, not here.

---

## Exercise 02 — The validated env-only config struct

The canonical `internal/config/config.go` is the Lecture 3 struct and `Load`. The test is the part most people skip and the part the grader checks hardest:

```go
// internal/config/config_test.go
package config

import "testing"

func TestLoad(t *testing.T) {
	t.Run("defaults apply when only DB URL is set", func(t *testing.T) {
		t.Setenv("NOTES_DATABASE_URL", "postgres://u:p@h:5432/d")
		cfg, err := Load()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.HTTPAddr != ":8080" {
			t.Errorf("HTTPAddr = %q, want :8080", cfg.HTTPAddr)
		}
		if cfg.LogLevel.String() != "INFO" {
			t.Errorf("LogLevel = %v, want INFO", cfg.LogLevel)
		}
	})

	t.Run("missing database URL is an error", func(t *testing.T) {
		// t.Setenv ensures isolation; the var is unset in this subtest.
		_, err := Load()
		if err == nil {
			t.Fatal("expected error for missing NOTES_DATABASE_URL, got nil")
		}
	})

	t.Run("prod without OTLP endpoint is an error", func(t *testing.T) {
		t.Setenv("NOTES_DATABASE_URL", "postgres://u:p@h:5432/d")
		t.Setenv("NOTES_ENV", "prod")
		if _, err := Load(); err == nil {
			t.Fatal("expected error for prod without OTLP endpoint")
		}
	})

	t.Run("invalid log level falls back to info", func(t *testing.T) {
		t.Setenv("NOTES_DATABASE_URL", "postgres://u:p@h:5432/d")
		t.Setenv("NOTES_LOG_LEVEL", "nonsense")
		cfg, _ := Load()
		if cfg.LogLevel.String() != "INFO" {
			t.Errorf("LogLevel = %v, want INFO fallback", cfg.LogLevel)
		}
	})
}
```

### Verification output

1. `go test ./internal/config/...` is green; all four subtests pass.
2. `NOTES_DATABASE_URL= notesd` (unset DB) prints `config error: NOTES_DATABASE_URL is required` to stderr and exits 1 (`echo $?` → 1).
3. With the env set, the startup line is JSON on stdout with `"database_configured":true` and **no `database_url` field**.
4. `grep -rn "os.Getenv" cmd/ internal/` returns hits only under `internal/config`.

### Common stumbles

The "secret in the logs" mistake: logging `"database_url", cfg.DatabaseURL`. The URL carries the password; logs are not a secret store and may ship to a third-party aggregator. Log `database_configured: true`, or a `Redacted()` form (the stretch).

The "boots without a database" mistake: giving `DatabaseURL` a default. A required setting must have no default so a missing value fails at startup, loud, rather than surfacing as a nil-pool panic on the first request. Fail fast.

The "still reads os.Getenv everywhere" mistake: leaving `os.Getenv("SOMETHING")` calls in handlers or the pool setup. Config is loaded *once* and passed down; scattered reads mean config can change mid-process and can't be validated or tested in one place. The grep is how the grader catches it.

The "uses a config framework" overreach: pulling in Viper or a YAML loader. Twelve-factor config is environment variables; a 60-line struct and `os.LookupEnv` is the whole solution. A framework adds a dependency and a file format you then have to keep config *out* of.

---

## Exercise 03 — The full local stack with Compose

The canonical `compose.yaml` is the Lecture 3 file. The `prometheus.yml` it mounts:

```yaml
# deploy/observability/prometheus.yml
global:
  scrape_interval: 5s
scrape_configs:
  - job_name: notes
    static_configs:
      - targets: ["notes:2112"]   # the in-network service name + metrics port
```

### Verification output

1. `docker compose up --build` brings up all five services; `docker compose ps` shows `notes` started *after* `postgres` is healthy.
2. `curl http://localhost:8080/healthz` → `ok`; `curl http://localhost:8080/readyz` → `ok`.
3. `POST /notes` then `GET /notes` round-trips through the compose Postgres.
4. `http://localhost:16686` (Jaeger) shows the trace for the request with the handler → service → Postgres spans.
5. `http://localhost:9091/targets` shows the `notes` target `UP`; the Grafana RED dashboard at `http://localhost:3000` reflects the traffic.
6. `docker compose down -v` removes the stack and the Postgres volume.

### Common stumbles

The "`notes` connection-refused to Postgres": you pointed `NOTES_DATABASE_URL` at `localhost` instead of the service name `postgres`. Inside the compose network, services reach each other by service name, not `localhost` — `localhost` in the `notes` container is the `notes` container itself.

The "`notes` started before the DB and crashed": you omitted `depends_on: condition: service_healthy`, so `notes` raced Postgres's boot and failed its first connection. The healthcheck gate is what serializes them. (A robust service also retries the connection — but the gate makes the demo reliable.)

The "Prometheus target is DOWN": the scrape config points at `localhost:2112` (Prometheus's own localhost) instead of `notes:2112`. From Prometheus's perspective the metrics endpoint is on the `notes` service, by name, in the compose network.

The "port 9090 collision": you mapped Prometheus to host 9090, which collides with the `notes` gRPC port. Map Prometheus to host 9091 (or any free port); the container-internal 9090 is fine, it is the host-side mapping that collides.

The "OTLP export silently does nothing": `NOTES_OTLP_ENDPOINT` points at `localhost:4318` instead of `jaeger:4318`. Same lesson as the DB — in-network, by service name.

---

## Synthesis — how the three exercises connect

The three exercises are the three properties the week promised, in order:

- **Exercise 01** produced the **artifact**: the same `notesd` you built across Weeks 5–9, now a ~18 MB static, non-root, distroless image instead of an `850 MB` toolchain dump.
- **Exercise 02** produced the **portability**: every setting in the environment, so the *same image* runs in dev, in the stack, and in the cluster, with only env values changing.
- **Exercise 03** produced the **proof**: the full stack composing locally, so "the container works with its dependencies" is something you saw, not something you hope holds in `kind`.

Stacked, they are the container contract Week 11 deploys: one image, configured from the environment, that runs identically wherever you point its variables — and that is exactly what a Kubernetes ConfigMap, Secret, and Deployment expect to be handed next week.
