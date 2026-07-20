# Lecture 1 — Multi-Stage Dockerfiles for Go, the Static `CGO_ENABLED=0` Binary, and the Build Cache

## Why this lecture exists

Week 9 ended with an observable service: `notes` emitting `slog` JSON to stdout, OpenTelemetry traces to Jaeger, and Prometheus RED metrics behind a Grafana dashboard. It ran with `go run ./cmd/notesd` on your laptop, against a Postgres you started in a container by hand. That is not a deployment. To put `notes` on a Kubernetes cluster next week, the first thing we need is a **container image** — a single, reproducible artifact that carries the compiled binary and exactly enough operating system to run it, and nothing else.

Go makes this almost too easy, and the "almost" is the whole lecture. A Go program compiles to one static binary. There is no interpreter to install, no `node_modules` to copy, no virtualenv to recreate. With `CGO_ENABLED=0` the binary links nothing at runtime — not even libc — so it can run on an empty image. That property is why the cloud-native world is written in Go: Kubernetes, Docker, Prometheus, and etcd all ship as single static binaries in tiny images. This lecture has three jobs. First, write the multi-stage Dockerfile that compiles `notesd` in a full `golang:1.22` build stage and ships only the binary on a minimal runtime stage. Second, make that binary genuinely static with `CGO_ENABLED=0`, and understand precisely what that switch turns off and what it can break. Third, make the build *fast* and *reproducible* by ordering the layers around the Go module and build caches.

By the end, `docker build` produces an image you can `docker run` locally, hit at `http://localhost:8080/healthz`, and measure with `docker images`. That image is the artifact Week 11's Kubernetes Deployment will run and the capstone will defend.

## "Deploy is a feature" — what it means here

The Dockerfile is **source code**. It is reviewed, it is versioned, it has a correct and an incorrect form, and it has measurable characteristics — build time, image size, attack surface — you measure rather than guess at. We are not "writing some Docker config." We are authoring the build artifact that defines what runs in production. A `notes` service that compiles cleanly under `go vet` but ships in an 800 MB image running as root is not done; the image is half the deliverable.

The reference for the topic is the Docker multi-stage-build guide at <https://docs.docker.com/build/building/multi-stage/> and, for Go specifically, the build-cache doc at <https://docs.docker.com/build/cache/>. Open both; this lecture is the guided tour, those are the manuals.

## The naive single-stage image, and why it is wrong

The first Dockerfile most people write looks like this:

```dockerfile
# DON'T do this — single stage, ships the whole Go toolchain to production.
FROM golang:1.22
WORKDIR /src
COPY . .
RUN go build -o /notesd ./cmd/notesd
ENTRYPOINT ["/notesd"]
```

It works. It is also wrong in three ways. The final image is built `FROM golang:1.22`, so it ships the Go compiler, the standard-library source, `git`, a full Debian userland, and a shell — none of which run in production, all of which are attack surface. It copies the entire build context (`COPY . .`) including `.git/`, your local `.env`, and the `compose.yaml`, into the image. And because the single `COPY . .` invalidates the cache on any file change, every build re-downloads every module. The resulting image is roughly **850 MB**. The binary it runs is about 20 MB. The runtime needs about that much.

## The multi-stage Dockerfile for `notesd`

A multi-stage build uses one stage to *build* and a second, lean stage to *run*, copying only the compiled binary across the stage boundary. The toolchain stays in the discarded build stage; the final image is `FROM` a minimal runtime base.

```mermaid
flowchart LR
  A["Build stage golang 1.22"] --> B["go mod download cached layer"]
  B --> C["go build CGO disabled static binary"]
  C --> D["Copy binary only across stage boundary"]
  D --> E["Runtime stage distroless static nonroot"]
  E --> F["Final image about 18MB"]
```
*The build stage compiles the static binary; only the binary crosses into the lean runtime stage.*

