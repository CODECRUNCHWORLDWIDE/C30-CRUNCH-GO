# Challenge 1 — Shrink the `notesd` Image Across Four Bases and Prove "Smaller Is Safer" with a Size Table and a CVE Scan

> **Time:** 2 hours. **Prerequisites:** Lectures 1–2; Exercise 1. A working `notesd` and Docker with BuildKit. **Citations:** the multi-stage guide at <https://docs.docker.com/build/building/multi-stage/>, the distroless repo at <https://github.com/GoogleContainerTools/distroless>, the distroless static README at <https://github.com/GoogleContainerTools/distroless/blob/main/base/README.md>, Docker Scout at <https://docs.docker.com/scout/>, and the build-cache doc at <https://docs.docker.com/build/cache/>.

## The premise

You have the multi-stage distroless Dockerfile from Lecture 1. This challenge turns "I followed the lecture" into "I measured it." You will build the *same* `notesd` binary on four runtime bases — single-stage `golang`, multi-stage `debian:12-slim`, multi-stage `distroless/static`, and `scratch` — produce a before/after table across image size, build time, and CVE count, and explain every number. The skill is not "make it smaller"; it is "prove the smaller thing is the same binary, and know what you traded."

Image size is not a vanity metric. It is pull time on every cold scale-out, attack surface on every CVE scan, and storage cost on every registry. An 18 MB distroless image pulls in a fraction of the time of a 95 MB debian image and a fortieth of the time of an 850 MB single-stage mistake; in a Kubernetes cluster that schedules your pod onto a node that has never pulled the image, that pull time is part of your pod-startup latency. So "smaller" cashes out as faster scheduling, fewer vulnerabilities, and lower registry cost — but only if the smaller image still runs the identical bytes of `notesd`. The measurement is how you prove it does.

The controlled-experiment discipline: keep the *build stage byte-for-byte identical* across all four images. Only the final `FROM` line changes. That is what lets you attribute the entire delta to the runtime base — one variable changed.

## Setup

Build the deliberately-wrong single-stage image first so you have the "before" number:

```dockerfile
# Dockerfile.single — the WRONG baseline, for measurement only.
FROM golang:1.22
WORKDIR /src
COPY . .
RUN CGO_ENABLED=0 go build -o /notesd ./cmd/notesd
ENTRYPOINT ["/notesd"]
```

Then build the three improved images, changing *only* the final `FROM`:

```dockerfile
# Dockerfile.debian    -> FROM debian:12-slim       (copy the binary, add a nonroot user, USER it)
# Dockerfile.distroless-> FROM gcr.io/distroless/static-debian12:nonroot
# Dockerfile.scratch   -> FROM scratch  (COPY the CA certs in, numeric USER 65532:65532)
```

The `scratch` variant must copy certs and the passwd entry it needs:

```dockerfile
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/notesd ./cmd/notesd

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /etc/passwd /etc/passwd
COPY --from=build /out/notesd /notesd
USER 65532:65532
EXPOSE 8080 9090 2112
ENTRYPOINT ["/notesd"]
```

## The measurement

Build all four and tabulate:

```bash
for v in single debian distroless scratch; do
  /usr/bin/time -p docker build --no-cache -t notesd:$v -f Dockerfile.$v . 2>build.$v.log
  size=$(docker images notesd:$v --format "{{.Size}}")
  echo "$v -> $size"
done
```

Where the bytes go, conceptually, so the table tells a story rather than just listing numbers:

```text
+----------------------------------+----------+---------------------------------------------+
| Image                            | Size     | What is in it                               |
+----------------------------------+----------+---------------------------------------------+
| single-stage (golang:1.22)       | ~850 MB  | the whole toolchain + stdlib source + binary|
| multi-stage (debian:12-slim)     | ~95 MB   | a Debian userland (shell, libc, coreutils)  |
|                                  |          | + your 16 MB binary                         |
| multi-stage (distroless/static)  | ~18 MB   | certs + tzdata + passwd + your 16 MB binary |
| multi-stage (scratch + certs)    | ~16 MB   | just the certs you copied + your binary     |
+----------------------------------+----------+---------------------------------------------+
```

