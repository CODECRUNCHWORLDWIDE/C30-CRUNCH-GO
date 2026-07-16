# Exercise 03 — Draft the Production Runbook and Have a Peer Follow It Cold

> **Time:** ~90 minutes. **Prerequisites:** Lecture 2; the deployed capstone. The SRE workbook playbooks chapter at <https://sre.google/workbook/playbooks/>.

## Goal

Write the first full draft of `production-runbook.md` with all five required sections, then hand it to a peer (or read it as if you had never seen the service) and have them attempt one section without asking you a question. Revise until it is followable cold.

## Steps

1. **Write `production-runbook.md`** in the repo root with the five sections (Lecture 2):
   1. **Deploy** — every command, the verification, and the abort path (a hung rollout).
   2. **Roll back** — the bad-rollout case (`kubectl rollout undo`) *and* the bad-migration case (expand-only safety; the `migrate down` for the reversible case; the rule against destructive migrations). State "roll back first, diagnose second."
   3. **Probe semantics** — what liveness and readiness mean for *this* service, and what `SIGTERM` triggers.
   4. **The five most likely outages** — each with the first three diagnostics (commands, not prose).
   5. **Observability entry points + who to page** — log/trace/metric commands; who to page (you).

2. **Make each section followable cold**: literal commands, expected output, and the next step when the output is wrong.

3. **Hand it to a peer.** Pick one section (ideally "roll back a bad rollout"). Have them execute it on your cluster without you narrating. Watch where they get stuck.

4. **Revise** the section they got stuck on until it is followable without questions.

5. **Note** what they got stuck on and how you fixed it (this is part of the deliverable — it proves the runbook was tested by use).

## Acceptance criteria

- `production-runbook.md` has all five sections.
- Each section contains literal commands, expected output, and a "when it's wrong" next step.
- A peer (or you, cold) successfully follows one section without asking a question.
- The rollback section distinguishes a bad rollout from a bad migration and states the expand-only safety rule.
- A note records what the peer got stuck on and the revision that fixed it.

## Stretch

Have the peer follow the *hardest* section — "the deploy is throwing 500s, walk me through it." The correct path (roll back first via section 2, then diagnose via section 4) should be derivable from the runbook alone. If they diagnose-before-rollback, the runbook did not make "roll back first" loud enough — fix that.
