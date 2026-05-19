# Current Official Lua Semantic Gaps

The default translated-case harness has no skipped known failures. These are
the concrete gaps found while expanding the official-suite translations and
should be fixed before translating dependent slices.

| Area | Current gap |
|---|---|
| Lua GC internals | `collectgarbage` protocol basics are supported, but finalizers, weak tables, and exact Lua GC aging/barrier behavior are not modeled. |
| UTF-8 strict/nonstrict validation edge cases | `utf8.codes`/`codepoint` now use byte positions for valid UTF-8, but the full official invalid/nonstrict edge matrix is not translated yet. |
| JIT official check | `GSCRIPT_OFFICIAL_CHECK_JIT=1` still exposes JIT-only semantic issues in closure and multi-return slices; default official semantic comparison is Lua vs GScript VM. |
