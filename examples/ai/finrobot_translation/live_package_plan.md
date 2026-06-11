# FinRobot Live External Package Plan

This plan keeps the remaining live FinRobot work out of the replay slice and
targets external package implementations. The replay examples now include
provider-free skeleton contracts; live vendors, finance formulas, renderers, and
web services are not Leia built-ins.

## Package Skeleton Status

| Skeleton source | Target external package | Target directory | Contract |
| --- | --- | --- | --- |
| `vendor_adapters.leia` | `leia-finrobot-vendor-adapters` | `packages/finrobot/vendor_adapters` | `contracts/vendor_adapter_contract.json` |
| `finance_normalizers.leia` | `leia-finrobot-normalizers` | `packages/finrobot/normalizers` | `contracts/finance_normalizer_contract.json` |
| `valuation_analytics.leia` | `leia-finrobot-valuation` | `packages/finrobot/valuation` | `contracts/valuation_contract.json` |
| `report_contract.leia` | `leia-finrobot-report-renderer` | `packages/finrobot/report_renderer` | `contracts/report_renderer_contract.json` |
| `web_product.leia` | `leia-finrobot-web-product` | `packages/finrobot/web_product` | `contracts/web_product_contract.json` |

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
