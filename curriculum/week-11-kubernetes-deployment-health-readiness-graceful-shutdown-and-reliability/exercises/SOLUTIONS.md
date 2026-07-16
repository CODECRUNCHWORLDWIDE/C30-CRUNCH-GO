# Week 11 Exercise Solutions

Worked solutions to the three exercises: the canonical implementation, the verification output the grader looks for, and the most common ways each gets done wrong. Read your own solution first; check it against the canonical one second. The point is not to copy — it is to surface the patterns and the failure modes so you recognize them when your own deploy or drain goes wrong.

---

## Exercise 01 — Deploy to `kind` with honest probes

The canonical manifests are the Lecture 1 ConfigMap, Secret, Deployment, and Service, plus a minimal in-cluster Postgres:

```yaml
# deploy/k8s/postgres.yaml — a single-replica Postgres for the cluster.
apiVersion: apps/v1
kind: Deployment
metadata: { name: postgres, labels: { app: postgres } }
spec:
  replicas: 1
  selector: { matchLabels: { app: postgres } }
  template:
    metadata: { labels: { app: postgres } }
    spec:
      containers:
        - name: postgres
          image: postgres:16
          env:
            - { name: POSTGRES_DB,       value: notes }
            - { name: POSTGRES_USER,     value: notes }
            - { name: POSTGRES_PASSWORD, value: devpass }
          ports: [{ containerPort: 5432 }]
          readinessProbe:
            exec: { command: ["pg_isready", "-U", "notes"] }
            periodSeconds: 5
---
apiVersion: v1
kind: Service
metadata: { name: postgres }
spec:
  selector: { app: postgres }
  ports: [{ port: 5432, targetPort: 5432 }]
```

### Verification output

1. `kubectl get pods -n notes` shows 3 `notes` pods at `1/1 Ready` and 1 `postgres` at `1/1`.
2. `curl http://localhost:8080/readyz` (through the port-forward) returns `ok`.
3. Scaling Postgres to 0 makes the `notes` pods go `0/1` (NotReady) within ~10s, with `RESTARTS` staying `0` — liveness unaffected.
4. `curl http://localhost:8080/healthz` still returns `ok` while Postgres is down.
5. Scaling Postgres back makes readiness recover to `1/1` with no restart.
6. `kubectl get pod -n notes <pod> -o jsonpath='{.spec.securityContext.runAsNonRoot}'` → `true`.

```text
$ kubectl get pods -n notes
NAME                     READY   STATUS    RESTARTS   AGE
notes-7d9f6c8b9-2xk4l    1/1     Running   0          2m
notes-7d9f6c8b9-8wq7m    1/1     Running   0          2m
notes-7d9f6c8b9-pl3vn    1/1     Running   0          2m
postgres-6c4d9f7-abcde   1/1     Running   0          3m

# After `kubectl scale deploy/postgres --replicas=0`:
notes-7d9f6c8b9-2xk4l    0/1     Running   0          3m   <- NotReady, RESTARTS still 0
```

### Common stumbles

