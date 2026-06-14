# RFC Process

An RFC is required when a change modifies Leia's stable user-facing contract.
The process is intentionally small: one Markdown document, focused examples,
and explicit implementation impact.

## When To Write An RFC

Write an RFC for:

- New syntax, contextual keywords, or grammar changes.
- Semantic changes to declarations, expressions, statements, modules, errors,
  concurrency, or AI dialect constructs.
- Public Go API changes.
- Standard-library capability or safety changes.
- Package manager resolution behavior.
- Runtime behavior that affects embedding, hot reload, security, or resource
  budgets.

Do not write an RFC for ordinary bug fixes, small docs edits, internal-only
refactors, or benchmark additions that do not change the public contract.

## File Layout

Use `rfcs/NNNN-short-name.md`. Start from
[`rfcs/0000-template.md`](https://github.com/never-labs/leia/blob/main/rfcs/0000-template.md).
Reserve `0000` for the template.

## Review Checklist

Every RFC should answer:

- What user problem does this solve?
- What is the minimal syntax or API?
- What exact spec sections change?
- What happens in interpreter, bytecode VM, JIT, formatter, linter, LSP, and
  docs?
- What are the security, sandboxing, and capability implications?
- What tests, examples, and benchmarks will prove the behavior?
- What migration or deprecation path is needed?

## Acceptance

An accepted RFC is a design decision, not an implementation. Implementation can
land in one or more follow-up pull requests. The first implementation PR should
link to the RFC and update the spec, tests, and examples needed to make the
contract real.

Rejected or withdrawn RFCs should remain in the repository when they capture a
useful design tradeoff. Mark their status clearly.
