# Leia CLI Reference

Generated from the current `leia` binary capabilities.

| Command | Usage | Summary |
|---|---|---|
| `bench` | `usage: leia bench [--manifest-check|--quick|--full|--guard|BENCH|compare [--quick]|strict|diagnose|audit|rank-luajit-gaps|debug-artifact|coverage|profile-exits|validate-lua-refs|submit-guard|jit-addr-map|regression-guard] [benchmark-harness-flags...]` | Run benchmark and benchmark-diagnostic harnesses. |
| `capabilities` | `usage: leia capabilities [--json]` | Report binary capabilities, stdlib modules, and supported tooling formats. |
| `check` | `usage: leia check [--json] [--quick|--full] [--no-fmt] [--no-lint] [--no-test] [--no-manifest] [--no-docs] [--no-editor] [--no-examples] [path-or-dir]` | Run formatter, linter, manifest, tests, docs, editor, and example checks as one local gate. |
| `ci` | `usage: leia ci [smoke|pr|perf|release] [--list] [--json] [--no-luajit] [--release-version VERSION]` | Run canonical local CI profiles. |
| `config` | `usage: leia config [--json] [path]` | Discover and validate project configuration. |
| `diag` | `usage: leia diag [dump|bundle] [diagnostic-flags...]` | Run production diagnostic dump and bundle tools. |
| `diagnose` | `usage: leia diagnose <benchmark> [diagnose-flags...]` | Collect benchmark timing, exit, and Tier 2 diagnostics. |
| `doc` | `usage: leia doc generate [flags]; leia doc check [--json]; leia doc site-check [--site-dir DIR] [--json]` | Generate reference docs or validate repository docs and rendered site output. |
| `editor` | `usage: leia editor smoke` | Validate editor-facing syntax, LSP, and packaging assets. |
| `env` | `usage: leia env [--json] [--path PATH]` | Report toolchain, project, cache, and platform environment. |
| `eval` | `usage: leia eval [--vm] [--jit=true|false] <source> [args...]` | Execute source passed on the command line. |
| `evaluate` | `usage: leia evaluate [--json|--format=text|json|html] [--report FILE] [--gate] [--parallel N] [--baseline FILE|--compare OLD NEW] [--list] [--filter TEXT] [--replay FILE|--record FILE|--update-golden FILE] [path-or-dir...]` | Run evaluation checks and emit an agent evaluation report. |
| `examples` | `usage: leia examples [list|show|run|check] [--json] [ID-or-path]` | List, inspect, run, or check repository example projects. |
| `fmt` | `usage: leia fmt [--check] [--write] [--json] [--stdin-file-name FILE] <path-or-dir> [...]` | Normalize source formatting. |
| `help` | `usage: leia help [command]` | Show command help. |
| `inspect` | `usage: leia inspect bytecode [--json] [--proto NAME] <file.leia>
       leia inspect directives [--json] <file.leia>` | Inspect compiled artifacts and file directives. |
| `lint` | `usage: leia lint [--json|--format=text|json|sarif] <path-or-dir> [...]` | Report source diagnostics. |
| `lsp` | `usage: leia lsp` | Run the Leia Language Server Protocol endpoint over stdio. |
| `mod` | `usage: leia mod [init|add|tidy|check|download|vendor|lock|list|graph|explain|capability|gomod|verify] [flags]` | Manage local module metadata and require graphs. |
| `playground` | `usage: leia playground [--addr ADDR] [--timeout DURATION] [--max-source-bytes N] [--max-steps N]` | Serve the local backend-powered Leia playground. |
| `repl` | `usage: leia repl` | Start the interactive shell. |
| `run` | `usage: leia run [--vm] [--jit=true|false] [--mod=readonly|vendor|mod] <file.leia> [args...]` | Run a script file. |
| `test` | `usage: leia test [--manifest-check] [--json|--format=text|json] [--output FILE] [--golden=auto|require|ignore|update] [--list] [--seed SEED] [path-or-dir]` | Run Leia test files and expected stdout checks. |
| `version` | `usage: leia version [--json]` | Report binary version and build metadata. |
