# Analytics Report Live Package Skeleton

This directory is a generic Leia package example for analytics reports. It is
provider-free by default: no finance provider SDK, model provider credential,
network call, browser renderer, or PDF renderer is required to validate the
contracts.

The skeleton covers these package surfaces:

- Finance normalizers for price and fundamentals rows with provenance.
- Valuation analytics for DCF, peer multiple, and target price synthesis.
- Sensitivity matrix output over discount-rate and growth assumptions.
- Chart and report artifact manifests.
- HTML and PDF renderer contracts that are planned, not invoked.

`package.manifest.json` declares the live package gates and capabilities.
`package.schema.json` describes the contract surfaces. `analytics_report.leia`
is the executable fixture that validates the contracts in interpreter and
bytecode tests.
