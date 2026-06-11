# FinRobot Evaluation Harness

FR-GAP-023 tracks provider-free regression coverage for the translated FinRobot
examples. The harness is intentionally data-driven:

- `manifest.json` inventories every offline `.records.json` fixture under
  `examples/ai/finrobot_translation`, pins fixture checksums, and records the
  golden evaluate report summary expected in CI.
- `records/` contains harness-owned golden fixtures when the current runtime
  request envelope differs from an older source fixture. The source fixture is
  still listed in the inventory so drift remains visible.
- CI can run each manifest entry with `leia evaluate --gate --report REPORT
  --replay RECORD SOURCE` and no live LLM provider environment.

The harness fixture version is `finrobot-eval-fixtures-v1`. Bump it whenever a
golden replay file or expected report summary changes.
