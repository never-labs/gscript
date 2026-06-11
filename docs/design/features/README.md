# Leia Feature Design Drafts

This directory contains review drafts for language and standard-library features
that are not yet part of the stable Leia contract.

The drafts focus on user-facing requirements:

- what the feature is for;
- how it should feel in Leia source;
- how it composes with other features;
- what must stay out of scope.

They intentionally avoid implementation plans. Parser, VM, JIT, formatter, LSP,
and standard-library implementation details should be designed after these
contracts are reviewed.

## Draft Index

| Draft | Scope |
|---|---|
| [Dialect System](dialect-system.md) | Unified `tag` literal/block DSL mechanism. |
| [Shell and Host Automation](shell-and-host-automation.md) | Shell ergonomics, commands, glob, paths, env/cwd scopes. |
| [Go-Style Imports](go-style-imports.md) | Import declarations and module naming, replacing Lua-like `require` as the primary style. |
| [AI Dialects](ai-dialects.md) | LLM/agent/model/tool/evaluate as official dialects over the existing AI runtime. |
| [FinRobot Translation Gap Audit](finrobot-translation-gap-audit.md) | Finance-agent workload audit mapped to general Leia dialects, stdlib, and package boundaries. |
| [Composition Features](composition-features.md) | Pipeline, `try`, destructuring, optional chaining, comprehensions, match, using. |
| [AI Coding Features](ai-coding-features.md) | Plan, patch, code query, record/replay, approval, transactions. |
| [Data and Array Programming](data-and-array-programming.md) | True data processing, SoA, matrix/vector, table transforms, array DSL. |
| [High-Level Domain Dialects](high-level-domain-dialects.md) | Web/API/mail/db/workflow/test as task-level DSLs that compose with lower-level dialects. |
| [Runtime Safety and Capabilities](runtime-safety-and-capabilities.md) | Capability requirements, sandbox semantics, and side-effect boundaries. |
| [Application Runtime Features](application-runtime-features.md) | Actor, event sourcing/timeline, live state, and self-describing programs. |
