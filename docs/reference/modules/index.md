# Leia Modules

Leia projects use `leia.mod` for module identity, script dependencies,
capability summaries, and optional Go-native binding metadata. The format is
line-oriented and intentionally close to Go module files.

```text
module github.com/example/tools
leia 0.1

capability fs.read
capability llm.turn

require github.com/never-labs/leia-raylib v0.1.0
replace github.com/example/local v0.1.0 => ./local

collection examples ./examples

go 1.24
go require github.com/gen2brain/raylib-go v0.0.0
go replace github.com/example/native => ./native
```

## Directives

| Directive | Meaning |
|---|---|
| `module PATH` | Required module path. Paths may contain letters, digits, `.`, `-`, `_`, `/`, and `:`. |
| `leia VERSION` | Leia language/module format version. Defaults to `0.1` when omitted. |
| `require PATH VERSION` | Leia module dependency. |
| `replace PATH [VERSION] => PATH` | Replace a Leia dependency with another path. |
| `capability NAME[,NAME...]` | Capability summary required by this module. `cap` is accepted as a shorthand. |
| `collection NAME PATH` | Named source collection such as examples or demos. |
| `go VERSION` | Go toolchain version used for generated native bindings. |
| `go require PATH VERSION` | Go dependency used by native bindings. |
| `go replace PATH [VERSION] => PATH` | Replacement for a Go-native dependency. |

Comments begin with `//`. Unknown directives are diagnostics.

## Commands

Local metadata commands do not need network access:

```bash
leia mod init --module github.com/example/project
leia mod list --json
leia mod graph --json
leia mod capability --json
leia mod verify --json
leia mod gomod
```

Dependency commands operate on `require` directives in `leia.mod`. `tidy` may
remove requirements that are not used by source files, so run it after imports
or `require(...)` calls exist in the project.

`download` currently resolves GitHub-style module paths. Use `replace`,
`vendor/`, or local collections for non-GitHub paths until another resolver is
configured.

```bash
leia mod add github.com/never-labs/leia-raylib@v0.1.0
leia mod tidy
leia mod download
leia mod vendor
leia mod verify --json
```

| Command | Purpose |
|---|---|
| `leia mod init` | Create a `leia.mod` file for a project. |
| `leia mod add` | Add one or more `require` directives. |
| `leia mod tidy` | Check local module metadata and report missing or removable entries. |
| `leia mod check` | Alias for `leia mod verify`; useful in scripts that prefer check-style verbs. |
| `leia mod download` | Resolve and cache remote Leia modules. |
| `leia mod vendor` | Copy resolved dependencies into `vendor/` for hermetic execution. |
| `leia mod lock` | Write or refresh `leia.sum` from the current dependency graph. |
| `leia mod verify` | Validate `leia.mod`, `leia.sum`, and cached module hashes. |
| `leia mod list` | Print resolved module metadata. |
| `leia mod graph` | Print dependency edges discovered from static `import` and `require(...)` uses in source files. |
| `leia mod explain` | Explain where a module path resolves from. |
| `leia mod capability` | Summarize capability requirements across the module graph. |
| `leia mod gomod` | Generate a Go `go.mod` view for native binding dependencies. |

`leia.sum` records resolved hashes for downloaded modules, vendored modules,
local collections, and local replacements. `vendor/` can be used for offline or
hermetic execution through `leia run --mod=vendor`.

`leia run --mod=readonly` and `leia run --mod=vendor` do not download or rewrite
module metadata. They build the runtime resolver from the already-present cache
or `vendor/` tree, including transitive dependencies recorded in each resolved
dependency's `leia.mod`. A missing transitive dependency is therefore reported
as a local resolution error instead of silently falling back to network access.

## Example Evidence

Package-managed examples exercise module metadata, capability summaries, and
native binding declarations:

```bash
go run ./cmd/leia mod verify --json examples/database/package_managed
go run ./cmd/leia mod verify --json examples/macos/package_managed
go run ./cmd/leia mod verify --json examples/ui/package_managed
go run ./cmd/leia mod verify --json examples/tooling/release_gate_project
go run ./cmd/leia mod verify --json examples/tooling/package_manager_workflow
```

The same projects are referenced from `tests/feature_matrix.json` under
`module_package_management`, alongside `internal/modpkg/modpkg_test.go`,
`tests/architecture/package_boundary_test.go`, and `cmd/leia/main_mod_test.go`.
The `module_download_vendor_cache` row also points at the CLI/runtime resolver
tests that cover `leia mod download`, `leia mod vendor`, `leia mod verify`,
`leia run --mod=readonly`, and `leia run --mod=vendor`.

## Design Contract

Leia module management is intentionally smaller than npm-style package systems:

- module paths can be GitHub-style repository paths;
- native Go dependencies stay explicit in `leia.mod`;
- capability summaries remain visible at module boundaries;
- readonly and vendor modes are preferred for reproducible execution.

See [Security and sandboxing](../security/index.md) for the host capability
model that consumes these summaries.
