# Lecture 1 — `kind`, the Deployment/Service/ConfigMap/Secret Manifests, the `securityContext`, and Honest Liveness vs Readiness Probes

## Why this lecture exists

Week 10 produced the artifact: a small, static, non-root, distroless `notes` image configured entirely from the environment, proven to compose with Postgres and the observability stack. This lecture puts it on a real Kubernetes cluster running on your laptop, with the manifests a production cluster runs and the probes that decide when a pod is alive and when it should take traffic.

Three jobs. First, stand up a `kind` cluster and get the Week 10 image running under a Deployment behind a Service, configured by a ConfigMap and a Secret — the same `NOTES_*` environment variables the compose stack used, no image rebuild. Second, set the pod `securityContext` that *enforces* the non-root image. Third — the heart of the lecture — wire liveness and readiness probes that tell the truth, and understand precisely why the difference between them is the question that separates engineers who have operated a service from engineers who have not.

The references: the `kind` quick-start at <https://kind.sigs.k8s.io/docs/user/quick-start/>, the Kubernetes Deployment doc at <https://kubernetes.io/docs/concepts/workloads/controllers/deployment/>, and the liveness/readiness/startup-probes doc at <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>. Open all three.

## The cluster is a real cluster — on your laptop

`kind` runs a genuine Kubernetes cluster inside Docker containers. The control plane, the kubelet, the API server — all real, all running in containers on your machine. The manifests you write here are the *same* manifests a managed cluster (GKE, EKS, AKS) runs; the only difference is where the nodes live. That is the whole point: you learn and prove the operational contract on `kind` for free, and the capstone's grading requirements are all satisfiable locally.

```bash
# Create a cluster.
kind create cluster --name notes

# Confirm it is real.
kubectl cluster-info --context kind-notes
kubectl get nodes
# NAME                  STATUS   ROLES           AGE   VERSION
# notes-control-plane   Ready    control-plane   30s   v1.30.0
```

The one `kind`-specific wrinkle: `kind` does not pull from your local Docker daemon automatically. You build the image (Week 10) and *load* it into the cluster's image store:

```bash
docker build -t notesd:dev .
kind load docker-image notesd:dev --name notes
```

In the Deployment you then reference `notesd:dev` with `imagePullPolicy: IfNotPresent` (or `Never`) so the kubelet uses the loaded image rather than trying to pull from a registry that does not have it. On a managed cluster you push to a registry and pull by digest; on `kind` you `load`. The manifest is otherwise identical. Citation: the `kind` loading-an-image doc at <https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster>.

## Config and secrets — ConfigMap and Secret

Week 10's 12-factor config pays off here directly: every setting `notes` needs is a `NOTES_*` environment variable, so the cluster just has to *supply* those variables. Non-secret config goes in a ConfigMap; the credential (the database URL, which carries the password) goes in a Secret. Both project into the pod as environment variables.

```yaml
# deploy/k8s/configmap.yaml — non-secret config.
apiVersion: v1
kind: ConfigMap
metadata:
  name: notes-config
data:
  NOTES_ENV: "prod"
  NOTES_HTTP_ADDR: ":8080"
  NOTES_GRPC_ADDR: ":9090"
  NOTES_METRICS_ADDR: ":2112"
  NOTES_OTLP_ENDPOINT: "jaeger-collector.observability:4318"
  NOTES_LOG_LEVEL: "info"
  NOTES_SHUTDOWN_TIMEOUT: "20s"
---
# deploy/k8s/secret.yaml — the database URL carries the password, so it is a Secret.
apiVersion: v1
kind: Secret
metadata:
  name: notes-secret
type: Opaque
stringData:
  NOTES_DATABASE_URL: "postgres://notes:devpass@postgres.notes:5432/notes?sslmode=disable"
```

A Secret is base64-encoded, not encrypted, at rest by default — it keeps the credential out of the ConfigMap and out of `kubectl get configmap -o yaml`, and it can be RBAC-restricted and (in production) encrypted at rest or sourced from an external secrets manager. For `kind` and the capstone, a Secret with `stringData` is correct; the discipline is that the password is *not* in the ConfigMap, the Deployment, or the image. Citation: the Kubernetes Secret doc at <https://kubernetes.io/docs/concepts/configuration/secret/> and the ConfigMap doc at <https://kubernetes.io/docs/concepts/configuration/configmap/>.