The jump from 850 to 95 is the multi-stage win (the toolchain never enters the final image). The jump from 95 to 18 is the distroless win (the Debian userland shrinks to a few files). The jump from 18 to 16 is the `scratch` win — only ~2 MB, which is the lesson: distroless/static buys the certs, tzdata, and non-root user for ~2 MB, so paying the `scratch` maintenance cost to save 2 MB is a bad trade.

## The CVE scan — "smaller is safer," quantified

Size correlates with CVE count because most CVEs live in OS packages. Scan the debian and distroless images and compare:

```bash
docker scout cves notesd:debian       # or: trivy image notesd:debian
docker scout cves notesd:distroless   # or: trivy image notesd:distroless
```

```text
+----------------------+----------------------+------------------------------------------+
| Image                | CVEs (typical shape) | Why                                      |
+----------------------+----------------------+------------------------------------------+
| notesd:debian        | dozens (mostly low/  | a Debian userland is hundreds of packages|
|                      | medium, OS packages) | each a potential CVE                     |
| notesd:distroless    | ~0 OS-package CVEs   | there are almost no packages to be       |
|                      | (only your Go deps)  | vulnerable; only the Go binary's deps    |
+----------------------+----------------------+------------------------------------------+
```

The distroless image's near-zero OS-package CVE count is the quantitative form of "smaller is safer": a package not in the image cannot have a CVE in the image. The CVEs that *remain* on distroless are in your Go module dependencies — which `govulncheck` (the Go-native scanner) finds and which no base-image swap fixes; you fix those by updating modules. Cite the distroless security note at <https://github.com/GoogleContainerTools/distroless#why-should-i-use-distroless-images>.

## Acceptance criteria

1. A `MEASUREMENTS.md` with the four-base size table, the build-time column, and the CVE-count column, with every number filled from your own builds.
2. A `docker history notesd:distroless` excerpt proving the only owned layer is the binary `COPY`, with no toolchain layer above the base.
3. A one-paragraph explanation of each jump: 850→95 (multi-stage), 95→18 (distroless), 18→16 (scratch), and why the last jump being small means distroless is the sweet spot.
4. The CVE scan output for the debian and distroless images, with a sentence explaining why a package not present cannot be a vulnerability.
5. A proof the binary is identical across the three improved images: same `sha256sum` of the extracted binary (`docker create` + `docker cp` the binary out of each, hash them) — the same bytes on three bases.

## Stretch goals

1. **`dive` the distroless image.** Run `dive notesd:distroless` and report the image efficiency (it should be ~100% — nothing added-then-deleted). Explain why a single-binary image cannot waste space.
2. **`govulncheck` the binary.** Run `govulncheck ./...` on the source and explain the difference between an *image* CVE scan (finds OS-package CVEs) and `govulncheck` (finds CVEs in code paths your binary actually reaches). Cite <https://pkg.go.dev/golang.org/x/vuln/cmd/govulncheck>.
3. **Digest-pin and re-measure.** Pin both base images by `@sha256:` digest and rebuild; confirm the size is identical and explain how digest-pinning makes the build reproducible and the supply chain auditable.

## Deliverable

`MEASUREMENTS.md` in the `notes` repo: the four-base size/build-time/CVE table, the `docker history` excerpt, the identical-binary hash proof, and the per-jump explanation. This report backs the capstone's "cloud-native posture" axis — the grader expects you to defend *why* the image is the size it is, not just that it is small. The line this challenge defends, in one sentence: *the production image is 18 MB and carries near-zero OS-package CVEs because it is exactly the static binary plus the CA certs it needs to talk TLS, and nothing an attacker could live off.*
