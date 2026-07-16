# Week 12 Exercise Solutions

Worked examples for the three exercises: a model ADR, a model load-finding writeup, and a model runbook section, plus what graders look for and the common stumbles. These are *capstone* deliverables, so the "solutions" are exemplars to measure your own against — not answers to copy, because the content must be about *your* service.

---

## Exercise 01 — The ADRs

A model ADR (the data-layer decision), to set the bar for depth:

```markdown
# ADR 0003: Use sqlc for the data layer, not an ORM

Date: 2026-06-19
Status: Accepted

## Context
The service reads and writes a small relational domain (notes, tags, owners) in
Postgres. We need typed access from Go with predictable, reviewable queries, and
we want to reason explicitly about transactions and concurrent-write hazards.

## Decision
Write SQL by hand in `query.sql`; generate type-safe Go with sqlc; access the
database through a repository interface backed by the generated code, with a
pgx pool. Migrations are golang-migrate (up and down), expand-then-contract.

## Alternatives considered
- GORM (or ent). Rejected: the query the ORM emits is hidden, N+1s are easy to
  introduce and hard to see, and the transaction/locking story is implicit. The
  convenience is real but it costs the legibility we value in review and the
  explicit control we need over the one multi-step transaction.
- database/sql with hand-written scanning. Rejected: sqlc generates the same
  thing with compile-time-checked queries and no scan boilerplate.

## Consequences
- Every query is visible SQL, reviewed like code; an N+1 is a query you can see.
- A schema change requires regenerating (sqlc) — a build step, caught in CI.
- We own transaction boundaries explicitly (ADR 0005), which is the point.
- No lazy-loading magic; relations are fetched with explicit JOINs or batches.
```

### What graders look for

- The "Decision" matches the code. (A reviewer will open `query.sql`.)
- The "Alternatives" are real and rejected for real reasons — not a strawman.
- The "Consequences" name a downside (the build step, the explicit JOINs) — every decision costs something.
- The five ADRs cover the most-probed decisions: concurrency, data layer, transports, container, rollout.

### Common stumbles

The "ADR with no alternatives" — a description of what you did, not a record of a choice. The alternatives section is the whole value; without it, it is documentation, not a decision record.

The "ADR that does not match the code" — claiming a decision you did not implement (a cache you describe but never built). The defense catches it immediately.

The "no consequences" — pretending the decision is free. A senior reviewer trusts an engineer who names the trade more than one who claims there was none.

---

## Exercise 02 — The load-test-and-trace report

A model finding writeup (the structure, with the N+1 example from Lecture 1):

```markdown
## Finding: N+1 + missing index on GET /notes (p99 spike under load)

### Load profile
hey -z 60s -c 50 GET /notes  -> 50 concurrent, 60s.

### Before
RED dashboard: p50 14ms, p95 60ms, p99 122ms (the tail). Errors 0.
A p99 outlier trace in Jaeger:
  GET /notes                 120ms
    notes.List (service)     118ms
      SELECT notes           115ms        <- slow: no index on (owner_id, created)
      SELECT tags (per note)   2ms x40    <- N+1: one query per note

### Fix
1. Migration 0004: CREATE INDEX idx_notes_owner_created ON notes(owner_id, created DESC);
2. Collapsed the per-note tag query into one batched query:
   SELECT note_id, tag FROM note_tags WHERE note_id = ANY($1);

### After
p50 6ms, p95 11ms, p99 9ms. The SELECT notes span is 8ms; tags is one 1ms span.
  finding            before   after
  SELECT notes p99   115ms    8ms
  tag queries        40       1
```

### What graders look for

- One finding, fully worked: dashboard → trace → fix → re-measure.
- Quantitative before/after (numbers, query counts), not "it felt faster."
- The fix is a real committed change (a migration + a query), not a config knob.
- The narrative is "I found it with the tools," demonstrating the observability is usable.

### Common stumbles

The "wall of graphs, no finding" — capturing dashboards without identifying *one* thing and fixing it. The report is about the loop (spot → localise → fix → verify), not the screenshots.

The "I knew where it was" — fixing a bug you already knew about without showing the tools finding it. The exercise is about *localising* with the dashboard and the trace; show that.

The "no after" — a finding with no re-measurement. The fix is unproven without the after numbers.

---

## Exercise 03 — The runbook section

A model "roll back a bad rollout" section (followable cold):

```markdown
## Roll back a bad rollout

Symptom: after a deploy, the service returns 5xx or /readyz is 503 on the new pods.

RULE: roll back FIRST, diagnose SECOND. A 3am incident is mitigated by moving
traffic to the known-good version, not by debugging the broken one under load.

1. Roll back to the previous ReplicaSet:
   kubectl rollout undo -n notes deploy/notes
2. Confirm the previous version is serving:
   kubectl rollout status -n notes deploy/notes     # "successfully rolled out"
   curl -s http://localhost:8080/readyz             # -> ok
   kubectl get pods -n notes                         # all 1/1 Ready, old image SHA
3. Why this is safe: migrations are expand-only, so the previous image's code
   still understands the current schema. (If the bad deploy included a
   destructive migration, see "Roll back a bad migration" — but it should not,
   per the expand-then-contract rule.)
4. THEN diagnose the bad version (now that traffic is safe):
   kubectl logs -n notes <a-bad-pod> --previous
   - the Jaeger trace for a failing request
   - the diff in the deploying commit (code + migrations + ADRs)

If `rollout undo` reports "no rollout history", the ReplicaSet was pruned;
deploy the last-known-good SHA explicitly:
   kubectl set image -n notes deploy/notes notes=notesd:<known-good-sha>
```

### What graders look for

- Literal commands, expected output, and a "when it's wrong" branch.
- The "roll back first" rule stated loudly.
- The expand-only-migration safety explained (why the rollback is safe).
- A peer can execute it without asking a question.

### Common stumbles

The "prose runbook" — "roll back the deployment to the previous version" with no command. A runbook is commands, not instructions to figure out the commands.

The "no failure branch" — only the happy path. The value of a runbook at 3am is the "when the output is wrong, do this" branch.

The "diagnose-first ordering" — leading the section with diagnosis. Under an incident, mitigate first; the runbook's ordering should make that the obvious first move.

---

## Synthesis — these are the defense, written down

The three exercises produce three of the capstone's defended artifacts:

- **The ADRs** are the architecture Q&A, written down — "why this way?" answered before you are asked.
- **The load-and-trace report** is the observability proof — "can you actually use the tracing?" answered with a found-and-fixed finding.
- **The runbook** is the operations Q&A — "it's 3am and it's broken, what do you do?" answered with a document a stranger can follow.

In the defense, the reviewer does not grade these by reading them silently — they make you defend the ADR, they ask how you found the latency finding, and they walk a runbook section without your narration. Writing all three well *is* preparing for the defense; there is no separate prep. The capstone is 30% of the grade and a non-functional one does not pass — but a functional one with sharp ADRs, a real trace finding, and a followable runbook is what turns "it works" into "this person can ship *and* operate," which is the whole point of C30.
