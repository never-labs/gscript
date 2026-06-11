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
- HTML and PDF renderer contracts that are planned, not invoked, and do not
  require renderer dependencies.
- HTML, PDF, and chart snapshot metadata for externally captured artifacts.
- Accessibility checklist, stale-data warning policy, and source annotation
  requirements that snapshots and report sections must carry forward.

`package.manifest.json` declares the live package gates and capabilities.
`package.schema.json` describes the contract surfaces. `analytics_report.leia`
is the executable fixture that validates the contracts in interpreter and
bytecode tests.

Renderer and snapshot contract notes:

- The package specifies artifact and snapshot metadata only. It does not import
  browser, charting, PDF, finance, or model-provider dependencies.
- HTML/PDF/chart snapshots must include artifact id, media type, status,
  renderer contract id, checksum placeholder, source refs, warning refs, and
  accessibility refs before an external package may render or capture them.
- Stale source annotations must produce a `stale_data_warning` entry naming the
  stale source id, and snapshots derived from stale sources must carry that
  warning ref.
- Source annotations require identity, title, kind, locator, freshness fields,
  stale flag, license, retrieval time, and evidence hash. Report and snapshot
  source refs must resolve to one of those annotations.
