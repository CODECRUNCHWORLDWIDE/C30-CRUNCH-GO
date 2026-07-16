# Challenge 2 — Protobuf Schema Evolution: v1 Clients Against v2 Servers, Guarded by `buf breaking`

> **Estimated time:** 2 hours. **Prerequisite:** Exercise 1 complete; the wire-format section of Lecture 1 understood. **Citations:** the "updating message types" rules <https://protobuf.dev/programming-guides/proto3/#updating>; field presence <https://protobuf.dev/programming-guides/field_presence/>; `buf breaking` <https://buf.build/docs/breaking/overview/>.

## The premise

Schema evolution is the production part of protobuf the tutorials skip. Your `notes` service has been live for nine months. Tomorrow you ship a schema change: a new field, a deprecated field, and a removed field. During the rolling deploy — half your servers v1, half v2, clients a mix of both — *every* combination must keep working. v1 client against v2 server. v2 client against v1 server. The whole point of protobuf's wire format is that this is *possible*; the whole point of this challenge is to prove it, and to make `buf breaking` the gate that stops a wire-incompatible change from ever merging.

## What you build

A two-version repository plus a cross-version test:

```
notes-evolution/
├── buf.yaml
├── buf.gen.yaml
├── proto/notes/v1/notes.proto        # the original (Exercise 1's Note)
├── proto/notes/v2/notes.proto        # the evolved schema
├── gen/                              # buf generate output for both
├── server/                           # one server, compiled against each version
├── client/                           # one client, compiled against each version
└── crossversion_test.go              # the (client × server) matrix
```

## Setup

### 1. The v1 `Note` (the baseline)

`proto/notes/v1/notes.proto` — exactly Exercise 1's `Note`, abridged here to the fields that change:

```proto
syntax = "proto3";
package crunch.notes.v1;
option go_package = "example.com/notesev/gen/notes/v1;notesv1";

import "google/protobuf/timestamp.proto";

message Note {
  string id = 1;
  string title = 2;
  string body = 3;
  repeated string tags = 4;
  google.protobuf.Timestamp created_at = 5;
  string legacy_color = 6;            // will be REMOVED in v2
}
```

### 2. The v2 `Note` (the evolution)

`proto/notes/v2/notes.proto` — three deliberate changes:

```proto
syntax = "proto3";
package crunch.notes.v2;
option go_package = "example.com/notesev/gen/notes/v2;notesv2";

import "google/protobuf/timestamp.proto";

message Note {
  string id = 1;
  string title = 2;
  string body = 3;
  repeated string tags = 4;
  google.protobuf.Timestamp created_at = 5;

  // CHANGE 1 (remove): legacy_color (field 6) is gone. Reserve number AND name
  // so nobody can ever reuse them and silently misinterpret old wire bytes.
  reserved 6;
  reserved "legacy_color";

  // CHANGE 2 (add, with presence): old clients omit it; new ones set it.
  optional google.protobuf.Timestamp archived_at = 7;

  // CHANGE 3 (add a structured block): a folder reference. Old clients send
  // nothing here; the field is simply absent on their wire bytes.
  message Folder {
    string id = 1;
    string name = 2;
  }
  Folder folder = 8;
}
```

Three rules were obeyed and they are the whole game: **no existing field's number changed, no existing field's type changed, and the removed field was reserved.** Everything else (adding fields, removing-and-reserving) is wire-safe.

### 3. Servers and clients

A v1 server and a v2 server each implement `GetNote`/`CreateNote` over the version they compiled against, print the fields they can see, and return a populated note. A v1 client and a v2 client each call with a populated `Note` and print the response.

## The cross-version test matrix

| Test | Client | Server | Expected behaviour |
|------|--------|--------|--------------------|
| T1   | v1     | v1     | Round-trips. Baseline. |
| T2   | v2     | v2     | Round-trips. `archived_at` and `folder` populated. |
| T3   | v1     | v2     | Round-trips. v2 server reads v1's fields 1–5; `legacy_color` (field 6) arrives as an *unknown field* and is preserved-but-ignored; `archived_at`/`folder` are empty. |
| T4   | v2     | v1     | Round-trips. v1 server reads fields 1–5; the v2-only fields 7 and 8 arrive as unknown fields and are **silently skipped**. **Critical.** |

T4 is the forward-compatibility guarantee in action: **a v1 server reading a v2 message sees field tags 7 and 8 it does not recognize, and skips them without error.** No exception, no rejected request — the unknown fields are dropped (or retained in `unknownFields`, depending on whether the field is later round-tripped). This is *why* "you may add new fields freely" is the evolution mantra.

