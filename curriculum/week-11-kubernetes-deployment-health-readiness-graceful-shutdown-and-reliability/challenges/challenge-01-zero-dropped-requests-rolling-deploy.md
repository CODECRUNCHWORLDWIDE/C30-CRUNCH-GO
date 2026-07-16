# Challenge 1 — Roll Out a New Version of `notes` Under Load on `kind` and Prove Zero Dropped Requests, with a Broken Control Run That Proves the Probes and the Drain Matter

> **Time:** 2 hours. **Prerequisites:** Lectures 1–2; Exercises 1–2; a deployed `notes` on `kind` with honest probes and graceful shutdown. **Citations:** the rolling-update doc at <https://kubernetes.io/docs/concepts/workloads/controllers/deployment/#rolling-update-deployment>, the probes doc at <https://kubernetes.io/docs/tasks/configure-pod-container/configure-liveness-readiness-startup-probes/>, the pod-termination doc at <https://kubernetes.io/docs/concepts/workloads/pods/pod-lifecycle/#pod-termination>, and `hey` at <https://github.com/rakyll/hey>.

## The premise

"One image, deployed to the cluster" is worthless if the deploy you reach with drops every in-flight request. This challenge proves the claim quantitatively: you run a steady stream of requests against the deployed `notes` while you roll out a new version, and you show the dropped-request count is zero (or you find the bug that makes it non-zero and fix it). This is the operational backbone of the whole track — and it is the reliability drill the capstone requires you to run on yourself.

The mechanism you are proving is a **handoff, not a swap**. With `maxUnavailable: 0` and `maxSurge: 1`, Kubernetes creates a new pod *alongside* the running ones, waits for its readiness probe to pass before adding it to the Service endpoints, and only then terminates an old pod — which fails its own readiness, waits for the endpoint removal to propagate, then drains its in-flight requests on `SIGTERM`. At no instant is there no ready pod taking traffic, and no terminating pod drops an in-flight request. Picture the timeline:

```text
  t0  v1-a, v1-b, v1-c: Ready, 100% traffic       v2-a: (does not exist)
  t1  v1-a, v1-b, v1-c: Ready                       v2-a: created, readiness polling, NOT in endpoints
  t2  v1-a, v1-b, v1-c: Ready                       v2-a: readiness PASSED, added to endpoints
  t3  v1-a: SIGTERM (readiness->503, drain)         v2-a, v1-b, v1-c: Ready, serving
  t4  v1-a: drained, exited                          v2-a, v1-b, v1-c: serving
       ^----- at no point is there no ready pod, and v1-a finished its in-flight work -----^
  ... repeat for v1-b, v1-c until all are v2.
```

By the end you will have produced: (a) a load-generator log across a full rollout showing zero non-2xx attributable to the deploy; (b) a written explanation of which two mechanisms — honest readiness and graceful drain — make the difference; and (c) two broken control runs that each remove one mechanism and watch requests drop.

## Setup

Confirm the Deployment has the safe rollout settings and the probes/drain from Exercises 1–2:

```bash
kubectl get deploy notes -n notes -o jsonpath='{.spec.strategy}' | jq
# {"type":"RollingUpdate","rollingUpdate":{"maxSurge":1,"maxUnavailable":0}}
```

`maxUnavailable: 0` is the load-bearing setting for *capacity*: it forbids the rollout from dropping below `replicas` ready pods, so there is always full capacity. `maxSurge: 1` lets one extra pod come up so the rollout can proceed.

A load generator — a steady stream against a cheap endpoint, classifying failures so a deploy-drop is distinguishable from an unrelated error:

```bash
#!/usr/bin/env bash
# loadgen.sh — run for the duration of the rollout.
URL="http://localhost:8080/readyz"   # through `kubectl port-forward svc/notes 8080:80`
end=$(( $(date +%s) + 120 ))
total=0; drop=0
while [ "$(date +%s)" -lt "$end" ]; do
  code=$(curl -s -o /dev/null -w '%{http_code}' --max-time 5 "$URL")
  total=$((total+1))
  case "$code" in
    2[0-9][0-9]) : ;;                                                      # fine
    000|502|503) drop=$((drop+1)); echo "$(date -u +%H:%M:%S.%3N) DROP: $code" ;;  # the one that matters
    *)           echo "$(date -u +%H:%M:%S.%3N) OTHER: $code" ;;
  esac
  sleep 0.02
done
echo "total=$total drop=$drop"
```

