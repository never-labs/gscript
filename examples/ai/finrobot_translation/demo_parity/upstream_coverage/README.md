# Upstream Coverage Ledger

This directory records source-to-target coverage for AI dialect migrations. The
ledger is intentionally generic: each upstream source module is mapped to a
provider-free Leia example, live-package skeleton, or explicit excluded scope.

For FinRobot, the source root is the local `.external/FinRobot` checkout and the
target root is `examples/ai/finrobot_translation`. Hashes are stable audit
anchors. `source_hash` values identify the audited upstream source snapshot when
available; `fixture_hashes` are recomputed from checked-in Leia fixtures or
manifests and do not require network access.

Coverage statuses:

- `covered_example`: a registered provider-free example exercises the source
  concept.
- `covered_live_package_skeleton`: a live-package skeleton records schemas,
  fixtures, capability gates, and clean-skip behavior.
- `partial_fixture_contract`: source behavior is represented by deterministic
  fixtures or contract metadata, but production behavior is not implemented.
- `mapped_optional_gate`: source behavior requires an optional dependency or
  live provider and is behind a disabled capability gate.
- `excluded_non_ai_surface`: source files are outside the AI dialect migration
  scope but are still listed with a reason and next action.

The ledger should stay provider-free: no live network calls, credentials,
provider SDK imports, or runtime-package dependencies are needed to validate it.
