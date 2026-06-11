# FinRobot Tutorial and Demo Parity Live Package Skeleton

This provider-free skeleton records the live package contract for FinRobot
tutorial and demo parity. It does not translate notebooks directly, import
AutoGen or Backtrader, call live finance providers, or enable network access.

The package links the existing tutorial and demo parity ledgers to checked-in
Leia examples, replay records, optional integration gates, demo smoke contracts,
and a notebook-to-Leia conversion checklist.

Default behavior remains fixture replay:

- `live_network_default=false`
- `real_dependency_import_default=false`
- optional gates default disabled
- missing credentials or optional dependencies must cleanly skip
