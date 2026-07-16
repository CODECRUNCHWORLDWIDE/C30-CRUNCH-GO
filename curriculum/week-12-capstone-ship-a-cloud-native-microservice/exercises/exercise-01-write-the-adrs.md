# Exercise 01 — Write Five Architecture Decision Records for Your Capstone

> **Time:** ~90 minutes. **Prerequisites:** Lecture 1; your integrated capstone. The ADR format at <https://adr.github.io/>.

## Goal

Write five ADRs in `docs/adr/` documenting the load-bearing decisions in your capstone. Each ADR turns "I built it this way" into "I chose this way, considered these alternatives, and accept these consequences" — the rehearsal for the defense Q&A.

## Steps

1. **Create `docs/adr/`** and number ADRs `0001-...md`, `0002-...md`, etc.

2. **Write five ADRs** covering at least five of these load-bearing decisions (pick the ones that are real in *your* service):
   - Channel vs mutex in your concurrent path.
   - `sqlc` vs an ORM for the data layer.
   - REST + gRPC over a shared service layer (vs one transport, vs gRPC-Gateway).
   - distroless/static vs alpine/debian for the runtime image.
   - `maxUnavailable: 0` and the rolling-update strategy.
   - The reliability stack (timeouts + jittered retries + breaker + shedding).
   - The transaction boundary for your one multi-step write.

3. **Use the four-part format** for each: Context, Decision, Alternatives considered, Consequences. Date and status each.

4. **Make the alternatives real.** "We considered X, rejected it because Y" — not a strawman. The alternatives section is what proves you made a *choice*, not a default.

5. **Cross-link** related ADRs (e.g. the dual-transport ADR references the error-mapping ADR).

## Acceptance criteria

- Five ADRs in `docs/adr/`, each with all four sections, dated and status-marked.
- Each "Decision" matches what the code actually does (a reviewer cross-checks).
- Each "Alternatives" names at least one real alternative with a real reason for rejecting it.
- Each "Consequences" names at least one downside or follow-on obligation (no decision is free).
- The ADRs collectively cover the decisions a reviewer is most likely to probe (concurrency, data layer, transports, container, rollout).

## Stretch

Write the ADR you would *least* like to defend — the decision you are least sure about — and be honest in the Consequences about what you would revisit. A reviewer rewards "here is what I would change" over false certainty.