## The Deployment and the Service

The Deployment declares the desired state — how many replicas of which image, with what config, what probes, what resources — and Kubernetes reconciles reality to it. The Service gives the pods a stable in-cluster name and load-balances across them.

```yaml
# deploy/k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: notes
  labels: { app: notes }
spec:
  replicas: 3
  selector:
    matchLabels: { app: notes }
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxSurge: 1          # at most one extra pod above replicas during a rollout
      maxUnavailable: 0    # never drop below `replicas` ready pods -> zero-capacity-loss rollout
  template:
    metadata:
      labels: { app: notes }
    spec:
      terminationGracePeriodSeconds: 30   # the SIGTERM->SIGKILL window (Lecture 2 budgets it)
      securityContext:
        runAsNonRoot: true                # kubelet REFUSES to start a container running as root
        runAsUser: 65532                  # the distroless 'nonroot' UID from Week 10
        fsGroup: 65532
      containers:
        - name: notes
          image: notesd:dev
          imagePullPolicy: IfNotPresent   # use the kind-loaded image; do not pull
          ports:
            - { name: http,    containerPort: 8080 }
            - { name: grpc,    containerPort: 9090 }
            - { name: metrics, containerPort: 2112 }
          envFrom:
            - configMapRef: { name: notes-config }
            - secretRef:    { name: notes-secret }
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true   # Week 10 logs to stdout, writes nothing but /tmp
            capabilities:
              drop: ["ALL"]                # the process needs no Linux capabilities
          volumeMounts:
            - { name: tmp, mountPath: /tmp }   # the one writable dir a read-only root needs
          resources:
            requests: { cpu: "100m", memory: "64Mi" }
            limits:   { cpu: "500m", memory: "256Mi" }
          livenessProbe:
            httpGet: { path: /healthz, port: http }
            initialDelaySeconds: 3
            periodSeconds: 10
            failureThreshold: 3
          readinessProbe:
            httpGet: { path: /readyz, port: http }
            initialDelaySeconds: 3
            periodSeconds: 5
            failureThreshold: 2
      volumes:
        - name: tmp
          emptyDir: {}
---
# deploy/k8s/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: notes
spec:
  selector: { app: notes }
  ports:
    - { name: http, port: 80,   targetPort: http }
    - { name: grpc, port: 9090, targetPort: grpc }
```

Read the security stanza, because it is where Week 10's non-root image becomes *enforced* rather than *hoped*. `runAsNonRoot: true` makes the kubelet refuse to start a container whose effective UID is 0 — a hard gate. `readOnlyRootFilesystem: true` means the process cannot write anywhere except the `emptyDir`-backed `/tmp`, so a compromise cannot drop a binary or persist; this is only possible because Week 10 made the service log to stdout and write nothing to disk. `capabilities: drop: ["ALL"]` removes every Linux capability — `notes` needs none. `allowPrivilegeEscalation: false` stops a setuid binary from gaining privilege. Together they are the Kubernetes `restricted` pod-security profile. Citation: <https://kubernetes.io/docs/concepts/security/pod-security-standards/>.

The `envFrom` is the 12-factor payoff: every key in the ConfigMap and the Secret becomes an environment variable in the container, so the same image that read `NOTES_DATABASE_URL` from the compose `environment:` block reads it from the Secret here, unchanged.

Deploy and observe:

```bash
kubectl create namespace notes
kubectl apply -n notes -f deploy/k8s/
kubectl get pods -n notes -w
# notes-7d9f...-abcde   0/1   Running   0   2s     <- not Ready yet (readiness probe polling)
# notes-7d9f...-abcde   1/1   Running   0   6s     <- Ready: /readyz returned 200, DB reachable
```

The `0/1 -> 1/1` transition is the readiness probe doing its job: the pod is `Running` (the process started) before it is `Ready` (the readiness probe passed). The Service routes traffic only to `Ready` pods. Reach it:

```bash
kubectl port-forward -n notes svc/notes 8080:80
curl -s http://localhost:8080/readyz   # ok
```

## Liveness vs readiness — the question that separates operators from authors

This is the conceptual heart of the lecture, and the single most-probed question in a cloud-native interview. Liveness and readiness answer *different* questions, check *different* things, and have *different* consequences when they fail.

