# Editors And LSP

Leia ships editor assets as part of the language product. The preferred model is
one shared language server plus editor-specific packaging for syntax
highlighting and common commands.

## Language Server

Start the language server from a checkout:

```bash
go run ./cmd/leia-lsp
```

After installing binaries, editor packages should execute:

```bash
leia-lsp
```

Current LSP capabilities:

| Capability | Status |
|---|---|
| Diagnostics | Parses open documents and publishes syntax diagnostics. |
| Formatting | Reuses the same formatter path as `leia fmt`. |
| Completion | Provides keywords, standard-library modules, and declarations. |
| Hover | Shows keyword, standard-library, and local declaration summaries. |
| Document links | Links local module import and require paths. |
| Document symbols | Exposes function declarations. |
| Definition and references | Supports single-document symbol navigation. |
| Rename | Supports single-document symbol rename. |
| Code lens | Adds evaluate-case actions for source-level regression checks. |
| Inlay hints | Shows local call parameter names and stdlib import hints. |
| Semantic tokens | Highlights keywords, declarations, stdlib namespaces, imports, dialects, and read-only const bindings. |

The current server intentionally keeps state in memory and operates on open
documents. It treats AI dialects and other tagged forms as normal Leia syntax
today; richer semantic diagnostics can be added later without changing editor
integration points.

## Syntax Highlighting

Two syntax asset families are maintained:

- TextMate grammars under `tools/syntax/textmate/`.
- Tree-sitter grammar under `tools/tree-sitter-leia/`.

Run the editor guard locally:

```bash
bash scripts/editor_check.sh
```

If the tree-sitter CLI is available and you want the full corpus check:

```bash
npm --prefix tools/tree-sitter-leia ci
bash scripts/editor_check.sh --require-tree-sitter
```

## VS Code

The VS Code extension lives under `editors/vscode/`. During development, install
or package it from that directory:

```bash
cd editors/vscode
npm install
npm run package
```

The extension should use `leia-lsp` for language features and the bundled
TextMate grammars for `.leia` and `leia.mod` highlighting.

Use `Leia: Restart Language Server` from the command palette after changing the
`leia.languageServer.executable` setting or replacing the `leia-lsp` binary.

## Emacs

The dependency-free major mode lives at `editors/emacs/leia-mode.el`.

For a local checkout:

```elisp
(add-to-list 'load-path "/path/to/leia/editors/emacs")
(require 'leia-mode)
(add-to-list 'auto-mode-alist '("\\.leia\\'" . leia-mode))
```

Use `M-x lsp` or another LSP client to connect to `leia-lsp`.

## Neovim, Helix, And Zed

Editor integration files are checked in:

- `editors/neovim/`
- `editors/helix/`
- `editors/zed/`

These should point to the local tree-sitter grammar until a public
`tree-sitter-leia` package is published. See
[`tools/tree-sitter-leia/README.md`](../../tools/tree-sitter-leia/README.md).

## Release Expectations

Before a public release:

- `bash scripts/editor_check.sh` should pass.
- release archives and `scripts/install.sh` should install both `leia` and
  `leia-lsp`.
- VS Code metadata should be marketplace-ready even if publication is delayed.
- Emacs mode should load without package-manager-specific assumptions.
- Tree-sitter metadata should use grammar name `leia`, file type `leia`, and
  scope `source.leia`.
- Any published editor package should call the shared `leia-lsp` server rather
  than reimplementing diagnostics or formatting.
