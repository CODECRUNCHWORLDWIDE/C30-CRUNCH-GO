# Exercise 01 — The Multi-Stage Distroless Dockerfile for `notesd`

> **Time:** ~90 minutes. **Prerequisites:** Lecture 1 (and Lecture 2 for the non-root/`.dockerignore` part). A working Week 9 `notes` service that builds with `go build ./cmd/notesd`.

## Goal

Write the production Dockerfile for `notesd`: a multi-stage build that compiles a static `CGO_ENABLED=0` binary in a `golang:1.22` stage and ships it on `gcr.io/distroless/static-debian12:nonroot` as the non-root UID 65532. Build it, run it against the compose Postgres, hit `/healthz`, and measure the image.

## Steps

1. **Create `Dockerfile`** at the repo root. It must:
   - Use a `golang:1.22 AS build` stage.
   - Copy `go.mod` and `go.sum` and run `go mod download` *before* copying the source (the layer-cache trick).
   - Compile with `CGO_ENABLED=0 GOOS=linux`, `-trimpath`, and `-ldflags="-s -w"`, output to `/out/notesd`.
   - Use BuildKit cache mounts for `/go/pkg/mod` and `/root/.cache/go-build`.
   - Use `gcr.io/distroless/static-debian12:nonroot` as the `final` stage.
   - `COPY --from=build` only the binary.
   - `USER 65532:65532`, `EXPOSE 8080 9090 2112`, `ENTRYPOINT ["/notesd"]`.

2. **Create `.dockerignore`** at the repo root excluding `.git/`, `*.env`/`.env*`, `**/testdata/`, `compose.yaml`, `deploy/`, and `*.md`.

3. **Build** with `docker build -t notesd:local .` and confirm the context-transfer size is a few hundred KB, not hundreds of MB.

4. **Prove the binary is static** by also building locally and inspecting:
   ```bash
   CGO_ENABLED=0 GOOS=linux go build -o /tmp/notesd ./cmd/notesd
   file /tmp/notesd   # should say "statically linked"
   ldd  /tmp/notesd   # should say "not a dynamic executable"
   ```

5. **Run** the image against the compose Postgres (`docker compose up -d postgres` first) and hit the liveness endpoint:
   ```bash
   docker run --rm -p 8080:8080 \
     -e NOTES_DATABASE_URL="postgres://notes:devpass@host.docker.internal:5432/notes?sslmode=disable" \
     notesd:local &
   curl -s http://localhost:8080/healthz   # ok
   ```

6. **Measure**: `docker images notesd:local --format "{{.Size}}"` (expect ~16–20 MB) and `docker history notesd:local` (confirm only the binary layer is yours).

## Acceptance criteria

- The image builds, is under 25 MB, and runs as UID 65532 (`docker inspect --format '{{.Config.User}}'` prints `65532:65532`).
- The binary is statically linked (`ldd` says "not a dynamic executable").
- `/healthz` returns `ok` from the running container.
- A second build after editing a `.go` file reuses the `go mod download` layer (look for `CACHED`).
- The build context transfer is a few hundred KB (the `.dockerignore` is working).

## Stretch

Add `Dockerfile.scratch` that ships on `scratch`, copying `/etc/ssl/certs/ca-certificates.crt` and using the numeric `USER 65532:65532`. Measure the size difference against distroless and explain (in a comment) why it is only ~2 MB.