```
+-----------+--------------------------------+-------------------+----------------------------+
| Probe     | Question it answers            | Depends on        | Consequence of failure     |
+-----------+--------------------------------+-------------------+----------------------------+
| liveness  | Is this process alive, or      | NOTHING external  | kubelet RESTARTS the       |
|           | wedged and needing a restart?  | (process-internal)| container                  |
| readiness | Should this pod receive        | EVERYTHING the    | pod removed from Service    |
|           | traffic right now?             | pod needs to serve| endpoints (no restart)     |
|           |                                | (DB, etc.)        |                            |
+-----------+--------------------------------+-------------------+----------------------------+
```

The failure of liveness is a **restart**. The failure of readiness is **removal from the load balancer** (no restart). That difference is everything.

**Why liveness must NOT check the database.** Suppose your liveness probe queried Postgres. Now Postgres has a 30-second hiccup (a failover, a brief network blip). Every pod's liveness probe fails. The kubelet restarts *every pod* — simultaneously, in a restart storm — even though every pod's *process* was perfectly healthy. You have converted a transient dependency blip into a self-inflicted outage: all your pods are now cold-starting at once, against a database that is still recovering, and the restart did nothing to fix the database. The rule: **liveness depends on nothing but the process itself.** A liveness probe should fail only when the process is genuinely wedged (a deadlock, a goroutine leak that exhausted memory) and a restart is the right cure.

**Why readiness MUST check the database.** Readiness answers "can this pod serve a request?" If Postgres is unreachable from this pod, the answer is no — and the readiness probe should return 503 so the Service stops routing traffic to it. The pod is not restarted (the process is fine); it is just pulled from the rotation until the database is reachable again, at which point readiness goes green and traffic returns. A readiness probe that *didn't* check the database would keep routing traffic to a pod that 500s every request.

The Go wiring, building on Week 9's health endpoints:

```go
// internal/health/health.go
package health

import (
	"context"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Liveness: the process is alive. Touches NOTHING external. If this handler
// runs at all, the HTTP server is up and the process is not wedged -> 200.
func Liveness(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// Readiness: can this pod serve traffic? Checks Postgres (and any other
// hard dependency). A failing dependency -> 503 -> pulled from the Service.
func Readiness(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Short deadline: readiness must answer fast or the probe times out
		// and the platform treats a slow check as a failure.
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		if err := pool.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("not ready: database unreachable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
```

Two subtleties. The readiness check has its own **short deadline** (2 seconds): a readiness probe must answer quickly: a slow check that exceeds the probe's timeout is itself treated as a failure, so the check must bound its own latency. And the readiness check pings the *pool*, not a fresh connection — it tests the connection the service actually uses. Citation: the probes doc at <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>.

## The startup probe — for slow starts

A static Go binary on distroless starts in milliseconds, so `notes` does not need one — but know it exists. A startup probe is for a process with a slow *initial* boot (warming a large cache, applying migrations at startup). While the startup probe is failing, the liveness probe is suppressed, so the kubelet does not kill a still-booting process for failing a liveness check it cannot yet pass. Once the startup probe passes once, liveness and readiness take over. For `notes`, the fast static binary means a small `initialDelaySeconds` on liveness is enough; the startup probe is the tool when boot is genuinely slow. Citation: the same probes doc, startup-probe section.

## What we built

By the end of Lecture 1, the cluster has:

- A `kind` cluster running the Week 10 `notes` image, loaded with `kind load`, with no rebuild.
- A Deployment (3 replicas, rolling-update strategy, `maxUnavailable: 0`), a Service load-balancing across the ready pods, a ConfigMap of non-secret config, and a Secret holding the database URL — every setting projected as a `NOTES_*` env var the 12-factor image reads.
- A hardened `securityContext` — `runAsNonRoot`, `readOnlyRootFilesystem`, dropped capabilities — that *enforces* the non-root image from Week 10.
- Honest probes: liveness (`/healthz`, depends on nothing) and readiness (`/readyz`, pings Postgres with a short deadline), and the understanding of why conflating them causes a restart storm or routes traffic to a dead pod.

The pod comes up, goes `Ready` only when the database is reachable, and serves traffic through the Service. But there is one thing it does not yet do correctly: when Kubernetes terminates it (a rollout, a scale-down), it gets `SIGTERM` and — right now — exits immediately, dropping in-flight requests. Lecture 2 fixes that: graceful shutdown, the disposability factor Week 10 deferred.
