#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

REQUIRE_TREE_SITTER=0
JSON_OUT=0
TREE_SITTER_STATUS="pending"
TREE_SITTER_COMMAND=""
EMACS_STATUS="pending"
EMACS_COMMAND=""

usage() {
  cat <<'EOF'
Usage: scripts/editor_check.sh [--require-tree-sitter] [--json] [--help]

Checks editor-facing assets for Leia. By default, tree-sitter corpus tests run
when a tree-sitter CLI is available and print an explicit skip when it is not.

Options:
  --require-tree-sitter
      Fail if neither tools/tree-sitter-leia/node_modules/.bin/tree-sitter nor
      a global tree-sitter command is available.
  --json
      Print a machine-readable editor asset report.
  --help
      Show this help.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --require-tree-sitter)
      REQUIRE_TREE_SITTER=1
      ;;
    --json)
      JSON_OUT=1
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

json_escape() {
  local value="$1"
  value="${value//\\/\\\\}"
  value="${value//\"/\\\"}"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  printf '%s' "$value"
}

print_json_report() {
  printf '{\n'
  printf '  "schema_version": 1,\n'
  printf '  "status": "pass",\n'
  printf '  "require_tree_sitter": %s,\n' "$(if [ "$REQUIRE_TREE_SITTER" = "1" ]; then printf true; else printf false; fi)"
  printf '  "tree_sitter_status": "%s",\n' "$(json_escape "$TREE_SITTER_STATUS")"
  printf '  "tree_sitter_command": "%s",\n' "$(json_escape "$TREE_SITTER_COMMAND")"
  printf '  "emacs_status": "%s",\n' "$(json_escape "$EMACS_STATUS")"
  printf '  "emacs_command": "%s",\n' "$(json_escape "$EMACS_COMMAND")"
  printf '  "textmate_grammars": [\n'
  printf '    "tools/syntax/textmate/leia.tmLanguage.json",\n'
  printf '    "tools/syntax/textmate/leia-mod.tmLanguage.json"\n'
  printf '  ],\n'
  printf '  "vscode_assets": [\n'
  printf '    "editors/vscode/package.json",\n'
  printf '    "editors/vscode/language-configuration.json",\n'
  printf '    "editors/vscode/snippets/leia.json",\n'
  printf '    "editors/vscode/syntaxes/leia.tmLanguage.json",\n'
  printf '    "editors/vscode/syntaxes/leia-mod.tmLanguage.json"\n'
  printf '  ],\n'
  printf '  "tree_sitter_assets": [\n'
  printf '    "tools/tree-sitter-leia/grammar.js",\n'
  printf '    "tools/tree-sitter-leia/src/grammar.json",\n'
  printf '    "tools/tree-sitter-leia/src/node-types.json"\n'
  printf '  ],\n'
  printf '  "smoke_tests": [\n'
  printf '    "tools/editor/smoke/editor_smoke.py"\n'
  printf '  ]\n'
  printf '}\n'
}

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
python3 - scripts/spec_preview.py <<'PY'
import ast
import pathlib
import sys

path = pathlib.Path(sys.argv[1])
ast.parse(path.read_text(encoding="utf-8"), filename=str(path))
PY
python3 tools/editor/smoke/editor_smoke.py >/dev/null

tree_sitter=""
if [ -x tools/tree-sitter-leia/node_modules/.bin/tree-sitter ]; then
  tree_sitter="$ROOT/tools/tree-sitter-leia/node_modules/.bin/tree-sitter"
elif command -v tree-sitter >/dev/null 2>&1; then
  tree_sitter="$(command -v tree-sitter)"
fi

if [ -n "$tree_sitter" ]; then
  TREE_SITTER_STATUS="verified"
  TREE_SITTER_COMMAND="$tree_sitter"
  (cd tools/tree-sitter-leia && "$tree_sitter" test >/dev/null)
elif [ "$REQUIRE_TREE_SITTER" = "1" ]; then
  echo "editor_check.sh: tree-sitter CLI is required; run: npm --prefix tools/tree-sitter-leia ci" >&2
  exit 1
else
  TREE_SITTER_STATUS="skipped"
  if [ "$JSON_OUT" != "1" ]; then
    echo "editor_check.sh: skipped tree-sitter corpus; run npm --prefix tools/tree-sitter-leia ci or pass --require-tree-sitter" >&2
  fi
fi

if command -v emacs >/dev/null 2>&1; then
  EMACS_STATUS="verified"
  EMACS_COMMAND="$(command -v emacs)"
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  emacs -Q --batch -L editors/emacs \
    --eval "(setq byte-compile-dest-file-function (lambda (_) \"$tmpdir/leia-mode.elc\"))" \
    -f batch-byte-compile editors/emacs/leia-mode.el >/dev/null
else
  EMACS_STATUS="skipped"
fi

if [ "$JSON_OUT" = "1" ]; then
  print_json_report
else
  echo "editor_check.sh: ok"
fi
