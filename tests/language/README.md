# Language Conformance Cases

This directory contains hand-translated Leia counterparts for selected
language conformance test-suite semantics. Each pair uses the same base name:

- `name.lua` is the Lua oracle.
- `name.leia` is the Leia translation.

`TestLanguageConformanceTranslatedCases` runs every pair with Lua and Leia VM, then
compares stdout exactly after trimming surrounding whitespace. Set
`LEIA_OFFICIAL_CHECK_JIT=1` to include Leia JIT in the comparison.

The source ideas come from the language conformance 5.4 test suite, but the files here
are intentionally small slices so unsupported Lua syntax can be translated
incrementally into Leia's Go-like syntax.
