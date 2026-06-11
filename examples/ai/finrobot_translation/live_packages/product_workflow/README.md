# FinRobot Product Workflow Live Package Skeleton

This directory defines the provider-free skeleton for a future
`leia-finrobot-product-workflow` live package. It does not import finance
providers, web frameworks, databases, LLM providers, or deployment SDKs. The
files here are manifest and contract fixtures that a live implementation must
pass before it can replace the replay-only examples.

## Scope

- Equity CLI workflow parity for staged report generation, stale data handling,
  artifact planning, and deterministic task history.
- Web product parity for route contracts, auth/session fixtures, report history,
  task logs, artifact downloads, CRUD operations, and admin views.
- UI/template snapshot parity for route-to-template mapping, static asset
  manifests, accessibility snapshot requirements, and visual regression
  metadata without browser, network, or credential dependencies.
- Auth/session lifecycle, report request state transitions, task event ordering,
  and owner-or-admin artifact download authorization fixtures.
- Route/session state contracts that bind each web route to fixture-backed
  session states, role requirements, side effects, and denied cases.
- CRUD command envelopes with actor, idempotency key, expected state, payload,
  and deterministic fixture result identifiers.
- Download artifact manifests with local fixture paths, media types, checksums,
  byte sizes, and no signed URL or remote object storage dependency.
- Approval boundaries that deny live provider calls, network fetches,
  credential reads, external deployment, and provider SDK imports by default.
- Workflow replay fixtures that specify deterministic inputs, ordered events,
  outputs, and no provider callbacks.
- Database migration contract for users, sessions, report requests, reports,
  artifacts, task events, audit events, and schema version tracking.
- Migration versioning and rollback fixtures for deterministic provider-free
  upgrade and downgrade checks.
- Deployment capability gates for local, container, cloud run, scheduler,
  object storage, database, and observability capabilities.

## Provider-free contract

The skeleton defaults `live_network` to `false`, declares no credential names,
and has no required or optional secrets. Capability gates describe what a future
package must request before it can perform live work. Tests must continue to run
with fixtures only.
