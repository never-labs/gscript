# Modules And Loading

`require(name)` loads built-in standard-library modules and project modules
according to runtime module options. Loaded module results are cached in
`package.loaded`.

```leia
json := require("json")
text := json.encode({ok: true})
same := require("json")
```

If a module returns a value, that value is the result of `require`. Requiring
the same module path again returns the cached module value unless the host
explicitly installs a different loader policy.

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
