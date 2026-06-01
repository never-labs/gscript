# AI-Native Syntax

AI-native syntax is a language layer over the `llm`, `msg`, `history`, and
`loop` standard-library modules. It must use the same provider, tracing,
record/replay, cancellation, and capability paths as direct library calls.

## Models

`models {}` declares model aliases and provider configuration. `default` may
refer to another alias. Alias cycles are invalid. Source code must not embed
API keys as string literals; credentials belong in environment variables or
host-provided configuration.

## Tools

`tool name(params) { body }` declares a callable tool value. Leading
`//leia:` comments define tool metadata such as capability requirements and
parameter descriptions. Every stable tool declaration must have an explicit
`//leia:requires` directive, using `none` when no capability is required.

## Agents

An `agent` is a callable workflow value. Agent configuration supplies defaults
for turns executed by the agent. Explicit fields on a `turn` override agent
configuration; agent configuration overrides host defaults.

Named agents bind their name in scope. Anonymous `agent { ... }` or
`agent(params) { ... }` expressions produce values that may be assigned or
called.

`flow { ... }` supplies a custom agent body. The flow body is lexical code.
Only `model`, `system`, `tools`, and `capabilities` are injected as flow-local
bindings from merged agent configuration. User declarations in the flow body
may shadow those names. `user`, `budget`, `response_format`, and `metadata` are
ambient configuration, not injected variables.

## Turns And Messages

`turn { ... }` performs one provider request and returns `(result, err)`.

`messages { ... }` constructs an ordered message list. Static histories may use
role fields such as `system`, `user`, and `assistant`; computed histories may
use message helper modules directly.

## Budgets

Public budget dimensions are `turns`, `calls`, `tokens`, and `time`. Provider
usage may include cost metadata, but money accounting is not a stable
script-level budget dimension.

## Errors

Recoverable provider, budget, validation, and tool failures return structured
`nil, err` results unless an API explicitly documents a runtime error. Trace
events must avoid prompt and tool-result leakage unless explicitly configured
by the host.
