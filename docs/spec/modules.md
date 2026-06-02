# Modules And Loading

`require(name)` loads enabled standard-library modules, host-registered
modules, and filesystem-backed project modules according to runtime module
options. `name` must be a string. Loaded module results are cached in
`package.loaded`.

```leia
json := require("json")
text := json.encode({ok: true})
same := require("json")
```

The result of `require(name)` is a single value. Standard-library and host
modules define their own module value, usually a table or callable function. If
a filesystem-backed module returns at least one value, the first value is the
module value and additional return values are discarded. If it returns no
values, the module value is `true`.

The observable `require(name)` algorithm is:

1. If `package.loaded[name]` is non-`nil`, return it.
2. If the runtime has an internal loaded-module cache entry for `name`, return
   that entry. This cache is an implementation detail; `package.loaded` is the
   portable observable table.
3. If `name` is an installed standard-library module that remains enabled by
   the active `LibFlags` policy, return its module value and store it in
   `package.loaded[name]`. Disabled standard-library modules are removed from
   the module cache and from `package.loaded`.
4. If `name` names a host-registered module, including an explicitly
   allowlisted `go:` module installed by the embedder, return that module and
   store it in `package.loaded[name]`. Source syntax never loads arbitrary Go
   packages by path.
5. If filesystem module loading is disabled, raise `module loading disabled`.
   This does not disable already loaded modules, enabled standard-library
   modules, or registered host modules.
6. Resolve a filesystem-backed `.leia` file with the resolver below. If the
   resolved path is outside the configured filesystem root, violates module byte
   or module depth limits, cannot be opened, or does not exist, raise a runtime
   error.
7. Execute the resolved file. If execution succeeds, normalize the return value
   to one module value, store that value in the internal cache where present and
   in `package.loaded[name]`, and return it.

A failed load does not produce a stable cached module value. A later successful
`require(name)` must still return the successful module value.

Filesystem-backed modules are cached only after successful execution. The
current implementation does not prefill `package.loaded[name]` with an
in-progress placeholder before running the file. A direct or indirect cycle such
as `a` requiring `b` requiring `a` therefore recurses until a configured module
depth limit or another runtime resource limit fails; it does not return a
partially initialized module.

Resolution uses the original string `name` as the `package.loaded` key. Path
normalization during filesystem resolution does not make different spelling
forms interchangeable cache keys unless an embedding policy documents that
aliasing explicitly.

Filesystem-backed project module resolution is deterministic:

1. A collection require of the form `prefix:path.to.module` matches a configured
   collection with the same `prefix`; it resolves under the collection root as
   `path/to/module.leia`.
2. Otherwise, the longest configured `replace` path that equals `name` or is a
   slash-prefix of `name` wins. An exact replace whose target ends in `.leia`
   loads that file; an exact replace whose target is not a `.leia` file appends
   `.leia` to the target path. Subpaths below a replace root convert dots to
   path separators and append `.leia`.
3. Otherwise, the longest local vendor or downloaded-cache entry that equals
   `name` or is a slash-prefix of `name` wins. Subpaths convert dots to path
   separators and append `.leia`; an exact cache or vendor module path loads
   `basename(name).leia` inside that entry root.
4. Otherwise, `name` resolves relative to the active require root or script
   directory. Names containing `/` or starting with `.` keep path syntax and add
   `.leia`; other names convert dots to path separators and add `.leia`. This
   is local module path lookup; the `module` directive in `leia.mod` does not by
   itself create a filesystem file.

The resolved file is still checked by filesystem root, module byte, module
depth, readonly, vendor, and capability controls before execution.

```leia run all
json1 := require("json")
json2 := require("json")
assert(json1 == json2)
assert(package.loaded["json"] == json1)
```

```leia run all
fs := require("fs")
path := require("path")
script := require("script")

modulePath := path.join(script.dir(), "local_helper.leia")
assert(fs.writefile(modulePath, `return {value: 7}`))

helper := require("local_helper")
assert(helper.value == 7)
assert(require("local_helper") == helper)
assert(package.loaded["local_helper"] == helper)
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

Because `package.loaded` is an ordinary table, any non-`nil` Leia value can be
used as a preloaded module value. A false module value is still loaded: only
`nil` means absent.

```leia run all
package.loaded["example.false"] = false
assert(require("example.false") == false)

fn := func(x) { return x + 1 }
package.loaded["example.callable"] = fn
assert(require("example.callable")(4) == 5)
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

Module mode controls how a discovered manifest is applied at runtime. Runtime
module setup is offline: it uses local replaces, already present vendor
directories, and already present cache directories; it does not run `git`,
download modules, or rewrite `leia.mod`.

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

Top-level module code executes in the same language as ordinary scripts.
Lexical declarations inside the module are private to that module execution
unless the module returns or stores a value that exposes them. Mutating globals
or host resources during module loading is a side effect of `require`; portable
modules should make those side effects explicit in their documented contract
and keep the returned module value stable across repeated requires.
