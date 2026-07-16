# Week 11 Resources — Kubernetes, Health/Readiness, Graceful Shutdown & Reliability

The canonical reading list for Week 11. Every URL has been opened and every technique referenced by the lectures, exercises, challenges, or the lab. Read what you need when you need it; the lecture notes tell you which section of which document is load-bearing for the technique under discussion.

Grouped by the role the document plays in the operational story — the cluster, the workload objects, the probes, graceful shutdown, rolling updates, the reliability patterns, and the SRE foundations. The "adjacent" section is the most valuable for the engineer who wants to outgrow the lectures; do not skip it.

## The cluster — `kind` and `kubectl`

- **`kind` quick-start** — <https://kind.sigs.k8s.io/docs/user/quick-start/>. Creating a cluster, the kubeconfig context.
- **`kind` — loading an image** — <https://kind.sigs.k8s.io/docs/user/quick-start/#loading-an-image-into-your-cluster>. `kind load docker-image`; why `imagePullPolicy: IfNotPresent` matters for local images.
- **Install `kubectl`** — <https://kubernetes.io/docs/tasks/tools/>. The CLI for everything this week.
- **`kubectl` cheat sheet** — <https://kubernetes.io/docs/reference/kubectl/cheatsheet/>. `apply`, `get -w`, `describe`, `logs --previous`, `port-forward`, `rollout status`, `scale`, `debug`.

## The workload objects

- **Deployments** — <https://kubernetes.io/docs/concepts/workloads/controllers/deployment/>. Replicas, the pod template, the rollout strategy; the canonical doc for the Deployment manifest.
- **Rolling update deployment** — <https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment>. `maxSurge`, `maxUnavailable`, and why `maxUnavailable: 0` gives a zero-capacity-loss rollout.
- **Service** — <https://kubernetes.io/docs/concepts/services-networking/service/>. ClusterIP, endpoints, how the Service load-balances across ready pods.
- **ConfigMap** — <https://kubernetes.io/docs/concepts/configuration/configmap/>. Non-secret config; `envFrom`.
- **Secret** — <https://kubernetes.io/docs/concepts/configuration/secret/>. The credential out of the ConfigMap; base64-at-rest; RBAC.
- **Configure a Pod to use a ConfigMap** — <https://kubernetes.io/docs/tasks/configure-pod-container/configure-pod-configmap/>. Projecting config as env vars — where the Week 10 12-factor config lands.

## The probes

- **Liveness, readiness, startup probes** — <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>. The three probe types; `httpGet`; `initialDelaySeconds`/`periodSeconds`/`failureThreshold`; the liveness-vs-readiness distinction. The single most important page this week.
- **Pod lifecycle** — <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/>. Pod phases, conditions, and the termination sequence.
- **Pod termination** — <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination>. `SIGTERM`, `terminationGracePeriodSeconds`, `SIGKILL`, the endpoint-removal-vs-SIGTERM race.
- **Container lifecycle hooks** — <https://kubernetes.io/docs/concepts/containers/container-lifecycle-hooks/>. The `preStop` hook; covering the endpoint-removal race.

## Graceful shutdown in Go

- **`net/http` `Server.Shutdown`** — <https://pkg.go.dev/net/http#Server.Shutdown>. Stops accepting, drains in-flight; `ErrServerClosed` is the clean-exit sentinel.
- **`os/signal` `NotifyContext`** — <https://pkg.go.dev/os/signal#NotifyContext>. A `context` cancelled on `SIGTERM`/`SIGINT` — the shutdown trigger.
- **`golang.org/x/sync/errgroup`** — <https://pkg.go.dev/golang.org/x/sync/errgroup>. Running the servers and the shutdown watcher together; the Week 4 tool applied to `main`.
- **gRPC `Server.GracefulStop`** — <https://pkg.go.dev/google.golang.org/grpc#Server.GracefulStop>. Refuse new RPCs, drain in-flight; `Stop()` for the forced fallback.
- **`pgxpool`** — <https://pkg.go.dev/github.com/jackc/pgx/v5/pgxpool>. `Close()` (called last) and `Ping(ctx)` (the readiness check).