The "pods never go Ready": the readiness probe path or port is wrong (e.g. pointed at the gRPC port), or `/readyz` cannot reach Postgres (the Secret's `NOTES_DATABASE_URL` points at `localhost` instead of the `postgres.notes` service name). `kubectl describe pod` shows the failing probe; `kubectl logs` shows the DB connection error.

The "ImagePullBackOff": you forgot `kind load docker-image`, or the Deployment's `imagePullPolicy` is `Always` (which tries to pull from a registry that does not have your local image). Set `imagePullPolicy: IfNotPresent` and `kind load` the image.

The "pods restart on Postgres outage": your **liveness** probe checks the database. That is the classic mistake — a DB blip becomes a restart storm. Liveness must depend on nothing external; only readiness checks Postgres.

The "container won't start, CreateContainerConfigError / runAsNonRoot": the image runs as root (you skipped the Week 10 `USER` line) but the `securityContext` says `runAsNonRoot: true`. The kubelet refuses it — correctly. Fix the image to run as 65532.

The "readonly filesystem" crash: `readOnlyRootFilesystem: true` without the `emptyDir` `/tmp` mount, and the service writes to `/tmp`. Mount the `emptyDir`.

---

## Exercise 02 — Graceful shutdown

The canonical solution is the Lecture 2 `run` skeleton. The `health` toggle that makes the readiness-fail-then-wait pattern work:

```go
// internal/health/ready.go
package health

import (
	"net/http"
	"sync/atomic"
)

var ready atomic.Bool

func init() { ready.Store(true) }

func SetReady()    { ready.Store(true) }
func SetNotReady() { ready.Store(false) } // called on SIGTERM, before the drain

// Readiness returns 503 once SetNotReady is called, even if the DB is fine —
// so on SIGTERM the Service stops routing BEFORE we stop accepting.
func Readiness(ping func() error) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if !ready.Load() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("shutting down"))
			return
		}
		if err := ping(); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("db unreachable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
```

### Verification output

1. `go test -race ./...` green.
2. The in-flight `?slow=8` request completes with 200 when its serving pod is deleted mid-request.
3. `kubectl logs <pod> --previous | grep drain` shows `draining` then `drain complete`.
4. `kubectl describe pod <pod>` on the terminated pod shows it exited `Completed`/exit 0, not `Error`/137 (SIGKILL).
5. The drain finishes within the grace period.

```text
$ kubectl logs -n notes notes-...-2xk4l --previous | grep -E 'drain|shutdown'
{"time":"...","level":"INFO","msg":"shutdown signal received, draining","grace":"20s"}
{"time":"...","level":"INFO","msg":"drain complete"}
```

### Common stumbles

The "in-flight request dropped": you close the pgx pool *before* draining the HTTP server, so the in-flight handler loses its connection mid-query. Order matters: drain servers first, close the pool last (the deferred `pool.Close()` after `g.Wait()`).

The "pod gets SIGKILLed (exit 137)": your shutdown budget exceeds `terminationGracePeriodSeconds`, so Kubernetes kills you mid-drain. Size the budget under the grace period with a margin.

The "g.Wait() reports an error on a clean shutdown": you did not special-case `http.ErrServerClosed`/`grpc.ErrServerStopped`. Those are the *clean*-exit sentinels returned by `Serve` when you call `Shutdown`/`GracefulStop`; treat them as success.

The "draining pod still gets new requests then refuses them": you skipped the readiness-fail-then-wait step, so kube-proxy was still routing to the pod when it stopped accepting. Flip readiness to not-ready, sleep `PreStopDelay`, *then* `Shutdown`.

The "GracefulStop hangs forever": a long-lived streaming RPC never finishes, and `GracefulStop` waits for it. Wrap it in a `select` with the budget and force `Stop()` if it blows — you must exit before SIGKILL.

---

## Exercise 03 — Reliability patterns

The canonical retry client is the Lecture 3 `Do`. The test:

```go
// internal/retry/retry_test.go
package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

type tempErr struct{}

func (tempErr) Error() string   { return "temp" }
func (tempErr) Temporary() bool { return true }

func TestDo(t *testing.T) {
	always := func(error) bool { return true }
	fast := func() (time.Duration, time.Duration) { return time.Millisecond, 5 * time.Millisecond }

	t.Run("succeeds first try", func(t *testing.T) {
		b, m := fast()
		calls := 0
		err := Do(context.Background(), 3, b, m, always, func(context.Context) error {
			calls++
			return nil
		})
		if err != nil || calls != 1 {
			t.Fatalf("err=%v calls=%d, want nil/1", err, calls)
		}
	})

	t.Run("retries then succeeds", func(t *testing.T) {
		b, m := fast()
		calls := 0
		err := Do(context.Background(), 5, b, m, always, func(context.Context) error {
			calls++
			if calls < 3 {
				return tempErr{}
			}
			return nil
		})
		if err != nil || calls != 3 {
			t.Fatalf("err=%v calls=%d, want nil/3", err, calls)
		}
	})

	t.Run("permanent error not retried", func(t *testing.T) {
		b, m := fast()
		calls := 0
		perm := errors.New("400 bad request")
		err := Do(context.Background(), 5, b, m,
			func(error) bool { return false }, // never retry
			func(context.Context) error { calls++; return perm })
		if !errors.Is(err, perm) || calls != 1 {
			t.Fatalf("err=%v calls=%d, want perm/1", err, calls)
		}
	})

	t.Run("context cancel beats the retry loop", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := Do(ctx, 5, 10*time.Millisecond, time.Second, always,
			func(context.Context) error { return tempErr{} })
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err=%v, want context.Canceled", err)
		}
	})
}
```

### Verification output

1. `go test -race ./internal/retry/...` green; all subtests pass.
2. The breaker unit test shows: closed → (5 failures) → open → (`ErrOpenState` fast) → (cooldown) → half-open → (success) → closed.
3. `hey -z 20s -c 200` against `NOTES_MAX_INFLIGHT=50`: ~50 concurrent succeed with low p99; the rest return 503 *fast* (microseconds, not the handler latency); the service does not crash.
4. `grep -rn "QueryRow\|Query\|Exec" internal/` shows every call site passing a deadline-bearing `context`, none passing `context.Background()`.

### Common stumbles

The "retry storm" mistake: backoff with no jitter. Every client that failed together retries together — a thundering herd. Use full jitter (`rand.Int63n([0,backoff))`).

The "retries outlive the request": no `select` on `ctx.Done()` between attempts, so a 3s-deadline request becomes a 30s retry marathon. The `select` ties the loop to the request deadline.

The "retrying a 400": the `shouldRetry` predicate returns true for non-transient errors. A 400 will fail again — retrying wastes the deadline. Retry only transient, idempotent failures; never `context.DeadlineExceeded`.

The "breaker never opens": `ReadyToTrip` requires too many requests, or you reset the window too often. Tune `ReadyToTrip` (>50% of ≥5) and the `Interval`.

The "load-shedding queues instead of sheds": you used a blocking semaphore acquire (`sem <- struct{}{}` with no `default`), which *queues* the excess instead of rejecting it — turning saturation into a latency collapse. The `default` case is the shed; without it there is no shedding.

The "503s are slow": you shed *after* doing expensive work. Shed at the *front* of the middleware chain, before the handler does anything, so a shed request costs microseconds.

---

## Synthesis — how the three exercises connect

The three exercises are the three halves of the operational contract (the math is deliberate — they overlap):

- **Exercise 01** produced the **honest probes**: liveness that never lies a healthy process into a restart storm, readiness that pulls a pod from rotation when it cannot serve. The cluster now knows the truth about each pod.
- **Exercise 02** produced the **graceful drain**: a `SIGTERM` finishes in-flight work instead of dropping it, using the Week 4 `context`/`errgroup` machinery on `main`. The cluster can now terminate a pod without breaking a request.
- **Exercise 03** produced the **blast-radius containment**: timeouts so a slow dependency fails fast, jittered retries so a blip is absorbed not amplified, a breaker so a dead dependency gets a rest, and load-shedding so saturation degrades gracefully.

Stacked, they are the operational contract Lab 11 assembles and the reliability drill proves: a rolling deploy drops zero requests (honest readiness + graceful drain), and a dependency outage stays contained (timeouts + retries + breaker). That is what "the platform team trusts this service" means, and it is exactly what the capstone's concurrency-and-reliability axis (20%) and cloud-native-posture axis (15%) are scored on.
