# FinRobot Demo and Regression Parity Ledger

This directory records provider-free parity status for FinRobot's root demo
scripts and the current `finrobot_equity/core/tests` regression surface. It is
an audit ledger, not a live Python compatibility layer.

`ledger.json` maps each source demo or regression test file to checked-in Leia
examples, evaluate-backed examples, replay records from
`evaluation_harness/manifest.json`, known gaps, and optional live gates. The
default remains provider-free: no AutoGen import, no Backtrader import, no
chart/PDF rendering, no live finance provider calls, and no network access.

Current scope:

- `agent_builder_demo.py`
- `test_module.py`
- `finrobot_equity/core/tests/test_generate_report.py`
- `finrobot_equity/core/tests/test_modules.py`
- `finrobot_equity/core/tests/test_retail_sentiment_client.py`

The corresponding tests live in
`tests/llm/finrobot_demo_parity_test.go`. They validate source inventory,
checked-in mappings, replay-record links, explicit gaps, and disabled live
gates.
