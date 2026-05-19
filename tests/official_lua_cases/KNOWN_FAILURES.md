# Current Official Lua Semantic Gaps

The default translated-case harness has no skipped known failures. These are
the concrete gaps found while expanding the official-suite translations and
should be fixed before translating dependent slices.

| Area | Current gap |
|---|---|
| Pattern escape compatibility | GScript string literals now support Go-style control, hex, Unicode, and decimal byte escapes; translated Lua pattern slices still need case-by-case mapping to GScript string/regexp semantics. |
| Yield inside `__pairs` | A VM `__pairs` closure that calls `coroutine.yield` completes only on the next resume after `pairs(t)` has already produced nil results. |
| Lua GC internals | `collectgarbage` protocol basics are supported, but finalizers, weak tables, and exact Lua GC aging/barrier behavior are not modeled. |
| UTF-8 strict/nonstrict validation edge cases | `utf8.codes`/`codepoint` now use byte positions for valid UTF-8, but the full official invalid/nonstrict edge matrix is not translated yet. |
| JIT official check | `GSCRIPT_OFFICIAL_CHECK_JIT=1` still exposes JIT-only semantic issues in closure and multi-return slices; default official semantic comparison is Lua vs GScript VM. |
