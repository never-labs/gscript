# Generic AI Model Registry

Provider-free live package skeleton for `generic.ai.model.registry`.

This package turns provider-neutral model aliases into deterministic execution
descriptors for fixture replay. It does not import vendor SDKs, require
credentials, or perform live model calls.

## Live Gate Contract

The package default is provider-free fixture replay. A real provider may only be
used by an explicit integration gate, not by ordinary examples or package
checks.

For a downstream GLM-compatible smoke, configure provider environment variables
in the shell:

```bash
export LEIA_GLM_BASE_URL=https://open.bigmodel.cn/api/anthropic
export LEIA_GLM_API_KEY
export LEIA_GLM_MODEL=glm-5.1
```

Then run:

```bash
go test ./tests/integration/llm -run '<downstream-live-gate-test>' -count=1 -v
```

`LEIA_GLM_API_KEY` is an environment reference only; no secret value belongs in
this directory. Downstream products may define domain-specific override
variables around the same provider-free package contract. The integration gate
runs when a configured key is available. Without a key, the test clean-skips and
provider-free defaults remain unchanged.

The contract covers:

- model alias registry entries
- default provider policy
- replay-safe execution descriptors
- redaction policy
- routing guard evidence for redirecting live provider candidates to replay
  descriptors
- capability flags
