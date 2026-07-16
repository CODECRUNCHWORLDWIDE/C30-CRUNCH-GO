# Week 10 Resources — Containerizing Go: Multi-Stage, Distroless, 12-Factor

The canonical reading list for Week 10. Every URL has been opened and every technique referenced by the lectures, exercises, challenges, or the lab. Read what you need when you need it; the lecture notes tell you which section of which document is load-bearing for the technique under discussion.

Grouped by the role the document plays in the container story — multi-stage builds, the static Go binary, distroless and image hardening, the build cache, 12-factor config, the local stack, and image measurement. The "adjacent" section is the most valuable for the engineer who wants to outgrow the lectures; do not skip it.

## Multi-stage Dockerfiles

- **Multi-stage builds (the canonical guide)** — <https://docs.docker.com/build/building/multi-stage/>. The build-then-run two-stage pattern; copying only the artifact across the stage boundary. Read this before Lecture 1.
- **Dockerfile reference** — <https://docs.docker.com/reference/dockerfile/>. Every instruction: `FROM ... AS`, `COPY --from`, `USER`, `EXPOSE`, `ENTRYPOINT`. The `USER` instruction in particular at <https://docs.docker.com/reference/dockerfile/#user>.
- **Multi-platform builds** — <https://docs.docker.com/build/building/multi-platform/>. `TARGETOS`/`TARGETARCH`, `--platform=$BUILDPLATFORM`, building amd64 and arm64 from one Dockerfile.
- **Best practices for Dockerfiles** — <https://docs.docker.com/develop/develop-images/dockerfile_best-practices/>. Layer ordering, minimizing layers, the `.dockerignore`.

## The static Go binary

- **`cmd/cgo`** — <https://pkg.go.dev/cmd/cgo>. What cgo is; what `CGO_ENABLED=0` turns off; why a cgo binary needs libc at runtime.
- **`net` — name resolution** — <https://pkg.go.dev/net#hdr-Name_Resolution>. The pure-Go vs cgo DNS resolver; when each is used; the `netgo`/`netcgo` build tags.
- **`go build` and `go help build`** — <https://pkg.go.dev/cmd/go#hdr-Compile_packages_and_dependencies>. The `-ldflags`, `-trimpath`, `-tags` flags used in the build.
- **Reducing Go binary size (the linker flags)** — <https://go.dev/doc/gdb> and the `-ldflags="-s -w"` discussion in the linker docs at <https://pkg.go.dev/cmd/link>. What `-s` (symbol table) and `-w` (DWARF) drop and what you lose.
- **`govulncheck`** — <https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck>. Scanning the *code paths your binary reaches* for CVEs in Go module dependencies — complements an image CVE scan.

## Distroless and image hardening

- **GoogleContainerTools/distroless (the repository)** — <https://github.com/GoogleContainerTools/distroless>. The source of every distroless image; the "why distroless" rationale at <https://github.com/GoogleContainerTools/distroless#why-should-i-use-distroless-images>.
- **The distroless `static` base README** — <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>. Exactly what `static`, `base`, and `cc` ship; the `nonroot` user; the `:debug` variants with a shell.
- **Docker `USER` and non-root containers** — <https://docs.docker.com/reference/dockerfile/#user>. Setting the runtime user; the high-port consequence.
- **Kubernetes pod-security-standards** — <https://kubernetes.io/docs/concepts/security/pod-security-standards/>. The `restricted` profile — `runAsNonRoot`, `readOnlyRootFilesystem`, dropped capabilities — that Week 11 enforces.
- **Kubernetes ephemeral debug containers** — <https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container>. How to attach a shell to a shell-less pod when you genuinely need one.
- **Docker Scout** — <https://docs.docker.com/scout/>. Image CVE scanning; the "smaller is safer" CVE-count claim in challenge-01.
- **Trivy** — <https://github.com/aquasecurity/trivy>. An alternative open-source image and filesystem vulnerability scanner.

## The build cache

