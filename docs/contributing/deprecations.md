# Deprecation Policy

Deprecation is how Leia removes or changes public behavior without surprising
users.

## What Can Be Deprecated

Deprecation applies to public contracts:

- Syntax and language semantics.
- Standard-library modules, functions, and capability names.
- Public Go APIs.
- CLI commands and flags.
- Module resolution behavior.
- AI dialect provider, agent, turn, replay, and evaluate report shapes.

Internal package names, tests, and private implementation details can change
without deprecation when they are not part of the public contract.

## Deprecation Steps

For nontrivial public changes:

1. Document the replacement.
2. Add a warning in CLI, lint, docs, or runtime diagnostics when practical.
3. Keep compatibility for at least one minor release after the warning.
4. Remove the old behavior only after tests, examples, and docs use the new
   path.

Before v1.0, maintainers may shorten this process for cleanup work, but the
replacement and rationale should still be documented.

## Compatibility Shims

Compatibility shims should be narrow and named by purpose, not by history. Avoid
`legacy`-named packages or files. If a shim remains useful, give it a current
domain name and document when it can be removed.

## CLI Flags

When a CLI flag is renamed, prefer accepting both names for a transition period
if doing so does not create ambiguous behavior. If two aliases are provided with
different values, the command should fail before doing work.

## Report Schemas

Machine-readable schemas, such as `leia test --json` and `leia evaluate --json`,
should carry explicit schema fields when they become externally consumed. Adding
fields is compatible. Renaming or removing fields requires a deprecation note or
a schema version bump.
