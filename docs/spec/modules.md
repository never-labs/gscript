# Modules And Loading

`require(name)` loads built-in standard-library modules, host-registered
modules, and project modules according to runtime module options. `name` must
be a string. Loaded module results are cached in `package.loaded`.

```leia
json := require("json")
text := json.encode({ok: true})
same := require("json")
```

If a module returns a value, that value is the result of `require`. Requiring
the same module path again returns the cached module value unless the host
explicitly installs a different loader policy.

The v1.0 stable resolution order is:

1. If `package.loaded[name]` is non-`nil`, return it.
2. If the runtime has an internal loaded-module cache entry for `name`, return
   that entry.
3. If `name` is an enabled standard-library module, return that module and
   store it in `package.loaded[name]`.
4. If `name` names a host-registered module or an allowlisted `go:` import,
   return that module and store it in `package.loaded[name]`. Host modules are
   subject to the active capability policy.
5. If filesystem module loading is disabled, raise a runtime error. This does
   not disable already loaded modules, standard-library modules, or registered
   host modules.
6. Resolve a filesystem-backed `.leia` module path using the project resolver
   below, then execute that file.
7. If the file returns at least one value, cache and return the first value. If
   it returns no values, cache and return `true`.

Filesystem-backed project module resolution is deterministic:

1. A collection require of the form `prefix:path.to.module` matches a configured
   collection with the same `prefix`; it resolves under the collection root as
   `path/to/module.leia`.
2. Otherwise, the longest configured `replace` path that equals `name` or is a
   slash-prefix of `name` wins. Exact replaces may point at a `.leia` file or at
   a module root. Subpaths below a replace root convert dots to path separators
   and append `.leia`.
3. Otherwise, the longest downloaded-cache or vendor entry that equals `name` or
   is a slash-prefix of `name` wins. Subpaths convert dots to path separators and
   append `.leia`; an exact module path loads the module's base file.
4. Otherwise, `name` resolves relative to the active require root or script
   directory. Names containing `/` or starting with `.` keep path syntax and add
   `.leia`; other names convert dots to path separators and add `.leia`.

The resolved file is still checked by filesystem root, module byte, module
depth, readonly, vendor, and capability controls before execution.

```leia run all
json1 := require("json")
json2 := require("json")
assert(json1 == json2)
assert(package.loaded["json"] == json1)
```

The `package.loaded` table is part of the lookup contract. A non-`nil` entry
short-circuits resolution and is returned directly. This is mainly useful for
embedders, tests, and compatibility shims; ordinary modules should prefer
returning a value from the module file.

```leia run all
package.loaded["example.preloaded"] = {answer: 42}
mod := require("example.preloaded")
assert(mod.answer == 42)
assert(require("example.preloaded") == mod)
```

If a `package.loaded[name]` entry is replaced with another non-`nil` value,
the next `require(name)` returns that replacement. Assigning `nil` removes the
table entry but does not specify a full unload operation: an implementation may
still have an internal loaded-module cache entry for `name`, and the normal
resolution order applies again.

```leia run
package.loaded["example.override"] = {value: 1}
first := require("example.override")
assert(first.value == 1)

package.loaded["example.override"] = {value: 2}
second := require("example.override")
assert(second.value == 2)

package.loaded["example.override"] = nil
third := require("example.override")
assert(third == second)
```

The module name must be a string.

```leia run all
ok, err := pcall(require, 123)
assert(!ok)
assert(type(err) == "string")
```

`leia.mod` describes a module path, Leia language/module format version,
dependencies, replacements, capability summaries, source collections, and
optional Go-native binding metadata. The current manifest grammar is line
oriented. Comments start with `//`; paths may contain letters, digits, `.`,
`-`, `_`, `/`, and `:`, but not `..` or `\`.

```text
module example.com/app
leia 0.1

capability fs.read net.client
capability tool.exec

