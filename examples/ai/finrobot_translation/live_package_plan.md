# FinRobot Live External Package Plan

This plan keeps the remaining live FinRobot work out of the replay slice and
targets external package implementations. The replay examples now include
provider-free skeleton contracts; live vendors, finance formulas, renderers, and
web services are not Leia built-ins.

## Package Skeleton Status

Current registered-example status: 48 runnable/checkable FinRobot examples are
discovered by `go run ./cmd/leia examples --json`. All 21 checked-in
live-package skeletons are registered provider-free examples: `analytics_report`,
`analyzer_report`, `backtest_strategy`, `chart_renderer`, `coding_notebook`,
`document_pipeline`, `earnings_transcript`, `equity_analysis_pipeline`, `factor_research`,
`finance_facade`, `finance_normalizers`, `html_ui_snapshots`, `news_catalyst`,
`optional_integrations`, `product_workflow`, `prompt_roles`, `report_renderer`,
`retail_sentiment`, `tutorial_demo_parity`, `valuation_engine`, and
`vendor_adapters`.

| Checked-in skeleton | Registered example | Target external package | Target directory | Contract |
| --- | --- | --- | --- | --- |
| `live_packages/vendor_adapters` | `live_packages/vendor_adapters/main.leia` | `leia-finrobot-vendor-adapters` | `packages/finrobot/vendor_adapters` | `contracts/vendor_adapter_contract.json` |
| `live_packages/factor_research` | `live_packages/factor_research/main.leia` | `leia-finrobot-factor-research` | `packages/finrobot/factor_research` | `contracts/factor_research_contract.json` |
| `live_packages/analytics_report` | `live_packages/analytics_report/analytics_report.leia` | `leia-finrobot-analytics-report` | `packages/finrobot/analytics_report` | `contracts/analytics_report_contract.json` |
| `live_packages/finance_normalizers` | `live_packages/finance_normalizers/main.leia` | `leia-finrobot-finance-normalizers` | `packages/finrobot/finance_normalizers` | `contracts/finance_normalizers_contract.json` |
| `live_packages/valuation_engine` | `live_packages/valuation_engine/main.leia` | `leia-finrobot-valuation-engine` | `packages/finrobot/valuation_engine` | `contracts/valuation_engine_contract.json` |
| `live_packages/report_renderer` | `live_packages/report_renderer/main.leia` | `leia-finrobot-report-renderer` | `packages/finrobot/report_renderer` | `contracts/report_renderer_contract.json` |
| `live_packages/product_workflow` | `live_packages/product_workflow/main.leia` | `leia-finrobot-web-product` | `packages/finrobot/web_product` | `contracts/web_product_contract.json` |
| `live_packages/optional_integrations` | `live_packages/optional_integrations/main.leia` | `leia-finrobot-optional-integrations` | `packages/finrobot/optional_integrations` | `contracts/optional_integration_capability_gates.json` |
| `live_packages/prompt_roles` | `live_packages/prompt_roles/main.leia` | `leia-finrobot-prompt-roles` | `packages/finrobot/prompt_roles` | `contracts/prompt_role_contract.json` |
| `live_packages/news_catalyst` | `live_packages/news_catalyst/main.leia` | `leia-finrobot-news-catalyst` | `packages/finrobot/news_catalyst` | `contracts/news_catalyst_contract.json` |
| `live_packages/finance_facade` | `live_packages/finance_facade/main.leia` | `leia-finrobot-finance-facade` | `packages/finrobot/finance_facade` | `contracts/finance_facade_contract.json` |
| `live_packages/document_pipeline` | `live_packages/document_pipeline/main.leia` | `leia-finrobot-document-pipeline` | `packages/finrobot/document_pipeline` | `contracts/document_pipeline_contract.json` |
| `live_packages/analyzer_report` | `live_packages/analyzer_report/main.leia` | `leia-finrobot-analyzer-report` | `packages/finrobot/analyzer_report` | `contracts/analyzer_report_contract.json` |
| `live_packages/coding_notebook` | `live_packages/coding_notebook/main.leia` | `leia-finrobot-coding-notebook` | `packages/finrobot/coding_notebook` | `contracts/coding_notebook_contract.json` |
| `live_packages/tutorial_demo_parity` | `live_packages/tutorial_demo_parity/main.leia` | `leia-finrobot-tutorial-demo-parity` | `packages/finrobot/tutorial_demo_parity` | `contracts/tutorial_demo_parity_contract.json` |
| `live_packages/chart_renderer` | `live_packages/chart_renderer/main.leia` | `leia-finrobot-chart-renderer` | `packages/finrobot/chart_renderer` | `contracts/chart_renderer_contract.json` |
| `live_packages/backtest_strategy` | `live_packages/backtest_strategy/main.leia` | `leia-finrobot-backtest-strategy` | `packages/finrobot/backtest_strategy` | `contracts/backtest_strategy_contract.json` |
| `live_packages/equity_analysis_pipeline` | `live_packages/equity_analysis_pipeline/main.leia` | `leia-finrobot-equity-analysis-pipeline` | `packages/finrobot/equity_analysis_pipeline` | `contracts/stage_dag_contract.json` |
| `live_packages/retail_sentiment` | `live_packages/retail_sentiment/main.leia` | `leia-finrobot-retail-sentiment` | `packages/finrobot/retail_sentiment` | `contracts/retail_sentiment_contract.json` |
| `live_packages/html_ui_snapshots` | `live_packages/html_ui_snapshots/main.leia` | `leia-finrobot-html-ui-snapshots` | `packages/finrobot/html_ui_snapshots` | `contracts/html_ui_snapshot_contract.json` |
| `live_packages/earnings_transcript` | `live_packages/earnings_transcript/main.leia` | `leia-finrobot-earnings-transcript` | `packages/finrobot/earnings_transcript` | `contracts/earnings_transcript_contract.json` |

## Contract Rules

- Every target package manifest must name its package, target directory,
  contract file, capabilities, optional credentials, replay fixture keys, and
  test gates.
- Live network and real dependency imports default to disabled. Tests for this
  plan only validate manifests and replay contracts; they must not call real providers.
- Capabilities are the boundary between the replay slice and live packages.
  Provider adapters, normalizers, valuation logic, report rendering, and web
  product behavior must be granted explicitly before replacing a replay fixture.
- Optional provider credentials must be absent-safe and redacted. Missing live
  credentials produce clean skips, not failing imports or hidden fallback calls.
- Leia core guarantees generic composition, capability contracts, trace/replay
  metadata, approval policy, and provider-free gates only. It does not guarantee
  built-in finance vendors, valuation formulas, report renderers, or product UI.

## Test Gates

The machine-readable gate list lives in
`live_package_plan_manifest.json`. It is intentionally separate from live
package implementations so this repository can keep testing the migration plan
without shipping live clients or performing real network I/O.
