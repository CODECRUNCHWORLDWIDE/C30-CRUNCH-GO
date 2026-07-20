# Lecture 2 — Distroless, the Non-Root User, `.dockerignore`, and Measuring the Image

## Why this lecture exists

Lecture 1 produced a working multi-stage image: a static `notesd` binary on `distroless/static-debian12:nonroot`, about 18 MB. This lecture turns "small" into "hardened and measured." Three jobs. First, understand exactly what distroless *is* — what it ships, what it omits, and the no-shell tradeoff that is a feature in production and a discipline in debugging. Second, run the service as a non-root user and prove it, plus the `.dockerignore` that keeps secrets and junk out of the build context. Third, measure the image rigorously — size across four bases, the layer breakdown with `docker history`, and the CVE count — because "smaller is safer" is a claim you back with numbers, not a slogan.

The reference for distroless is the GoogleContainerTools repository at <https://github.com/GoogleContainerTools/distroless> and its base-image README at <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>. Open both.

## What distroless actually is

"Distroless" is a misnomer that sticks: the images are *based on* Debian (the `-debian12` suffix), but they strip everything a distribution normally bundles — no shell, no package manager, no `apt`, no `coreutils`, no busybox. The `static` variant ships precisely:

```
gcr.io/distroless/static-debian12 contents
-------------------------------------------
/etc/ssl/certs/ca-certificates.crt   <- TLS to outbound endpoints works
/etc/passwd, /etc/group              <- a 'nonroot' user (UID 65532) exists
/usr/share/zoneinfo/                 <- time.LoadLocation works
/tmp                                 <- a writable temp dir
(that is essentially all)
```

No `/bin/sh`. No `/bin/ls`. No `/usr/bin/apt`. There is nothing to `exec` into and nothing for an attacker to live off the land with after a compromise — no shell to spawn, no package manager to pull a toolkit, no `curl` to exfiltrate with. The CVE surface is correspondingly tiny: a `debian:12` base carries hundreds of packages, each a potential CVE; distroless/static carries a handful of files. Citation: the distroless base README at <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>.

The variants, smallest to largest:

```
+-------------------------------+--------------------------------------------------+
| distroless variant            | adds (over the previous)                         |
+-------------------------------+--------------------------------------------------+
| static-debian12               | certs, tzdata, passwd — for fully-static binaries|
| base-debian12                 | + glibc, libssl — for cgo binaries needing libc  |
| cc-debian12                   | + libgcc, libstdc++ — for C/C++ deps             |
| (debug variants: :debug)      | + a busybox shell, for break-glass debugging     |
+-------------------------------+--------------------------------------------------+
```

A static Go binary (Lecture 1) uses `static`. A cgo Go binary (DNS via libc) would need `base`. For `notes` — `CGO_ENABLED=0`, pure-Go resolver — `static` is correct.

## The no-shell tradeoff, and why it is acceptable

