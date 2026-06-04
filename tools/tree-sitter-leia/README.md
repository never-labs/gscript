# tree-sitter-leia

Minimal tree-sitter grammar for Leia source files.

This first version is intentionally scoped to editor/tooling use and tracks the
stable user-facing grammar in `docs/spec/grammar.ebnf` where practical. It
covers:

- basic declarations and statements
- functions and function literals
- table, list, and dense literals
- goroutine/channel/select syntax

## Verify

From this directory:

```sh
npm install
npm test
```

If the tree-sitter CLI is already available globally, `tree-sitter generate`
and `tree-sitter test` are sufficient.

## Downstream Editor Integration

This grammar is intended to be consumed from this directory until Leia has a
published tree-sitter package. Keep downstream integrations pointed at:

- grammar name: `leia`
- file type: `leia`
- scope: `source.leia`
- source path: `/path/to/leia/tools/tree-sitter-leia`
- generated parser: `src/parser.c`

The grammar currently provides parser support only. Hosts that need syntax
highlighting queries should either add their own tree-sitter queries or keep
using the TextMate assets in `tools/syntax/textmate`.

### Neovim

Register the parser with `nvim-treesitter` and use the local grammar path:

```lua
local parser_config = require("nvim-treesitter.parsers").get_parser_configs()

parser_config.leia = {
  install_info = {
    url = "/path/to/leia/tools/tree-sitter-leia",
    files = { "src/parser.c" },
    generate_requires_npm = false,
    requires_generate_from_grammar = false,
  },
  filetype = "leia",
}

vim.filetype.add({
  extension = { leia = "leia" },
})
```

After adding the config, run `:TSInstall leia` or `:TSUpdate leia`.

### Helix

Add a local grammar entry to `languages.toml`:

```toml
[[language]]
name = "leia"
scope = "source.leia"
file-types = ["leia"]
grammar = "leia"

[[grammar]]
name = "leia"
source = { path = "/path/to/leia/tools/tree-sitter-leia" }
```

Then run:

```sh
hx --grammar fetch
hx --grammar build
```

### Zed

Use a Zed extension grammar entry that points at this directory with a `file://`
repository URL:

```toml
[grammars.leia]
repository = "file:///path/to/leia/tools/tree-sitter-leia"
rev = "<commit-sha>"
```

The language registration should use `leia` as the grammar name, `source.leia`
as the scope, and `.leia` as the file extension.

### Emacs `treesit`

The bundled `editors/emacs/leia-mode.el` is dependency-free and does not require
tree-sitter. For an Emacs 29+ treesit experiment, install this grammar from the
local checkout and remap `leia-mode` after the parser is available:

```elisp
(add-to-list 'treesit-language-source-alist
             '(leia "/path/to/leia/tools/tree-sitter-leia"))
(treesit-install-language-grammar 'leia)

(add-to-list 'major-mode-remap-alist '(leia-mode . leia-ts-mode))
```

`leia-ts-mode` is not bundled yet; downstream packages should define it with
`define-derived-mode` and `treesit-parser-create` until the repository adds a
native treesit mode.
