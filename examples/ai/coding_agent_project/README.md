# Coding Agent Project Example

This directory is an offline, replay-backed project-level AI example. It models
a small coding-agent repair loop over a fixture workspace:

- inspect the target file with `read_file`
- find related tests with `search_text`
- apply an initial patch with `apply_patch`
- run the local test command with `run_shell`
- retry the patch after a deterministic failing test
- run the test command again and finish from replayed evidence

Run it through the examples gate:

```bash
go run ./cmd/leia examples check ai/coding_agent_project
```

