# Week 10 — Quiz

Ten multiple-choice questions covering multi-stage Dockerfiles, static Go builds, distroless and `scratch`, non-root containers, the build cache, and 12-factor config. Treat it as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — Why multi-stage

A `notesd` image built `FROM golang:1.22` as a single stage is ~850 MB; the multi-stage distroless build is ~18 MB. The size difference is mostly because:

- (A) The multi-stage build compresses layers more aggressively.
- (B) The single-stage final image ships the whole Go toolchain (compiler, stdlib source, git, a shell), none of which runs in production; the multi-stage final image is `FROM` a runtime base and carries only the compiled binary.
- (C) The multi-stage build strips the Go runtime, which the single-stage build keeps.
- (D) Single-stage images cannot use layer caching.

<details>
<summary>Answer</summary>

**(B).** A single-stage image built `FROM golang:1.22` ships the entire toolchain to production; the multi-stage final image is `FROM` a runtime base (distroless) and carries only the binary. The ~47x difference is the toolchain and stdlib source that never run. Citation: <https://docs.docker.com/build/building/multi-stage/>.

</details>

## Question 2 — Why `CGO_ENABLED=0`

Setting `CGO_ENABLED=0` before `go build` is required for the binary to run on `gcr.io/distroless/static-debian12` because:

- (A) It makes the binary smaller.
- (B) It produces a fully static binary that uses the pure-Go DNS resolver and user lookup, so the binary depends on no libc at runtime — and distroless/static has no libc.
- (C) It enables the race detector.
- (D) It is required for cross-compilation.

<details>
<summary>Answer</summary>

**(B).** `CGO_ENABLED=0` switches `net` and `os/user` to their pure-Go implementations, so the binary links no libc and is fully static — the prerequisite for distroless/static and `scratch`. Citation: <https://pkg.go.dev/cmd/cgo>.

</details>

## Question 3 — The "no such file or directory" startup failure

A container built without `CGO_ENABLED=0` fails to start on distroless/static with "no such file or directory," even though the binary is clearly present. The cause is:

- (A) The binary was not copied into the image.
- (B) The binary is dynamically linked against libc, and distroless/static has no libc and no dynamic linker (the *interpreter* is missing, not the binary).
- (C) The `ENTRYPOINT` path is wrong.
- (D) The image is corrupt.

<details>
<summary>Answer</summary>

**(B).** A cgo-linked binary is dynamically linked against libc; distroless/static has no libc and no dynamic-linker interpreter, so the kernel reports "no such file or directory" for the missing *interpreter*. Fix: `CGO_ENABLED=0`. Citation: <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>.

</details>

## Question 4 — The layer-cache trick

The Dockerfile copies `go.mod`/`go.sum` and runs `go mod download` *before* `COPY . .`. The reason is:

- (A) `go mod download` requires the source to be absent.
- (B) The download layer is keyed only on the module files, so editing a `.go` file does not invalidate it — Docker reuses the cached modules and skips to `go build`.
- (C) It makes the final image smaller.
- (D) `COPY . .` cannot run before a `RUN` step.

<details>
<summary>Answer</summary>

**(B).** The download layer depends only on `go.mod`/`go.sum`; copying them and downloading before `COPY . .` means a source edit reuses the cached download layer. Look for `CACHED` on the `go mod download` step. Citation: <https://docs.docker.com/build/cache/>.

</details>

## Question 5 — distroless vs scratch

For `notes`, which talks TLS to Postgres and to an OTLP collector, `gcr.io/distroless/static-debian12:nonroot` is preferred over `scratch` because:

- (A) `scratch` cannot run static Go binaries.
- (B) distroless/static ships CA certs, tzdata, and a `nonroot` user for ~2 MB, so you get TLS and a non-root user without hand-maintaining a cert copy; `scratch` saves only ~2 MB and makes you supply all three yourself.
- (C) distroless is faster at runtime.
- (D) `scratch` runs as root and cannot be changed.

<details>
<summary>Answer</summary>

**(B).** distroless/static ships CA certs, tzdata, and the `nonroot` user for ~2 MB; `scratch` saves only ~2 MB and makes you copy all three in yourself. For a TLS-talking service, distroless is the sweet spot. Citation: <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>.

