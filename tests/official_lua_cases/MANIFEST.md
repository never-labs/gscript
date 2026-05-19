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

Current translated passing cases: 327.

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
| `calls_anonymous_invocation` | `calls.lua` | Immediately invoked anonymous functions and simple multi-return anonymous closures. |
| `calls_builtin_missing_args` | `calls.lua` | Missing-argument errors for core builtins through protected calls. |
| `calls_fixedpoint_returns` | `calls.lua` | Fixed-point function calls, closure-returning calls, recursive multi-return unpack. |
| `calls_incorrect_args_more` | `calls.lua` | Extra arguments ignored by fixed-arity builtins/calls and nil parameter adjustment behavior. |
| `calls_long_method_name_more` | `calls.lua` | Long method-name definition and repeated self-style invocation. |
| `calls_method_recursion` | `calls.lua` | Local recursion, method calls, chained method calls. |
| `calls_multireturn` | `calls.lua` | Missing argument nils, fixed multi-return assignment, tail recursion. |
| `calls_nested_method_more` | `calls.lua` | Nested table function assignment and explicit self-style method mutation. |
| `calls_recursion_error_more` | `calls.lua` | Recursive protected error propagation and moderate tail-recursive descent. |
| `calls_return_values` | `calls.lua`, `constructs.lua` | Multiple return assignment and nil-return basics. |
| `calls_tail_varargs` | `calls.lua` | Empty vararg forwarding through a tail call. |
| `calls_tail_varargs_more2` | `calls.lua` | Tail vararg forwarding into a sink function for empty, one-argument, and two-argument calls. |
| `calls_type_basic` | `calls.lua` | `type` over core values and builtins. |
| `calls_call_chain` | `calls.lua` | Chained `__call` metatables ending in `table.pack`. |
| `calls_extra_builtin_args_more` | `calls.lua` | Extra arguments to raw/table/math builtins are ignored where Lua specifies fixed-argument behavior. |
| `closure_upvalues` | `closure.lua` | Closure capture and independent upvalues. |
| `closure_for_control` | `closure.lua` | Independent closure captures for loop-created locals. |
| `closure_identity_more` | `closure.lua` | Distinct closures created in loops and stable identity when returning the same captured closure. |
| `closure_if_branch_upvalues_more` | `closure.lua` | Closures created in different conditional branches maintain independent per-branch upvalues. |
| `closure_break_upvalues` | `closure.lua` | Upvalues closed through break paths remain distinct from later locals. |
| `closure_repeat_until_upvalues` | `closure.lua` | Closures created inside repeat-until style loops close per-iteration locals correctly. |
| `closure_tailcall_upvalue` | `closure.lua` | Upvalues captured by closures returned through a tail-call style vararg helper remain valid. |
| `constructs_loops_tables` | `constructs.lua` | Conditional chains, translated loops, table constructors. |
| `constructs_loop_break_count` | `constructs.lua` | Nested loops, `break`, and table writes. |
| `constructs_loop_break_repeat_more` | `constructs.lua` | Nested counting loops plus repeat-until/break control-flow cases. |
| `constructs_function_branches_more` | `constructs.lua` | Function branch returns, nil fallthrough, and table constructor expression fields. |
| `constructs_if_expr_tables` | `constructs.lua` | If/elseif return chains and table constructors containing logical expression fields. |
| `constructs_multireturn_tables` | `constructs.lua` | Multi-return table constructor behavior for stable cases. |
| `constructs_precedence` | `constructs.lua` | Arithmetic, concat, logical precedence, unary/power precedence. |
| `constructs_short_circuit` | `constructs.lua` | `and`/`or` short-circuit value semantics and comparisons. |
| `coroutine_create_gofunction` | `coroutine.lua` | `coroutine.create`/`resume` over native functions, including errors and dead status. |
| `coroutine_wrap_basic` | `coroutine.lua` | `coroutine.wrap`, yield/resume values, simple generator. |
| `coroutine_yield_resume_values` | `coroutine.lua` | Resume arguments returned from `yield`, generator state. |
| `errors_assert_pcall` | `errors.lua` | `assert`, `error`, and `pcall` basics. |
| `errors_common_runtime_failures` | `errors.lua` | Protected common runtime failures: missing math arguments, failed asserts, nil arithmetic, and missing builtin args. |
| `errors_error_edge_values` | `errors.lua` | No-argument `error`, raw error level argument, protected runtime errors, and bad-argument propagation. |
| `errors_pcall_basic` | `errors.lua` | Protected calls around errors and successful builtins. |
| `errors_pcall_xpcall_values` | `errors.lua` | `pcall` multi-return/error-object propagation and `xpcall` handlers. |
| `errors_xpcall_args_more` | `errors.lua` | `xpcall` forwarding arguments to the protected function and handler return-value propagation. |
| `events_arith_compare` | `events.lua` | Arithmetic metamethod basics. |
| `events_call_levels` | `events.lua` | Several levels of `__call` delegation. |
| `events_compare_metamethods` | `events.lua` | VM comparison metamethod dispatch for `__lt`, `__le`, and `__eq`, including reversed `>`/`>=` forms. |
| `events_compare_ordering` | `events.lua` | Comparison metamethod return values over numeric/string table wrappers. |
| `events_compare_sets` | `events.lua` | Partial-order set comparison, raw equality bypass, and table-key lookup ignoring `__eq`. |
| `events_compare_compat_more` | `events.lua` | Comparison metamethod compatibility when related metatables share comparison functions. |
| `events_concat_chain_more` | `events.lua` | Chained `__concat` metamethod results that keep returning wrapped tables. |
| `events_concat_metamethod` | `events.lua` | `__concat` metamethod with strings, numbers, and tables. |
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
| `literals_strings_basic` | `literals.lua`, `strings.lua` | Basic string literal behavior that maps to GScript escapes. |
| `literals_long_brackets_more` | `literals.lua` | Long-bracket string delimiter edge cases translated to equivalent literal values. |
| `literals_long_string_more` | `literals.lua` | Long string literal length and substring checks over a 960-byte literal. |
| `locals_scope` | `locals.lua` | Nil locals, local shadowing, scope checks. |
| `locals_basic_more` | `locals.lua` | Local nil assignment/returns, multiple nil returns, and nested local shadowing checks. |
| `math_floor_power` | `math.lua` | Numeric literals, floor-equivalent checks, negative powers. |
| `math_floor_ceil_minmax` | `math.lua` | `floor`, `ceil`, `max`, and `min` stable cases. |
| `math_float_notation_coercion` | `math.lua` | Basic float notation and arithmetic coercion from numeric strings. |
| `math_float_int_order_edges` | `math.lua` | Float/integer ordering edges and representative large float equality/inequality checks. |
| `math_floor_ceil_large_more` | `math.lua` | Additional `floor`/`ceil` type-preserving checks and exact powers-of-two rounding boundaries. |
| `math_fmod_more2` | `math.lua` | `math.fmod` integer/float result-type consistency across small signed operands and same-bound integer cases. |
| `math_fmod_integer_more` | `math.lua` | `math.fmod` integer/float result-type behavior over signed operands and zero-divisor errors. |
| `math_implicit_conversion_more` | `math.lua` | Implicit arithmetic conversion for numeric strings across multiplication, addition, subtraction, division, and unary minus. |
| `math_lib_basic` | `math.lua` | Common math library functions and `math.type`. |
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
| `nextvar_table_insert_string_more` | `nextvar.lua` | `table.insert` and `table.remove` over string sequences with explicit insertion positions and append. |
| `nextvar_table_maxn` | `nextvar.lua` | Deprecated `table.maxn` Lua implementation over numeric, string, and floating numeric keys. |
| `nextvar_table_length_nils_more` | `nextvar.lua` | Table length over nil-only constructors, trailing nils, incremental array fills, and sparse nil deletion. |
| `nextvar_table_remove_sequences` | `nextvar.lua` | `table.remove` over string sequences, `#t+1` no-op removal, and middle/tail deletion. |
| `nextvar_table_remove_more` | `nextvar.lua` | Additional `table.remove` edge cases for zero index storage, append/remove order, invalid positions, and middle deletion. |
| `pm_captures_basic_more` | `pm.lua` | Additional captures, empty captures, anchored captures, trimming captures, and position-capture replacement in `gsub`. |
| `pm_find_empty_anchor` | `pm.lua` | Empty pattern finds and anchored pattern finds. |
| `pm_find_nul_more2` | `pm.lua` | Empty-pattern find, literal find in NUL-containing strings, and missing NUL pattern checks. |
| `pm_find_match_more_ascii` | `pm.lua` | Additional ASCII pattern find/match repetition, anchors, frontier-like punctuation, and negated classes. |
| `pm_find_nul_strings` | `pm.lua` | `string.find` over NUL-containing strings, start offsets, final-byte matches, and missing NUL suffixes. |
| `pm_gsub_captures` | `pm.lua` | `gsub` replacement captures, whole-match captures, and position captures. |
| `pm_gsub_capture_reorder_more` | `pm.lua` | Additional `gsub` replacement capture reordering, whole-match replacement, and replacement limits. |
| `pm_gsub_empty_match_more` | `pm.lua` | Empty-match `gsub` progression plus start/end anchors on empty subjects. |
| `pm_gsub_error_subset_more` | `pm.lua` | `gsub` replacement-string errors for invalid capture indexes and invalid percent escapes. |
| `pm_gsub_replacement_more` | `pm.lua` | Additional `gsub` capture replacement, empty-subject anchors, empty-match replacement progression, and replacement limits. |
| `pm_gsub_table_fallback_more` | `pm.lua` | `gsub` table replacements with missing keys, false values, position captures, and fallback to original text. |
| `pm_gsub_table_replacement_more` | `pm.lua` | `gsub` table replacements by whole match/first capture, missing replacement fallback, and invalid replacement value errors. |
| `pm_gsub_trim` | `pm.lua` | Simple `gsub`, trimming, whitespace replacement, empty-string anchors. |
| `pm_gmatch_words_more` | `pm.lua` | `gmatch` iteration over word captures and multi-capture numeric assignments. |
| `pm_gmatch_numeric_pairs_more` | `pm.lua` | `gmatch` two-capture numeric assignments into a table followed by `pairs` verification. |
| `pm_match_captures_ascii` | `pm.lua` | ASCII `string.match` captures with `%w`, `%d`, empty captures, and anchored failure cases. |
| `pm_match_classes_repetition_more` | `pm.lua` | Additional pattern repetition, anchors, minimal matches, negated classes, and `%S`/`%C` classes. |
| `pm_match_classes_more2` | `pm.lua` | Additional pattern class and repetition checks for `%l`, `%a`, `*`, `+`, escaped `$`, and missing matches. |
| `pm_match_repetition` | `pm.lua` | Greedy repetition pattern matches. |
| `pm_malformed_pattern_errors_more` | `pm.lua` | Malformed pattern inputs for unfinished captures and malformed character classes raise errors. |
| `sort_custom_comparator` | `sort.lua` | VM-closure comparators for descending sort, empty arrays, equal elements, and string ordering checks. |
| `sort_invalid_order_function` | `sort.lua` | `table.sort` does not call comparators for empty ranges and rejects inconsistent order functions. |
| `sort_metatable_lt_more` | `sort.lua` | Default `table.sort` ordering through element `__lt` metamethods. |
| `sort_order` | `sort.lua` | Default `table.sort` over permutations. |
| `sort_permutation_more` | `sort.lua` | Additional recursive permutation sorting over 4- and 5-element arrays, including duplicate values. |
| `sort_table_insert_errors` | `sort.lua` | `table.insert` wrong-arity and bad-table argument errors. |
| `sort_pack_unpack` | `sort.lua` | `table.pack` nil/count behavior. |
| `sort_pack_nil_counts_more` | `sort.lua` | Additional `table.pack` count preservation across interior and trailing nil arguments. |
| `sort_table_insert_remove_concat` | `sort.lua` | `table.insert`, `table.remove`, `table.concat`. |
| `sort_table_move` | `sort.lua` | `table.move` forward/backward/overlapping/empty moves. |
| `sort_unpack_ranges` | `sort.lua` | `table.unpack` direct assignment, bounded ranges, singleton ranges, and empty ranges. |
| `strings_basic` | `strings.lua` | String compare, sub/find/len/byte/char/case/rep/reverse basics. |
| `strings_byte_ascii_indices` | `strings.lua` | ASCII `string.byte` positive/negative range indices and empty-range nil behavior. |
| `strings_byte_char_edges` | `strings.lua` | `string.byte` and `string.char` edge checks that avoid known multi-return gaps. |
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
| `utf8_multibyte_offsets` | `utf8.lua` | Multibyte UTF-8 length, offsets, codepoints. |
| `utf8_offset_len_errors` | `utf8.lua` | `utf8.offset` and indexed `utf8.len` position bounds and continuation-byte errors. |
| `vararg_forwarding` | `vararg.lua` | Vararg capture and simple forwarding. |
| `vararg_pack` | `calls.lua`, `vararg.lua` | Vararg count, `table.pack`, forwarding. |
| `vararg_select` | `vararg.lua` | Positive-index `select` and protected out-of-range calls. |
| `vararg_tail_missing_args` | `vararg.lua` | Tail-call forwarding with missing arguments preserves nil and later returned values. |

Next recommended conversion order:

1. Fix known semantic gaps that block larger official slices (nested multi-return, file-mode inline function/table arguments, floor division, bitwise operators).
2. Continue `strings.lua` pattern/gsub/format/concat slices after those gaps shrink.
3. Continue `events.lua` comparison/rawlen/protected metatable slices after metamethod and `rawlen` support improve.
4. Continue `math.lua` integer/bitwise/floor-division slices after lexer/operator support is added.
