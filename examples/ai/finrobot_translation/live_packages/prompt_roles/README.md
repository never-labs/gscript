# FinRobot Prompt Roles Live Package Skeleton

This skeleton records provider-free contracts for FinRobot prompt and role
migration work. It covers prompt catalogs from `agent_library`, `prompts`,
`utils`, `text_generator_agents`, and `equity_agents`; role profile versioning;
section-agent output shape; the `TERMINATE` completion convention; and source
evidence validation.

The package is intentionally fixture-only:

- `provider_free` is `true`.
- `live_network` defaults to `false`.
- `live_model_calls` is `false`.
- No credentials or secret environment variables are declared.
- `main.leia` is a static smoke fixture and does not import provider SDKs.
