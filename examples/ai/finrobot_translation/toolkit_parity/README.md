# Toolkit and Schema Parity Ledger

This directory records provider-free parity for the generic AI tool dialect
capabilities exercised by FinRobot's `finrobot/toolkits.py`.

The ledger is not a FinRobot syntax extension and is not a Python compatibility
layer. It maps the source helper concepts onto portable tool registration,
caller/executor separation, input schema metadata, capability tags, result
envelopes, structured errors, and output adapters that any AI tool runtime can
implement.

Default parity is intentionally inert:

- no AutoGen import
- no Pandas import
- no provider SDK import
- no network access
- no local code execution
- no live finance data dependency

`ledger.json` is the canonical audit artifact. The schemas and fixtures in this
directory make the tool dialect contracts concrete without binding them to a
provider, model API, or FinRobot-only grammar.
