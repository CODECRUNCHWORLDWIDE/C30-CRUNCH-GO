# Challenge 2 — Write Two System-Design Dossiers, Naming the Go Primitives and Cloud-Native Pieces

> **Time:** 2 hours. **Prerequisites:** the whole track; the capstone as the worked example. **Citations:** the SYLLABUS career-pack system-design prompts; the SRE book for the reliability reasoning; the Go primitives from across C30.

## The premise

A senior backend-Go interview almost always includes a system-design round, and the C30 difference is that you design *with a Go lens* — you name the actual Go primitives (a bounded worker pool with `errgroup`, a `context`-cancellable pipeline, a `pgx` transaction, a `gobreaker` breaker, a readiness probe, an expand-only migration), not generic boxes-and-arrows. This challenge has you write two design dossiers from the SYLLABUS prompts. The capstone is your worked example to reason from, but the dossiers must go *past* what you built — the point is to show you can reason about a system you have not yet built.

## The six prompts (pick two)

```
   prompt                                what it stresses
   ------------------------------------- --------------------------------------------
   design a URL shortener                read-heavy scale, caching, key generation
   design a rate limiter                 token bucket, distributed state, races
   design a job queue                    at-least-once delivery, idempotency, workers
   design a feature-flag service         low-latency reads, push vs poll, consistency
   design a deploy that drops zero reqs  readiness + graceful drain + rollout strategy
   design a service that survives a DB    failover, timeouts, retries, breaker, the
     failover                            readiness probe telling the truth
```

The last two are the strongest choices for a C30 graduate, because your capstone *is* a worked example of both — the dossier pushes each one realistic step past what you built.

## What a dossier must contain

Each dossier is 2–3 pages and must name specifics, not hand-wave. Four required parts (the SYLLABUS career-pack standard):

1. **The data model.** Entities, keys, and *the indexes that make the hot queries fast*. "A `urls` table keyed on the short code, with a unique index on the code and a covering index on `(code) INCLUDE (target, created)` so the redirect read is index-only." Name the columns and the indexes, not "a database."

2. **The scaling story.** Where state lives, what scales horizontally, what the bottleneck is and *at what load*. "The service is stateless (12-factor), so it scales horizontally behind a Service; the bottleneck is the database read at ~50k redirects/s, addressed by a read-through cache (Redis) with the short-code → target mapping, which moves the bottleneck to cache capacity at ~500k/s." Name the number and the next bottleneck.

3. **The failure modes.** What happens when the database is slow, when a dependency is down, when a deploy is bad — and *how each is detected and mitigated*. "A slow database trips the per-call `context` timeout (fail fast, not wedge); a down database trips the `gobreaker` breaker (sub-ms fail-fast, bounded goroutines) and flips readiness to 503 (pulled from the Service); a bad deploy is caught by the new pods' readiness probe (never takes traffic) and rolled back with `kubectl rollout undo`." This is your Weeks 9–11 reasoning, generalised.

4. **The Go primitives.** Explicitly name the Go pieces: the `errgroup`-bounded worker pool, the `context` propagation, the `pgx` pool and transaction, the `sqlc` queries, the `gobreaker` breaker, the retry-with-jitter, the graceful shutdown, the readiness probe. The Go lens is what distinguishes a C30 dossier from a generic one.

## Pushing past the capstone

The dossier exists to show you can reason about a system you have *not* built, so push each design one realistic step past your capstone:

```
   capstone reality              push the dossier to
   ----------------------------- -----------------------------------------------
   one Postgres                  "now there are 10x reads — add a read replica
                                  and route reads to it; reason about replica lag"
   3 replicas on kind            "now it's 100 pods across 3 regions — reason
                                  about the data layer and cross-region latency"
   one service                   "now a tenant has 100M rows — reason about
                                  partitioning and the index that still works"
   a single dependency           "now there are 3 downstreams — reason about the
                                  blast radius if the slowest one is slow"
```

Writing the design you already built is the easy half; the dossier proves you can reason past it.

## Acceptance criteria

1. Two dossiers (2–3 pages each) from two different SYLLABUS prompts, in `portfolio/system-design/`.
2. Each has all four parts: the data model (with named indexes), the scaling story (with a bottleneck and a load number), the failure modes (with detection + mitigation), and the named Go primitives.
3. Each pushes one realistic step past the capstone (a scale, a region, a row count, a dependency count).
4. The reasoning names actual Go and cloud-native pieces, not generic boxes — a reviewer can tell you have *built* one of these, not just read about it.
5. The reliability reasoning (failure modes) is consistent with what you proved in Weeks 9–11 (timeouts, retries, breaker, honest readiness, graceful drain).

## Stretch goals

1. **Whiteboard one live.** Have a peer give you one of the remaining four prompts cold and design it in 30 minutes out loud, naming the Go primitives. This is the actual interview format; the dossiers are the rehearsal.
2. **Design the failure you survived.** Take your reliability-drill postmortem's scenario and write the "design a service that survives this" dossier — your drill is the evidence the design works.

## Deliverable

Two dossiers in `portfolio/system-design/`, plus a note on which prompt you would be *least* comfortable being asked cold (and a plan to close that gap). This is a SYLLABUS career-pack deliverable and the system-design-round prep. The line this challenge defends: *I can design a cloud-native Go service to a senior bar — naming the data model, the scaling bottleneck, the failure modes, and the actual Go primitives — because I built one, operated it, and broke it on purpose, so the design is reasoning from experience, not from a blog post.*