(`000` is curl's code for "connection failed entirely" — the most damning drop signature. A `503` from a *draining* pod that still got a request is a deploy-drop; a `503` from a readiness check during a deliberate DB outage is not — for this challenge, keep Postgres healthy so every 503/000 is deploy-attributable.) Or use `hey -z 120s -c 10 http://localhost:8080/readyz` and read its status-code distribution; the homemade loop is here so the measurement is transparent.

## The rollout drill

1. `kubectl port-forward -n notes svc/notes 8080:80` in one terminal.
2. Start `loadgen.sh` in a second terminal. Note the start time.
3. In a third terminal, trigger a rollout (change a ConfigMap value or bump an image tag and `kubectl set image`, or just `kubectl rollout restart deploy/notes -n notes`).
4. Watch the rollout: `kubectl rollout status deploy/notes -n notes` and `kubectl get pods -n notes -w` — observe new pods reach `1/1 Ready` before old pods terminate, and old pods log `draining`/`drain complete`.
5. When `loadgen.sh` finishes, record `total` and `drop`.

A correct configuration reports `drop=0`: every request was served by *some* ready pod throughout, because new pods took traffic only after readiness passed and old pods drained before dying.

One thing to get right so the zero is meaningful: the load generator must run *across* the whole rollout, not finish before it starts. A `kind` rollout of 3 pods takes tens of seconds; size the loadgen window to cover it, and cross-check the loadgen timestamps against the `kubectl get pods` transitions so the zero is anchored to a rollout that actually happened.

## The two broken control runs

Prove each mechanism matters by removing it and watching requests drop.

**Control A — remove graceful drain.** Set `terminationGracePeriodSeconds: 1` (so the pod is `SIGKILL`ed almost immediately, before it can drain) *or* deploy a build without the shutdown handler. Run the loadgen across a rollout. You will see `DROP` lines — in-flight requests on terminating pods are killed mid-flight. Record `drop > 0`, then restore the grace period.

**Control B — remove honest readiness (lying readiness).** Make `/readyz` return 200 immediately on startup, *before* the pool is connected (a readiness probe that lies "ready" when it is not). Roll out. The new pod is added to endpoints before it can actually serve, so requests routed to it during its startup window fail. Record `drop > 0`, then restore honest readiness.

```text
  control run            mechanism removed         expected loadgen result
  graceful-drain off     grace period -> 1s        drop > 0  (in-flight killed mid-rollout)
  lying readiness        /readyz 200 before pool   drop > 0  (traffic to a not-really-ready pod)
  both intact (baseline) -                          drop = 0  (handoff works)
```

Control A shows what happens when the *terminating* side breaks — old pods drop their in-flight work. Control B shows what happens when the *incoming* side breaks — new pods take traffic they cannot serve. Said together: graceful drain protects the requests *already in flight* on a dying pod, and honest readiness protects requests from being *routed to* a pod that cannot serve them. Remove either and the rollout drops requests; you need both.

## Acceptance criteria

1. A `ROLLOUT-REPORT.md` with the Deployment strategy captured, the baseline rollout's `total`/`drop=0`, and the `kubectl get pods` transition snapshots (before / during the handoff / after).
2. The wall-clock window of the rollout, cross-checked against the loadgen window, so the `drop=0` is anchored to a rollout that actually overlapped the load.
3. Control run A (graceful-drain off) captured, showing `drop > 0`, with the one-line reason.
4. Control run B (lying readiness) captured, showing `drop > 0`, with the one-line reason.
5. A 200-word section naming the two mechanisms that produce zero-downtime — honest readiness + graceful drain — and the role of `maxUnavailable: 0`.

## Stretch goals

1. **`maxSurge`/`maxUnavailable` sweep.** Try `maxUnavailable: 1` and explain how it trades capacity for rollout speed, and when each setting is right. Cite the rolling-update doc.
2. **Real load with `k6` or `hey` on a write path.** Run the drill against `POST /notes` (a write) and confirm zero drops *and* no duplicate notes (graceful drain finishes the write exactly once). Reason about why an interrupted write is worse than an interrupted read.
3. **Pod Disruption Budget.** Add a `PodDisruptionBudget` (`minAvailable: 2`) and explain how it protects the rollout (and a node drain) from taking too many pods down at once. Cite <https://kubernetes.io/docs/tasks/run-application/configure-pdb/>.

## Deliverable

`ROLLOUT-REPORT.md` in the `notes` repo: the strategy, the baseline `drop=0` with pod-transition snapshots, the two broken control runs with `drop>0`, and the 200-word mechanism explanation. This report backs the capstone's **reliability-drill** deliverable (the "rolling deploy under load" drill) and the live-demo moment where you roll a deploy under load and the grader watches `/readyz` stay green throughout. The line this challenge defends, in one sentence: *the rollout served every request because a new pod took traffic only after its readiness passed, and an old pod drained its in-flight work on `SIGTERM` before it died — honest readiness and graceful drain, both present, are what made the handoff drop zero requests.*
