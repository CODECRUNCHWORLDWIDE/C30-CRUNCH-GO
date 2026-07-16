# Challenge 1 — Land One Pull Request in an Open-Source Go Project

> **Time:** 2 hours (plus review-cycle wait). **Prerequisites:** the whole track. **Citations:** the project's `CONTRIBUTING.md`, the Go contribution guide at <https://go.dev/doc/contribute>, and the ecosystem repos (chi, pgx, sqlc, OpenTelemetry-Go, a CNCF tool).

## The premise

You have spent twelve weeks *consuming* the Go cloud-native ecosystem — `chi`, `pgx`, `sqlc`, the OpenTelemetry Go SDK, `gobreaker`, and Kubernetes itself (which is written in Go). This challenge crosses the line from consumer to contributor: land one merged (or at least review-ready, opened, and engaged) pull request in an open-source Go project. It is the SYLLABUS portfolio recommendation — "one PR merged into an open-source Go project (a CNCF project, a chi/pgx/sqlc ecosystem library, an OpenTelemetry Go component, or a Kubernetes tool)" — and it is the portfolio item that proves you can read and improve code you did not write.

The bar is *real engagement*, not a typo fix. A doc improvement that clarifies a genuinely confusing section, a small bug fix with a test, a missing example, or a well-scoped feature with the maintainer's buy-in all count. A whitespace-only PR does not.

## Finding the PR

Good sources of a first contribution:

```
   where to look                         what to look for
   ------------------------------------- -----------------------------------------
   the libraries you used in the capstone a rough edge YOU hit: a confusing doc, a
   (chi, pgx, sqlc, otel-go, gobreaker)   missing example, a small bug
   "good first issue" / "help wanted"     pre-triaged, maintainer-blessed scope
     labels on those repos
   the Kubernetes ecosystem (kind, a      docs, test coverage, a small fix
     controller, a Prometheus exporter)
   the bug you found in the capstone      if it is actually in a dependency, fix it
                                          upstream and cite it in your portfolio
```

The strongest PR is one that comes from *using* the library in your capstone — you hit a confusing doc or a small bug, you understood it (you have the context), and you fix it for the next person. That story ("I was building X, hit Y, fixed it upstream") is exactly what the portfolio writeup wants.

## The process

1. **Read the `CONTRIBUTING.md`** and the project's PR conventions *first*. Every project has its own (DCO sign-off, a CLA, a commit-message format, a test requirement). Following them is half the battle and the fastest way to a maintainer's "yes."
2. **Open an issue or comment first** for anything non-trivial — "I'd like to fix X, here's my plan" — so you do not spend two hours on a PR the maintainer would reject for scope.
3. **Make the change small and focused.** One concern per PR. Include a test for a behaviour change. Run the project's linters and tests locally before pushing (`go test ./...`, `go vet`, the project's CI commands).
4. **Write the PR description** like an ADR-lite: what, why, how you tested it, and what you considered. A clear description gets reviewed faster.
5. **Engage with review.** Address the feedback promptly and graciously; a merged PR almost always goes through a round or two. The engagement is part of the deliverable even if the merge lands after the defense.

## Acceptance criteria

1. A link to the opened PR in a public Go open-source repo, following that project's contribution conventions.
2. The PR is *substantive* — a doc clarification of a genuinely confusing section, a bug fix with a test, a missing example, or a scoped feature — not a whitespace/typo-only change.
3. The PR description explains what, why, and how it was tested.
4. Evidence of engagement: you responded to any maintainer feedback (or, if not yet reviewed, the PR is well-formed and conventions-following).
5. A one-paragraph portfolio writeup: what you contributed, to which project, and what using it in the capstone taught you that motivated the fix.

## Stretch goals

1. **Get it merged.** A merged PR (the green "Merged" badge) is the strongest version. If the review cycle outlasts the defense, document the in-flight state and update when it lands.
2. **Fix the capstone's own dependency bug upstream.** If your load-and-trace report or your tests surfaced a bug in `pgx`/`sqlc`/`otel-go`, fix it upstream and link the capstone finding to the upstream PR — a tight "found it building, fixed it for everyone" story.
3. **Contribute a `gobreaker`/reliability example.** Many reliability libraries lack a worked Kubernetes-context example; contribute one based on your Week 11 work.

## Deliverable

The PR link, the description, the engagement evidence, and the portfolio paragraph, recorded in `portfolio/open-source-pr.md`. This is a SYLLABUS portfolio item and a defense talking point ("walk me through a time you read code you didn't write"). The line this challenge defends: *I do not just use the cloud-native Go ecosystem; I can read it, find a rough edge, and fix it for the next person — which is the difference between an operator of the ecosystem and a contributor to it.*