```dockerfile
# syntax=docker/dockerfile:1

# ---- build stage: the full Go toolchain, discarded after compile ----
FROM golang:1.22 AS build
WORKDIR /src

# Copy only the module files first and download. This layer is cached and
# only re-runs when go.mod or go.sum changes — not on every source edit.
# This is the single biggest build-time win.
COPY go.mod go.sum ./
RUN go mod download

# Now copy the rest of the source and compile.
COPY . .

# CGO_ENABLED=0 makes the binary fully static (no libc dependency), which is
# what lets it run on distroless/static and scratch. -trimpath strips local
# paths for reproducibility; -ldflags "-s -w" drops the symbol table and DWARF
# debug info to shrink the binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/notesd \
    ./cmd/notesd

# ---- runtime stage: distroless static, no shell, non-root ----
FROM gcr.io/distroless/static-debian12:nonroot AS final
# The distroless static image ships CA certs, tzdata, /etc/passwd with a
# 'nonroot' user (UID 65532), and nothing else — no shell, no package manager.
COPY --from=build /out/notesd /notesd

# Bind a high port: a non-root process cannot bind 80. 8080 for REST, 9090 for
# gRPC, 2112 for /metrics — documented here, configured via the environment.
EXPOSE 8080 9090 2112

# The 'nonroot' user is the image default, but state it explicitly.
USER 65532:65532

ENTRYPOINT ["/notesd"]
```

Read the order. The `COPY go.mod go.sum ./` followed by `RUN go mod download`, *before* the `COPY . .` of the source, is the layer-cache trick: Docker caches each `RUN` and `COPY` as a layer keyed on its inputs, and a layer is reused if its inputs are unchanged. Because the download layer depends only on the module files, editing a `.go` file does not invalidate it — the build skips straight to `go build` and reuses the downloaded modules. On a service with dozens of dependencies (and `notes` has `chi`, `pgx`, the OTel SDK, the Prometheus client, and `grpc-go`), that is the difference between a 5-second and a 60-second rebuild. Citation: <https://docs.docker.com/build/cache/>.

The `CGO_ENABLED=0` is the line that makes everything downstream possible, and §"What `CGO_ENABLED=0` actually does" below explains it in full. The `GOOS=linux` is explicit so the build is correct even when you run `docker build` on a macOS or Windows host — the binary must be a Linux binary because the runtime image is Linux.

`-trimpath` removes the absolute paths of your build machine from the binary, so two people building the same commit get byte-identical output (reproducibility). `-ldflags="-s -w"` drops the symbol table (`-s`) and the DWARF debug info (`-w`); on `notesd` that shaves the binary from ~22 MB to ~16 MB. You lose the ability to run a symbol-rich debugger against the production binary, which is the right trade for a service you debug through logs and traces (Week 9), not a debugger attached in production.

## What `CGO_ENABLED=0` actually does

By default, Go's `net` and `os/user` packages can use **cgo** — they call into the host's C library for DNS resolution (`getaddrinfo`) and user lookups (`getpwuid`). A cgo-linked Go binary is therefore *dynamically* linked against libc, and it will not run on an image that has no libc — which is exactly what `scratch` and `distroless/static` are.

Setting `CGO_ENABLED=0` tells the Go toolchain to use the **pure-Go** implementations instead: the pure-Go DNS resolver and the pure-Go user lookup. The resulting binary depends on nothing at runtime — it is fully static. That is the prerequisite for `scratch` and the recommended setting for `distroless/static`.

```
   cgo path (CGO_ENABLED=1, the default)
   -------------------------------------
   net.Lookup* -> getaddrinfo (libc)   <-- needs libc in the runtime image
   os/user     -> getpwuid    (libc)   <-- needs /etc/passwd + libc

   pure-Go path (CGO_ENABLED=0)
   ----------------------------
   net.Lookup* -> Go DNS resolver       <-- reads /etc/resolv.conf, no libc
   os/user     -> parses /etc/passwd    <-- no libc
```

