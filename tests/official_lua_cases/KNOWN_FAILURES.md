# Current Official Lua Semantic Gaps

The default translated-case harness has no skipped known failures. These are
the concrete gaps found while expanding the official-suite translations and
should be fixed before translating dependent slices.

| Area | Current gap |
|---|---|
| JIT official check | `GSCRIPT_OFFICIAL_CHECK_JIT=1` still exposes JIT-only semantic issues in closure and multi-return slices; default official semantic comparison is Lua vs GScript VM. |
