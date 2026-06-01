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

cmp -s tools/syntax/textmate/leia.tmLanguage.json editors/vscode/syntaxes/leia.tmLanguage.json
cmp -s tools/syntax/textmate/leia-mod.tmLanguage.json editors/vscode/syntaxes/leia-mod.tmLanguage.json

node -c editors/vscode/extension.js >/dev/null
python3 -m py_compile scripts/spec_preview.py

if command -v emacs >/dev/null 2>&1; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  emacs -Q --batch -L editors/emacs \
    --eval "(setq byte-compile-dest-file-function (lambda (_) \"$tmpdir/leia-mode.elc\"))" \
    -f batch-byte-compile editors/emacs/leia-mode.el >/dev/null
fi

echo "editor_check.sh: ok"
