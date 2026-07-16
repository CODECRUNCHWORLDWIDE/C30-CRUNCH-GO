# Week 10 — Homework

Six practice problems consolidating the week's container and config material. They are sized to ~45 minutes each. Do them after the lectures and the exercises; several feed directly into Lab 10. Cite the URLs you used while solving each one in the commit message of your homework branch.

## Problem 1 — The image-hardening audit

Take your `notesd` Dockerfile and write a one-page audit of its hardening posture. For each item, state whether the Dockerfile satisfies it and why it matters:

1. Multi-stage (the Go toolchain does not ship to the runtime image).
2. Static binary (`CGO_ENABLED=0`; `ldd` says "not a dynamic executable").
3. Runs as a non-root user (`USER 65532:65532`; binds high ports).
4. The runtime base is minimal (distroless/static, not `debian`, not `golang`).
5. No secrets baked into image layers (verify with `docker history --no-trunc`).
6. A `.dockerignore` keeps `.git/`, `*.env`, and the manifests out of the build context.

Then identify one further hardening step you have not taken (pinning bases by digest, a read-only root filesystem, dropping all Linux capabilities) and describe how you would add it.

Cite the multi-stage guide at <https://docs.docker.com/build/building/multi-stage/> and the distroless static README at <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>.

Deliverable: `homework/01-image-hardening-audit.md`.

## Problem 2 — The static-binary investigation

Build `notesd` two ways and document the difference:

1. `CGO_ENABLED=0 go build` — inspect with `file` and `ldd`; run it on `distroless/static`; confirm it works.
2. `CGO_ENABLED=1 go build` (the default if you omit it) — inspect with `ldd` (it now lists `libc.so.6`); try to run it on `distroless/static`; capture the failure and explain the "no such file or directory" error (the missing dynamic linker, not the missing binary).

Then explain, in two sentences each, what `CGO_ENABLED=0` does to DNS resolution and `os/user` lookups, and name the one configuration (`nsswitch.conf` with LDAP/NIS) where the pure-Go resolver differs from the cgo one.

Cite the cgo docs at <https://pkg.go.dev/cmd/cgo> and the `net` resolver note at <https://pkg.go.dev/net#hdr-Name_Resolution>.

Deliverable: `homework/02-static-binary.md`.

## Problem 3 — The build-cache experiment

Demonstrate the layer-cache trick quantitatively. With BuildKit:

1. Build with the *wrong* order (`COPY . .` before `go mod download`), edit a `.go` file, rebuild, and time it.
2. Build with the *right* order (module files + download before source), edit a `.go` file, rebuild, and time it (look for `CACHED` on the download step).
3. Add BuildKit cache mounts and rebuild after a source edit; time it.

Tabulate the three rebuild times and explain why each is what it is.

Cite the build-cache doc at <https://docs.docker.com/build/cache/> and the cache-mount reference at <https://docs.docker.com/build/cache/optimize/>.

Deliverable: `homework/03-build-cache.md`.

## Problem 4 — The 12-factor config conversion

Take one setting in `notes` that is currently a flag or a constant and convert it to env-driven config end to end:

1. Add the field to the `Config` struct and `Load`.
2. Give it a sensible default (or make it required and validate it).
3. Pass it down from `main` — remove the flag/const.
4. Add a table-driven test asserting the default, an override, and (if required) the missing-value error.
5. Wire it into the `compose.yaml` `environment:` block.

Explain why "the same image, only env values differ across environments" is the property that makes a 12-factor image deployable to dev, the stack, and the cluster unchanged.

Cite factor III at <https://12factor.net/config>.

Deliverable: `homework/04-12factor-config.md` plus the code change.

## Problem 5 — The secret-leak demonstration

Prove why a baked-in secret is a leak even after "removal":

1. Write a throwaway `Dockerfile.leak` that `COPY .env` into a layer and then `RUN rm .env` in a *later* layer.
2. Build it.
3. Recover the password from the image with `docker history --no-trunc` (you will see the `COPY .env`) and by extracting the layer tarball (`docker save` + `tar` the layer that added `.env`).
4. Explain why the `rm` in a later layer does not remove the secret from the earlier layer where it was added.

Then state the correct pattern: config (including secrets) comes from the runtime environment, never from a file in the image.

Cite the build-context/`.dockerignore` doc at <https://docs.docker.com/build/concepts/context/#dockerignore-files> and factor III at <https://12factor.net/config>.

Deliverable: `homework/05-secret-leak.md`.

## Problem 6 — The disposability gap

Document what is and is not done in the disposability story (factor IX) for `notes`:

1. **Fast startup** — done. Measure it: `time docker run --rm notesd:local --version` (or a fast-exit flag) and report the milliseconds. Explain why a static Go binary on distroless starts so fast.
2. **Graceful shutdown** — *not yet done* (it is Week 11's lecture). Describe what currently happens when `notes` receives `SIGTERM` (the default Go behaviour: the process exits immediately, dropping in-flight requests), and write a two-paragraph spec of what graceful shutdown *should* do (stop accepting, drain in-flight within the grace period, close the pool, exit) — the spec you will implement next week.

Confirm `ShutdownTimeout` is already in the config struct (it should be, from Lecture 3), so the value is configured before the behaviour exists.

Cite factor IX at <https://12factor.net/disposability> and the `os/signal` docs at <https://pkg.go.dev/os/signal>.

Deliverable: `homework/06-disposability-gap.md`.

## Submission

Push the six deliverables on a branch named `week10-homework/<your-handle>` and open a PR against the C30 curriculum repository. The PR description links each file and includes a 100-word summary of what you learned. The single most common review comment is "where is your citation for this claim" — preempt it by linking the Docker doc, the distroless README, or the 12factor.net factor for every non-trivial assertion.

Cited pages this homework draws from: <https://docs.docker.com/build/building/multi-stage/>, <https://docs.docker.com/build/cache/>, <https://github.com/GoogleContainerTools/distroless>, <https://pkg.go.dev/cmd/cgo>, <https://12factor.net/config>, and <https://12factor.net/disposability>.
