# Leia Neovim Integration

This directory contains local tree-sitter assets for Neovim.

```lua
vim.filetype.add({
  extension = {
    leia = "leia",
  },
  filename = {
    ["leia.mod"] = "leia",
    ["leia.sum"] = "leia",
  },
})

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
```

Copy or symlink `editors/neovim/queries/leia` into a directory on Neovim's
runtime path to enable highlights.
