# FinRobot optional integrations live package skeleton

This package is a provider-free skeleton for the FinGPT, FinRL, FinML, Backtrader, mplfinance, OpenBB, and Ollama optional integrations.

It is intentionally fixture-only:

- `provider_free` is true.
- `live_network_default` is false.
- `real_dependency_import_default` is false.
- credentials are empty.
- every optional dependency has a clean skip gate.
- fixture metadata is recorded in `fixtures/provider_free_fixture_index.json`.

Future provider-specific packages must satisfy these capability gates before importing SDKs, opening local endpoints, or using live credentials.