- **Docker build cache** — <https://docs.docker.com/build/cache/>. Why copying `go.mod`/`go.sum` and downloading before `COPY . .` is the biggest build-time win.
- **Optimize the build cache (cache mounts)** — <https://docs.docker.com/build/cache/optimize/>. The `--mount=type=cache` syntax for `/go/pkg/mod` and `/root/.cache/go-build`.
- **The GHA cache backend** — <https://docs.docker.com/build/cache/backends/gha/>. Persisting the cache across cold CI runners (out of scope this week, flagged for the capstone CI).
- **`.dockerignore` and build context** — <https://docs.docker.com/build/concepts/context/#dockerignore-files>. Keeping `.git/`, `*.env`, and the manifests out of the context.

## 12-factor config

- **The Twelve-Factor App** — <https://12factor.net/>. The whole methodology; read the index, then the four load-bearing factors below.
- **Factor III — Config** — <https://12factor.net/config>. Config in the environment; the "could you open-source it now" test.
- **Factor VI — Processes** — <https://12factor.net/processes>. Stateless, share-nothing processes; state in a backing service.
- **Factor IX — Disposability** — <https://12factor.net/disposability>. Fast startup, graceful shutdown on `SIGTERM` (the Week 11 half).
- **Factor XI — Logs** — <https://12factor.net/logs>. Logs as event streams to stdout; the platform routes them.
- **`os.LookupEnv` / `os.Getenv`** — <https://pkg.go.dev/os#LookupEnv>. Reading the environment; `LookupEnv` distinguishes "unset" from "empty," which matters for required-vs-default.
- **`os/signal`** — <https://pkg.go.dev/os/signal>. `signal.NotifyContext` for `SIGTERM`/`SIGINT` — the graceful-shutdown primitive previewed here and built in Week 11.

## The local stack

- **Compose specification** — <https://docs.docker.com/compose/>. The `compose.yaml` schema; services, networks, volumes.
- **Compose `depends_on` and healthchecks** — <https://docs.docker.com/reference/compose-file/services/#depends_on>. The `condition: service_healthy` gate that serializes the service behind Postgres's boot.
- **Compose `healthcheck`** — <https://docs.docker.com/reference/compose-file/services/#healthcheck>. The `pg_isready` healthcheck on the Postgres service.
- **Postgres official image** — <https://hub.docker.com/_/postgres>. The `POSTGRES_*` env vars; the data volume.
- **Jaeger all-in-one** — <https://www.jaegertracing.io/docs/latest/getting-started/>. The OTLP-enabled all-in-one image; the UI on 16686, OTLP/HTTP on 4318.

## Image measurement and tooling

- **`docker images` / `docker history`** — <https://docs.docker.com/reference/cli/docker/image/history/>. The size and layer breakdown; proving only the binary layer is yours.
- **`dive`** — <https://github.com/wagoodman/dive>. Interactive per-layer inspection and the "efficiency" metric.
- **Pull by digest (reproducibility)** — <https://docs.docker.com/reference/cli/docker/image/pull/#pull-an-image-by-digest-immutable-identifier>. Pinning bases by `@sha256:` for an auditable, reproducible build.

## Adjacent reading — strongly recommended

- **"Building small containers" (Go blog / community)** — the Go team and the distroless maintainers' guidance on `CGO_ENABLED=0`, static linking, and `scratch`/distroless choices; cross-reference with <https://github.com/GoogleContainerTools/distroless>.
- **Kubernetes "Configure a Pod to use a ConfigMap"** — <https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/>. Where the env vars come from in the cluster (Week 11) — read it now to see what the 12-factor config feeds into.
- **The OWASP Docker security cheat sheet** — <https://cheatsheetseries.owasp.org/cheatsheets/Docker_Security_Cheat_Sheet.html>. Non-root, read-only root filesystem, dropped capabilities, no secrets in layers — the hardening checklist beyond the lectures.
- **SLSA / supply-chain** — <https://slsa.dev/>. Digest pinning, provenance, and reproducible builds in the larger supply-chain-security frame.

## Bookmarks worth saving past C30

- The multi-stage-build guide.
- The distroless repository and its base README.
- The Docker build-cache doc.
- The Twelve-Factor App (config, logs, processes, disposability).
- The Compose specification.

By the end of this week you should have all five pinned. Containerizing a Go service correctly is a handful of decisions — multi-stage, static, distroless, non-root, env-config — and these documents are the source for each; the time saved by not re-deriving them on the next service is why they are bookmarks.
