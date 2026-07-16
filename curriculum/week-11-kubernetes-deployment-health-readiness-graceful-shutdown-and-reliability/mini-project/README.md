# Lab 11 — Deploy `notes` to Kubernetes: Honest Probes, Graceful Drain, Reliability Patterns, and a Zero-Drop Rolling Deploy

> **Time:** ~5 hours (Friday studio) plus the Saturday reliability drill. **Prerequisites:** Lectures 1–3; Exercises 1–3; ideally both challenges. The Week 10 `notes` image. **Citations:** the Kubernetes probes/deployment/lifecycle/security docs, the `kind` quick-start, the AWS backoff article, the SRE book, and `gobreaker` — all linked in `resources.md`.

This is the Week 11 lab and the capstone's Kubernetes-and-reliability deliverable in miniature. You take the same `notes` image you built in Week 10 — no rebuild — and you deploy it to a `kind` cluster with manifests, honest probes, graceful shutdown, and the four reliability patterns, then prove a rolling deploy under load drops zero requests. You do not build a new service. You operate the one you containerized.

## What you ship

1. **The manifests** (`deploy/k8s/`): Deployment (3 replicas, `RollingUpdate` `maxUnavailable: 0`/`maxSurge: 1`, the hardened `securityContext`, `terminationGracePeriodSeconds: 30`), Service, ConfigMap, Secret, and an in-cluster Postgres.
2. **Honest probes**: liveness (`/healthz`, no dependencies) and readiness (`/readyz`, pings Postgres with a short deadline), proven to behave correctly under a Postgres outage (NotReady, not a restart storm).
3. **Graceful shutdown**: on `SIGTERM`, fail readiness, wait for endpoint propagation, drain HTTP + gRPC, flush traces, close the pool last — within a budget under the grace period.
4. **The reliability patterns**: a `context` deadline on every outbound call, server timeouts, a retry-with-jitter client, a circuit breaker, and a load-shedding middleware.
5. **The zero-drop proof**: a `ROLLOUT-REPORT.md` (from challenge-01) showing a rolling deploy under load with `drop=0`, plus the broken control runs.

## The deploy, end to end

```bash
# 1. Cluster + image (no rebuild — the Week 10 image).
kind create cluster --name notes
docker build -t notesd:dev .
kind load docker-image notesd:dev --name notes
kubectl create namespace notes
kubectl apply -n notes -f deploy/k8s/

# 2. Pods come up and go Ready only when Postgres is reachable.
kubectl get pods -n notes -w        # 0/1 -> 1/1 as readiness passes

# 3. Reach the service through the Service.
kubectl port-forward -n notes svc/notes 8080:80 &
curl -s http://localhost:8080/readyz                       # ok
curl -s -XPOST http://localhost:8080/notes -d '{"title":"lab11","body":"on the cluster"}'

# 4. Prove honest probes: kill Postgres, pods go NotReady (not restarted), recover.
kubectl scale -n notes deploy/postgres --replicas=0        # readiness -> 503, pods 0/1
kubectl scale -n notes deploy/postgres --replicas=1        # readiness recovers, no restart

# 5. Prove graceful drain: an in-flight request survives a pod termination.
curl -s "http://localhost:8080/notes?slow=8" &
kubectl delete pod -n notes <serving-pod>                  # the curl still returns 200

# 6. Prove zero-drop rollout: loadgen across a rollout.
./loadgen.sh & kubectl rollout restart deploy/notes -n notes
# loadgen reports drop=0
```

## Acceptance criteria

A passing Lab 11 satisfies all of the following, demonstrably:

- **Deployed and reachable.** All 3 pods `1/1 Ready`; the Service routes; `/readyz` returns `ok` through the port-forward.
- **Honest probes.** A Postgres outage makes pods NotReady (readiness 503) with **zero restarts** (liveness stays 200); recovery is automatic with no restart.
- **Hardened.** `runAsNonRoot: true`, `readOnlyRootFilesystem: true`, dropped capabilities — and the pod actually runs (the Week 10 image is genuinely non-root).
- **Graceful drain.** An in-flight request started before a pod termination completes with 200; the pod logs `draining`/`drain complete` and exits 0, not 137.
- **Reliability patterns present and tested.** Every `pgx` call has a deadline; the retry client is jittered, capped, and `context`-bound; the breaker opens/half-opens/closes; the shedder returns fast 503s under saturation. `go test -race ./...` green.
- **Zero-drop rollout.** `ROLLOUT-REPORT.md` shows `drop=0` under load across a rollout, with the pod-transition snapshots and the two broken control runs.
- **Clean under the C30 bar.** `go vet`, `staticcheck`, `go test -race` all green.

## How this feeds the capstone

This lab is the capstone's **deliverable 6 (Kubernetes deployment)** and a large part of **deliverable 8 (reliability-drill postmortem)** in miniature. The capstone requires "Deployment + Service + ConfigMap manifests, honest liveness and readiness probes, and a graceful shutdown that drains in-flight work on `SIGTERM` within the termination grace period. Runs on `kind`." That is exactly this lab. The capstone's reliability-drill postmortem is one of the two challenges this week — the rolling-deploy-under-load (challenge-01) or the dependency-outage (challenge-02) — written up as a ~3–5-page postmortem. Do the lab and at least one challenge well, and the capstone's Kubernetes axis and reliability-drill deliverable are most of the way done.

The manifests, the graceful-shutdown code, and the reliability patterns all carry forward unchanged into the capstone repo. Week 12 does not add new infrastructure — it integrates, documents (the runbook), drills (the postmortem), and defends what Weeks 5–11 built.

## Submission

Push the manifests under `deploy/k8s/`, the graceful-shutdown `main`, the `internal/retry`/`internal/downstream`/`internal/middleware` reliability code, and `ROLLOUT-REPORT.md` to the `notes` repo on a `week11-lab` branch and open a PR. The PR description states the rollout's `drop=0` result, links the `ROLLOUT-REPORT.md`, and includes the `kubectl get pods` transition snapshot and the `kubectl logs ... | grep drain` output proving graceful shutdown. The single most common review comment is "your liveness probe checks the database" (the restart-storm bug) or "the in-flight request was dropped" (the close-pool-first ordering bug) — preempt both by pasting the Postgres-outage `RESTARTS: 0` evidence and the in-flight-survives-termination curl result.
