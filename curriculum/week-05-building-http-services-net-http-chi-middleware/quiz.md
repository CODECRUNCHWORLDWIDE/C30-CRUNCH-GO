# Week 5 — Quiz

Ten multiple-choice questions covering `net/http`, the 1.22 routing patterns, middleware, the handler/service/repository seam, JSON I/O, status codes, and graceful shutdown. Treat the quiz as a closed-book check; the answer key with reasoning is at the bottom.

## Question 1 — The handler interface

What is `http.Handler`?

- (A) A struct you must embed in every handler.
- (B) The interface `interface { ServeHTTP(http.ResponseWriter, *http.Request) }`; `http.HandlerFunc` adapts an ordinary function to it.
- (C) A function type `func(w, r) error`.
- (D) A `chi`-specific type, not part of the standard library.

<details>
<summary>Answer</summary>

**(B).** `http.Handler` is the one-method interface `ServeHTTP(w, r)`. `http.HandlerFunc` is an adapter that lets an ordinary `func(w, r)` satisfy it. This is the whole foundation of `net/http`. Citation: <https://pkg.go.dev/net/http#Handler>.

</details>

## Question 2 — Go 1.22 routing

With `mux.HandleFunc("GET /notes/{id}", h)` registered and no other `/notes` routes, a `POST /notes/{id}` request receives:

- (A) The same handler `h`, because the path matches.
- (B) An automatic `405 Method Not Allowed` with an `Allow` header, because the path has a route but not for `POST`.
- (C) A `404 Not Found`.
- (D) A panic, because the method does not match.

<details>
<summary>Answer</summary>

**(B).** The 1.22 `ServeMux` matches on method. A path that has a `GET` route but no `POST` route returns an automatic `405` with the correct `Allow` header — the router does this for you. Citation: <https://go.dev/blog/routing-enhancements>.

</details>

## Question 3 — The middleware shape

The canonical Go middleware signature is:

- (A) `func(w http.ResponseWriter, r *http.Request)`
- (B) `func(next http.Handler) http.Handler`
- (C) `func(r *http.Request) (*http.Response, error)`
- (D) `interface { Middleware(*http.Request) }`

<details>
<summary>Answer</summary>

**(B).** A middleware takes the next handler and returns a new handler: `func(next http.Handler) http.Handler`. The uniform signature is what lets middleware compose into a chain. Citation: <https://pkg.go.dev/github.com/go-chi/chi/v5#Mux.Use>.

</details>

## Question 4 — Onion order

You stack `Chain(mux, RequestID, Logger, Recoverer)`. Which middleware sees the request first, and why does it matter for logging?

- (A) `Recoverer` first; order does not affect logging.
- (B) `RequestID` first (outermost), so the `Logger` inside it can read the request ID it set. If `Logger` were outer, it would log an empty ID.
- (C) `Logger` first, because logging must happen before anything else.
- (D) They run concurrently; there is no order.

<details>
<summary>Answer</summary>

**(B).** `RequestID` is outermost, so it sets the ID before the inner `Logger` runs and can read it. Swapping them makes the logger log an empty ID. Order is a decision, not an accident. Citation: Lecture 2, the onion model.

</details>

## Question 5 — The seam

In the handler/service/repository seam, which statement is correct?

- (A) The handler contains the business logic; the service just forwards to the database.
- (B) The handler parses/validates/translates and calls one service method; the service owns logic and imports no `net/http`; the repository owns persistence behind an interface.
- (C) The service imports `net/http` to read the request.
- (D) The repository contains the validation rules.

<details>
<summary>Answer</summary>

**(B).** Handler = HTTP (parse, validate, call one service method, translate). Service = logic, HTTP-free, ctx-threaded. Repository = persistence behind an interface. Each layer testable in isolation. Citation: Lecture 1, the seam.

</details>

## Question 6 — Safe JSON decode

Which combination correctly hardens a JSON request decode?

