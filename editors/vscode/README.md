# Leia VS Code Extension

This directory contains the Leia editor integration for VS Code.

## Features

- Syntax highlighting for `.leia`, `leia.mod`, and `leia.sum` files.
- Bracket, comment, and string pairing rules.
- Snippets for functions, tests, AI-native agents, tools, turns, and channel-based concurrency.
- Commands that call the repository CLI:
  - `Leia: Run Current File`
  - `Leia: Test Workspace`
  - `Leia: Format Current File`
  - `Leia: Lint Workspace`
  - `Leia: Check Workspace`
  - `Leia: Preview Language Spec`
- Built-in VS Code tasks for the same operations.

## Development

Open this directory in VS Code and run the extension host, or package it with
`vsce` after installing the VS Code extension tooling.

The extension assumes a `leia` executable is available on `PATH`. Override it
with the `leia.executable` setting when using a local build:

```json
{
  "leia.executable": "/absolute/path/to/leia"
}
```

The spec preview command runs `scripts/spec_preview.py` in the current
workspace and opens `docs/spec/index.html`.

## Tasks

Run `Tasks: Run Task` and choose one of the built-in `Leia` tasks, or define a
workspace task with the `leia` task type:

```json
{
  "label": "Leia: test language cases",
  "type": "leia",
  "command": "test",
  "path": "tests/language"
}
```
