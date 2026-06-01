# Leia Syntax Assets

This directory contains editor-neutral syntax assets for Leia source.

- `textmate/leia.tmLanguage.json` is the shared TextMate grammar for `.leia`
  files.
- `textmate/leia-mod.tmLanguage.json` highlights `leia.mod` manifests.

Editor packages should consume these files directly or copy them mechanically
without changing the grammar rules.
