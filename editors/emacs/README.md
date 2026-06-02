# Leia Emacs Mode

This directory contains a dependency-free Emacs major mode for Leia.

## Install

Add this directory to `load-path` and require the mode:

```elisp
(add-to-list 'load-path "/path/to/leia/editors/emacs")
(require 'leia-mode)
```

The mode registers:

- `*.leia` as `leia-mode`
- `leia.mod` as `leia-mod-mode`

## Features

- Syntax highlighting for Leia keywords, builtins, directives, strings, numbers,
  comments, and common standard library modules.
- Syntax highlighting for `leia.mod` module files.
- `//` line comments and `/* ... */` block comments in Leia source.
- Basic delimiter-oriented indentation.
- CLI-backed commands:
  - `C-c C-r` / `M-x leia-run-current-file`
  - `C-c C-f` / `M-x leia-format-current-file`
  - `C-c C-c` / `M-x leia-check-format-current-file`
  - `C-c C-t` / `M-x leia-test`
- Optional Eglot integration through `leia-lsp`.

Set `leia-command` if the CLI is not available as `leia`:

```elisp
(setq leia-command "/path/to/leia")
```

Enable LSP support when Eglot is available:

```elisp
(setq leia-lsp-command "/path/to/leia-lsp")
(leia-eglot-setup)
(add-hook 'leia-mode-hook #'eglot-ensure)
```

## Tree-sitter

This package does not currently bundle an Emacs `treesit` major mode. Emacs 29+
users who want to experiment with tree-sitter should install the local grammar
from `tools/tree-sitter-leia` and define or consume a downstream `leia-ts-mode`.

```elisp
(add-to-list 'treesit-language-source-alist
             '(leia "/path/to/leia/tools/tree-sitter-leia"))
(treesit-install-language-grammar 'leia)
```

Use `leia` as the grammar symbol and `source.leia` as the language scope.
