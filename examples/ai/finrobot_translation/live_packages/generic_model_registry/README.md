# Generic AI Model Registry

Provider-free live package skeleton for `generic.ai.model.registry`.

This package turns provider-neutral model aliases into deterministic execution
descriptors for fixture replay. It does not import vendor SDKs, require
credentials, or perform live model calls.

The contract covers:

- model alias registry entries
- default provider policy
- replay-safe execution descriptors
- redaction policy
- routing guard evidence for redirecting live provider candidates to replay
  descriptors
- capability flags
