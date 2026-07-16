# Week 5 — Homework

Six practice problems that consolidate the week's material. They are sized to ~45 minutes each. Do them after the lectures and the exercises; do them before the mini-project. Cite the URLs you used while solving each one in the commit message of your homework branch.

## Problem 1 — The status-code map, defended

Write a 250-word post defending a status code for each of the following responses, citing the relevant RFC 9110 section for each:

1. A `POST /notes` that succeeds.
2. A `GET /notes/{id}` where the id does not exist.
3. A `POST /notes` with a body that is valid JSON but has an empty required `title`.
4. A `POST /notes` with a body that is not valid JSON at all.
5. A `DELETE /notes/{id}` that succeeds.
6. A `POST /notes` for a note whose id already exists (a uniqueness conflict).
7. A request to `/notes` with the `TRACE` method, which the service does not implement.

For each, state the code, one sentence of justification, and the RFC 9110 section. Then explain the difference between 400 and 422 in one paragraph.

Cite RFC 9110 §15 at <https://www.rfc-editor.org/rfc/rfc9110#name-status-codes>.

Deliverable: `homework/01-status-codes.md`.

## Problem 2 — The seam, audited

Take the lecture's `notes` handler/service/repository code (or your Exercise 3). For each of the following proposed changes, say whether it *belongs* in the handler, the service, or the repository, and why:

1. Trimming whitespace from a title before storing it.
2. Rejecting a request whose body is larger than 1 MiB.
3. Deciding that a note older than 90 days is "archived."
4. Returning a 404 when a note is missing.
5. Generating the note's ID.
6. Caching the most-recently-read note in memory.

Then write a short paragraph: what is the single litmus test for "does this belong in the service or the handler?" (Hint: would a CLI or a gRPC server calling the same logic need it?)

Cite the interface guidance from Week 2 and the `net/http` docs at <https://pkg.go.dev/net/http>.

Deliverable: `homework/02-seam-audit.md`.

## Problem 3 — Write a middleware

Write a new middleware, in the `func(http.Handler) http.Handler` shape, that does *one* of the following (your choice), with a test that proves it works using `httptest`:

- **A:** A simple per-IP rate limiter (token bucket, `golang.org/x/time/rate`) that returns 429 when the bucket is empty.
- **B:** An API-key auth middleware that reads `Authorization: Bearer <key>`, validates it against a set, and returns 401 on a missing key or 403 on an invalid one; on success, puts the caller's identity on the context.
- **C:** A `gzip` response-compression middleware that wraps the `ResponseWriter` when the client sends `Accept-Encoding: gzip`.

Document where in the chain your middleware belongs (relative to RequestID, Logger, Recoverer, Timeout) and why.

Cite `golang.org/x/time/rate` at <https://pkg.go.dev/golang.org/x/time/rate> (for A), the `net/http` auth header semantics in RFC 9110 §11 (for B), or `compress/gzip` at <https://pkg.go.dev/compress/gzip> (for C).

Deliverable: `homework/03-middleware.md` with the code, the test, and the placement justification.

## Problem 4 — JSON decode hardening, demonstrated

Write a small program (or test) that demonstrates each of the following decode failures and the status code your handler returns for each:

1. A body that exceeds `http.MaxBytesReader`'s cap.
2. A body with an unknown field (`DisallowUnknownFields`).
3. A body with a type mismatch (a string where a number is expected).
4. Two JSON objects glued together (the `dec.More()` check).
5. A completely empty body.

For each, paste the error message `json.Decoder` produces and the status code your `writeError` maps it to. Then write 150 words on why `DisallowUnknownFields` is worth the strictness (and the one case where it is *not* — versioned APIs that must tolerate fields a newer client sends).

Cite <https://pkg.go.dev/encoding/json#Decoder> and <https://pkg.go.dev/net/http#MaxBytesReader>.

Deliverable: `homework/04-json-hardening.md`.

## Problem 5 — Graceful shutdown timing, measured

Build the slow-handler service from Challenge 2 (or reuse it). Run three experiments and report the numbers:

1. **Adequate grace.** Handler sleeps 200ms, grace period 5s. Drive load, send SIGTERM, count drained vs dropped. Expect zero dropped.
2. **Inadequate grace.** Handler sleeps 200ms, grace period 50ms. Same. Count dropped and confirm `Shutdown` returns `context.DeadlineExceeded`.
3. **New connections during drain.** Confirm a connection attempted after SIGTERM is refused, not served.

Then answer: how should the grace period relate to (a) your slowest legitimate request and (b) Kubernetes' `terminationGracePeriodSeconds`? Why must the drain budget be *shorter* than the pod grace period?

Cite <https://pkg.go.dev/net/http#Server.Shutdown> and <https://pkg.go.dev/os/signal#NotifyContext>.

Deliverable: `homework/05-shutdown-timing.md` with the three experiments.

## Problem 6 — `httptest` two ways

Test the same `GET /v1/notes/{id}` handler two ways, and write up the difference:

1. **Unit:** `httptest.NewRequest` + `httptest.NewRecorder`, calling the handler directly with a fake service. No network, no router.
2. **Integration:** `httptest.NewServer(newRouter(...))` with a real `http.Client` hitting `srv.URL`. Real network (loopback), real router, real middleware.

For each, paste the test code, and write a paragraph on when you reach for which: what does the integration test catch that the unit test cannot (routing, middleware order, content negotiation), and what does it cost (speed, a real socket)?

Cite <https://pkg.go.dev/net/http/httptest>.

Deliverable: `homework/06-httptest-two-ways.md`.

## Submission

Push the six deliverables on a branch named `week05-homework/<your-handle>` and open a PR against the C30 curriculum repository. The PR description should link to each of the six files and include a 100-word summary of what you learned.

The teaching staff reviews homework PRs within 5 business days. Reviews focus on whether you have read the citations and whether your reasoning holds together. The single most common review comment is "where is your citation for this claim" — preempt it by linking the Go package doc or the RFC section for every non-trivial assertion.

Cited references this homework draws from: <https://pkg.go.dev/net/http>, <https://go.dev/blog/routing-enhancements>, <https://pkg.go.dev/encoding/json>, <https://pkg.go.dev/log/slog>, <https://pkg.go.dev/github.com/go-chi/chi/v5>, <https://pkg.go.dev/net/http#Server.Shutdown>, <https://pkg.go.dev/net/http/httptest>, <https://www.rfc-editor.org/rfc/rfc9110>.
