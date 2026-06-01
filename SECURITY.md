# Security Policy

Leia is an embeddable scripting runtime. Security depends on both the runtime
defaults and the host application's selected libraries, capabilities, resource
budgets, and Go bindings.

## Supported Versions

Leia is under active development. Until the first stable release, only the
current `main` branch and the latest published tag, when one exists, receive
security fixes.

## Reporting A Vulnerability

Do not file public issues for vulnerabilities. Use GitHub's private security
advisory flow for this repository when available. If that is unavailable, open a
minimal public issue that says a private security report is needed, without
including exploit details.

Include:

- affected commit or tag;
- platform and Go version;
- whether scripts run through interpreter, bytecode VM, JIT, or embedding API;
- enabled libraries and capabilities;
- minimal reproduction steps;
- expected and observed impact.

## Runtime Security Model

For untrusted scripts:

- start with `leia.SecuritySandbox()`;
- enable only required libraries and capabilities;
- set CPU-step, native-call, recursion, and wall-time budgets;
- avoid exposing arbitrary Go reflection or process/network APIs;
- keep AI provider API keys in host configuration or environment variables, not
  source files.

The security reference is [docs/reference/security/index.md](docs/reference/security/index.md).
