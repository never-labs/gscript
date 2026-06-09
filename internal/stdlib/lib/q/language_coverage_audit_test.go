package q

import (
	"strings"
	"testing"
)

func TestQLanguageCoverageAudit(t *testing.T) {
	type qCoverageCase struct {
		category string
		name     string
		src      string
		scope    string
		priority string
	}

	cases := []qCoverageCase{
		{category: "arithmetic_math", name: "plus", src: "1+2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "minus", src: "3-2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "multiply", src: "2*3", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "divide", src: "6%2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "div", src: "7 div 2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "mod", src: "7 mod 2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "neg", src: "neg 3", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "abs", src: "abs -3", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "signum", src: "signum -3 0 2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "reciprocal", src: "reciprocal 2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "exp", src: "exp 1", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "floor", src: "floor 1.7", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "ceiling", src: "ceiling 1.2", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "sqrt", src: "sqrt 4", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "log", src: "log 1", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "xexp", src: "2 xexp 3", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "xlog", src: "2 xlog 8", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "sin", src: "sin 0", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "cos", src: "cos 0", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "tan", src: "tan 0", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "asin", src: "asin 0", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "acos", src: "acos 1", scope: "target", priority: "covered"},
		{category: "arithmetic_math", name: "atan", src: "atan 0", scope: "target", priority: "covered"},

		{category: "aggregate_stats", name: "sum", src: "sum 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "prd", src: "prd 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "avg", src: "avg 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "min", src: "min 3 1 2", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "max", src: "max 3 1 2", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "med", src: "med 1 3 2", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "var", src: "var 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "dev", src: "dev 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "count", src: "count 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "wavg", src: "1 2 3 wavg 10 20 30", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "svar", src: "svar 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "sdev", src: "sdev 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "wsum", src: "1 2 3 wsum 10 20 30", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "cor", src: "1 2 3 cor 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "cov", src: "1 2 3 cov 1 2 3", scope: "target", priority: "covered"},
		{category: "aggregate_stats", name: "scov", src: "1 2 3 scov 1 2 3", scope: "target", priority: "covered"},

		{category: "window_scan", name: "sums", src: "sums 1 2 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "prds", src: "prds 1 2 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "maxs", src: "maxs 3 1 2", scope: "target", priority: "covered"},
		{category: "window_scan", name: "mins", src: "mins 3 1 2", scope: "target", priority: "covered"},
		{category: "window_scan", name: "avgs", src: "avgs 1 2 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "deltas", src: "deltas 10 12 15", scope: "target", priority: "covered"},
		{category: "window_scan", name: "ratios", src: "ratios 2 4 10", scope: "target", priority: "covered"},
		{category: "window_scan", name: "differ", src: "differ 10 10 20", scope: "target", priority: "covered"},
		{category: "window_scan", name: "msum", src: "2 msum 1 2 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "mavg", src: "2 mavg 1 2 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "mcount", src: "2 mcount 1 0N 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "mmax", src: "2 mmax 3 1 4", scope: "target", priority: "covered"},
		{category: "window_scan", name: "mmin", src: "2 mmin 3 1 4", scope: "target", priority: "covered"},
		{category: "window_scan", name: "mdev", src: "2 mdev 1 2 3", scope: "target", priority: "covered"},
		{category: "window_scan", name: "ema", src: "0.5 ema 1 2 3", scope: "target", priority: "covered"},

		{category: "list_ops", name: "til", src: "til 4", scope: "target", priority: "covered"},
		{category: "list_ops", name: "first", src: "first 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "last", src: "last 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "reverse", src: "reverse 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "where", src: "where 101b", scope: "target", priority: "covered"},
		{category: "list_ops", name: "group", src: "group 1 2 1", scope: "target", priority: "covered"},
		{category: "list_ops", name: "distinct", src: "distinct 1 2 1", scope: "target", priority: "covered"},
		{category: "list_ops", name: "asc", src: "asc 3 1 2", scope: "target", priority: "covered"},
		{category: "list_ops", name: "desc", src: "desc 3 1 2", scope: "target", priority: "covered"},
		{category: "list_ops", name: "iasc", src: "iasc 3 1 2", scope: "target", priority: "covered"},
		{category: "list_ops", name: "idesc", src: "idesc 3 1 2", scope: "target", priority: "covered"},
		{category: "list_ops", name: "rank", src: "rank 30 10 20", scope: "target", priority: "covered"},
		{category: "list_ops", name: "xrank", src: "2 xrank 40 10 30 20", scope: "target", priority: "covered"},
		{category: "list_ops", name: "bin", src: "10 20 30 bin 20", scope: "target", priority: "covered"},
		{category: "list_ops", name: "binr", src: "10 20 20 30 binr 20", scope: "target", priority: "covered"},
		{category: "list_ops", name: "raze", src: "raze (1 2;3 4)", scope: "target", priority: "covered"},
		{category: "list_ops", name: "enlist", src: "enlist 42", scope: "target", priority: "covered"},
		{category: "list_ops", name: "rotate", src: "1 rotate 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "fills", src: "fills 0N 2 0N", scope: "target", priority: "covered"},
		{category: "list_ops", name: "next", src: "next 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "prev", src: "prev 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "xprev", src: "2 xprev 10 20 30 40", scope: "target", priority: "covered"},
		{category: "list_ops", name: "except", src: "1 2 3 except 2", scope: "target", priority: "covered"},
		{category: "list_ops", name: "inter", src: "1 2 3 inter 2 3 4", scope: "target", priority: "covered"},
		{category: "list_ops", name: "union", src: "1 2 union 2 3", scope: "target", priority: "covered"},
		{category: "list_ops", name: "in", src: "2 4 in 1 2 3", scope: "target", priority: "covered"},
		{category: "list_ops", name: "within", src: "10 20 30 within 15 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "find_unique", src: "?1 2 1 3", scope: "target", priority: "covered"},
		{category: "list_ops", name: "find_index", src: "10 20 30?20", scope: "target", priority: "covered"},
		{category: "list_ops", name: "take_scalar", src: "3#10 20", scope: "target", priority: "covered"},
		{category: "list_ops", name: "drop", src: "1_10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "cut_keyword", src: "0 2_10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "cut_function", src: "cut[0 2;10 20 30]", scope: "target", priority: "covered"},
		{category: "list_ops", name: "sublist", src: "1 2 sublist 10 20 30", scope: "target", priority: "covered"},
		{category: "list_ops", name: "cross", src: "1 2 cross `a`b", scope: "target", priority: "covered"},
		{category: "list_ops", name: "vector_left_reshape", src: "2 3#1 2 3 4 5 6", scope: "target", priority: "covered"},

		{category: "logic_compare", name: "and", src: "1b and 0b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "or", src: "1b or 0b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "not", src: "not 101b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "all", src: "all 101b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "any", src: "any 101b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "null", src: "null 0N 1", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "equal", src: "1 2 3=2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "lt", src: "1 2 3<2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "gt", src: "1 2 3>2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "le", src: "1 2 3<=2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "ge", src: "1 2 3>=2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "ne", src: "1 2 3<>2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "match", src: "1 2~1 2", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "max_symbol", src: "2|3", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "min_symbol", src: "2&3", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "bool_atom_literal", src: "1b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "bool_compact_literal", src: "101b", scope: "target", priority: "covered"},
		{category: "logic_compare", name: "bool_spaced_literal", src: "0 1b", scope: "target", priority: "covered"},

		{category: "string_symbol", name: "string", src: "string `AAPL`MSFT", scope: "target", priority: "covered"},
		{category: "string_symbol", name: "lower", src: "lower `AAPL`MSFT", scope: "target", priority: "covered"},
		{category: "string_symbol", name: "upper", src: `upper "a" "b"`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "like", src: "`AAPL`MSFT like `A*", scope: "target", priority: "covered"},
		{category: "string_symbol", name: "trim", src: `trim " AAPL "`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "ltrim", src: `ltrim " AAPL "`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "rtrim", src: `rtrim " AAPL "`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "ss", src: `"banana" ss "an"`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "ssr", src: `ssr["banana";"an";"ON"]`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "sv", src: `"," sv "AAPL" "MSFT"`, scope: "target", priority: "covered"},
		{category: "string_symbol", name: "vs", src: `"," vs "AAPL,MSFT"`, scope: "target", priority: "covered"},

		{category: "matrix", name: "reshape_matrix", src: "2 2#1 2 3 4", scope: "target", priority: "covered"},
		{category: "matrix", name: "flip_matrix", src: "flip 2 3#1 2 3 4 5 6", scope: "target", priority: "covered"},
		{category: "matrix", name: "flip_list_of_lists", src: "flip (1 2 3;4 5 6)", scope: "target", priority: "covered"},
		{category: "matrix", name: "mmu", src: "mmu[(2 2#1 2 3 4);(2 2#5 6 7 8)]", scope: "target", priority: "covered"},
		{category: "matrix", name: "inv", src: "inv 2 2#1 0 0 1", scope: "target", priority: "covered"},

		{category: "condition_control", name: "dollar_cond", src: "$[1;2;3]", scope: "target", priority: "covered"},
		{category: "condition_control", name: "question_cond", src: "?[0;2;3]", scope: "target", priority: "covered"},
		{category: "condition_control", name: "lambda_cond", src: "f:{$[x>0;1;-1]};f[-2]", scope: "target", priority: "covered"},
		{category: "condition_control", name: "if", src: "a:0;if[1;a:42];a", scope: "target", priority: "covered"},
		{category: "condition_control", name: "do", src: "a:0;do[3;a:a+2];a", scope: "target", priority: "covered"},
		{category: "condition_control", name: "while", src: "i:0;while[i<3;i:i+1];i", scope: "target", priority: "covered"},
		{category: "condition_control", name: "augmented_assignment", src: "i:0;i+:1;i", scope: "target", priority: "covered"},

		{category: "apply_index_cast", name: "at_index", src: "x:10 20 30;x@1", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "dot_index", src: "x:10 20 30;x . 1", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "deep_index", src: "x:(10 20;30 40);x . (1;0)", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "dot_apply", src: ".[+;(1;2)]", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "lambda_apply", src: "{x+y} . (4;5)", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "symbol_cast", src: "`$\"hello\"", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "parse_long", src: "\"J\"$\"42\"", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "parse_int", src: "\"I\"$\"42\"", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "named_long_cast", src: "`long$\"42\"", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "named_int_cast", src: "`int$\"42\"", scope: "target", priority: "covered"},
		{category: "apply_index_cast", name: "named_float_cast", src: "`float$\"3.5\"", scope: "target", priority: "covered"},
	}

	var failures []string
	byCategory := map[string]int{}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.category+"/"+tc.name, func(t *testing.T) {
			byCategory[tc.category]++
			if _, err := Eval(tc.src); err != nil {
				failures = append(failures, tc.category+"/"+tc.name+" scope="+tc.scope+" priority="+tc.priority+" expr="+tc.src+" err="+err.Error())
				t.Errorf("Eval(%q) failed: %v", tc.src, err)
			}
		})
	}
	if len(failures) > 0 {
		t.Fatalf("q language coverage audit failures:\n%s", strings.Join(failures, "\n"))
	}
	for category, count := range byCategory {
		t.Logf("q language coverage audit category=%s cases=%d", category, count)
	}
}
