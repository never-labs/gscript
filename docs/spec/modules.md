# Modules And Loading

`require(name)` loads built-in standard-library modules and project modules
according to runtime module options. Loaded module results are cached in
`package.loaded`.

`leia.mod` describes a module path, Leia language/module format version,
dependencies, replacements, capability summaries, source collections, and
optional Go-native binding metadata. `leia.sum` records remote or vendored
module hashes when the module toolchain is used.

Module paths may be GitHub-style repository paths. Leia does not require a
central registry for basic module use.

`import "go:..." as name` is explicit host binding syntax. It does not
automatically reflect arbitrary Go packages; embedders must provide bindings
through the public Go API and the active capability policy may still reject use.

Readonly and vendor module modes are intended for reproducible execution. Host
applications should choose the module mode explicitly when running untrusted
scripts.