The pure-Go DNS resolver reads `/etc/resolv.conf` and `/etc/nsswitch.conf` directly. In a Kubernetes pod that is exactly what you want — the cluster's DNS config lands in `/etc/resolv.conf` and the pure-Go resolver honours it. The one edge case to know: some exotic `nsswitch.conf` configurations (LDAP/NIS user databases, certain mDNS setups) only work through the cgo resolver. For a cloud-native service talking to Postgres by DNS name and resolving service names in a cluster, the pure-Go resolver is correct and the edge cases do not arise. If you ever genuinely need the cgo resolver, you cannot use `distroless/static` — you use `distroless/base` (which has libc) or `distroless/cc`. For `notes`, static is right.

You can make the static-ness belt-and-braces explicit with build tags:

```bash
CGO_ENABLED=0 go build -tags netgo,osusergo -ldflags="-s -w" -o notesd ./cmd/notesd
```

`netgo` forces the pure-Go DNS resolver and `osusergo` forces the pure-Go user lookup, regardless of the cgo setting. With `CGO_ENABLED=0` they are already in effect, so the tags are documentation more than necessity — but on a team where someone might flip `CGO_ENABLED` back on, the tags keep the binary static anyway. Citation: the cgo docs at <https://pkg.go.dev/cmd/cgo> and the Go `net` package's resolver note at <https://pkg.go.dev/net#hdr-Name_Resolution>.

Prove the binary is static after building it locally:

```bash
CGO_ENABLED=0 GOOS=linux go build -o notesd ./cmd/notesd
file notesd
# notesd: ELF 64-bit LSB executable, x86-64, ... statically linked, ...
ldd notesd
# not a dynamic executable          <- the proof: nothing to link at runtime
```

`statically linked` and `not a dynamic executable` are the two lines that mean this binary will run on `scratch`. If `ldd` lists `libc.so.6`, you built with cgo on, and the binary will fail to start on a distroless/static or scratch image with a cryptic "no such file or directory" (the missing dynamic linker — a famously confusing error, because the file *is* there; its *interpreter* is not).

## BuildKit cache mounts — making rebuilds instant

The `go mod download` layer caches the modules, but the *compile* output (`/root/.cache/go-build`) is thrown away with the build stage every time. BuildKit cache mounts persist both across builds without baking them into a layer:

```dockerfile
# syntax=docker/dockerfile:1
FROM golang:1.22 AS build
WORKDIR /src

COPY go.mod go.sum ./
# Cache the downloaded modules across builds.
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY . .
# Cache both the module cache and the compile cache. Now a rebuild after a
# one-line source change reuses every unchanged package's compiled output.
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build \
      -trimpath -ldflags="-s -w" -o /out/notesd ./cmd/notesd
```

A cache mount is a directory BuildKit keeps between builds, mounted into the `RUN` step but *not* persisted into the resulting layer. The module cache and the build cache survive, so the second build after a source edit recompiles only the changed package and relinks — seconds, not a minute. The mounts require BuildKit (the default in modern Docker); the `# syntax=docker/dockerfile:1` directive at the top opts into the syntax that supports them. Citation: <https://docs.docker.com/build/cache/optimize/>.

A caveat for CI: cache mounts are local to the builder, so a fresh CI runner starts cold. The fix in CI is the registry/GHA cache backend (`--cache-from`/`--cache-to type=gha`), out of scope here but flagged in the resources. Locally, the cache mounts are pure win.

## Cross-compilation with build args

Docker Buildx exposes the target platform as build args, so the same Dockerfile builds a correct binary whether the image is `linux/amd64` or `linux/arm64` (Apple-silicon laptops, ARM cloud nodes):

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.22 AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=$TARGETOS GOARCH=$TARGETARCH go build \
      -trimpath -ldflags="-s -w" -o /out/notesd ./cmd/notesd
