# Exercise 01 — Deploy `notes` to a `kind` Cluster with Honest Probes

> **Time:** ~90 minutes. **Prerequisites:** Lecture 1; the Week 10 `notes` image; `kind` and `kubectl` installed.

## Goal

Stand up a `kind` cluster, run an in-cluster Postgres, deploy the Week 10 `notes` image under a Deployment behind a Service with a ConfigMap and a Secret, wire honest liveness/readiness probes and a hardened `securityContext`, and reach the service through the Service.

## Steps

1. **Create the cluster** and load the image (no rebuild needed if Lab 10 is done):
   ```bash
   kind create cluster --name notes
   docker build -t notesd:dev .          # the Week 10 Dockerfile
   kind load docker-image notesd:dev --name notes
   kubectl create namespace notes
   ```

2. **Deploy an in-cluster Postgres** (`deploy/k8s/postgres.yaml`): a Deployment + Service named `postgres` in the `notes` namespace, `postgres:16`, with the `notes` db/user/password set via env. (A single replica is fine for the lab.)

3. **Write the manifests** under `deploy/k8s/`:
   - `configmap.yaml` — the non-secret `NOTES_*` config (env, ports, OTLP endpoint, log level, shutdown timeout).
   - `secret.yaml` — `NOTES_DATABASE_URL` (points at `postgres.notes:5432`).
   - `deployment.yaml` — 3 replicas, `RollingUpdate` with `maxUnavailable: 0` / `maxSurge: 1`, `envFrom` the ConfigMap and Secret, the hardened `securityContext` (`runAsNonRoot`, `runAsUser: 65532`, `readOnlyRootFilesystem`, dropped capabilities, an `emptyDir` `/tmp`), `terminationGracePeriodSeconds: 30`, and the liveness/readiness probes.
   - `service.yaml` — a ClusterIP exposing http and grpc.

4. **Wire the probes**: liveness `httpGet /healthz` (depends on nothing), readiness `httpGet /readyz` (pings Postgres with a short deadline). Confirm the `notes` `/readyz` handler pings the pool.

5. **Apply and observe**:
   ```bash
   kubectl apply -n notes -f deploy/k8s/
   kubectl get pods -n notes -w        # watch 0/1 -> 1/1 (readiness passing)
   kubectl port-forward -n notes svc/notes 8080:80
   curl -s http://localhost:8080/readyz   # ok
   ```

6. **Prove the readiness probe is honest**: scale Postgres to 0 (`kubectl scale -n notes deploy/postgres --replicas=0`), watch the `notes` pods go `1/1 -> 0/1` (readiness now 503, pods pulled from the Service), confirm `/healthz` still returns 200 (liveness unaffected — the process is alive), then scale Postgres back and watch readiness recover.

## Acceptance criteria

- All 3 `notes` pods reach `1/1 Ready`; the Service routes to them.
- `curl /readyz` returns `ok` through the port-forward.
- When Postgres is down, readiness returns 503 and the pods go `0/1` (NotReady) but are **not restarted** (liveness still 200, restart count stays 0).
- When Postgres returns, readiness recovers to `1/1` with no pod restart.
- `kubectl get pod -n notes <pod> -o jsonpath='{.spec.securityContext.runAsNonRoot}'` is `true`; a pod attempting to run as root would be refused.

## Stretch

Add a `startupProbe` and explain (in a comment) when it would matter for a service with a slow boot, and why `notes` (a fast static binary) does not need one. Also try `kubectl debug` to attach an ephemeral container to a shell-less pod.