- (A) `json.Unmarshal(io.ReadAll(r.Body))` with no limits.
- (B) `http.MaxBytesReader` to cap the body, a `json.Decoder` with `DisallowUnknownFields`, and a `dec.More()` check for trailing garbage.
- (C) `json.NewDecoder(r.Body).Decode(&v)` alone, since the standard library caps body size automatically.
- (D) Reading the body into a string and parsing it by hand.

<details>
<summary>Answer</summary>

**(B).** Cap the body with `MaxBytesReader`, reject unknown fields with `DisallowUnknownFields`, and reject trailing garbage with `dec.More()`. (C) is wrong — the standard library does *not* cap body size automatically; that is the bug `MaxBytesReader` fixes. Citation: <https://pkg.go.dev/encoding/json#Decoder.DisallowUnknownFields>, <https://pkg.go.dev/net/http#MaxBytesReader>.

</details>

## Question 7 — 400 vs 422

A client sends valid JSON but with an empty required `title` field. The correct status is:

- (A) `400 Bad Request`, because any client error is a 400.
- (B) `422 Unprocessable Entity`, because the body parsed correctly but failed a validation rule (`400` is for *malformed* input; `422` for input that parsed but is semantically invalid).
- (C) `500 Internal Server Error`.
- (D) `200 OK` with an empty title.

<details>
<summary>Answer</summary>

**(B).** 422 (Unprocessable Entity): the body parsed (so not 400, which is for *malformed* input) but failed a semantic rule. The 400-vs-422 distinction is exactly "could not parse" vs "parsed but invalid." Citation: RFC 9110 §15.

</details>

## Question 8 — Graceful shutdown return value

After you call `srv.Shutdown(ctx)`, the `srv.ListenAndServe()` call that was running returns:

- (A) `nil`.
- (B) `http.ErrServerClosed`, which is the *success* sentinel for a clean shutdown — not a failure; distinguish it with `errors.Is`.
- (C) `context.Canceled`.
- (D) A panic.

<details>
<summary>Answer</summary>

**(B).** `ListenAndServe` returns `http.ErrServerClosed` after a clean `Shutdown`. That is the success sentinel, not a failure — treat any *other* error as fatal. Citation: <https://pkg.go.dev/net/http#pkg-variables>.

</details>

## Question 9 — The shutdown context

When calling `srv.Shutdown(ctx)` from a handler triggered by `signal.NotifyContext`, you should pass:

- (A) The already-cancelled signal context, since the signal fired.
- (B) A fresh `context.WithTimeout(context.Background(), graceperiod)`, so `Shutdown` has a future deadline to bound the in-flight drain.
- (C) `context.TODO()`.
- (D) No context; `Shutdown` takes none.

<details>
<summary>Answer</summary>

**(B).** Pass a *fresh* context with a future deadline. The signal context is already cancelled (that is what triggered the shutdown); reusing it would make `Shutdown` return instantly without draining. Citation: <https://pkg.go.dev/net/http#Server.Shutdown>.

</details>

## Question 10 — `ReadHeaderTimeout`

Why is `ReadHeaderTimeout` the single most important `http.Server` timeout to set?

- (A) It caps the response size.
- (B) Without it, a slow-loris client can dribble request-header bytes one at a time and tie up a server goroutine indefinitely; `ReadHeaderTimeout` bounds the time to read the headers.
- (C) It controls keep-alive duration.
- (D) It is required for HTTP/2.

<details>
<summary>Answer</summary>

**(B).** `ReadHeaderTimeout` defends against slow-loris: a client dribbling header bytes one per second would otherwise hold a goroutine forever. If you set one timeout, set this. Citation: <https://pkg.go.dev/net/http#Server>.

</details>

---

## Self-assessment

- 9-10: you can ship the layered `notes-api` and defend every layer and status code.
- 7-8: re-read the lecture notes on the questions you missed; the citations point to the exact docs.
- 5-6: re-read all three lecture notes and redo the exercises, especially the seam in Exercise 3.
- 0-4: rewind to Lecture 1. The mini-project depends on the seam and the status-code map being second nature.
