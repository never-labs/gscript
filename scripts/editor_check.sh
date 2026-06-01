#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

python3 -m json.tool tools/syntax/textmate/leia.tmLanguage.json >/dev/null
python3 -m json.tool tools/syntax/textmate/leia-mod.tmLanguage.json >/dev/null
python3 -m json.tool editors/vscode/package.json >/dev/null
python3 -m json.tool editors/vscode/language-configuration.json >/dev/null
python3 -m json.tool editors/vscode/snippets/leia.json >/dev/null
python3 -m json.tool editors/vscode/syntaxes/leia.tmLanguage.json >/dev/null
python3 -m json.tool editors/vscode/syntaxes/leia-mod.tmLanguage.json >/dev/null
python3 -m json.tool tools/tree-sitter-leia/src/grammar.json >/dev/null
python3 -m json.tool tools/tree-sitter-leia/src/node-types.json >/dev/null

cmp -s tools/syntax/textmate/leia.tmLanguage.json editors/vscode/syntaxes/leia.tmLanguage.json
cmp -s tools/syntax/textmate/leia-mod.tmLanguage.json editors/vscode/syntaxes/leia-mod.tmLanguage.json

node -c editors/vscode/extension.js >/dev/null
node -c tools/tree-sitter-leia/grammar.js >/dev/null
python3 -m py_compile scripts/spec_preview.py
python3 tools/editor/smoke/editor_smoke.py >/dev/null

tree_sitter=""
if [ -x tools/tree-sitter-leia/node_modules/.bin/tree-sitter ]; then
  tree_sitter="$ROOT/tools/tree-sitter-leia/node_modules/.bin/tree-sitter"
elif command -v tree-sitter >/dev/null 2>&1; then
  tree_sitter="$(command -v tree-sitter)"
fi

if [ -n "$tree_sitter" ]; then
  (cd tools/tree-sitter-leia && "$tree_sitter" test >/dev/null)
fi

if command -v emacs >/dev/null 2>&1; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  emacs -Q --batch -L editors/emacs \
    --eval "(setq byte-compile-dest-file-function (lambda (_) \"$tmpdir/leia-mode.elc\"))" \
    -f batch-byte-compile editors/emacs/leia-mode.el >/dev/null
fi

echo "editor_check.sh: ok"
