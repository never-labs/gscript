# FinRobot Package Manifest Audit

This directory defines the provider-free package boundary audit for the
FinRobot live package skeletons under `../live_packages`.

The audit is intentionally read-only. It checks that skeleton manifests keep
network access disabled, entrypoints resolve to checked-in files, referenced
fixtures/schemas/contracts exist, capability prefixes remain consistent with
`../live_package_plan_manifest.json`, every fixture index capability declared by
any package manifest stays declared by both that manifest and the plan, and package
boundaries continue to carry the no-built-in guarantee required by the AI
dialect migration plan.

`ledger.json` is the canonical audit artifact. `schema.json` describes the
ledger shape used by `tests/llm/finrobot_package_manifest_audit_test.go`.
