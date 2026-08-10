# Behavioural testing: audit log + metrics as the oracle

The principle:

> **Make the application record what it did, then assert on that record.**

The audit log and the metrics endpoint exist for compliance and for operations.
They are also, together, a complete description of a run — so a test can drive a
fixed scenario, capture both, and diff them against a committed baseline. The
thing you built to watch production in real time becomes the thing that tells
you the app still works.

## Three layers, three questions

| | Question | Shape |
|---|---|---|
| `audit_log` | What did it *do*? | An ordered, durable narrative: who crossed the admin boundary, with what inputs |
| `/metrics` | What *is* it? | Counters — things that happened. Gauges — facts about now |
| The snapshot | Did any of that *change*? | A diff against a reviewed baseline |

Counters and gauges are not interchangeable, and the split is deliberate. A
counter belongs to the code that caused the event, so a case declares it on its
own `Env`. A gauge is a fact nobody owns — a row count would drift the first
time the purge job deleted something — so a background loop reads it from the
database. Row counts are swept per table rather than hand-picked, which means a
table added later shows up in the snapshot on its own. See
[metrics-and-shutdown.md](metrics-and-shutdown.md).

## Why the snapshot catches what unit tests do not

A normal test asserts what its author thought to check. A snapshot asserts
**everything the application recorded about itself**, so a regression nobody
anticipated still shows up as a diff.

That is not theoretical. In the codebase this kit was carved from, a GraphQL
directive shipped without a runtime implementation, and gqlgen therefore failed
*every* call to the mutation using it at argument-unmarshalling time — before
any resolver ran. Every unit test passed: the case logic was correct, and the
audit row still appeared, because it is written at the parent field before the
child fails. What changed was the *shape of the run*: the mutation's counter
stayed at zero and its table's row gauge never moved. A snapshot notices that.
An assertion nobody wrote does not.

The lesson generalises: **the seams are where things break, and the recorded
behaviour is what spans the seams.**

## What makes it work

**Determinism is the whole cost.** A snapshot test that is flaky gets deleted
within a month. So the run uses a fresh database, a fixed order of operations,
and normalises away everything that legitimately varies — row ids, timestamps,
IPs, the Go and process collector metrics. Anything that cannot be made
deterministic is excluded *explicitly*.

So is anything that moves for reasons unrelated to what the application does.
The count of rows in `schema_migrations` is the number of migration files, not a
behaviour: pinning it would make every routine migration fail this suite and
send its author to re-record a baseline they had no reason to read. The suite
asserts the table is non-empty instead, which is the part that was ever worth
proving. A baseline that cries wolf teaches people to regenerate without
looking, and then it is not a baseline.

**The record has to be worth asserting on.** Two design choices did that work:

- `action` is `mutation:banUser` — the schema field, not the client's operation
  name. A stable identifier a test can rely on.
- `detail` is the operation's variables, so the *inputs* are part of the record
  and a behavioural change in what was asked for is visible.

Nothing in this schema is a secret, which is what makes the second choice safe
today. It stops being safe the moment a mutation carries one, and
[graphql.md](graphql.md) says exactly what has to be added on that day —
including the assertion to add back here: *the raw secret must not appear
anywhere in the snapshot*. That single line covers every future leak path
through the audit log, including ones nobody has thought of. In the codebase
this grew from, a token leaked twice through two different mechanisms; a
targeted test only catches the shape it was written for, a snapshot catches the
class.

**The baseline is code and gets reviewed like code.** Regenerating it with
`UPDATE_SNAPSHOTS=1` and not reading the diff is the failure mode. The diff *is*
the review: it says, in plain terms, "this change altered what the application
does".

## What it does not do

A snapshot tells you *that* something changed, never *why*. It pairs with
targeted tests, it does not replace them:

- `e2e/bans.spec.ts` asserts a specific security property — a live session is
  revoked on its next request. A snapshot cannot express that.
- `e2e/gate.spec.ts` asserts a property of the *absence* of the panel.
- `cmd/server/tx_test.go` asserts ordering invariants — the nested-transaction
  guard, and an enqueue rolling back with its transaction — that leave no trace
  in either the audit log or the metrics.

Use the snapshot for breadth and the targeted tests for the properties you
actually care about.

## The order of the run is a dependency chain

`playwright.config.ts` spells it out as one project per spec, each depending on
the previous. That is not a preference: the suite shares one mutable database
and ends by killing the panel, so a broken link must report everything
downstream as *skipped* rather than run it against state that never happened.

```
panel-down → gate → panel-up
  → auth                 the admin signs in, which is what bootstraps the
                         admin account, and caches its session
  → boundary-and-audit   caches the non-admin session, and asserts the denial
                         reached the audit log
  → bans                 revokes and restores that exact cached session
  → admins               grants and revokes the admin role on it
  → snapshot             reads the state all of the above accumulated
  → drain                kills the panel, so nothing can follow it
```

`gate` runs against a stand where the panel container is stopped, and
`panel-up` starts it again. Nothing has touched the database at that point, so
the stop and start are invisible to every later spec.

## Running it

```bash
scripts/e2e.sh                      # assert against the committed baseline
UPDATE_SNAPSHOTS=1 scripts/e2e.sh   # rewrite it, then read the diff
```

The baseline lives in `e2e/__snapshots__/audit-and-metrics.snapshot.json`. It is
not committed with the kit: it is a recording of a run, and a run needs docker.
The first `UPDATE_SNAPSHOTS=1` on your machine creates it, and *that* diff is
the one to read most carefully, because it is the whole description of what the
kit does.

It should read as a legible trace, roughly:

```json
{"action": "sign_in",            "email": "admin@example.com", "detail": ""},
{"action": "sign_in",            "email": "user@example.com",  "detail": ""},
{"action": "denied:query:users", "email": "user@example.com",  "detail": ""},
{"action": "mutation:banUser",   "email": "admin@example.com", "detail": "{\"input\":{\"reason\":\"…\",\"userId\":2}}"},
{"action": "mutation:unbanUser", "email": "admin@example.com", "detail": "{\"input\":{\"userId\":2}}"}
```

You can read that and tell whether the application still behaves the way you
meant it to. That is the entire point.

The counters alongside it carry information the log alone does not. Two
`mutation:removeAdmin` rows next to
`starterkit_admin_changed_total{action="remove"} 1` says the first attempt was
refused — an attempt and an effect are separate facts, and the snapshot holds
both.
