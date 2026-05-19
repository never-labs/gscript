# Current Official Lua Semantic Gaps

The default translated-case harness has no skipped known failures. These are
the concrete gaps found while expanding the official-suite translations and
should be fixed before translating dependent slices.

| Area | Current gap |
|---|---|
| Floor division | GScript lexes `//` as a line comment, so Lua floor-division slices need syntax support or a faithful translated helper. |
| Bitwise operators | Lua operators such as `|`, `&`, `~`, `<<`, and `>>` are not accepted by the GScript lexer. |
| Hexadecimal numerals | GScript rejected `0xFFFFFFFF` in file mode; translated cases use decimal equivalents where possible. |
| Numeric table key `0` | Fixed for newly allocated mixed array tables; still needs broader regression coverage across typed/dense table paths. |
| Typed numeric table key `0` | Dense typed int arrays can still return numeric zero for an unset `t[0]`; seen while translating `constructs.lua` loop-table checks. |
| Numeric/control escapes | Lua-style numeric/control string escapes such as `"\000"` and `\a\b\f\r\v` are not fully equivalent in GScript string literals/patterns. |
| Nested multireturn in file mode | `table.unpack(a)` or `string.byte(...)` nested under another call/table constructor can collapse differently when loaded from a `.gs` file. |
| Table constructors with recursive multireturns | Official `constructs.lua` cases like `{f(3), f(5), f(10)}` do not yet match Lua in `.gs` file mode. |
| Parenthesized call adjustment | Lua adjusts `(f())` to one result; GScript probes returned multiple values. |
| Tail-call vararg forwarding with values | Empty vararg tail forwarding passes; forwarding real arguments through a tail-call wrapper failed in `.gs` file mode. |
| Inline function literals in file mode | Some inline function literals passed to `pcall`, `table.sort`, or higher-order functions resolve differently from named functions. |
| Inline table literals as function args in file mode | `table.move({ ... }, ...)` and some `table.unpack({ ... })` call sites differ from assigning the table to a local first. |
| `table.sort` comparator in file mode | Passing a comparator function to `table.sort` from a translated file can hit a nil-function path. |
| Dynamic metatable construction in loops | A direct `setmetatable({i}, {__call = u})`-style translation required explicit local tables in `.gs` file mode. |
| `__call` delegation returning varargs | Chained `__call` invocation count works, but returned vararg table contents differed in `.gs` file mode. |
| Yield inside `__pairs` | A VM `__pairs` closure that calls `coroutine.yield` completes only on the next resume after `pairs(t)` has already produced nil results. |
| Lua GC internals | `collectgarbage` protocol basics are supported, but finalizers, weak tables, and exact Lua GC aging/barrier behavior are not modeled. |
| UTF-8 strict/nonstrict validation edge cases | `utf8.codes`/`codepoint` now use byte positions for valid UTF-8, but the full official invalid/nonstrict edge matrix is not translated yet. |
| `string.pack` family | `string.pack`, `string.unpack`, and `string.packsize` are nil/unsupported. |
| JIT official check | `GSCRIPT_OFFICIAL_CHECK_JIT=1` still exposes JIT-only semantic issues in closure and multi-return slices; default official semantic comparison is Lua vs GScript VM. |
