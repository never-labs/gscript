# Platforms And Execution Modes

Leia has one semantic baseline and several execution modes.

## Semantic Baseline

The interpreter defines script behavior. The bytecode VM and JIT are
accelerators: they must preserve interpreter-visible results, errors,
capability checks, resource-budget behavior, and deoptimization behavior.

## Execution Modes

| Mode | Purpose | Availability |
|---|---|---|
| Interpreter | Baseline semantics and fallback execution. | All supported Go platforms. |
| Bytecode VM | Faster portable execution and most CLI/embedding runs. | All supported Go platforms. |
| Method JIT | Native ARM64 hot-path acceleration. | ARM64 builds where JIT support is enabled. |
| Hosted callbacks | Go functions, host modules, LLM providers, file/network/process APIs. | Depends on embedder policy and capabilities. |

Use `leia capabilities --json` to inspect a built binary:

```bash
go run ./cmd/leia capabilities --json
```

The JSON report includes execution modes, commands, standard-library layers,
default imports, tooling surfaces, LLM support, and builtin dialect metadata.
The `dialects` array is derived from the runtime dialect registry and the
`default_imports` array is derived from the same alias table used by stdlib
installation. These are the supported machine-readable paths for editors,
playgrounds, and automation tools to discover installed tagged forms and global
convenience bindings:

```json
{
  "default_imports": [
    {"name": "sqrt", "module": "math", "member": "sqrt"},
    {"name": "mat", "module": "linalg", "member": "matrix"}
  ],
  "dialects": [
    {
      "name": "sh",
      "category": "host",
      "capabilities": ["process.shell"],
      "builtin": true,
      "eval": true,
      "block": false
    }
  ]
}
```

## JIT Expectations

The JIT is an implementation detail, not a language feature or whole-language
native-code contract. Programs must not depend on a function being compiled
natively; unsupported operations and non-hot paths must continue through the
VM/runtime fallback. Hosts that need strict resource budget checkpoints,
untrusted-script sandboxing, or deterministic fallback may disable JIT.

JIT-sensitive changes need correctness tests and performance evidence. See
[`../../contributing/performance.md`](../../contributing/performance.md).

## Support Levels

| Level | Meaning |
|---|---|
| Tested | The current release notes include evidence for this OS, architecture, Go version, and execution mode. |
| Supported | Release blockers cover the combination and maintainers intend to fix regressions. |
| Available | The project can build or run there, but the release does not include full evidence. |
| Unknown | No current release evidence exists. |

ARM64 JIT support is only claimed when `leia capabilities --json` and the
release notes both show it enabled for that build. Otherwise the interpreter and
VM remain the semantic baseline.

## Platform Policy

Release notes should state:

- OS and architecture tested;
- Go version used;
- whether bytecode VM and JIT were enabled;
- whether LuaJIT reference benchmarks were available;
- any disabled host capabilities or live-provider integrations.

Do not claim broad platform support from one local run. Treat untested
combinations as unknown until they are covered by release evidence.