require github.com/example/lib v1.2.3
replace github.com/example/lib v1.2.3 => ./third_party/lib
collection assets ./assets

go 1.25
go require example.com/native v1.0.0
go replace example.com/native => ./native
```

The `module` directive is required. `leia` records the Leia language/module
format version; if it is absent, the current parser defaults it to `0.1`.
`require` stores a module path and a version string. The implementation treats
versions as opaque strings, but the downloader currently supports GitHub tag
downloads for `github.com/owner/repo[/subdir]` requirements. `replace` maps a
module path, optionally for one version, to another path; runtime module
options only use local replacement targets (`./...` or absolute paths). `go`,
`go require`, and `go replace` are metadata for `leia mod gomod`; they do not
enable source-level `go:` imports by themselves.

`capability` (or `cap`) is a declarative summary of host capabilities the
module expects. Capabilities may be written as separate fields or comma
separated values. The module loader does not grant permissions from this field;
hosts still enforce the active capability policy. `leia mod capability` reads
the main manifest and locally available dependency manifests, then reports a
capability universe and per-module matrix. Missing dependency manifests are
warnings, not proof that a module needs no capabilities.

For example, this manifest says the module expects filesystem reads and network
client access:

```text
module example.com/report
leia 0.1

capability fs.read,net.client
```

Running `leia mod capability --json` from that module produces a machine-readable
summary of declared capabilities. The exact output order is not part of the
language contract, but the data model is a universe of capability names plus a
per-module matrix.

`collection name path` configures collection requires such as
`require("assets:icons.logo")`. The name may contain letters, digits, `_`, and
`-`. Relative collection paths are resolved from the directory containing the
nearest `leia.mod`.

`leia.sum` records hashes produced by module tooling. `leia mod lock` writes
entries for local collections, local replaces, and locally available downloaded
or vendored requirements. `leia mod download` and `leia mod vendor` update
remote module entries after the dependency is present locally. The current file
format is:

```text
collection NAME TARGET h1:BASE64_SHA256
replace PATH VERSION_OR_- TARGET h1:BASE64_SHA256
module PATH VERSION TARGET h1:BASE64_SHA256
```

Hashes are computed over stable path/data pairs. Directory hashes include only
`.leia` files and `leia.mod`, skip VCS directories, sort paths, and use the
`h1:` prefix with base64-encoded SHA-256 bytes. `leia mod verify` and
`leia mod check` compare current local content with `leia.sum`; if no
`leia.sum` exists, sum verification is skipped.

Module paths may be GitHub-style repository paths. Leia does not require a
central registry for basic module use.

`import "go:..." as name` is explicit host binding syntax. It does not
automatically reflect arbitrary Go packages; embedders must provide bindings
through the public Go API and the active capability policy may still reject use.

```leia
import "go:net/http" as http
```

The declaration above is only valid when the embedder has allowlisted and
registered the `go:net/http` binding. Source syntax alone never grants host
access.

Module mode controls how a discovered manifest is applied at runtime:

| Mode | Runtime behavior |
| --- | --- |
| `mod` | Use local replaces, existing vendor entries, and existing module-cache entries. It does not download or mutate files. |
| `readonly` | Same offline behavior as `mod` for current resolution. It is the mode hosts should choose when manifest/cache mutation is disallowed. |
| `vendor` | Ignore the module cache and resolve required remote modules from `vendor/PATH@VERSION`. Missing remote vendor entries stay visible to resolution and fail normally when the module file is unavailable. |

When `ModuleOptionsForScriptMode` is used, the nearest ancestor `leia.mod`
sets the require root, collections, local replaces, module mode, and any local
vendor/cache entries already present. Invalid manifests are ignored by that
runtime helper rather than repaired. The CLI module commands are the mutating
surface: `leia mod add`, `tidy`, `download`, `vendor`, and `lock` may update
`leia.mod`, `vendor/`, the module cache, or `leia.sum`; ordinary script
execution does not.
