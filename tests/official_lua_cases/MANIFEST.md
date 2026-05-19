# Official Lua Case Translation Manifest

Source baseline:

- Suite: official Lua 5.4.8 tests
- URL: `https://www.lua.org/tests/lua-5.4.8-tests.tar.gz`
- SHA-256: `9581d5a7c39ffbf29b8ccde2709083c380f7bbddbd968dcb15712d2f2e33f4e5`

Harness:

```bash
go test ./tests -run TestOfficialLuaTranslatedCases -count=1 -v
```

The harness compares stdout from:

1. Lua oracle: `*.lua`
2. GScript VM: `gscript -vm *.gs`

Set `GSCRIPT_OFFICIAL_CHECK_JIT=1` to also compare `gscript -jit *.gs`.

Current translated passing cases: 423.

| Case | Official source area | Notes |
|---|---|---|
| `bitwise_bit32_errors` | `bitwise.lua` | `bit32` argument error behavior for invalid values and missing shift counts. |
| `bitwise_bit32_arshift_more` | `bitwise.lua` | Additional `bit32.arshift` positive, negative, large shift, and negative displacement cases. |
| `bitwise_bit32_extract_replace` | `bitwise.lua` | `bit32.btest`, `extract`, `replace`, default widths, and invalid field/width errors. |
| `bitwise_bit32_float_conversion` | `bitwise.lua` | `bit32` conversion of integral floats and large wrapped float operands. |
| `bitwise_bit32_identities` | `bitwise.lua` | `bit32` boolean algebra identities and shift/rotate invariants over representative 32-bit values. |
| `bitwise_bit32_rotate` | `bitwise.lua` | `bit32.lrotate`/`rrotate` compatibility-library rotate semantics. |
| `bitwise_bit32_shift_sweep` | `bitwise.lua` | `bit32.lshift` checked against floating modulo arithmetic over representative 24-bit values and shift counts. |
| `bitwise_bit32_shifts` | `bitwise.lua` | `bit32.lshift`, `rshift`, and unsigned-result `arshift` edge cases. |
| `bitwise_bit32_varargs` | `bitwise.lua` | `bit32` empty and multi-argument `band`/`bor`/`bxor`/`btest` defaults. |
| `bitwise_bit32_wrap_more` | `bitwise.lua` | Additional `bit32.band` wrapping for positive and negative values around 2^33 and 2^40. |
| `bitwise_direct_ops_more` | `bitwise.lua`, `bwcoercion.lua` | First-class bitwise expression operators, complement, bit-clear translation, shifts, and small accumulation loop. |
| `bits_go_host_more` | `bitwise.lua`, `bwcoercion.lua` | Go-host 64-bit `bits` helper operations, rotations, bit tests/mutations, counts, and invalid shift diagnostics. |
| `container_sort_go_host_more` | `api.lua`, `sort.lua` | Go-host container set/queue/stack/heap helpers and `sort` namespace ordering/search helpers. |
| `api_testkit_runtime_diagnostics` | `api.lua` | GScript `testkit` replacement for Lua's private C API test library: memory snapshots/checks, value inspection, protected calls, and function identity. |
| `attrib_const_defer_gscript` | `locals.lua`, `constructs.lua` | GScript Go-style `const` readonly binding checks and `defer` LIFO cleanup on error; intentionally not Lua `<const>/<close>` syntax. |
| `attrib_require_builtin_modules_more` | `attrib.lua` | `require` returns cached builtin standard-library modules and exposes them through `package.loaded`. |
| `attrib_require_go_host_modules_more` | `attrib.lua` | Go-host standard-library modules return their global table from `require` and share identity through `package.loaded`. |
| `calls_anonymous_invocation` | `calls.lua` | Immediately invoked anonymous functions and simple multi-return anonymous closures. |
| `calls_builtin_missing_args` | `calls.lua` | Missing-argument errors for core builtins through protected calls. |
| `calls_fixedpoint_returns` | `calls.lua` | Fixed-point function calls, closure-returning calls, recursive multi-return unpack. |
| `calls_inline_function_args` | `calls.lua`, `sort.lua` | Inline function literals passed directly to `pcall`, `table.sort`, and higher-order call sites. |
| `calls_inline_table_args` | `calls.lua`, `sort.lua` | Inline table literals used directly as function arguments. |
| `calls_incorrect_args_more` | `calls.lua` | Extra arguments ignored by fixed-arity builtins/calls and nil parameter adjustment behavior. |
| `calls_long_method_name_more` | `calls.lua` | Long method-name definition and repeated self-style invocation. |
| `calls_method_recursion` | `calls.lua` | Local recursion, method calls, chained method calls. |
| `calls_multireturn` | `calls.lua` | Missing argument nils, fixed multi-return assignment, tail recursion. |
| `calls_multireturn_adjust_more` | `calls.lua`, `constructs.lua`, `vararg.lua` | Parenthesized call adjustment, nested multi-return argument adjustment, recursive multi-return table constructors, and value-bearing vararg tail forwarding. |
| `calls_nested_method_more` | `calls.lua` | Nested table function assignment and explicit self-style method mutation. |
| `calls_recursion_error_more` | `calls.lua` | Recursive protected error propagation and moderate tail-recursive descent. |
| `calls_ret2_pack_adjust_more` | `calls.lua` | Recursive multi-return unpack feeding assignment, table constructors, and vararg packing in currently supported positions. |
| `calls_return_values` | `calls.lua`, `constructs.lua` | Multiple return assignment and nil-return basics. |
| `calls_tail_call_metamethod_more` | `calls.lua` | Tail-style calls through `__call` metamethod tables and chained callable tables ending in a recursive function. |
| `calls_tail_varargs` | `calls.lua` | Empty vararg forwarding through a tail call. |
| `calls_tail_varargs_more2` | `calls.lua` | Tail vararg forwarding into a sink function for empty, one-argument, and two-argument calls. |
| `calls_type_basic` | `calls.lua` | `type` over core values and builtins. |
| `calls_call_chain` | `calls.lua` | Chained `__call` metatables ending in `table.pack`. |
| `calls_extra_builtin_args_more` | `calls.lua` | Extra arguments to raw/table/math builtins are ignored where Lua specifies fixed-argument behavior. |
| `calls_fixed_arity_nested_adjust_more` | `calls.lua` | Fixed-arity calls collapse nested multi-return arguments to one value and numeric mutual recursion remains JIT-compatible. |
| `closure_upvalues` | `closure.lua` | Closure capture and independent upvalues. |
| `closure_for_control` | `closure.lua` | Independent closure captures for loop-created locals. |
| `closure_identity_more` | `closure.lua` | Distinct closures created in loops and stable identity when returning the same captured closure. |
| `closure_if_branch_upvalues_more` | `closure.lua` | Closures created in different conditional branches maintain independent per-branch upvalues. |
| `closure_break_upvalues` | `closure.lua` | Upvalues closed through break paths remain distinct from later locals. |
| `closure_multilevel_state_more` | `closure.lua` | Multi-level closures capture outer state and independent mutable locals across returned functions. |
| `closure_repeat_until_upvalues` | `closure.lua` | Closures created inside repeat-until style loops close per-iteration locals correctly. |
| `closure_tailcall_upvalue` | `closure.lua` | Upvalues captured by closures returned through a tail-call style vararg helper remain valid. |
| `constructs_loops_tables` | `constructs.lua` | Conditional chains, translated loops, table constructors. |
| `constructs_label_goto_control` | `goto.lua`, `constructs.lua` | Go-style `label:`/`goto label` forward, backward, loop-exit, and function-local control flow translated against equivalent Lua loops. |
| `constructs_loop_break_count` | `constructs.lua` | Nested loops, `break`, and table writes. |
| `constructs_loop_break_repeat_more` | `constructs.lua` | Nested counting loops plus repeat-until/break control-flow cases. |
| `constructs_recursive_multireturn_more` | `constructs.lua` | Recursive multi-return calls in table constructors, first-result adjustment before concatenation, and non-number fallthrough returns. |
| `constructs_function_branches_more` | `constructs.lua` | Function branch returns, nil fallthrough, and table constructor expression fields. |
| `constructs_if_expr_tables` | `constructs.lua` | If/elseif return chains and table constructors containing logical expression fields. |
| `constructs_multireturn_tables` | `constructs.lua` | Multi-return table constructor behavior for stable cases. |
| `constructs_precedence` | `constructs.lua` | Arithmetic, concat, logical precedence, unary/power precedence. |
| `constructs_short_circuit` | `constructs.lua` | `and`/`or` short-circuit value semantics and comparisons. |
| `coroutine_create_gofunction` | `coroutine.lua` | `coroutine.create`/`resume` over native functions, including errors and dead status. |
| `coroutine_multi_yield_resume_more` | `coroutine.lua` | Multi-value `coroutine.yield` propagation through `coroutine.resume`, including interior nil and resume arguments. |
| `coroutine_self_resume_status_more` | `coroutine.lua` | Coroutine resume/status progression across suspended yields, return, and repeated dead-coroutine resumes. |
| `coroutine_status_yieldable_more` | `coroutine.lua` | Coroutine status transitions around yield/resume plus `coroutine.isyieldable` inside and outside a coroutine. |
| `coroutine_wrap_basic` | `coroutine.lua` | `coroutine.wrap`, yield/resume values, simple generator. |
| `coroutine_wrap_sieve_more` | `coroutine.lua` | Coroutine-wrapped generator/filter pipeline for the prime sieve pattern. |
| `coroutine_yield_resume_values` | `coroutine.lua` | Resume arguments returned from `yield`, generator state. |
| `crypto_go_host_more` | `api.lua`, `strings.lua` | Go-host crypto helper equality, AES-GCM round trip, decrypt error returns, random byte/hex shape, and key generation validation. |
| `csv_go_host_more` | `files.lua`, `strings.lua` | Go-host CSV parse/encode helpers with headers, custom separator, quoting, trimming, and malformed input diagnostics. |
| `errors_assert_pcall` | `errors.lua` | `assert`, `error`, and `pcall` basics. |
| `errors_common_runtime_failures` | `errors.lua` | Protected common runtime failures: missing math arguments, failed asserts, nil arithmetic, and missing builtin args. |
| `errors_error_edge_values` | `errors.lua` | No-argument `error`, raw error level argument, protected runtime errors, and bad-argument propagation. |
| `errors_pcall_basic` | `errors.lua` | Protected calls around errors and successful builtins. |
| `errors_pcall_xpcall_values` | `errors.lua` | `pcall` multi-return/error-object propagation and `xpcall` handlers. |
| `errors_xpcall_args_more` | `errors.lua` | `xpcall` forwarding arguments to the protected function and handler return-value propagation. |
| `errors_xpcall_nested_error_more` | `errors.lua` | Nested `xpcall` recovery flow and table-valued message handler transformation. |
| `events_arith_compare` | `events.lua` | Arithmetic metamethod basics. |
| `events_call_levels` | `events.lua` | Several levels of `__call` delegation. |
| `events_compare_metamethods` | `events.lua` | VM comparison metamethod dispatch for `__lt`, `__le`, and `__eq`, including reversed `>`/`>=` forms. |
| `events_compare_ordering` | `events.lua` | Comparison metamethod return values over numeric/string table wrappers. |
| `events_compare_sets` | `events.lua` | Partial-order set comparison, raw equality bypass, and table-key lookup ignoring `__eq`. |
| `events_compare_compat_more` | `events.lua` | Comparison metamethod compatibility when related metatables share comparison functions. |
| `events_concat_chain_more` | `events.lua` | Chained `__concat` metamethod results that keep returning wrapped tables. |
| `events_concat_metamethod` | `events.lua` | `__concat` metamethod with strings, numbers, and tables. |
| `events_concat_numeric_operand_more` | `events.lua` | VM concat chains preserve right-associative `__concat` dispatch and pass adjacent numeric operands as numbers. |
| `events_dynamic_call_metatable` | `calls.lua`, `events.lua` | Dynamic metatable construction in loops and `__call` delegation preserving vararg table contents. |
| `events_eq_invalidate` | `events.lua` | Dynamic `__eq` removal and replacement on an existing metatable. |
| `events_eq_invalidate_more` | `events.lua` | Additional `__eq` invalidation and replacement checks across two metatable-sharing tables. |
| `events_index_chain` | `events.lua` | Chained `__index` fallback tables/functions. |
| `events_index_function_parent_more` | `events.lua` | Function-valued `__index` parent lookup, argument shape, and ordinary keys bypassing fallback. |
| `events_index_newindex_call` | `events.lua` | `__index`, `__newindex`, and `__call`. |
| `events_newindex_chain` | `events.lua` | `__newindex` table/function forwarding. |
| `events_newindex_existing` | `events.lua` | Existing-key writes bypass `__newindex`; missing-key writes trigger it. |
| `events_newindex_loop_more` | `events.lua` | `__newindex` delegation through a self-referential parent table to a grandparent handler. |
| `events_newindex_table_redirect_more` | `events.lua` | `__newindex` table redirection, matching `__index` reads and self-referential redirect tables. |
| `events_metatable_protection_more` | `events.lua` | Protected `__metatable` behavior for `getmetatable`/`setmetatable` plus normal set/get metatable returns. |
| `events_metatable_basic_more` | `events.lua` | Basic `getmetatable`, protected metatable, `__tostring`, and rejected protected `setmetatable`. |
| `events_partial_order_more` | `events.lua` | Partial-order set comparison through `__lt` and `__le` metamethods over copied key sets. |
| `events_rawget_rawset` | `events.lua` | `rawget`, `rawset`, `setmetatable`, `getmetatable`. |
| `events_rawget_more2` | `events.lua` | Extra-argument behavior for `rawset` and `rawget` while preserving the first value/key semantics. |
| `events_rawlen` | `events.lua` | `rawlen` for tables and strings, bypassing `__len`. |
| `events_rawlen_more2` | `events.lua` | Additional raw length checks for `__len` tables, strings, long strings, and invalid arguments. |
| `gc_collectgarbage_modes` | `gc.lua`, `gengc.lua` | `collectgarbage` incremental/generational mode switching and invalid legacy tuning options. |
| `gc_collectgarbage_more2` | `gc.lua` | Additional `collectgarbage` running-state, explicit collection, and mode transition return values. |
| `gc_collectgarbage_protocol` | `gc.lua`, `gengc.lua` | `collectgarbage` collect/count/step/stop/restart/isrunning protocol basics. |
| `gc_basic_more` | `gc.lua` | Basic `collectgarbage` running state, count, stop/restart, step result type, and explicit collection. |
| `gc_stats_defer_cleanup` | `gc.lua`, `tracegc.lua` | GScript Go-host GC diagnostics via `collectgarbage("stats")`; deterministic cleanup is covered by runtime `defer` tests and docs. |
| `literals_strings_basic` | `literals.lua`, `strings.lua` | Basic string literal behavior that maps to GScript escapes. |
| `literals_long_brackets_more` | `literals.lua` | Long-bracket string delimiter edge cases translated to equivalent literal values. |
| `literals_long_string_more` | `literals.lua` | Long string literal length and substring checks over a 960-byte literal. |
| `literals_control_escapes_more` | `literals.lua` | Control-character, decimal, NUL, and hexadecimal string escape semantics translated to equivalent byte values. |
| `locals_scope` | `locals.lua` | Nil locals, local shadowing, scope checks. |
| `locals_basic_more` | `locals.lua` | Local nil assignment/returns, multiple nil returns, and nested local shadowing checks. |
| `locals_repeat_shadow_more2` | `locals.lua` | Parameter nil assignment, nested local shadowing, branch-local bindings, and repeat-loop shadowed locals. |
| `log_go_host_more` | `api.lua` | Go-host log levels, formatting, deterministic output, history, count, clear, and level filtering. |
| `math_floor_power` | `math.lua` | Numeric literals, floor-equivalent checks, negative powers. |
| `math_floor_ceil_minmax` | `math.lua` | `floor`, `ceil`, `max`, and `min` stable cases. |
| `math_float_notation_coercion` | `math.lua` | Basic float notation and arithmetic coercion from numeric strings. |
| `math_float_int_order_edges` | `math.lua` | Float/integer ordering edges and representative large float equality/inequality checks. |
| `math_floor_ceil_large_more` | `math.lua` | Additional `floor`/`ceil` type-preserving checks and exact powers-of-two rounding boundaries. |
| `math_fmod_more2` | `math.lua` | `math.fmod` integer/float result-type consistency across small signed operands and same-bound integer cases. |
| `math_fmod_integer_more` | `math.lua` | `math.fmod` integer/float result-type behavior over signed operands and zero-divisor errors. |
| `math_implicit_conversion_more` | `math.lua` | Implicit arithmetic conversion for numeric strings across multiplication, addition, subtraction, division, and unary minus. |
| `math_lib_basic` | `math.lua` | Common math library functions and `math.type`. |
| `math_log_angle_more` | `math.lua` | Additional angle conversion, two-argument `atan`, logarithm base conversion, and trigonometric identity checks. |
| `math_modf_huge` | `math.lua` | `math.modf`, `math.huge`. |
| `math_modf_finite_edges` | `math.lua` | `math.modf` over positive/negative fractions and large finite integral floats. |
| `math_modf_inf_integer` | `math.lua` | `math.modf` over infinities and integer arguments, including returned numeric subtypes. |
| `math_mod_consistency_more` | `math.lua` | Integer and float modulo consistency over small signed ranges plus modulo at identical integer bounds. |
| `math_mod_large_precision_more` | `math.lua` | Additional modulo sign, subtype, fractional, and pi precision checks. |
| `math_modulo_lua_semantics` | `math.lua` | Lua modulo semantics for positive/negative integer and float operands. |
| `math_nan_compare_more` | `math.lua` | NaN order comparisons against integers and integer bounds always evaluate false. |
| `math_nan_zero_more` | `math.lua` | `-0.0`, signed reciprocals, infinities, and NaN self/cross comparisons. |
| `math_minmax_abs_round_more` | `math.lua` | Additional `math.min`/`max` varargs, `abs`, `floor`, and `ceil` signed rounding checks. |
| `math_minmax_more2` | `math.lua` | Additional `math.min`/`max` checks across integers, floats, extreme integer bounds, and very large floats. |
| `math_nan_inf_basic` | `math.lua` | NaN non-equality, infinity checks, `math.huge`, `math.type`, and large float rounding. |
| `math_order_coercion` | `math.lua` | Float/integer ordering and numeric-string arithmetic coercion. |
| `math_order_ops` | `math.lua` | Numeric and string order operators. |
| `math_numeric_strings_more` | `math.lua` | Numeric string coercion for arithmetic, exponentiation, modulo, whitespace, and hexadecimal strings. |
| `math_random_protocol` | `math.lua` | `math.random` ranges, raw integer mode, bad ranges, and `math.randomseed` state replay protocol. |
| `math_random_errors_more` | `math.lua` | Additional `math.random` argument-count, empty-interval, and bounded integer result checks. |
| `math_random_range_more` | `math.lua` | `math.random` float, single-bound, and two-bound result ranges and integer result types. |
| `math_random_small_intervals_more` | `math.lua` | `math.random` coverage for small signed intervals and singleton min/max integer intervals. |
| `math_random_unit_edges_more` | `math.lua` | `math.random` singleton and near-bound min/max integer interval checks. |
| `math_tointeger_basic` | `math.lua` | `math.tointeger` numeric conversion basics. |
| `math_tointeger_edges_more` | `math.lua` | Additional `math.tointeger` string, NaN, infinity, and `floor`/`ceil` infinity edge cases. |
| `math_tonumber_base` | `math.lua` | `tonumber` with bases 2..36 and invalid digit checks. |
| `math_tonumber_base_loop_more` | `math.lua` | `tonumber` base conversion over bases 2 through 36 and signed base-36 edge values. |
| `math_tonumber_base_more` | `math.lua` | Additional `tonumber` base conversion over bases 2..36, whitespace, signs, and alphanumeric digits. |
| `math_tonumber_basic` | `math.lua` | `tonumber` decimal parsing and invalid decimal strings. |
| `math_tonumber_decimal_edges` | `math.lua` | Additional decimal `tonumber` numeric inputs, signed fractional forms, and malformed decimals. |
| `math_tonumber_decimal_more2` | `math.lua` | More decimal `tonumber` inputs: numeric arguments, infinities, signed fractions, empty/malformed strings, and non-string rejection. |
| `math_tonumber_hex` | `math.lua` | `tonumber` hexadecimal integer/fraction/binary-exponent parsing and malformed hex rejection. |
| `math_tonumber_hex_more2` | `math.lua` | Additional signed hexadecimal float strings, binary exponent notation, and fractional hex values. |
| `math_tonumber_invalid_more` | `math.lua` | Additional invalid `tonumber` formats, invalid bases/digits, and rejected inf/nan strings. |
| `math_tonumber_invalid_more2` | `math.lua` | Wider invalid `tonumber` coverage across bases, infinities/NaNs, embedded spaces, and malformed decimals. |
| `math_tonumber_overflow_more` | `math.lua` | `tonumber` preserving integer bounds and converting very large decimal strings to floats. |
| `math_transcendentals_more` | `math.lua` | Additional `atan`, periodic `sin`, decimal `tonumber`, and medium-sized exact integer cancellation checks. |
| `math_transcendentals` | `math.lua` | Trigonometric, logarithmic, exponential, `sqrt`, and `fmod` basics. |
| `math_unsigned_compare` | `math.lua` | `math.ult` unsigned integer comparisons. |
| `nextvar_empty_numeric_for` | `nextvar.lua` | Empty generic/numeric loops and single-iteration positive/negative-step numeric loops. |
| `nextvar_empty_loops_more` | `nextvar.lua` | Empty `pairs` and numeric loop forms plus single-iteration positive and negative-step loops. |
| `nextvar_generic_for_multivalue_more` | `nextvar.lua` | Generic-for iterator state/control protocol with multi-return iterator payload prefix. |
| `nextvar_ipairs_index_metamethod` | `nextvar.lua` | `ipairs` iteration through ordinary indexing and `__index` fallback. |
| `nextvar_ipairs_next` | `nextvar.lua` | Table length growth, `ipairs`, `next`, boolean array values. |
| `nextvar_ipairs_false_more` | `nextvar.lua` | `ipairs` visits false values, skips hash-only tables, and exposes stable iterator function identity. |
| `nextvar_ipairs_protocol_more` | `nextvar.lua` | `ipairs` iterator protocol, hash-only tables producing no iterations, and stable iterator identity. |
| `nextvar_iterator_errors_more` | `nextvar.lua` | `next` iterator identity, invalid-key errors, missing-argument errors for `pairs`/`ipairs`, and empty table traversal. |
| `nextvar_hash_length_growth` | `nextvar.lua` | Table length growth after inserting and deleting many hash-part string keys. |
| `nextvar_length_nil` | `nextvar.lua` | Length with nil array slots, sparse deletion checks, `ipairs` false values. |
| `nextvar_length_more` | `nextvar.lua` | Additional table length checks for empty/nil constructors, negative keys, and 0..40 array fills. |
| `nextvar_next_pairs_copy` | `nextvar.lua` | `next` table traversal copied into another table, cross-checked with `pairs`. |
| `nextvar_next_function_identity_more` | `nextvar.lua` | `next` function identity and direct traversal over a small hash table. |
| `nextvar_next_tail_more` | `nextvar.lua` | `next` traversal after repeatedly moving the only live array key to the sparse tail. |
| `nextvar_next_sparse_tail` | `nextvar.lua` | `next` over a table repeatedly deleting previous array keys, leaving only the sparse tail. |
| `nextvar_numeric_for_float_counts` | `nextvar.lua` | Numeric `for` boundary counts over integer-looking floats, fractional limits, and negative steps. |
| `nextvar_numeric_for_more2` | `nextvar.lua` | Additional integer and float numeric-for boundary counts with positive and negative steps. |
| `nextvar_pairs_delete` | `nextvar.lua` | Deleting current keys while iterating mixed-key tables with `pairs`. |
| `nextvar_pairs_key_types_more` | `nextvar.lua` | `pairs`/`next` traversal over integer, float, string, function, boolean, and table keys plus deletion during traversal. |
| `nextvar_pairs_many_string_keys` | `nextvar.lua` | Large string-key table traversal with `pairs`. |
| `nextvar_pairs_metamethod` | `nextvar.lua` | `__pairs` metamethod protocol returning iterator, state, and control value. |
| `nextvar_pairs_protocol_more2` | `nextvar.lua` | Default `pairs` return triple and a `__pairs` iterator protocol with state/control values. |
| `nextvar_pairs_return_triple` | `nextvar.lua` | Default `pairs` returns iterator, state table, and nil control value. |
| `nextvar_pairs_string_keys` | `nextvar.lua` | `pairs` and `next` over file-mode string-key tables. |
| `nextvar_power_keys_more` | `nextvar.lua` | Table length behavior with powers-of-two integer keys up to large sparse indexes. |
| `nextvar_table_insert_general_more` | `nextvar.lua` | General `table.insert`/`remove` behavior over arrays with negative and metadata-like hash keys. |
| `nextvar_table_insert_remove_edges` | `nextvar.lua` | `table.insert`/`table.remove` boundary positions, non-sequence keys, and zero-key removal behavior. |
| `nextvar_table_insert_remove_proxy_metamethods` | `nextvar.lua`, `sort.lua` | `table.insert` and `table.remove` over proxy tables using `__len`, `__index`, and `__newindex`, including JIT official-check mode. |
| `nextvar_table_insert_string_more` | `nextvar.lua` | `table.insert` and `table.remove` over string sequences with explicit insertion positions and append. |
| `nextvar_table_maxn` | `nextvar.lua` | Deprecated `table.maxn` Lua implementation over numeric, string, and floating numeric keys. |
| `nextvar_table_length_nils_more` | `nextvar.lua` | Table length over nil-only constructors, trailing nils, incremental array fills, and sparse nil deletion. |
| `nextvar_table_remove_sequences` | `nextvar.lua` | `table.remove` over string sequences, `#t+1` no-op removal, and middle/tail deletion. |
| `nextvar_table_remove_more` | `nextvar.lua` | Additional `table.remove` edge cases for zero index storage, append/remove order, invalid positions, and middle deletion. |
| `os_go_host_env_expand_more` | `files.lua`, `main.lua` | Go-host environment get/set/unset and environment expansion helpers with argument diagnostics. |
| `os_time_date_host_more` | `main.lua`, `api.lua` | Go-host OS time/date/clock/process diagnostics with fixed timestamp formatting and shape checks. |
| `pm_captures_basic_more` | `pm.lua` | Additional captures, empty captures, anchored captures, trimming captures, and position-capture replacement in `gsub`. |
| `pm_escape_compat_more` | `pm.lua`, `strings.lua` | Hex/NUL pattern classes, negated class escapes, dot-newline matching, and standalone balanced-pattern matching/replacement. |
| `pm_find_empty_anchor` | `pm.lua` | Empty pattern finds and anchored pattern finds. |
| `pm_frontier_compat_more` | `pm.lua`, `strings.lua` | Byte-oriented `%f[...]` frontier assertions across find, match, gmatch, and gsub. |
| `pm_find_nul_more2` | `pm.lua` | Empty-pattern find, literal find in NUL-containing strings, and missing NUL pattern checks. |
| `pm_find_match_more_ascii` | `pm.lua` | Additional ASCII pattern find/match repetition, anchors, frontier-like punctuation, and negated classes. |
| `pm_find_nul_strings` | `pm.lua` | `string.find` over NUL-containing strings, start offsets, final-byte matches, and missing NUL suffixes. |
| `pm_gsub_captures` | `pm.lua` | `gsub` replacement captures, whole-match captures, and position captures. |
| `pm_gsub_capture_reorder_more` | `pm.lua` | Additional `gsub` replacement capture reordering, whole-match replacement, and replacement limits. |
| `pm_gsub_empty_match_more` | `pm.lua` | Empty-match `gsub` progression plus start/end anchors on empty subjects. |
| `pm_gsub_error_subset_more` | `pm.lua` | `gsub` replacement-string errors for invalid capture indexes and invalid percent escapes. |
| `pm_gsub_function_balanced_more` | `pm.lua` | Balanced `%b` patterns plus function-valued `gsub` replacements with nil-return no-substitution behavior. |
| `pm_gsub_replacement_more` | `pm.lua` | Additional `gsub` capture replacement, empty-subject anchors, empty-match replacement progression, and replacement limits. |
| `pm_gsub_table_fallback_more` | `pm.lua` | `gsub` table replacements with missing keys, false values, position captures, and fallback to original text. |
| `pm_gsub_table_replacement_more` | `pm.lua` | `gsub` table replacements by whole match/first capture, missing replacement fallback, and invalid replacement value errors. |
| `pm_gsub_trim` | `pm.lua` | Simple `gsub`, trimming, whitespace replacement, empty-string anchors. |
| `pm_gmatch_words_more` | `pm.lua` | `gmatch` iteration over word captures and multi-capture numeric assignments. |
| `pm_gmatch_numeric_pairs_more` | `pm.lua` | `gmatch` two-capture numeric assignments into a table followed by `pairs` verification. |
| `pm_pattern_runtime_more` | `pm.lua` | Pattern runtime callbacks: `gmatch` start positions and function-valued `gsub` replacements with nil/false no-substitution. |
| `rand_go_host_more` | `api.lua`, `math.lua` | Go-host deterministic seeded random replay, range/shape checks, collection helpers, UUID/bytes shape, and argument validation. |
| `process_go_host_entry_exit_more` | `main.lua` | Go-host process argument/entrypoint helpers and host-controlled process exit errors. |
| `process_exec_run_more` | `main.lua`, `api.lua` | Go-host process command helpers for lookup, run/exec/shell, stdin/env, stdout/stderr, exit codes, and process environment shape. |
| `pm_match_captures_ascii` | `pm.lua` | ASCII `string.match` captures with `%w`, `%d`, empty captures, and anchored failure cases. |
| `pm_match_classes_repetition_more` | `pm.lua` | Additional pattern repetition, anchors, minimal matches, negated classes, and `%S`/`%C` classes. |
| `pm_match_classes_more2` | `pm.lua` | Additional pattern class and repetition checks for `%l`, `%a`, `*`, `+`, escaped `$`, and missing matches. |
| `pm_match_repetition` | `pm.lua` | Greedy repetition pattern matches. |
| `pm_malformed_pattern_errors_more` | `pm.lua` | Malformed pattern inputs for unfinished captures and malformed character classes raise errors. |
| `pm_pattern_nul_magic_more` | `pm.lua` | NUL bytes inside patterns, character classes, escaped NUL matches, and magic characters after NUL. |
| `sort_custom_comparator` | `sort.lua` | VM-closure comparators for descending sort, empty arrays, equal elements, and string ordering checks. |
| `sort_invalid_order_function` | `sort.lua` | `table.sort` does not call comparators for empty ranges and rejects inconsistent order functions. |
| `sort_binary_string_order_more` | `sort.lua` | Default `table.sort` byte ordering over strings including an embedded-NUL prefix. |
| `sort_metatable_lt_more` | `sort.lua` | Default `table.sort` ordering through element `__lt` metamethods. |
| `sort_len_noninteger_more` | `sort.lua` | `__len` metamethods returning non-integer and negative values, including empty negative-length sort. |
| `sort_order` | `sort.lua` | Default `table.sort` over permutations. |
| `sort_permutation_more` | `sort.lua` | Additional recursive permutation sorting over 4- and 5-element arrays, including duplicate values. |
| `sort_reverse_closure_more` | `sort.lua` | Reverse `table.sort` comparator closure with mutable comparison counter. |
| `sort_table_insert_errors` | `sort.lua` | `table.insert` wrong-arity and bad-table argument errors. |
| `sort_pack_unpack` | `sort.lua` | `table.pack` nil/count behavior. |
| `sort_pack_nil_counts_more` | `sort.lua` | Additional `table.pack` count preservation across interior and trailing nil arguments. |
| `sort_table_insert_remove_concat` | `sort.lua` | `table.insert`, `table.remove`, `table.concat`. |
| `sort_table_move` | `sort.lua` | `table.move` forward/backward/overlapping/empty moves. |
| `sort_table_proxy_metamethods` | `sort.lua`, `events.lua` | `table.move`, `table.unpack`, and `table.sort` over proxy tables using `__index`, `__newindex`, and `__len`. |
| `sort_unpack_ranges` | `sort.lua` | `table.unpack` direct assignment, bounded ranges, singleton ranges, and empty ranges. |
| `sort_unpack_sparse_boundary_more` | `sort.lua` | Extreme sparse `table.unpack` ranges fail quickly at the host multi-return boundary instead of walking the whole range. |
| `strings_basic` | `strings.lua` | String compare, sub/find/len/byte/char/case/rep/reverse basics. |
| `strings_byte_ascii_indices` | `strings.lua` | ASCII `string.byte` positive/negative range indices and empty-range nil behavior. |
| `strings_byte_char_edges` | `strings.lua` | `string.byte` and `string.char` edge checks over bounded byte ranges and invalid character values. |
| `strings_byte_char_more2` | `strings.lua` | Additional `string.byte` index ranges, nil results, newline bytes, `string.char(255)`, and empty `string.char`. |
| `strings_char_ascii_more` | `strings.lua` | Additional ASCII `string.char` formatting and round-trip byte checks. |
| `strings_compare_binary_more` | `strings.lua` | Binary-safe string comparison ordering, including embedded NUL bytes and prefix ordering. |
| `strings_format_flags_extended` | `strings.lua` | Extended `string.format` flags, precision, alternate forms, and empty precision output. |
| `strings_format_error_subset_more` | `strings.lua` | Stable `string.format` error paths for invalid conversions and missing values. |
| `strings_format_exponent_more` | `strings.lua` | Additional exponent-format width/sign patterns and empty string precision behavior. |
| `strings_format_flags_more` | `strings.lua` | Additional `string.format` integer flags, alternate hex forms, sign/space flags, and string precision/width cases. |
| `strings_format_iso_flags_more` | `strings.lua` | Additional ISO-C-required `string.format` flags for octal/hex alternate forms, integer precision, `%c`, `%G`, and empty string precision. |
| `strings_format_large32_subset_more` | `strings.lua` | 32-bit boundary numeric formatting for hex, decimal, unsigned, and octal conversions. |
| `strings_format_long_precision` | `strings.lua` | Long `%s` precision truncation, wide left padding, and NUL-framed formatted strings. |
| `strings_format_long_number_more` | `strings.lua` | Long fixed-point formatting for a large finite float and round-trip numeric parse. |
| `strings_format_numeric_flags_more` | `strings.lua` | Additional `string.format` numeric and character flags: alternate forms, padding, precision, literal percent, and empty precision strings. |
| `strings_format_numbers_flags` | `strings.lua` | Numeric/string `string.format` conversions and supported flags. |
| `strings_format_pointer` | `strings.lua` | `string.format("%p")` null handling, pointer-like values, width, and left alignment. |
| `strings_format_quote_literals` | `strings.lua` | `string.format("%q")` literal forms for NUL strings, booleans, nil, integers, infinities, NaN, and unsupported values. |
| `strings_format_string_values` | `strings.lua` | `string.format("%s")` over nil/booleans, precision truncation, padding, and NUL-containing strings. |
| `strings_format_tostring` | `strings.lua` | `tostring` and simple `string.format`. |
| `strings_format_tostring_more2` | `strings.lua` | `%s` formatting for nil/booleans with width and precision truncation. |
| `strings_find_plain_more` | `strings.lua` | Plain `string.find` with negative starts, embedded NUL lookup, literal pattern characters, and returned ranges. |
| `strings_find_empty_more` | `strings.lua` | Additional `string.find` empty-pattern, literal-pattern, and start-position edge cases. |
| `strings_gmatch_coroutine` | `strings.lua` | `string.gmatch` iterator wrapped in a coroutine. |
| `strings_gmatch_coroutine_more2` | `strings.lua` | `string.gmatch` iterator state after one direct call and one coroutine-wrapped resume. |
| `strings_patterns_basic` | `strings.lua` | `match`, `find`, `gsub`, `gmatch` basics. |
| `strings_pointer_format_more` | `strings.lua` | Additional `%p` formatting checks for null-like primitives, table pointers, width, and alignment. |
| `strings_rep_reverse_tostring` | `strings.lua` | `string.rep` separators, binary-safe `reverse`, repeated length checks, and `tostring` basics. |
| `strings_rep_tostring_ascii_more` | `strings.lua` | Additional ASCII `upper`/`lower`, `rep` separator, empty reverse, repeated lengths, and primitive `tostring` checks. |
| `strings_sub_find_len_ascii_more` | `strings.lua` | Additional ASCII `string.sub`, `string.find`, empty-find, and length boundary checks. |
| `strings_string_pack_go_style` | `tpack.lua` | GScript string namespace binary pack/unpack/packsize compatibility using Go-style formats. |
| `strings_string_pack_more` | `tpack.lua` | Additional pack/unpack integer, fixed-byte, endian, offset, and fixed-size coverage translated to GScript's Go-style binary formats. |
| `strings_sub_boundary_more` | `strings.lua` | `string.sub` omitted end, empty ranges, negative indices, and `math.mininteger`/`maxinteger` boundaries. |
| `strings_table_concat_binary` | `strings.lua` | `table.concat` with NUL-containing strings and long repeated ranges. |
| `strings_table_concat_empty_errors_more` | `strings.lua` | Additional `table.concat` empty ranges and bad argument/value errors. |
| `strings_table_concat_errors` | `strings.lua` | `table.concat` argument/type errors and numeric element conversion. |
| `strings_table_concat_errors_more2` | `strings.lua` | Additional `table.concat` bad-table and invalid element error paths. |
| `strings_table_concat_long_ranges_more` | `strings.lua` | Long-array `table.concat` joins and additional bounded-range edge cases. |
| `strings_table_concat_more2` | `strings.lua` | Additional `table.concat` empty tables, long joined arrays, singleton/range slices, and out-of-range empty slices. |
| `strings_table_concat_ranges_more3` | `strings.lua` | Additional `table.concat` range, empty-slice, and medium repeated join behavior. |
| `strings_table_concat_ranges` | `strings.lua` | `table.concat` ranges and separators. |
| `strings_transform_basic` | `strings.lua` | `sub`, `lower`, `upper`, `reverse`, `rep`. |
| `strings_tostring_metamethod` | `strings.lua` | `tostring` dispatch through `__tostring`, fallback `__name` prefixing, and invalid metamethod return errors. |
| `utf8_basic` | `utf8.lua` | Basic `utf8.len`, `char`, `codepoint`, `offset`. |
| `utf8_boundary_chars_more` | `utf8.lua` | UTF-8 boundary codepoints, surrogate-adjacent values, and invalid `utf8.char` ranges. |
| `utf8_char_codes` | `utf8.lua` | `utf8.char` and simple codepoint checks. |
| `utf8_char_range_errors` | `utf8.lua` | `utf8.char` empty/ASCII construction and out-of-range codepoint errors. |
| `utf8_complex_string_more` | `utf8.lua` | UTF-8 offset and codepoint checks over a mixed Japanese/Latin/accented string. |
| `utf8_codepoint_bounds` | `utf8.lua` | `utf8.codepoint` multibyte ranges, empty ranges, out-of-bounds errors, and max valid codepoint. |
| `utf8_codes_empty_iterator` | `utf8.lua` | Direct calls to an empty `utf8.codes` iterator with unusual control values return nil. |
| `utf8_codes_iterator` | `utf8.lua` | `utf8.codes` iterator, byte positions, and `utf8.offset(..., 0)` current-character behavior. |
| `utf8_invalid_sequences_more` | `utf8.lua` | Invalid UTF-8 continuation, overlong, surrogate, and out-of-range sequences across len/codepoint/codes/offset. |
| `utf8_multibyte_offsets` | `utf8.lua` | Multibyte UTF-8 length, offsets, codepoints. |
| `utf8_len_range_more` | `utf8.lua` | UTF-8 length, offsets, negative offsets, and codepoint ranges over a mixed multibyte/NUL string. |
| `utf8_offset_len_errors` | `utf8.lua` | `utf8.offset` and indexed `utf8.len` position bounds and continuation-byte errors. |
| `utf8_validation_helpers_more` | `utf8.lua` | Go-style structured UTF-8 validation diagnostics and non-strict sanitization helpers over invalid edge sequences. |
| `uuid_go_host_more` | `api.lua`, `strings.lua` | Go-host UUID nil value, validation, parsing, version/variant metadata, and generated UUID shape checks. |
| `vec_color_go_host_more` | `api.lua`, `math.lua` | Go-host vector and color helper constructors, predicates, arithmetic helpers, conversion, and invalid input diagnostics. |
| `vec_color_geometry_hsl_more` | `api.lua`, `math.lua` | Go-host vector geometry helpers and color HSL/HSV/lighten/darken/grayscale/mix/alpha transforms. |
| `vararg_forwarding` | `vararg.lua` | Vararg capture and simple forwarding. |
| `vararg_call_unpack_more` | `vararg.lua` | `table.unpack`-driven call forwarding, explicit nil-count handling, fixed-parameter adjustment, and vararg builtin dispatch. |
| `vararg_method_recursive_more` | `vararg.lua` | Method-call vararg indexing plus recursive vararg forwarding through nested one-less helpers. |
| `vararg_pack` | `calls.lua`, `vararg.lua` | Vararg count, `table.pack`, forwarding. |
| `vararg_select` | `vararg.lua` | Positive-index `select` and protected out-of-range calls. |
| `vararg_tail_missing_args` | `vararg.lua` | Tail-call forwarding with missing arguments preserves nil and later returned values. |

