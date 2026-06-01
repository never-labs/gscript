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

Set `leia-command` if the CLI is not available as `leia`:

```elisp
(setq leia-command "/path/to/leia")
```
