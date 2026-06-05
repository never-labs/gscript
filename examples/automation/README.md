# Automation Examples

This directory contains project-level examples for small offline automation
workflows. They should run from a clean checkout without network access,
windowing, real LLM providers, process execution, secrets, or host files.

Run the release risk digest example with:

```bash
go run ./cmd/leia examples check --jobs=6 examples/automation/release_risk_digest.leia
```