The cost of distroless: `kubectl exec -it notes-pod -- sh` fails with "executable file not found" — there is no shell. You cannot drop into a production container to poke around. For an engineer used to shelling in, this feels like a loss. It is not, and the reason is the whole point of Phase III: **you debug a cloud-native service through its observability, not by shelling into it.** Week 9 gave you the three signals — `slog` logs to stdout, OpenTelemetry traces to Jaeger, Prometheus metrics to Grafana. A latency regression is localised from the Grafana dashboard to the trace span (Week 9's lab), not by `top` inside the container. The shell you are giving up is the shell an attacker also wanted.

For the rare genuine break-glass case, distroless ships `:debug` variants with a busybox shell, and Kubernetes 1.25+ has **ephemeral debug containers** (`kubectl debug -it notes-pod --image=busybox --target=notes`) that attach a debug container sharing the pod's namespaces without rebuilding your image. You keep the production image shell-less and attach a shell *to the pod* only when you genuinely need one. Citation: the Kubernetes ephemeral-containers doc at <https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container>.

```mermaid
flowchart TD
  A["notes pod on distroless image"] --> B{"Need to investigate"}
  B -->|"Normal case"| C["Read logs traces and metrics"]
  B -->|"Break glass case"| D["kubectl debug attach ephemeral container"]
  D --> E["Debug container shares pod namespaces"]
  C --> F["Production image stays shell-less"]
  E --> F
```
*Debugging happens through observability first; a shell is attached to the pod only for the rare break-glass case.*

## Running as a non-root user

A container that runs as UID 0 (root) is a container escape away from root on the node. Most images run as root by default; most production incidents that turn a container compromise into a node compromise start there. The fix is two lines and one binary property:

```dockerfile
FROM gcr.io/distroless/static-debian12:nonroot AS final
COPY --from=build /out/notesd /notesd
USER 65532:65532          # the 'nonroot' user/group the image ships
EXPOSE 8080 9090 2112
ENTRYPOINT ["/notesd"]
```

`USER 65532:65532` runs the process as the non-root `nonroot` user. The `:nonroot` image tag already defaults to this user, but stating it explicitly is documentation that survives a base-image change. Using the *numeric* UID (`65532`) rather than the name (`nonroot`) matters on `scratch`, where there is no `/etc/passwd` to resolve the name, and it matters for the Kubernetes `runAsNonRoot` check next week, which compares the numeric UID against 0.

The binary property: a non-root process **cannot bind ports below 1024**. So `notes` binds 8080 (REST), 9090 (gRPC), and 2112 (the conventional Prometheus `/metrics` port), never 80 or 443. The platform (Compose, Kubernetes) maps an external port to these. This is why every example in this track binds high ports — it is not arbitrary; it is the consequence of not running as root.

Proving non-root is awkward precisely *because* the image is hardened — there is no `id`, no `whoami`, no shell to run them in. Three ways to confirm it:

```bash
# 1. Inspect the image's configured user (the reliable, shell-free way).
docker inspect notesd:local --format '{{.Config.User}}'
# 65532:65532

# 2. The /etc/passwd the image ships names the user.
docker run --rm --entrypoint /notesd notesd:local --version   # runs as 65532

# 3. In Kubernetes (Week 11), the securityContext enforces it:
#    runAsNonRoot: true makes the kubelet REFUSE to start a container
#    whose effective UID is 0 — a hard gate, not a hope.
```

The Kubernetes `securityContext` is where non-root becomes *enforced* rather than *configured*, and Lecture 1 of Week 11 wires it: `runAsNonRoot: true`, `runAsUser: 65532`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, and `capabilities: drop: [ALL]`. The Dockerfile sets the default; the pod spec enforces it. Citation: the Kubernetes pod-security-standards doc at <https://kubernetes.io/docs/concepts/security/pod-security-standards/> and the Docker `USER` reference at <https://docs.docker.com/reference/dockerfile/#user>.

## The `.dockerignore`

Without a `.dockerignore`, `COPY . .` ships the entire repo into the build context — `.git/` (history, possibly old secrets), your local `.env` (the database password), `testdata/` fixtures, the `compose.yaml`, and the README — all slow to transfer and some a leak risk. The fix is one file at the repo root:

```
# .dockerignore
.git/
.github/
*.env
.env*
**/testdata/
**/*_test.go
compose.yaml
compose.*.yaml
Dockerfile
.dockerignore
README.md
*.md
deploy/                 # the Kubernetes manifests (Week 11) — not needed in the image
bin/
tmp/
```

Two notes. Excluding `*.env` and `.env*` is the load-bearing security line: a `.env` with `NOTES_DATABASE_URL=postgres://notes:realpassword@...` copied into a layer is a credential anyone can recover with `docker history --no-trunc` or by extracting the layer tarball — even after you "remove" it in a later layer, because the layer where it was added persists. Config comes from the *runtime* environment (Lecture 3), never from a file in the image. Excluding `**/*_test.go` is not strictly necessary (the binary does not include them) but keeps the build context small, which speeds the context transfer. Citation: <https://docs.docker.com/build/concepts/context/#dockerignore-files>.

Verify the `.dockerignore` is taking effect by watching the context-transfer size in the build output:

```bash
docker build -t notesd:local . 2>&1 | grep "transferring context"
# => transferring context: 312.45kB   <- a few hundred KB, not hundreds of MB
```

If it reports hundreds of megabytes, your `.dockerignore` is missing or in the wrong directory — it must sit at the build-context root (the directory you pass to `docker build`), not next to the Dockerfile if those differ.

## Measuring the image — size, layers, CVEs

"Smaller is safer" is a measurable claim. Three tools make it concrete.

**Size, across four bases.** Build the same `notesd` binary on four runtime bases and tabulate:

```bash
for base in single multi-debian distroless scratch; do
  docker build -t notesd:$base -f Dockerfile.$base . >/dev/null
  printf "%-16s " "$base"
  docker images notesd:$base --format "{{.Size}}"
done
```

```text
+----------------------------------+----------+--------------------------------------+
| Image                            | Size     | Notes                                |
+----------------------------------+----------+--------------------------------------+
| single-stage (FROM golang:1.22)  | ~850 MB  | ships the whole toolchain — wrong    |
| multi-stage (debian:12-slim)     | ~95 MB   | binary + a Debian userland + shell   |
| multi-stage (distroless/static)  | ~18 MB   | binary + certs + tzdata + passwd     |
| multi-stage (scratch + certs)    | ~16 MB   | binary + the certs you copied in     |
+----------------------------------+----------+--------------------------------------+
```

The jump from 850 to 95 is the multi-stage win (the toolchain never enters the final image). The jump from 95 to 18 is the distroless win (the OS userland shrinks to a handful of files). The jump from 18 to 16 is the `scratch` win — and it is *small*, which is the lesson: distroless/static buys you the certs, tzdata, and non-root user for ~2 MB, so paying the `scratch` maintenance cost (hand-copying certs) to save 2 MB is a bad trade. Distroless is the sweet spot.

**Layers, with `docker history`.** The layer breakdown proves no toolchain leaked into the final image:

```bash
docker history notesd:distroless --format "{{.Size}}\t{{.CreatedBy}}" | head
```

```text
0B        ENTRYPOINT ["/notesd"]
0B        USER 65532:65532
0B        EXPOSE 8080 9090 2112
16.2MB    COPY /out/notesd /notesd # buildkit          <- your binary, the only layer you own
2.4MB     (distroless/static base layers)              <- certs, tzdata, passwd
```

The `COPY /out/notesd` layer is the only one you own; everything below it is the immutable base. If you see a layer of hundreds of megabytes above the base, a build stage leaked into `final` — almost always because the final `FROM` is the SDK, not a runtime base. Citation: <https://docs.docker.com/reference/cli/docker/image/history/>.

**Interactive layer inspection with `dive`.** For a richer view, `dive notesd:distroless` walks every layer interactively and reports the image's "efficiency" (wasted space from files added and later deleted). For a single-binary Go image there is nothing to waste — efficiency is ~100% — which is itself the proof that the image is exactly the binary and nothing else. Citation: <https://github.com/wagoodman/dive>.

**CVE count, with `docker scout` or `trivy`.** Image size correlates with CVE count because most CVEs are in OS packages. Scan the debian-based and distroless images and compare:

```bash
docker scout cves notesd:multi-debian   # tens of CVEs in the Debian userland
docker scout cves notesd:distroless     # near-zero — there are almost no packages to be vulnerable
```

The distroless image's near-zero CVE count is the quantitative form of "smaller is safer": a package that is not in the image cannot have a CVE in the image. This is the stretch in challenge-01 — proving "smaller is safer" is a CVE-count claim, not just a megabyte claim. Citation: <https://docs.docker.com/scout/> and Trivy at <https://github.com/aquasecurity/trivy>.

## Reproducible builds

Two engineers building the same commit should get a byte-identical image. Two levers, both in Lecture 1's Dockerfile, get most of the way:

- `-trimpath` removes the build machine's absolute paths (`/home/alice/notes/...`) from the binary, so the binary does not embed who built it or where.
- Pinning the base images **by digest**, not by tag, removes the "the tag moved" non-determinism:

```dockerfile
FROM golang:1.22@sha256:<digest> AS build
# ...
FROM gcr.io/distroless/static-debian12:nonroot@sha256:<digest> AS final
```

A tag like `golang:1.22` is mutable — the `1.22` your build pulled today is not the `1.22` from last month after a patch release. The digest is the immutable content hash; pinning it means "this exact base image, forever," which is what makes a build reproducible and a supply chain auditable. The cost is you must bump the digest deliberately when you want a patch, which is the point — base-image updates become a reviewed change, not a silent drift. Citation: the Docker image-pull-by-digest note at <https://docs.docker.com/reference/cli/docker/image/pull/#pull-an-image-by-digest-immutable-identifier>.

## What we built

By the end of Lecture 2, the repo has:

- A hardened runtime image: `notesd` on distroless/static, running as the non-root UID 65532, binding high ports, with no shell and a near-zero CVE count.
- A `.dockerignore` that keeps `.git/`, `*.env`, and the manifests out of the build context, verified by the few-hundred-KB context-transfer size.
- A measured image: a four-base size table, a `docker history` proving only the binary layer is ours, and a CVE scan proving distroless is quantitatively safer than a debian base.
- The reproducibility levers — `-trimpath` and digest-pinned bases — for an auditable build.

The image is now small, static, non-root, and shell-less — exactly the four properties the slogan promised. Lecture 3 makes it read every setting from the environment, so the *same* image runs in dev, in the compose stack, and in the cluster, with nothing baked in but the binary.
