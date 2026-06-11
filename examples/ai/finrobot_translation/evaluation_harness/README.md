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

The harness also carries a generic AI evaluation dialect specimen:

- `generic_ai_evaluation.source.txt` is materialized as a temporary `.leia`
  file by the parity test and exercises `evaluate`, `eval.load_jsonl`,
  `eval.case`, `eval.metric`, `eval.judge`, `eval.usage`, and `eval.budget`
  without live providers.
- `generic_ai_evaluation_dataset.jsonl` is the dataset manifest fixture.
- `generic_ai_evaluation.records.json` is the provider-free judge replay stub.

`manifest.json` is the contract of record for judge specs, metric registry,
dataset shape, strict record/replay matching, scoring trace fields, failure
envelope, and golden threshold gates. Tests consume those manifest sections so
the harness remains a reusable AI evaluation capability instead of a
FinRobot-specific test fixture.