</details>

## Question 6 — Running as non-root

`USER 65532:65532` in the runtime stage:

- (A) Is required for the container to start.
- (B) Runs the process as the non-root `nonroot` user the distroless image ships, so a container escape does not land an attacker as root — and is why the service binds high ports (8080, not 80), since a non-root process cannot bind below 1024.
- (C) Makes the image smaller.
- (D) Is only meaningful in Kubernetes.

<details>
<summary>Answer</summary>

**(B).** `USER 65532:65532` runs the process as the non-root user the image ships, so a container escape does not yield root; it is also why the service binds high ports, since non-root cannot bind below 1024. Citation: <https://docs.docker.com/reference/dockerfile/#user>.

</details>

## Question 7 — Where config belongs

The 12-factor rule (factor III) says `notes`'s database password should live:

- (A) In a `const dbPassword` in the source.
- (B) In a `config.prod.yaml` copied into an image layer.
- (C) In the process environment (`NOTES_DATABASE_URL`), read once at startup, so the same image runs in every environment and no credential is in the binary or a layer.
- (D) In the Dockerfile as an `ENV` instruction.

<details>
<summary>Answer</summary>

**(C).** Factor III: config in the environment, read once at startup. A const or a baked-in file welds the image to one environment and leaks the credential; the environment keeps the image identical across deploys. Citation: <https://12factor.net/config>.

</details>

## Question 8 — Logs as event streams

A 12-factor process (factor XI) handles logs by:

- (A) Writing to `/var/log/notes.log` and rotating it in-process.
- (B) Writing the event stream to stdout (JSON via `slog`) and letting the execution environment capture and route it — never managing its own log files.
- (C) Posting each log line to a logging API over HTTP.
- (D) Buffering logs in memory and flushing on shutdown.

<details>
<summary>Answer</summary>

**(B).** Factor XI: write the event stream to stdout and let the platform route it; never manage log files in-process (a file in a container is lost on reschedule and invisible to `kubectl logs`). Citation: <https://12factor.net/logs>.

</details>

## Question 9 — Why stateless matters for the cluster

`notes` keeps all durable state in Postgres and none in the process. This statelessness (factor VI) matters because:

- (A) It makes the binary smaller.
- (B) It lets Kubernetes run N replicas of the same image behind a Service and round-robin requests, because any replica can serve any request — no request depends on state held only in one replica.
- (C) It is required for distroless.
- (D) It speeds up startup.

<details>
<summary>Answer</summary>

**(B).** Factor VI: a stateless process lets the cluster run N replicas and balance across them, because any replica can serve any request. In-memory session state would break the round-robin. State goes in Postgres. Citation: <https://12factor.net/processes>.

</details>

## Question 10 — The disposability gap this week

By the end of Week 10, the disposability story (factor IX) for `notes` is:

- (A) Fully complete — fast startup and graceful shutdown both done.
- (B) Fast startup is done (a static Go binary starts in milliseconds) and logs/statelessness are in place, but **graceful shutdown on `SIGTERM` is Week 11's work** — currently the process exits immediately on `SIGTERM`, dropping in-flight requests.
- (C) Not started — the binary starts slowly.
- (D) Irrelevant until the service is in the cloud.

<details>
<summary>Answer</summary>

**(B).** Week 10 delivers fast startup, stdout logs, and statelessness; graceful shutdown on `SIGTERM` is Week 11's lecture. Until then the process exits immediately on `SIGTERM` and drops in-flight requests. Citation: <https://12factor.net/disposability>.

</details>

---

## Self-assessment

- 9-10: you can containerize a Go service to a hardened, 12-factor image without further reading.
- 7-8: re-read the lecture notes on the questions you missed; the citations point to the exact pages. The cgo/static and distroless-vs-scratch questions are the two that bite.
- 5-6: re-read all three lecture notes and redo the exercises, paying attention to `CGO_ENABLED=0` and the env-only config struct.
- 0-4: rewind to Lecture 1. Lab 10 and the capstone container axis assemble every pattern this quiz tests; the image is half the capstone deliverable.
