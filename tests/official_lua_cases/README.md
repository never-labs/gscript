# Official Lua Translation Cases

This directory contains hand-translated GScript counterparts for selected
official Lua test-suite semantics. Each pair uses the same base name:

- `name.lua` is the Lua oracle.
- `name.gs` is the GScript translation.

`TestOfficialLuaTranslatedCases` runs every pair with Lua and GScript VM, then
compares stdout exactly after trimming surrounding whitespace. Set
`GSCRIPT_OFFICIAL_CHECK_JIT=1` to include GScript JIT in the comparison.

The source ideas come from the official Lua 5.4 test suite, but the files here
are intentionally small slices so unsupported Lua syntax can be translated
incrementally into GScript's Go-like syntax.
