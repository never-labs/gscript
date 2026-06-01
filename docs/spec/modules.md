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

`leia.mod` describes a module path, Leia language/module format version,
dependencies, replacements, capability summaries, source collections, and
optional Go-native binding metadata. `leia.sum` records remote or vendored
module hashes when the module toolchain is used.

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

Readonly and vendor module modes are intended for reproducible execution. Host
applications should choose the module mode explicitly when running untrusted
scripts.