Recent audit-added coverage:

| Case | Official source area | Notes |
|---|---|---|
| `code_explicit_spread_more` | `code.lua`, `db.lua` | GScript explicit `spread(expr)` and `table.spread` expansion in call arguments and table constructors. |
| `api_arith_metamethod_chain_more` | `api.lua`, `events.lua` | Arithmetic metamethod chaining for `__add`, `__mod`, and unary minus over wrapped table values. |
| `big_generated_eval_env_more` | `big.lua` | Generated chunk/table execution with explicit environment mutation and large-enough array indexing. |
| `binary_namespace_more` | `tpack.lua` | GScript `binary` namespace pack/unpack/size with Go-style endian and field tokens. |
| `bytes_hash_base64_go_host_more` | `api.lua`, `strings.lua` | Go-host `bytes`, `hash`, and `base64` helpers for buffers, encodings, checksums, HMAC, and error returns. |
| `bytes_numeric_buffer_more` | `api.lua`, `strings.lua` | Go-host bytes buffer numeric little-endian writes, byte/string reads, hex round trips, concat, and reset behavior. |
| `db_gscript_diagnostics_more` | `db.lua` | GScript diagnostic helpers for function metadata and value inspection in VM-translated file mode. |
| `debug_host_helpers_more` | `db.lua` | Go-host `debug.traceback`, `debug.stack`, `debug.globals`, `debug.info`, `debug.value`, and `debug.goStack` diagnostics. |
| `db_vm_debug_parity_more` | `db.lua` | VM file-mode `debug.stack`, numeric `debug.info(level)`, source metadata, and hook/sink event observability. |
| `files_file_lines_streams_more` | `files.lua` | File-handle `lines()` iterator, standard stream handles, and open/closed `io.type` results. |
| `files_io_read_formats_more` | `files.lua` | File `read` line-with-newline format, byte-count reads, zero-byte EOF probe, partial EOF reads, and ordered multi-format returns. |
| `files_seek_overwrite_more` | `files.lua` | File-handle `seek` position reporting and overwrite semantics followed by whole-file readback. |
| `files_tmpfile_flush_type_more` | `files.lua` | `io.tmpfile`, `file:flush`, seek-to-start readback, and closed-file `io.type` reporting. |
| `fs_path_go_host_more` | `files.lua`, `main.lua` | Go-host `fs` and `path` helpers for temp files, directory creation, path operations, copy/rename/read/write, and error returns. |
| `fs_path_glob_cwd_more` | `files.lua`, `main.lua` | Go-host current-directory controls, globbing, temp dirs, and absolute/relative path helpers with cwd restoration. |
| `goto_simple_paths_more` | `goto.lua` | Direct label/goto forward and backward paths plus function-local label chains translated to GScript label syntax. |
| `go_channel_host_more` | `attrib.lua`, `api.lua` | Go-style channels and goroutines: buffered production, range over close, nil receive after close, and capacity/close error paths. |
| `heavy_generated_concat_more` | `heavy.lua` | Bounded generated string-concatenation chunk mirroring the official heavy generated-program pressure pattern. |
| `http_background_server_more` | `api.lua`, `main.lua` | Go-host HTTP server/router background mode returns closeable handles with addr/url and supports local client round-trips. |
| `json_go_host_more` | `api.lua`, `strings.lua` | Go-host `json` encode/decode/pretty round trips, nested values, invalid JSON, and trailing-data rejection. |
| `main_generated_chunk_eval_more` | `main.lua`, `code.lua` | Generated chunk compilation with explicit lexical environment and protected syntax-error handling. |
| `main_script_file_vm_more` | `main.lua`, `code.lua` | VM-aware `script.eval`/`compile`/`loadFile`/`runFile` with current globals, relative file loading, env sync, and sandbox options. |
| `main_vm_loader_more` | `main.lua`, `attrib.lua` | VM-aware global `load`/`loadfile`/`dofile` and file-backed `require` share current globals, relative script directory, and `package.loaded` cache. |
| `matrix_host_dense_more` | `api.lua`, `attrib.lua` | Go-host `matrix.dense` plus `matrix.getf`/`setf` flat access and stable argument/index error paths. |
| `net_http_background_roundtrip_more` | `api.lua`, `main.lua` | Go-host `net` client helpers against local background HTTP server, including JSON response parsing and error returns. |
| `net_http_methods_more` | `api.lua`, `main.lua` | Go-host loopback HTTP methods for PUT/PATCH/DELETE and configurable `net.request` headers, body, timeout, and redirect behavior. |
| `regexp_go_host_more` | `pm.lua`, `strings.lua` | Go RE2 regexp helpers, compiled objects, submatches, split/replace, and invalid-pattern errors. |
| `regexp_submatch_limits_more` | `pm.lua`, `strings.lua` | Go RE2 all-submatch helpers, compiled regexp limits, split/find limits, and subexpression counts. |
| `time_compress_encoding_go_host_more` | `main.lua`, `strings.lua` | Deterministic Go-host `time`, `compress`, and `encoding` helpers over fixed timestamps, round trips, and decode errors. |
| `compress_error_levels_more` | `main.lua`, `strings.lua` | Go-host gzip/zlib/deflate explicit compression levels, fallback levels, bad-input errors, and missing-argument diagnostics. |
| `encoding_ini_xml_roundtrip_more` | `strings.lua`, `main.lua` | Go-host INI encode/decode round trips, XML escape/unescape numeric entities, and malformed base32 decode errors. |
| `url_go_host_more` | `main.lua`, `strings.lua` | Go-host URL parse/build/query/escape/join helpers and invalid escape handling. |
| `main_script_process_more` | `main.lua`, `code.lua` | GScript `script.eval`/`script.compile` environment options and host-controlled `process.args`/`process.entry`. |
| `strings_go_helpers_more` | `strings.lua` | GScript Go-host string helpers: split, trim variants, replaceAll, join, title, padding, and numeric detection. |
| `table_go_helpers_more` | `sort.lua` | GScript Go-host table helpers: keys, values, contains, indexOf, copy, merge, count, unique, reverse, slice, and zip. |
| `table_higher_order_vm_callbacks_more` | `sort.lua` | GScript table higher-order helpers `map`, `filter`, `reduce`, and `fromArray` with VM script callbacks. |
| `table_proxy_concat_flatten_more` | `sort.lua`, `events.lua` | `table.concat` over proxy tables and `table.flatten` over inline nested table literals. |
| `tracegc_stats_progress_more` | `tracegc.lua`, `gc.lua` | Go-host GC stats shape and explicit collection progress compared with Lua `collectgarbage` count/running observability. |
| `utf8_go_helpers_more` | `utf8.lua` | GScript Go-host UTF-8 helpers: reverse, codepoint substring, Unicode case conversion, char classes, validate, and sanitize. |
| `verybig_method_constants_more` | `verybig.lua` | Large constant table access with method calls, self chaining, and closure reads beyond the RK-style boundary. |
| `all_harness_flags_format` | `all.lua` | Harness option defaults, message suppression, and compact count formatting. |
| `api_metamethod_compare_len_concat_more` | `api.lua` | Comparison, length, and concat metamethod dispatch. |
| `api_raw_ops_more` | `api.lua` | Raw table and metatable operations around `rawget`, `rawset`, `getmetatable`, and `setmetatable`. |
| `api_table_self_keys_more` | `api.lua` | Tables used as self-referential keys and values. |
| `attrib_scope_shadow_more` | `attrib.lua` | Lexical shadowing and closure capture of mutable locals. |
| `attrib_unpack_assignment_more` | `attrib.lua` | Multi-result unpack adjustment in assignment targets. |
| `big_table_growth_more` | `big.lua` | Array growth, concatenation, length, and high-index table access. |
| `big_table_sparse_pressure_more2` | `big.lua` | Sparse table deletion pressure and remaining indexed values. |
| `bwcoercion_bit32_numeric_strings` | `bwcoercion.lua` | Integer string coercion for bitwise operations and invalid numeric strings. |
| `bwcoercion_bit32_string_edges_more2` | `bwcoercion.lua` | Whitespace, hex, negative, float-like, and 32-bit wrap string inputs for bitwise coercion. |
| `calls_closure_parameter_capture_more` | `calls.lua` | Closure capture of function parameters and arity mismatch handling. |
| `calls_multiline_adjust_more` | `calls.lua` | Multiline function calls and return-value adjustment. |
| `calls_tail_missing_matrix_more` | `calls.lua` | Tail-return propagation with missing trailing values. |
| `closure_loop_mutation_more2` | `closure.lua` | Per-iteration closure state and nested mutable upvalues. |
| `closure_shared_sibling_upvalues_more` | `closure.lua` | Sibling closures sharing and shadowing captured locals. |
| `code_arithmetic_constants_more` | `code.lua` | Constant arithmetic boundaries and unary/operator precedence. |
| `code_comparison_immediates_more` | `code.lua` | Comparisons against numeric and string immediates in branches. |
| `code_constant_branch_matrix_more2` | `code.lua` | Branch selection across constant comparison combinations. |
| `code_string_constant_closure_more` | `code.lua` | Long string constants captured through nested closures. |
| `constructs_boolean_paths_more` | `constructs.lua` | Boolean short-circuit paths in expressions, `if`, and `while`. |
| `constructs_silly_loop_scope_more` | `constructs.lua` | Constant-condition loops and local shadowing inside table constructors. |
| `coroutine_recursive_yield_more` | `coroutine.lua` | Yielding through recursive coroutine calls. |
| `coroutine_tail_yield_more` | `coroutine.lua` | Yielded values and resumed arguments through wrapped coroutine loops. |
| `cstack_pattern_complexity_more` | `cstack.lua` | Pattern matching stack use with repeated optional captures. |
| `cstack_pattern_recursion_small` | `cstack.lua` | Small recursion-pressure variant for pattern matching stack behavior. |
| `db_nested_call_flow_more` | `db.lua` | Nested call evaluation, field assignment results, and closure state flow. |
| `db_pcall_nested_errors` | `db.lua` | Nested `pcall` behavior and propagated arithmetic error messages. |
| `db_upvalue_closure_flow` | `db.lua` | Closure/upvalue mutation flow across sibling functions and independent returned counters. |
| `db_vararg_transfer_values` | `db.lua`, `vararg.lua` | Vararg transfer, `table.unpack`, `table.pack`, and nil-preserving vararg counts. |
| `errors_assert_messages_more` | `errors.lua` | `assert` message selection, missing-argument failure, and successful multi-return passthrough. |
| `errors_call_index_failures_more` | `errors.lua` | Protected runtime failures for nil calls, missing method calls, bad method indexing, and nil arithmetic. |
| `errors_non_string_messages_deeper` | `errors.lua` | Non-string error objects through `pcall` and `xpcall` handler transformation. |
| `errors_non_string_messages_more` | `errors.lua` | Table and nil error messages plus `assert` failures with string, table, and nil messages. |
| `errors_runtime_failures_more` | `errors.lua` | Protected concat, invalid `collectgarbage`, illegal yield, and bad method-self failures. |
| `errors_runtime_messages_more` | `errors.lua` | Protected runtime errors for arithmetic, calls, length, and invalid comparisons. |
| `errors_xpcall_handler_more` | `errors.lua` | `xpcall` handler results for string, nested error, and table-valued error objects. |
| `events_newindex_self_metatable_more` | `events.lua` | `__newindex` function dispatch through a self-metatable parent and raw assignment side effects. |
| `files_append_read_all_more` | `files.lua` | File append, read-all, `io.lines`, close, and remove behavior over a temporary file. |
| `files_io_read_numbers` | `files.lua` | Numeric `file:read` formats parse integers/floats and stop cleanly at non-numeric input. |
| `files_io_write_read_lines` | `files.lua` | File write/read line formats and `io.lines` count behavior including a final unterminated line. |
| `files_tmpname_remove_rename` | `files.lua` | `os.tmpname`, `os.rename`, and `os.remove` success/failure return protocol. |
| `gc_collectgarbage_stop_step_more` | `gc.lua` | `collectgarbage` stop, step, restart, and `isrunning` state preservation. |
| `gc_generational_table_barrier_slice` | `gc.lua`, `gengc.lua` | Generational mode table barrier behavior across collection steps and mode restoration. |
| `gengc_running_mode_restore_more` | `gengc.lua` | Generational GC mode switching keeps the collector running across explicit steps and restore. |
| `goto_flow_equivalent_more` | `goto.lua` | Forward/backward label flow through blocks and functions with repeated label paths. |
| `goto_if_branch_equivalent_more` | `goto.lua` | Conditional branches jumping to shared labels, unreachable code, and branch-specific returns. |
| `heavy_concat_pressure_more2` | `heavy.lua`, `strings.lua` | Moderate string/table concatenation pressure with length and substring invariants. |
| `heavy_string_growth_small` | `heavy.lua`, `strings.lua` | Repeated string growth and table concatenation over small expanding strings. |
| `literals_line_comment_strings_more` | `literals.lua` | Line comments and escaped newline/carriage-return string literal values. |
| `literals_string_table_more2` | `literals.lua` | String literals inside table constructors, including empty strings, newlines, brackets, and equals signs. |
| `locals_block_assignment_scope_more` | `locals.lua` | Block-local shadowing and assignment scope behavior inside nested blocks and loops. |
| `locals_many_shadow_slots_more2` | `locals.lua` | Multiple local shadow slots across loop blocks and parameter shadowing with nil defaults. |
| `locals_shadowing_repeat_more` | `locals.lua` | Local shadowing across branches, function names, and repeat-until assignments. |
| `main_multiline_chunk_values` | `main.lua`, `literals.lua` | Multiline chunk parsing with long strings, escaped newlines, and multi-return values. |
| `main_print_write_more` | `main.lua`, `files.lua` | Top-level `io.write` and `print` output plus function availability checks. |
| `math_negative_powers_more` | `math.lua` | Negative exponent and reciprocal power identities over small signed integer bases. |
| `math_tonumber_long_decimal_more` | `math.lua` | Long decimal `tonumber` parsing, malformed decimal rejection, and decimal difference consistency. |
| `nextvar_checknext_more` | `nextvar.lua` | `next` traversal copies mixed array/hash tables and cross-checks against `pairs`. |
| `nextvar_numeric_for_fractional_more` | `nextvar.lua` | Fractional numeric `for` loops count iterations and final values for positive and negative steps. |
| `nextvar_pairs_delete_strings_more` | `nextvar.lua` | Deleting string keys during `pairs` traversal visits all entries and leaves the table empty. |
| `pm_class_sets_ascii_more` | `pm.lua` | ASCII pattern class sets and negated `%d`, `%s`, and `%w` classes. |
| `pm_gsub_limit_count_more` | `pm.lua` | `string.gsub` replacement limits, zero-limit behavior, no-match counts, and repeated-pattern counts. |
| `pm_match_capture_suffix_more` | `pm.lua` | Captures before suffixes, empty captures, anchored failures, and alpha/digit capture pairs. |
| `pm_match_minimal_more` | `pm.lua` | Minimal and greedy pattern matching across anchors, classes, optional matches, and negated classes. |
| `sort_equal_false_more` | `sort.lua` | `table.sort` with comparator returning nil preserves false elements and handles empty arrays. |
| `sort_invalid_order_more2` | `sort.lua` | Inconsistent `table.sort` order functions raise invalid-order errors across small array sizes. |
| `sort_month_names_more` | `sort.lua` | Default lexical `table.sort` ordering for month-name strings. |
| `sort_unpack_assignment_more` | `sort.lua` | `table.unpack` assignment over bounded, empty, and sparse singleton ranges. |
| `strings_byte_range_multi_more` | `strings.lua` | `string.byte` multi-return ranges with positive, negative, and empty index windows. |
| `strings_char_errors_more` | `strings.lua` | `string.char` empty call and out-of-byte-range argument errors. |
| `strings_find_offsets_more` | `strings.lua` | `string.find` start offsets, empty-pattern edge cases, pattern matching, and plain-find mode. |
| `strings_format_percent_char_more` | `strings.lua` | `string.format` literal percent, `%c`, width flags, and combined percent/integer formatting. |
| `strings_rep_separator_more` | `strings.lua` | `string.rep` separator behavior for zero, one, empty-string, and repeated-string counts. |
| `strings_reverse_ascii_more` | `strings.lua` | ASCII `string.reverse` over empty, ordinary, numeric, and repeated strings. |
| `strings_sub_method_more` | `strings.lua` | String method-call syntax for `sub`, `upper`, `lower`, and separator `rep`. |
| `tpack_pack_count_more` | `tpack.lua` | `table.pack` preserves `n` and nil positions through direct and vararg calls. |
| `tpack_select_more` | `tpack.lua` | `select` count, positive indexes, negative indexes, and multi-value tail selection. |
| `tpack_vararg_edges_more` | `tpack.lua` | Vararg packing, selecting, and unpacking preserve nil, false, and tail values. |
| `utf8_supplementary_more` | `utf8.lua` | Supplementary-plane UTF-8 codepoints checked through `len`, `offset`, `codepoint`, and `codes`. |
| `vararg_argument_adjust_more` | `vararg.lua` | Vararg argument adjustment preserves nils and validates fixed-result comparison behavior. |
| `vararg_long_parameter_list_more` | `vararg.lua` | Long fixed parameter lists receive nil defaults and collect overflow arguments in varargs. |
| `verybig_many_constants_more2` | `verybig.lua` | Large constant tables exercise indexed constants beyond common bytecode constant thresholds. |
| `verybig_rk_constants_more` | `verybig.lua` | Large constant access combines RK-style numeric constants, method calls, and closure reads. |

Next recommended conversion order:

1. Continue broad `api.lua` translations using `testkit`, raw operations, protected calls, and existing debug/runtime diagnostics.
2. Keep recent audit-added coverage as a short change log and fold it into the main alphabetical table during larger manifest maintenance.
3. Continue compatibility slices in `strings.lua`, `events.lua`, and `math.lua` where existing Go-style APIs provide feature-equivalent coverage.
