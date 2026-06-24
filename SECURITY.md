# Security Policy

Leia is an embeddable scripting runtime. Security depends on both the runtime
defaults and the host application's selected libraries, capabilities, resource
budgets, and Go bindings.

## Supported Versions

Leia is under active development before its first stable release.

| Version | Security support |
|---|---|
| `main` | Supported. |
| Latest published tag | Supported when a public tag exists. |
| Older tags | Unsupported unless the release notes say otherwise. |

## Reporting A Vulnerability

Do not file public issues for vulnerabilities or exploit details. Use GitHub's
private security advisory flow for this repository. If you cannot access that
flow, email security@never-labs.com with a minimal subject and no exploit
payload in the subject line.

Include:

- affected commit or tag;
- platform and Go version;
- whether scripts run through interpreter, bytecode VM, JIT, or embedding API;
- enabled libraries and capabilities;
- minimal reproduction steps;
- expected and observed impact.

The target initial response time is five business days. Coordinated disclosure
timelines are handled case by case according to exploitability, affected
versions, and available mitigations.

## Runtime Security Model

`leia.New()` is a trusted-local runtime constructor. It is not a sandbox by
itself. The security boundary is the combination of enabled libraries,
capabilities, resource budgets, module loading policy, and Go bindings exposed
by the host process.

For untrusted scripts:

- start with `leia.SecuritySandbox()`;
- enable only required libraries and capabilities;
- set CPU-step, native-call, recursion, and wall-time budgets;
- avoid exposing arbitrary Go reflection or process/network APIs;
- keep AI provider API keys in host configuration or environment variables, not
  source files.

The security reference is [docs/reference/security/index.md](docs/reference/security/index.md).