## The reliability patterns

- **The Go context blog** — <https://go.dev/blog/context>. Deadlines and cancellation — the timeout backbone.
- **`context` package** — <https://pkg.go.dev/context>. `WithTimeout`, `WithCancel`, propagation; `DeadlineExceeded`/`Canceled`.
- **Exponential Backoff and Jitter (AWS Architecture blog)** — <https://aws.amazon.com/blogs/architecture/exponential-backoff-and-jitter/>. The canonical study; no jitter vs equal jitter vs full jitter, and why full jitter wins.
- **`sony/gobreaker`** — <https://github.com/sony/gobreaker>. The circuit-breaker library; `Settings`, `ReadyToTrip`, the closed/open/half-open states, `ErrOpenState`.
- **`hey`** — <https://github.com/rakyll/hey>. The load generator for the rollout and saturation drills.
- **`k6`** — <https://k6.io/docs/>. A scriptable load tester (alternative to `hey`) for the stretch drills.

## The pod `securityContext`

- **Pod security standards** — <https://kubernetes.io/docs/concepts/security/pod-security-standards/>. The `restricted` profile: `runAsNonRoot`, `readOnlyRootFilesystem`, dropped capabilities, no privilege escalation.
- **Configure a security context for a pod or container** — <https://kubernetes.io/docs/tasks/configure-pod-container/security-context/>. The exact fields; how `runAsNonRoot` enforces the Week 10 non-root image.
- **Debug a running pod (ephemeral containers)** — <https://kubernetes.io/docs/tasks/debug/debug-application/debug-running-pod/#ephemeral-container>. Attaching a shell to a shell-less distroless pod when you genuinely need one.
- **Pod Disruption Budget** — <https://kubernetes.io/docs/tasks/run-application/configure-pdb/>. Protecting a rollout/drain from taking too many pods down (challenge-01 stretch).

## The SRE foundations

- **Google SRE book** — <https://sre.google/sre-book/table-of-contents/>. The canonical operations text.
- **Addressing Cascading Failures** — <https://sre.google/sre-book/addressing-cascading-failures/>. Circuit breaking, retries, the failure modes the patterns prevent.
- **Handling Overload** — <https://sre.google/sre-book/handling-overload/>. Load-shedding, graceful degradation, why a bounded queue beats an unbounded one.
- **SRE workbook — incident response** — <https://sre.google/workbook/incident-response/>. The discipline behind the reliability-drill postmortem.

## Adjacent reading — strongly recommended

- **"Kubernetes best practices: terminating with grace" (GKE blog)** — the graceful-shutdown-on-`SIGTERM` narrative with the endpoint-removal race spelled out; cross-reference with the pod-termination doc.
- **The Twelve-Factor App, factor IX (disposability)** — <https://12factor.net/disposability>. The graceful-shutdown principle the Go code implements; carried from Week 10.
- **"Making the Netflix API more resilient" / Hystrix lineage** — the origin of the circuit-breaker pattern in microservices; `gobreaker` is the Go descendant. Read for the *why* behind the state machine.
- **The Kubernetes "Production-Grade Container Orchestration" overview** — <https://kubernetes.io/docs/concepts/overview/>. The mental model of the control loop, for the engineer arriving without C15.

## Bookmarks worth saving past C30

- The liveness/readiness/startup-probes doc (you will re-read it for every service you deploy).
- The pod-termination doc.
- `net/http` `Server.Shutdown` and `os/signal` `NotifyContext`.
- The AWS backoff-and-jitter article.
- The Google SRE book's cascading-failures and overload chapters.

By the end of this week you should have all five pinned. Operating a Go service in Kubernetes is a handful of recurring decisions — honest probes, a graceful drain budget, timeouts, jittered retries, a breaker, shedding — and these documents are the source for each; the time saved by not re-deriving them on the next service, or at 3am during an incident, is why they are bookmarks.
