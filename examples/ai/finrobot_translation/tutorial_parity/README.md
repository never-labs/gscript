# FinRobot Tutorial Parity Ledger

This directory records provider-free parity status for FinRobot beginner and
advanced tutorials. It does not add executable tutorial translations; it maps
each upstream notebook to the existing Leia examples and live package skeletons
that already cover the same contract surface.

`ledger.json` is the source of truth. The ledger intentionally keeps
`live_network_default` and `real_dependency_import_default` false. Optional
live behavior is represented as capability gates that must be enabled outside
default tests and must cleanly skip when credentials or local services are
absent.

Current scope:

- 7 beginner notebooks from `tutorials_beginner`.
- 6 advanced notebooks from `tutorials_advanced`.
- Existing Leia mappings under `examples/ai/finrobot_translation`.
- Existing live package skeleton mappings for vendor adapters, analytics/report
  contracts, product workflow, and optional integrations.

The corresponding tests live in `tests/llm/finrobot_tutorial_parity_test.go`.
They validate ledger completeness, checked-in mapping targets, explicit gaps,
and the no-live-network default.