Write each cell as a test. The shape, for T4:

```go
func TestT4_V2Client_To_V1Server(t *testing.T) {
	addr := startV1Server(t) // boots a v1 server on 127.0.0.1:0, returns addr
	conn, _ := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	defer conn.Close()

	// A v2 CLIENT against a v1 SERVER.
	client := notesv2.NewNotesServiceClient(conn)
	resp, err := client.CreateNote(context.Background(), &notesv2.CreateNoteRequest{
		Note: &notesv2.Note{
			Id:         "n-1",
			Title:      "evolves",
			ArchivedAt: timestamppb.Now(),                 // v2-only field 7
			Folder:     &notesv2.Note_Folder{Id: "f-1"},   // v2-only field 8
		},
	})
	if err != nil {
		t.Fatalf("v2->v1 should round-trip, got %v", err)
	}
	// The v1 server saw title and id; it could not see ArchivedAt or Folder.
	// Assert in the v1 server's log that fields 7 and 8 were absent from its view.
	if resp.GetNote().GetTitle() != "evolves" {
		t.Fatalf("title lost across versions: %q", resp.GetNote().GetTitle())
	}
}
```

Repeat for T1, T2, T3.

## Guard it with `buf breaking`

The whole discipline becomes a one-line CI gate. From the repo with v1 committed on `main` and v2 in your working tree:

```bash
buf breaking --against '.git#branch=main'
```

With the three wire-safe changes above, this passes silently. Now sabotage it three ways and watch it fire:

```bash
# (a) reuse the reserved number for a new field
#     string something = 6;   -> "reuses reserved number 6"

# (b) change an existing field's type
#     int64 title = 2;        -> "changed type from string to int64"

# (c) renumber an existing field
#     string title = 9;       -> "changed field number for title from 2 to 9"
```

Each is a wire break `buf breaking` catches before it can merge. The lint side complements it: `buf lint` enforces the *style* rules (versioned package, `*_UNSPECIFIED` enums) that keep the schema reviewable. Lint overview: <https://buf.build/docs/lint/overview/>.

## Acceptance criteria

- [ ] v1 and v2 protos both `buf lint` clean and `buf build` clean.
- [ ] All four matrix cells (T1–T4) pass as Go tests.
- [ ] T4 demonstrates a v1 server reading a v2 message and silently skipping fields 7 and 8 — no error returned.
- [ ] T3 demonstrates a v2 server reading a v1 message with `archived_at`/`folder` empty and `legacy_color` not surfaced as a real field.
- [ ] `buf breaking --against '.git#branch=main'` passes for the three wire-safe changes.
- [ ] Each of the three sabotage cases (a, b, c) is shown to make `buf breaking` fail, with the failure message captured.

## Reflection (write into RESULTS.md)

1. **Why bump the package to `v2`?** The challenge changed `crunch.notes.v1` to `crunch.notes.v2`. What breaks if you keep `v1` for an additive-only change? When is bumping the version mandatory versus merely tidy, and at what level (URL path, port) does the version live for a clean rolling deploy?
2. **The skip-unknown rule.** A v1 server skips v2's fields 7 and 8. In some workflows that silent drop is exactly wrong — you wanted to reject the unexpected input. Can you opt into strict parsing in grpc-go, and what would you trade away by doing so?
3. **The dangerous "safe" change.** Removing `legacy_color` is wire-safe — but if a v1 client's behavior *depended* on the server honoring that color, the feature silently stops working. Cross-version tests are a static check; what *behavioral* assertion would catch this regression?
4. **`optional` on `archived_at`.** Why declare it `optional` rather than a bare field? What changes in the v2→v2 case if you drop the keyword, and what changes in the v2→v1 case?
5. **A type change that "looks" safe.** `int32 → int64` shares the VARINT wire type and is compatible; `int32 → sint32` is *not*, despite both being VARINT. Why? (Hint: ZigZag.) What does `buf breaking` say about each?

## Stretch goals (optional)

- **Add a v3 with a deliberate wire break** (change `title` from `string` to `bytes`), run the matrix, and predict each failure mode *before* running.
- **Write the deprecation timeline:** how would your team migrate v1 → v2 over six weeks with safe gates — schema merge, server rollout, client rollout, v1 retirement — and what telemetry tells you each gate is safe to pass?

## Submission

Place all artifacts under `challenges/challenge-02/`. Commit with:

```
challenge-02: notes schema evolution; four-cell matrix + buf breaking gate
```

Include `RESULTS.md` with the five reflection answers, the four-cell matrix results, and the captured `buf breaking` output for both the passing change and the three sabotage cases.