```

`--platform=$BUILDPLATFORM` pins the *build* stage to the native architecture of the build machine (so the compiler runs natively, fast — no emulation), while `GOARCH=$TARGETARCH` cross-compiles the binary for the *target*. Go's cross-compilation is a single environment variable because there is no C toolchain to cross-build (that is another reason `CGO_ENABLED=0` is the happy path — cgo cross-compilation needs a cross C toolchain in the image, which is painful). To build a multi-arch image: `docker buildx build --platform linux/amd64,linux/arm64 -t notesd:multi --push .`. Citation: the Buildx multi-platform doc at <https://docs.docker.com/build/building/multi-platform/>.

## Building and running it locally

```bash
# From the notes repo root.
docker build -t notesd:local .

# Run it. Every setting comes from the environment (Lecture 3); the minimum
# is the database URL and the port config. Map host 8080 to container 8080.
docker run --rm -p 8080:8080 \
  -e NOTES_HTTP_ADDR=":8080" \
  -e NOTES_DATABASE_URL="postgres://notes:devpass@host.docker.internal:5432/notes?sslmode=disable" \
  -e NOTES_LOG_LEVEL="info" \
  notesd:local

# In another shell — the liveness endpoint should answer.
curl -s http://localhost:8080/healthz
# ok
```

Then measure:

```bash
docker images notesd:local --format "{{.Repository}}:{{.Tag}} {{.Size}}"
# notesd:local 18.4MB
```

The single-stage image was ~850 MB; the multi-stage distroless image is ~16–20 MB — the binary plus the ~2 MB distroless base. That is the multi-stage win and the static-binary win compounding: the SDK never enters the final image, and the runtime base is almost nothing.

## `scratch` vs distroless — and why we pick distroless/static

Two minimal runtime bases are in scope:

```
+-------------------------------+----------+-----------------------------------------+
| Base                          | Size add | What it ships                           |
+-------------------------------+----------+-----------------------------------------+
| scratch                       | 0 B      | NOTHING — no certs, no tzdata, no passwd |
| gcr.io/distroless/static      | ~2 MB    | CA certs, tzdata, /etc/passwd (nonroot), |
|   -debian12:nonroot           |          | no shell, no package manager            |
+-------------------------------+----------+-----------------------------------------+
```

`scratch` is the truly empty image. A static Go binary runs on it — but the moment your service makes an outbound HTTPS call (to an OTLP endpoint, to an external API), the TLS handshake fails because there are **no CA certificates** to verify the server. On `scratch` you must copy the certs in yourself:

```dockerfile
FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /out/notesd /notesd
ENTRYPOINT ["/notesd"]
```

You also lose `/etc/passwd` (so `USER nonroot` by name does not resolve — you must use the numeric `USER 65532:65532`) and timezone data (so `time.LoadLocation` fails). Distroless/static ships all three — CA certs, tzdata, and a `nonroot` user — for about 2 MB. For `notes`, which talks TLS to Postgres and to the OTLP collector, **distroless/static is the right default**: you get the certs and the non-root user for free, the size difference from `scratch` is negligible, and you do not hand-maintain a cert copy. We use `scratch` in challenge-01 only to measure the difference and prove we understand what distroless carries. Citation: the distroless static README at <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>.

## What we built

By the end of Lecture 1, the repo has:

- A multi-stage Dockerfile for `notesd` that downloads modules from a cached layer, compiles a static `CGO_ENABLED=0` binary in the `golang:1.22` stage, and ships only that binary on `distroless/static-debian12:nonroot` — ~18 MB instead of ~850 MB.
- BuildKit cache mounts that make a rebuild after a source edit take seconds instead of a minute.
- A static binary proven static with `file` and `ldd`, ready to run on `scratch` or distroless.
- Cross-compilation via `TARGETARCH` so the same Dockerfile builds amd64 and arm64.
- A clear rule for `scratch` vs distroless/static, grounded in CA certs, tzdata, and the non-root user.

The slogan: the binary is the artifact, and the artifact is source code. Compile it static, strip it, ship it on almost nothing — Lecture 2 hardens that "almost nothing" into a non-root, shell-less, measured image, and Lecture 3 makes the service read every setting from the environment so the *same* image runs everywhere.
