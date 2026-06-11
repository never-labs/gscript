# FinRobot Equity Report Translation

This example translates the FinRobot equity report pipeline into an offline
Leia workflow. It uses deterministic local fixtures for financial metrics,
market data, peer comparison, news themes, catalysts, and risks, then replays
section-agent outputs from `../equity_report.records.json`.

Run it with:

```sh
go run ./cmd/leia evaluate --replay examples/ai/finrobot_translation/equity_report.records.json examples/ai/finrobot_translation/equity_report.leia
```

The workflow intentionally does not call FMP, OpenAI, Adanos, chart rendering,
HTML rendering, or PDF generation. Those runtime integrations remain in the
source FinRobot implementation and are tracked in `../GAPS.md`.
