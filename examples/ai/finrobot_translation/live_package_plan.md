# FinRobot Live External Package Plan

This plan keeps the remaining live FinRobot work out of the replay slice and
targets external package implementations. The replay examples now include
provider-free skeleton contracts; live vendors, finance formulas, renderers, and
web services are not Leia built-ins.

## Package Skeleton Status

Current registered-example status: 31 runnable/checkable FinRobot examples are
discovered by `go run ./cmd/leia examples --json`. Six checked-in
live-package skeletons are registered examples (`analytics_report`,
`factor_research`, `news_catalyst`, `optional_integrations`, `prompt_roles`,
and `vendor_adapters`); the product workflow skeleton is
manifest/contracts/schemas/fixtures only and is covered by tests through the
existing workflow examples.

| Checked-in skeleton | Registered example | Target external package | Target directory | Contract |
| --- | --- | --- | --- | --- |
| `live_packages/vendor_adapters` | `live_packages/vendor_adapters/main.leia` | `leia-finrobot-vendor-adapters` | `packages/finrobot/vendor_adapters` | `contracts/vendor_adapter_contract.json` |
| `live_packages/factor_research` | `live_packages/factor_research/main.leia` | `leia-finrobot-factor-research` | `packages/finrobot/factor_research` | `contracts/factor_research_contract.json` |
| `live_packages/analytics_report` | `live_packages/analytics_report/analytics_report.leia` | `leia-finrobot-normalizers` | `packages/finrobot/normalizers` | `contracts/finance_normalizer_contract.json` |
| `live_packages/analytics_report` | `live_packages/analytics_report/analytics_report.leia` | `leia-finrobot-valuation` | `packages/finrobot/valuation` | `contracts/valuation_contract.json` |
| `live_packages/analytics_report` | `live_packages/analytics_report/analytics_report.leia` | `leia-finrobot-report-renderer` | `packages/finrobot/report_renderer` | `contracts/report_renderer_contract.json` |
| `live_packages/product_workflow` | Covered through `equity_cli_workflow.leia` and `web_product.leia`; no standalone registered `.leia` file | `leia-finrobot-web-product` | `packages/finrobot/web_product` | `contracts/web_product_contract.json` |
| `live_packages/optional_integrations` | `live_packages/optional_integrations/main.leia` | `leia-finrobot-optional-integrations` | `packages/finrobot/optional_integrations` | `contracts/optional_integration_capability_gates.json` |
| `live_packages/prompt_roles` | `live_packages/prompt_roles/main.leia` | `leia-finrobot-prompt-roles` | `packages/finrobot/prompt_roles` | `contracts/prompt_role_contract.json` |
| `live_packages/news_catalyst` | `live_packages/news_catalyst/main.leia` | `leia-finrobot-news-catalyst` | `packages/finrobot/news_catalyst` | `contracts/news_catalyst_contract.json` |

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
