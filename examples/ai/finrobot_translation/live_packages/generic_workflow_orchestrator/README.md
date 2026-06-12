# Generic workflow orchestrator live package skeleton

Provider-free contract skeleton for `generic.ai.workflow.orchestration` and
`ai.workflow.orchestrate`.

This package describes workflow graph execution, planning graph to stage
projection, stage input/output contracts, handoff trace records, deterministic
retry/cache metadata, workflow result shape, and trace emission hooks. It
intentionally contains no live provider SDKs, network calls, credentials, or
runtime imports.
