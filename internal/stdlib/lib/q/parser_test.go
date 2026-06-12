package q

import (
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/stdlib/lib/data"
)

func TestParseSelectWhere(t *testing.T) {
	query := mustParse(t, "select price,size from trades where price>100")

	if query.Kind != SelectQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
	if len(query.Columns) != 2 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	if query.Columns[0].Name != "price" || query.Columns[1].Name != "size" {
		t.Fatalf("column names = %#v", query.Columns)
	}
	where, ok := query.Where.(Binary)
	if !ok || where.Op != ">" {
		t.Fatalf("where = %#v", query.Where)
	}
	if left, ok := where.Left.(Ident); !ok || left.Name != "price" {
		t.Fatalf("where left = %#v", where.Left)
	}
	if right, ok := where.Right.(Number); !ok || right.Text != "100" {
		t.Fatalf("where right = %#v", where.Right)
	}
}

func TestParseQSQLLogicalSymbolOperators(t *testing.T) {
	query := mustParse(t, "select sym from trades where ((price>100)&(size>10))|flagged")

	where, ok := query.Where.(Binary)
	if !ok || where.Op != "|" {
		t.Fatalf("where = %#v, want top-level | binary", query.Where)
	}
	left, ok := where.Left.(Binary)
	if !ok || left.Op != "&" {
		t.Fatalf("where left = %#v, want & binary", where.Left)
	}
	if _, ok := where.Right.(Ident); !ok {
		t.Fatalf("where right = %#v, want flagged ident", where.Right)
	}
}

func TestParseSelectAllColumns(t *testing.T) {
	query := mustParse(t, "select * from trades where price>100")

	if query.Kind != SelectQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
	if len(query.Columns) != 1 || query.Columns[0].Name != "*" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	if _, ok := query.Columns[0].Expr.(AllColumns); !ok {
		t.Fatalf("column expr = %#v, want AllColumns", query.Columns[0].Expr)
	}
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}
	if len(lowered.Plan.Select) != 1 || lowered.Plan.Select[0].Name != "*" {
		t.Fatalf("lowered select = %#v", lowered.Plan.Select)
	}
	lit, ok := lowered.Plan.Select[0].Expr.(data.Literal)
	if !ok {
		t.Fatalf("lowered expr = %#v, want literal sentinel", lowered.Plan.Select[0].Expr)
	}
	if _, ok := lit.Value.(AllColumns); !ok {
		t.Fatalf("literal = %#v, want AllColumns", lit.Value)
	}
}

func TestParseOmittedProjectionDefaultsToAllColumns(t *testing.T) {
	tests := []struct {
		src  string
		kind QueryKind
	}{
		{src: "select from trades where price>100", kind: SelectQuery},
		{src: "exec from trades where price>100", kind: ExecQuery},
	}
	for _, tt := range tests {
		query := mustParse(t, tt.src)
		if query.Kind != tt.kind || query.From != "trades" {
			t.Fatalf("%q header = %#v", tt.src, query)
		}
		if len(query.Columns) != 1 || query.Columns[0].Name != "*" {
			t.Fatalf("%q columns = %#v", tt.src, query.Columns)
		}
		if _, ok := query.Columns[0].Expr.(AllColumns); !ok {
			t.Fatalf("%q column expr = %#v, want AllColumns", tt.src, query.Columns[0].Expr)
		}
	}
}

func TestParseGroupedSelectWithAliasesAndAggregates(t *testing.T) {
	query := mustParse(t, "select notional:sum price*size, fills:count i by sym from trades where price>100")

	if query.Kind != SelectQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
	if len(query.Columns) != 2 || query.Columns[0].Name != "notional" || query.Columns[1].Name != "fills" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	sum, ok := query.Columns[0].Expr.(Call)
	if !ok || sum.Func != "sum" {
		t.Fatalf("notional expr = %#v", query.Columns[0].Expr)
	}
	product, ok := sum.Arg.(Binary)
	if !ok || product.Op != "*" {
		t.Fatalf("sum arg = %#v", sum.Arg)
	}
	count, ok := query.Columns[1].Expr.(Call)
	if !ok || count.Func != "count" {
		t.Fatalf("fills expr = %#v", query.Columns[1].Expr)
	}
	if len(query.By) != 1 {
		t.Fatalf("by = %#v", query.By)
	}
	if query.By[0].Name != "sym" {
		t.Fatalf("by name = %#v", query.By[0])
	}
	if by, ok := query.By[0].Expr.(Ident); !ok || by.Name != "sym" {
		t.Fatalf("by expr = %#v", query.By[0].Expr)
	}
}

func TestParseSelectAndExecByWithoutProjection(t *testing.T) {
	tests := []struct {
		src  string
		kind QueryKind
	}{
		{src: "select by sym from trades", kind: SelectQuery},
		{src: "exec by sym from trades", kind: ExecQuery},
		{src: "select by sym,bucket from trades order by sym asc", kind: SelectQuery},
	}
	for _, tt := range tests {
		query := mustParse(t, tt.src)
		if query.Kind != tt.kind || query.From != "trades" {
			t.Fatalf("%q header = %#v", tt.src, query)
		}
		if len(query.Columns) != 0 {
			t.Fatalf("%q columns = %#v, want empty projection", tt.src, query.Columns)
		}
		if len(query.By) == 0 {
			t.Fatalf("%q by is empty", tt.src)
		}
	}
}

func TestParseComputedProjectionDefaultName(t *testing.T) {
	query := mustParse(t, "select price*size from trades")
	if len(query.Columns) != 1 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	if query.Columns[0].Name != "price*size" {
		t.Fatalf("computed projection name = %q, want price*size", query.Columns[0].Name)
	}
}

func TestParseOrderByComputedProjectionDefaultName(t *testing.T) {
	query := mustParse(t, "select price*size from trades order by price*size desc")
	if len(query.OrderBy) != 1 {
		t.Fatalf("order by = %#v", query.OrderBy)
	}
	if query.OrderBy[0].Column != "price*size" || !query.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", query.OrderBy)
	}
	if _, ok := query.OrderBy[0].Expr.(Binary); !ok {
		t.Fatalf("order expr = %#v, want Binary", query.OrderBy[0].Expr)
	}
}

func TestParsePercentDivideExpressions(t *testing.T) {
	query := mustParse(t, "select ratio:a%b,spread:((price-arrival)*10000)%arrival from trades where (price%qty)>10 order by price%qty desc")
	if len(query.Columns) != 2 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	ratio, ok := query.Columns[0].Expr.(Binary)
	if !ok || ratio.Op != "%" {
		t.Fatalf("ratio expr = %#v, want %% Binary", query.Columns[0].Expr)
	}
	spread, ok := query.Columns[1].Expr.(Binary)
	if !ok || spread.Op != "%" {
		t.Fatalf("spread expr = %#v, want %% Binary", query.Columns[1].Expr)
	}
	product, ok := spread.Left.(Binary)
	if !ok || product.Op != "*" {
		t.Fatalf("spread left = %#v, want * Binary", spread.Left)
	}
	if inner, ok := product.Left.(Binary); !ok || inner.Op != "-" {
		t.Fatalf("spread product left = %#v, want - Binary", product.Left)
	}
	where, ok := query.Where.(Binary)
	if !ok || where.Op != ">" {
		t.Fatalf("where = %#v", query.Where)
	}
	if left, ok := where.Left.(Binary); !ok || left.Op != "%" {
		t.Fatalf("where left = %#v, want %% Binary", where.Left)
	}
	if len(query.OrderBy) != 1 {
		t.Fatalf("order by = %#v", query.OrderBy)
	}
	if order, ok := query.OrderBy[0].Expr.(Binary); !ok || order.Op != "%" {
		t.Fatalf("order expr = %#v, want %% Binary", query.OrderBy[0].Expr)
	}
}

func TestParseExecSymbolWhere(t *testing.T) {
	query := mustParse(t, "exec price from trades where sym=`AAPL")

	if query.Kind != ExecQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
	if len(query.Columns) != 1 || query.Columns[0].Name != "price" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	where, ok := query.Where.(Binary)
	if !ok || where.Op != "=" {
		t.Fatalf("where = %#v", query.Where)
	}
	if right, ok := where.Right.(Symbol); !ok || right.Name != "AAPL" {
		t.Fatalf("symbol = %#v", where.Right)
	}
}

func TestParseExecDictExpression(t *testing.T) {
	query := mustParse(t, "exec sym!price from trades")

	if query.Kind != ExecQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
	if len(query.Columns) != 1 || query.Columns[0].Name != "sym!price" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	dict, ok := query.Columns[0].Expr.(DictExpr)
	if !ok {
		t.Fatalf("projection = %#v, want DictExpr", query.Columns[0].Expr)
	}
	if key, ok := dict.Keys.(Ident); !ok || key.Name != "sym" {
		t.Fatalf("dict keys = %#v", dict.Keys)
	}
	if value, ok := dict.Values.(Ident); !ok || value.Name != "price" {
		t.Fatalf("dict values = %#v", dict.Values)
	}
}

func TestParseWhereLiteralsAndOrderLimit(t *testing.T) {
	query := mustParse(t, `select sym,price,size from trades where venue="XNYS" order by price desc limit 2`)

	where, ok := query.Where.(Binary)
	if !ok || where.Op != "=" {
		t.Fatalf("where = %#v", query.Where)
	}
	if right, ok := where.Right.(String); !ok || right.Value != "XNYS" {
		t.Fatalf("string literal = %#v", where.Right)
	}
	if len(query.OrderBy) != 1 || query.OrderBy[0].Column != "price" || !query.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", query.OrderBy)
	}
	if query.Limit == nil || *query.Limit != 2 {
		t.Fatalf("limit = %#v", query.Limit)
	}
}

func TestParseDistinctAndTake(t *testing.T) {
	query := mustParse(t, "select distinct sym,price from trades order by price desc take 2")

	if !query.Distinct {
		t.Fatalf("distinct = false")
	}
	if len(query.Columns) != 2 || query.Columns[0].Name != "sym" || query.Columns[1].Name != "price" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	if len(query.OrderBy) != 1 || query.OrderBy[0].Column != "price" || !query.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", query.OrderBy)
	}
	if query.Take == nil || *query.Take != 2 {
		t.Fatalf("take = %#v", query.Take)
	}
}

func TestParseExtendedAggregateFunctions(t *testing.T) {
	query := mustParse(t, "select v:var price,d:dev price,m:med price,w:wavg price by sym from trades")

	if len(query.Columns) != 4 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	for i, want := range []string{"var", "dev", "med", "wavg"} {
		call, ok := query.Columns[i].Expr.(Call)
		if !ok || call.Func != want {
			t.Fatalf("column %d expr = %#v, want %s call", i, query.Columns[i].Expr, want)
		}
	}
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if len(lowered.Plan.Aggregates) != 4 {
		t.Fatalf("aggregates = %#v", lowered.Plan.Aggregates)
	}
	for i, want := range []string{"var", "dev", "med", "wavg"} {
		if lowered.Plan.Aggregates[i].Func != want {
			t.Fatalf("aggregate %d func = %q, want %q", i, lowered.Plan.Aggregates[i].Func, want)
		}
	}
}

func TestParseInnerJoin(t *testing.T) {
	query := mustParse(t, "select sym,price,bid from trades join quotes on sym where price>100")

	if query.Join == nil || query.Join.Kind != "inner" || query.Join.Right != "quotes" {
		t.Fatalf("join = %#v", query.Join)
	}
	if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}
	if query.Where == nil {
		t.Fatalf("where is nil")
	}
}

func TestParseInnerJoinAlias(t *testing.T) {
	query := mustParse(t, "select sym,price,bid from trades inner join quotes on sym where price>100")

	if query.Join == nil || query.Join.Kind != "inner" || query.Join.Right != "quotes" {
		t.Fatalf("join = %#v", query.Join)
	}
	if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}
	if query.Where == nil {
		t.Fatalf("where is nil")
	}
}

func TestParseInnerJoinShortAlias(t *testing.T) {
	query := mustParse(t, "select sym,price,bid from trades ij quotes on sym where price>100")

	if query.Join == nil || query.Join.Kind != "inner" || query.Join.Right != "quotes" {
		t.Fatalf("join = %#v", query.Join)
	}
	if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}
	if query.Where == nil {
		t.Fatalf("where is nil")
	}
}

func TestParseInnerJoinWithDifferentKeyNames(t *testing.T) {
	query := mustParse(t, "select id,value from left join right on id=account_id")

	if query.Join == nil || query.Join.Right != "right" {
		t.Fatalf("join = %#v", query.Join)
	}
	if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "id" || query.Join.Keys[0].Right != "account_id" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}
}

func TestParseInnerJoinWithMultipleAliasedKeys(t *testing.T) {
	query := mustParse(t, "select id,value from accounts join fills on id=account_id,venue=exchange where value>0")

	if query.Join == nil || query.Join.Right != "fills" {
		t.Fatalf("join = %#v", query.Join)
	}
	want := []JoinKey{
		{Left: "id", Right: "account_id"},
		{Left: "venue", Right: "exchange"},
	}
	if len(query.Join.Keys) != len(want) {
		t.Fatalf("join keys len = %d, want %d: %#v", len(query.Join.Keys), len(want), query.Join.Keys)
	}
	for i := range want {
		if query.Join.Keys[i] != want[i] {
			t.Fatalf("join key %d = %#v, want %#v", i, query.Join.Keys[i], want[i])
		}
	}
	if query.Where == nil {
		t.Fatalf("where is nil")
	}
}

func TestParseAndLowerChainedJoins(t *testing.T) {
	query := mustParse(t, "select trade_id,sym,venue,bid,region from trades ij quotes on sym,ts lj venues on venue where bid>100")

	if query.Join == nil || len(query.Joins) != 2 {
		t.Fatalf("joins = first %#v all %#v, want two joins", query.Join, query.Joins)
	}
	if query.Joins[0].Kind != "inner" || query.Joins[0].Right != "quotes" {
		t.Fatalf("first join = %#v", query.Joins[0])
	}
	if query.Joins[1].Kind != "left" || query.Joins[1].Right != "venues" {
		t.Fatalf("second join = %#v", query.Joins[1])
	}
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || len(lowered.Joins) != 2 {
		t.Fatalf("lowered joins = first %#v all %#v, want two joins", lowered.Join, lowered.Joins)
	}
	if lowered.Joins[0].Kind != "inner" || lowered.Joins[0].Right != "quotes" {
		t.Fatalf("lowered first join = %#v", lowered.Joins[0])
	}
	if lowered.Joins[1].Kind != "left" || lowered.Joins[1].Right != "venues" {
		t.Fatalf("lowered second join = %#v", lowered.Joins[1])
	}
}

func TestParseRejectsClauseKeywordsAsJoinKeys(t *testing.T) {
	for _, src := range []string{
		"select sym,price from trades join quotes on where",
		"select sym,price from trades join quotes on sym=where",
		"select sym,price from trades join quotes on sym,where price>0",
		"select sym,price from trades aj quotes on sym,order by sym",
	} {
		if _, err := Parse(src); err == nil {
			t.Fatalf("Parse(%q) returned nil error", src)
		}
	}
}

func TestParseAsofJoin(t *testing.T) {
	query := mustParse(t, "select sym,ts,price,bid from trades aj quotes on sym,ts where price>0")

	if query.Join == nil || query.Join.Kind != "asof" || query.Join.Right != "quotes" {
		t.Fatalf("join = %#v", query.Join)
	}
	want := []JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}}
	if len(query.Join.Keys) != len(want) {
		t.Fatalf("join keys len = %d, want %d: %#v", len(query.Join.Keys), len(want), query.Join.Keys)
	}
	for i := range want {
		if query.Join.Keys[i] != want[i] {
			t.Fatalf("join key %d = %#v, want %#v", i, query.Join.Keys[i], want[i])
		}
	}
	if query.Where == nil {
		t.Fatalf("where is nil")
	}
}

func TestParseAsofJoinVariantAliases(t *testing.T) {
	cases := map[string]string{
		"aj0":  "asof0",
		"ajf":  "asof_fill",
		"ajf0": "asof_fill0",
	}
	for keyword, wantKind := range cases {
		src := "select sym,ts,price,bid from trades " + keyword + " quotes on sym,ts where price>0"
		query := mustParse(t, src)
		if query.Join == nil || query.Join.Kind != wantKind || query.Join.Right != "quotes" {
			t.Fatalf("%s join = %#v", keyword, query.Join)
		}
		want := []JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}}
		if !reflect.DeepEqual(query.Join.Keys, want) {
			t.Fatalf("%s join keys = %#v, want %#v", keyword, query.Join.Keys, want)
		}
		if query.Where == nil {
			t.Fatalf("%s where is nil", keyword)
		}
	}
}

func TestParseLeftJoinAliases(t *testing.T) {
	for _, src := range []string{
		"select sym,price,sector from trades left join refs on sym where price>0",
		"select sym,price,sector from trades lj refs on sym where price>0",
	} {
		query := mustParse(t, src)
		if query.Join == nil || query.Join.Kind != "left" || query.Join.Right != "refs" {
			t.Fatalf("%s join = %#v", src, query.Join)
		}
		if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
			t.Fatalf("%s join keys = %#v", src, query.Join.Keys)
		}
		if query.Where == nil {
			t.Fatalf("%s where is nil", src)
		}
	}
}

func TestParseUnionJoinAlias(t *testing.T) {
	query := mustParse(t, "select sym,price,venue from trades uj refs on sym order by sym asc")
	if query.Join == nil || query.Join.Kind != "union" || query.Join.Right != "refs" {
		t.Fatalf("join = %#v", query.Join)
	}
	if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}
}

func TestParsePlusJoinAlias(t *testing.T) {
	query := mustParse(t, "select sym,qty from trades pj refs on sym")
	if query.Join == nil || query.Join.Kind != "plus" || query.Join.Right != "refs" {
		t.Fatalf("join = %#v", query.Join)
	}
	if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}
}

func TestParseUnionPlusAndWindowJoinAliases(t *testing.T) {
	cases := []struct {
		src  string
		kind string
	}{
		{"select sym,qty from left uj right on sym", "union"},
		{"select sym,qty from left pj right on sym", "plus"},
		{"select sym,qty from left wj right on sym", "window"},
	}
	for _, tc := range cases {
		query := mustParse(t, tc.src)
		if query.Join == nil || query.Join.Kind != tc.kind || query.Join.Right != "right" {
			t.Fatalf("%s join = %#v, want kind %s right", tc.src, query.Join, tc.kind)
		}
		if len(query.Join.Keys) != 1 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[0].Right != "sym" {
			t.Fatalf("%s join keys = %#v", tc.src, query.Join.Keys)
		}
	}
}

func TestParseWindowJoinBoundsAndWJ1(t *testing.T) {
	query := mustParse(t, "select sym,ts,bid from trades wj1[-5 0] quotes on sym,ts")
	if query.Join == nil || query.Join.Kind != "window1" || query.Join.Right != "quotes" {
		t.Fatalf("join = %#v", query.Join)
	}
	if query.Join.Window == nil {
		t.Fatalf("window bounds are nil")
	}
	if low, ok := query.Join.Window.Low.(Binary); !ok || low.Op != "-" {
		t.Fatalf("low bound = %#v, want negative literal", query.Join.Window.Low)
	}
	if high, ok := query.Join.Window.High.(Number); !ok || high.Text != "0" {
		t.Fatalf("high bound = %#v", query.Join.Window.High)
	}
	if len(query.Join.Keys) != 2 || query.Join.Keys[0].Left != "sym" || query.Join.Keys[1].Left != "ts" {
		t.Fatalf("join keys = %#v", query.Join.Keys)
	}

	timespanQuery := mustParse(t, "select sym,ts,bid from trades wj[-0D00:01:00 0D00:00:00] quotes on sym,ts")
	if timespanQuery.Join == nil || timespanQuery.Join.Window == nil {
		t.Fatalf("timespan window join = %#v", timespanQuery.Join)
	}
	low, ok := timespanQuery.Join.Window.Low.(Binary)
	if !ok || low.Op != "-" {
		t.Fatalf("timespan low bound = %#v, want negative timespan literal", timespanQuery.Join.Window.Low)
	}
	if temporal, ok := low.Right.(Temporal); !ok || temporal.Kind != "timespan" || temporal.Text != "0D00:01:00" {
		t.Fatalf("timespan low right = %#v", low.Right)
	}
	if high, ok := timespanQuery.Join.Window.High.(Temporal); !ok || high.Kind != "timespan" || high.Text != "0D00:00:00" {
		t.Fatalf("timespan high bound = %#v", timespanQuery.Join.Window.High)
	}
}

func TestParseWindowJoinCommaSeparatedBounds(t *testing.T) {
	query := mustParse(t, "select sym,ts,bid from trades wj[-5,0] quotes on sym,ts")
	if query.Join == nil || query.Join.Kind != "window" || query.Join.Window == nil {
		t.Fatalf("join = %#v", query.Join)
	}
	if low, ok := query.Join.Window.Low.(Binary); !ok || low.Op != "-" {
		t.Fatalf("low bound = %#v, want negative literal", query.Join.Window.Low)
	}
	if high, ok := query.Join.Window.High.(Number); !ok || high.Text != "0" {
		t.Fatalf("high bound = %#v", query.Join.Window.High)
	}

	timespanQuery := mustParse(t, "select sym,ts,bid from trades wj[-0D00:01:00,0D00:00:00] quotes on sym,ts")
	if timespanQuery.Join == nil || timespanQuery.Join.Window == nil {
		t.Fatalf("timespan window join = %#v", timespanQuery.Join)
	}
	low, ok := timespanQuery.Join.Window.Low.(Binary)
	if !ok || low.Op != "-" {
		t.Fatalf("timespan low bound = %#v, want negative timespan literal", timespanQuery.Join.Window.Low)
	}
	if temporal, ok := low.Right.(Temporal); !ok || temporal.Kind != "timespan" || temporal.Text != "0D00:01:00" {
		t.Fatalf("timespan low right = %#v", low.Right)
	}
	if high, ok := timespanQuery.Join.Window.High.(Temporal); !ok || high.Kind != "timespan" || high.Text != "0D00:00:00" {
		t.Fatalf("timespan high bound = %#v", timespanQuery.Join.Window.High)
	}
}

func TestParseRejectsInvalidWindowJoinBounds(t *testing.T) {
	for _, src := range []string{
		"select sym,ts,bid from trades wj[-5] quotes on sym,ts",
		"select sym,ts,bid from trades wj[-5,0,1] quotes on sym,ts",
		"select sym,ts,bid from trades wj[,] quotes on sym,ts",
		"select sym,ts,bid from trades wj[-0D00:01:00,] quotes on sym,ts",
	} {
		if _, err := Parse(src); err == nil {
			t.Fatalf("Parse(%q) returned nil error", src)
		}
	}
}

func TestParseJoinAliasesWithDifferentKeyNames(t *testing.T) {
	tests := []struct {
		src      string
		kind     string
		right    string
		rightKey string
		leftKey  string
	}{
		{"select id,value from accounts ij fills on id=account_id,venue=exchange", "inner", "fills", "exchange", "venue"},
		{"select id,value from accounts lj fills on id=account_id,venue=exchange", "left", "fills", "exchange", "venue"},
		{"select sym,ts,bid from trades aj quotes on sym,t=quote_time", "asof", "quotes", "quote_time", "t"},
		{"select sym,ts,bid from trades aj quotes on sym=ticker,t=quote_time", "asof", "quotes", "quote_time", "t"},
		{"select sym,ts,bid from trades wj[-5,0] quotes on sym,t=quote_time", "window", "quotes", "quote_time", "t"},
		{"select sym,ts,bid from trades wj[-5,0] quotes on sym=ticker,t=quote_time", "window", "quotes", "quote_time", "t"},
		{"select sym,ts,bid from trades wj1[-5,0] quotes on sym,t=quote_time", "window1", "quotes", "quote_time", "t"},
		{"select sym,ts,bid from trades wj1[-5,0] quotes on sym=ticker,t=quote_time", "window1", "quotes", "quote_time", "t"},
	}
	for _, tt := range tests {
		query := mustParse(t, tt.src)
		if query.Join == nil || query.Join.Kind != tt.kind || query.Join.Right != tt.right {
			t.Fatalf("%s join = %#v, want kind %s right %s", tt.src, query.Join, tt.kind, tt.right)
		}
		if len(query.Join.Keys) != 2 {
			t.Fatalf("%s join keys = %#v", tt.src, query.Join.Keys)
		}
		if strings.Contains(tt.src, "sym=ticker") {
			first := query.Join.Keys[0]
			if first.Left != "sym" || first.Right != "ticker" {
				t.Fatalf("%s first join key = %#v, want sym=ticker", tt.src, first)
			}
		}
		second := query.Join.Keys[1]
		if second.Left != tt.leftKey || second.Right != tt.rightKey {
			t.Fatalf("%s second join key = %#v, want %s=%s", tt.src, second, tt.leftKey, tt.rightKey)
		}
	}
}

func TestParseHashTakePrefix(t *testing.T) {
	query := mustParse(t, "2#select sym,price from trades")

	if query.Take == nil || *query.Take != 2 {
		t.Fatalf("take = %#v", query.Take)
	}
	if query.Kind != SelectQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
}

func TestParseXbarCall(t *testing.T) {
	query := mustParse(t, "select qty:sum size by bucket:xbar 10 ts from trades")
	if len(query.By) != 1 {
		t.Fatalf("by = %#v", query.By)
	}
	if query.By[0].Name != "bucket" {
		t.Fatalf("by name = %#v", query.By[0])
	}
	call, ok := query.By[0].Expr.(Call)
	if !ok || call.Func != "xbar" {
		t.Fatalf("by call = %#v", query.By[0].Expr)
	}
	arg, ok := call.Arg.(Vector)
	if !ok || len(arg.Items) != 2 {
		t.Fatalf("xbar arg = %#v", call.Arg)
	}
	if n, ok := arg.Items[0].(Number); !ok || n.Text != "10" {
		t.Fatalf("xbar interval = %#v", arg.Items[0])
	}
	if ident, ok := arg.Items[1].(Ident); !ok || ident.Name != "ts" {
		t.Fatalf("xbar expr = %#v", arg.Items[1])
	}

	computed := mustParse(t, "select n:count i by bucket:xbar 10 price+size from trades")
	call, ok = computed.By[0].Expr.(Call)
	if !ok || call.Func != "xbar" {
		t.Fatalf("computed xbar call = %#v", computed.By[0].Expr)
	}
	arg, ok = call.Arg.(Vector)
	if !ok || len(arg.Items) != 2 {
		t.Fatalf("computed xbar arg = %#v", call.Arg)
	}
	if expr, ok := arg.Items[1].(Binary); !ok || expr.Op != "+" {
		t.Fatalf("computed xbar expression = %#v, want binary + inside xbar", arg.Items[1])
	}
}

func TestParseTimeSeriesPrefixProjectionCalls(t *testing.T) {
	query := mustParse(t, "select p:prev price,n:next price,d:deltas price,f:fills price,r:ratios price,s:sums size,prd:prds size,mi:mins price,ma:maxs price,a:avgs price from trades")

	if len(query.Columns) != 10 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	for i, want := range []string{"prev", "next", "deltas", "fills", "ratios", "sums", "prds", "mins", "maxs", "avgs"} {
		call, ok := query.Columns[i].Expr.(Call)
		if !ok || call.Func != want {
			t.Fatalf("column %d expr = %#v, want %s call", i, query.Columns[i].Expr, want)
		}
		if ident, ok := call.Arg.(Ident); !ok || (ident.Name != "price" && ident.Name != "size") {
			t.Fatalf("column %d arg = %#v, want column ident", i, call.Arg)
		}
	}
}

func TestParseTimeSeriesDyadicProjectionCalls(t *testing.T) {
	query := mustParse(t, "select xp:2 xprev price,ms:3 msum size,ma:3 mavg price,mc:3 mcount price,mn:3 mmin price,mx:3 mmax price from trades")

	if len(query.Columns) != 6 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	for i, want := range []string{"xprev", "msum", "mavg", "mcount", "mmin", "mmax"} {
		call, ok := query.Columns[i].Expr.(Call)
		if !ok || call.Func != want {
			t.Fatalf("column %d expr = %#v, want %s call", i, query.Columns[i].Expr, want)
		}
		args, ok := call.Arg.(Vector)
		if !ok || len(args.Items) != 2 {
			t.Fatalf("column %d args = %#v, want two-item vector", i, call.Arg)
		}
		if n, ok := args.Items[0].(Number); !ok || n.Text == "" {
			t.Fatalf("column %d left arg = %#v, want numeric width", i, args.Items[0])
		}
		if ident, ok := args.Items[1].(Ident); !ok || (ident.Name != "price" && ident.Name != "size") {
			t.Fatalf("column %d right arg = %#v, want column ident", i, args.Items[1])
		}
	}
}

func TestParseConditionalProjectionCall(t *testing.T) {
	query := mustParse(t, "select signed_qty:?[side=`buy;size;0-size] from trades")
	if len(query.Columns) != 1 {
		t.Fatalf("columns = %#v", query.Columns)
	}
	call, ok := query.Columns[0].Expr.(Call)
	if !ok || call.Func != "?" {
		t.Fatalf("expr = %#v, want conditional call", query.Columns[0].Expr)
	}
	args, ok := call.Arg.(Vector)
	if !ok || len(args.Items) != 3 {
		t.Fatalf("conditional args = %#v", call.Arg)
	}
	if _, ok := args.Items[0].(Binary); !ok {
		t.Fatalf("condition = %#v, want Binary", args.Items[0])
	}
}

func TestParseUpdateAndDeleteSkeleton(t *testing.T) {
	update := mustParse(t, "update price:price*1.1 from trades where sym=`AAPL")
	if update.Kind != UpdateQuery || update.From != "trades" {
		t.Fatalf("update header = %#v", update)
	}
	if len(update.Columns) != 1 || update.Columns[0].Name != "price" {
		t.Fatalf("update columns = %#v", update.Columns)
	}
	if _, ok := update.Where.(Binary); !ok {
		t.Fatalf("update where = %#v", update.Where)
	}

	del := mustParse(t, "delete from trades where price<100")
	if del.Kind != DeleteQuery || del.From != "trades" {
		t.Fatalf("delete header = %#v", del)
	}
	if len(del.Columns) != 0 {
		t.Fatalf("delete columns = %#v", del.Columns)
	}
	if _, ok := del.Where.(Binary); !ok {
		t.Fatalf("delete where = %#v", del.Where)
	}
}

func TestParseDeleteColumnListAndLower(t *testing.T) {
	tests := []string{
		"delete price,size from trades",
		"delete price size from trades",
	}
	for _, src := range tests {
		query := mustParse(t, src)
		if query.Kind != DeleteQuery || query.From != "trades" {
			t.Fatalf("%q header = %#v", src, query)
		}
		if len(query.Columns) != 2 || query.Columns[0].Name != "price" || query.Columns[1].Name != "size" {
			t.Fatalf("%q columns = %#v", src, query.Columns)
		}
		for _, column := range query.Columns {
			ident, ok := column.Expr.(Ident)
			if !ok || ident.Name != column.Name {
				t.Fatalf("%q column = %#v, want identifier", src, column)
			}
		}
		lowered, err := Lower(query)
		if err != nil {
			t.Fatalf("Lower(%q) returned error: %v", src, err)
		}
		want := []data.Symbol{"price", "size"}
		if !reflect.DeepEqual(lowered.Mutation.DeleteColumns, want) {
			t.Fatalf("Lower(%q) delete columns = %#v, want %#v", src, lowered.Mutation.DeleteColumns, want)
		}
	}
}

func TestParseDeleteRejectsClauseKeywordsAsColumnNames(t *testing.T) {
	for _, src := range []string{
		"delete where from trades",
		"delete price,where from trades",
		"delete price by from trades",
		"delete price, from trades",
	} {
		if _, err := Parse(src); err == nil {
			t.Fatalf("Parse(%q) returned nil error", src)
		}
	}
}

func TestParseSignedLiteralsInWhereAndMutation(t *testing.T) {
	query := mustParse(t, "select price from trades where price>-5")
	where, ok := query.Where.(Binary)
	if !ok || where.Op != ">" {
		t.Fatalf("where = %#v", query.Where)
	}
	negative, ok := where.Right.(Binary)
	if !ok || negative.Op != "-" {
		t.Fatalf("where right = %#v, want negative literal", where.Right)
	}
	if left, ok := negative.Left.(Number); !ok || left.Text != "0" {
		t.Fatalf("negative left = %#v", negative.Left)
	}
	if right, ok := negative.Right.(Number); !ok || right.Text != "5" {
		t.Fatalf("negative right = %#v", negative.Right)
	}

	update := mustParse(t, "update price:-1 from trades where span>-0D00:01:00")
	if len(update.Columns) != 1 || update.Columns[0].Name != "price" {
		t.Fatalf("update columns = %#v", update.Columns)
	}
	if negative, ok := update.Columns[0].Expr.(Binary); !ok || negative.Op != "-" {
		t.Fatalf("update assignment = %#v, want negative literal", update.Columns[0].Expr)
	}
}

func TestParseUpdateByClauseAndLowerProducesGroupedMutation(t *testing.T) {
	query := mustParse(t, "update price:avg price by sym from trades where price>0")
	if query.Kind != UpdateQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
	}
	if len(query.By) != 1 || query.By[0].Name != "sym" {
		t.Fatalf("by = %#v", query.By)
	}
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error for grouped update: %v", err)
	}
	if lowered.Mutation == nil || len(lowered.Mutation.ByExprs) != 1 || len(lowered.Mutation.Assignments) != 1 {
		t.Fatalf("grouped mutation = %#v", lowered.Mutation)
	}
	if lowered.Mutation.Assignments[0].Func != "avg" {
		t.Fatalf("grouped mutation func = %q, want avg", lowered.Mutation.Assignments[0].Func)
	}
}

func TestParseMutationByBoundaries(t *testing.T) {
	if _, err := Parse("delete from trades by sym"); err == nil {
		t.Fatalf("Parse accepted delete by")
	}
	_, err := Lower(mustParse(t, "update price:price+1 by sym from trades"))
	if err == nil {
		t.Fatalf("Lower accepted grouped update without aggregate assignment")
	}
	if !strings.Contains(err.Error(), "requires an aggregate expression") {
		t.Fatalf("Lower grouped update error = %q, want aggregate diagnostic", err.Error())
	}
}

func TestParseInsertAndUpsertValues(t *testing.T) {
	insert := mustParse(t, "insert into trades (sym,price,size) values (`AAPL,100.5,10)")
	if insert.Kind != InsertQuery || insert.From != "trades" {
		t.Fatalf("insert header = %#v", insert)
	}
	if len(insert.Columns) != 3 || insert.Columns[0].Name != "sym" || insert.Columns[2].Name != "size" {
		t.Fatalf("insert columns = %#v", insert.Columns)
	}
	if len(insert.Values) != 3 {
		t.Fatalf("insert values = %#v", insert.Values)
	}
	if sym, ok := insert.Values[0].(Symbol); !ok || sym.Name != "AAPL" {
		t.Fatalf("insert sym value = %#v", insert.Values[0])
	}

	upsert := mustParse(t, "upsert into trades values (`MSFT,101,20)")
	if upsert.Kind != UpsertQuery || upsert.From != "trades" {
		t.Fatalf("upsert header = %#v", upsert)
	}
	if len(upsert.Columns) != 0 || len(upsert.Values) != 3 {
		t.Fatalf("upsert payload = columns %#v values %#v", upsert.Columns, upsert.Values)
	}
}

func TestParseInsertRejectsUnstableShapes(t *testing.T) {
	for _, src := range []string{
		"insert trades values (`AAPL,100)",
		"insert into trades (sym,price) values (`AAPL)",
		"insert into trades (sym,) values (`AAPL)",
		"upsert into where values (`AAPL)",
	} {
		if _, err := Parse(src); err == nil {
			t.Fatalf("Parse(%q) returned nil error", src)
		}
	}
}

func TestParseWhereScalarComparisons(t *testing.T) {
	tests := []struct {
		src   string
		op    string
		right any
	}{
		{`select price from trades where price<=100`, "<=", Number{Text: "100"}},
		{`select price from trades where price>=100`, ">=", Number{Text: "100"}},
		{`select price from trades where sym<>` + "`AAPL", "<>", Symbol{Name: "AAPL"}},
		{`select price from trades where active=true`, "=", Bool{Value: true}},
		{`select price from trades where stale=false`, "=", Bool{Value: false}},
		{`select price from trades where note=null`, "=", Null{}},
	}
	for _, tt := range tests {
		query := mustParse(t, tt.src)
		where, ok := query.Where.(Binary)
		if !ok || where.Op != tt.op {
			t.Fatalf("%s where = %#v", tt.src, query.Where)
		}
		if where.Right != tt.right {
			t.Fatalf("%s right = %#v, want %#v", tt.src, where.Right, tt.right)
		}
	}
}

func TestParseWhereCommaAndSemicolonForms(t *testing.T) {
	tests := []string{
		"select sym,price from trades where sym=`AAPL,price>=100",
		"select sym,price from trades where sym=`AAPL;price>=100",
	}
	for _, src := range tests {
		query := mustParse(t, src)
		where, ok := query.Where.(Binary)
		if !ok || where.Op != "and" {
			t.Fatalf("%s where = %#v, want implicit and", src, query.Where)
		}
		if left, ok := where.Left.(Binary); !ok || left.Op != "=" {
			t.Fatalf("%s left predicate = %#v", src, where.Left)
		}
		if right, ok := where.Right.(Binary); !ok || right.Op != ">=" {
			t.Fatalf("%s right predicate = %#v", src, where.Right)
		}
	}
}

func TestParseTemporalComparisonsAndTypedNulls(t *testing.T) {
	tests := []struct {
		src   string
		right any
	}{
		{`select price from trades where month=2024.01`, Temporal{Kind: "month", Text: "2024.01"}},
		{`select price from trades where month=2024.01m`, Temporal{Kind: "month", Text: "2024.01m"}},
		{`select price from trades where date=2024.01.02`, Temporal{Kind: "date", Text: "2024.01.02"}},
		{`select price from trades where dt=2024.01.02T09:30:00.001`, Temporal{Kind: "datetime", Text: "2024.01.02T09:30:00.001"}},
		{`select price from trades where span=1D09:30:00.001`, Temporal{Kind: "timespan", Text: "1D09:30:00.001"}},
		{`select price from trades where span=-0D00:01:00`, Binary{Op: "-", Left: Number{Text: "0"}, Right: Temporal{Kind: "timespan", Text: "0D00:01:00"}}},
		{`select price from trades where minute=09:30`, Temporal{Kind: "minute", Text: "09:30"}},
		{`select price from trades where second=09:30:00`, Temporal{Kind: "second", Text: "09:30:00"}},
		{`select price from trades where time=09:30:00.001`, Temporal{Kind: "time", Text: "09:30:00.001"}},
		{`select price from trades where ts=2024.01.02D09:30:00.001`, Temporal{Kind: "timestamp", Text: "2024.01.02D09:30:00.001"}},
		{`select price from trades where month=0Nm`, TypedNull{Kind: "month"}},
		{`select price from trades where date=0Nd`, TypedNull{Kind: "date"}},
		{`select price from trades where dt=0Nz`, TypedNull{Kind: "datetime"}},
		{`select price from trades where span=0Nn`, TypedNull{Kind: "timespan"}},
		{`select price from trades where minute=0Nu`, TypedNull{Kind: "minute"}},
		{`select price from trades where second=0Nv`, TypedNull{Kind: "second"}},
		{`select price from trades where time=0Nt`, TypedNull{Kind: "time"}},
		{`select price from trades where ts=0Np`, TypedNull{Kind: "timestamp"}},
	}
	for _, tt := range tests {
		query := mustParse(t, tt.src)
		where, ok := query.Where.(Binary)
		if !ok || where.Op != "=" {
			t.Fatalf("%s where = %#v", tt.src, query.Where)
		}
		if where.Right != tt.right {
			t.Fatalf("%s right = %#v, want %#v", tt.src, where.Right, tt.right)
		}
	}
}

func TestLowerAllNullTemporalVectorKinds(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select * from ([] m:0Nm 0Nm; d:0Nd 0Nd; z:0Nz 0Nz; n:0Nn 0Nn; u:0Nu 0Nu; v:0Nv 0Nv; t:0Nt 0Nt; p:0Np 0Np)"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Frame == nil {
		t.Fatalf("lowered frame is nil")
	}
	for name, want := range map[string]data.Kind{
		"m": data.KindMonth,
		"d": data.KindDate,
		"z": data.KindDateTime,
		"n": data.KindTimespan,
		"u": data.KindMinute,
		"v": data.KindSecond,
		"t": data.KindTime,
		"p": data.KindTimestamp,
	} {
		if got, ok := lowered.Frame.Schema().Kind(data.Symbol(name)); !ok || got != want {
			t.Fatalf("%s kind = %s, ok %v; want %s", name, got, ok, want)
		}
	}
}

func TestLowerTemporalTimeOfDayPromotion(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select * from ([] s:09:30 0Nv 09:30:01; t:09:30 0Nv 09:30:01.250)"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if got, ok := lowered.Frame.Schema().Kind("s"); !ok || got != data.KindSecond {
		t.Fatalf("s kind = %s, ok %v; want %s", got, ok, data.KindSecond)
	}
	if got, ok := lowered.Frame.Schema().Kind("t"); !ok || got != data.KindTime {
		t.Fatalf("t kind = %s, ok %v; want %s", got, ok, data.KindTime)
	}
}

func TestLowerSupportsBoolByteCharTypedNullSuffixes(t *testing.T) {
	lowered, err := Lower(mustParse(t, `select * from ([] b:0Nb true;x:0Nx 1;c:0Nc "a")`))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Frame == nil {
		t.Fatalf("lowered frame is nil")
	}
	for name, want := range map[data.Symbol]data.Kind{
		"b": data.KindBool,
		"x": data.KindU8,
		"c": data.KindString,
	} {
		if got, ok := lowered.Frame.Schema().Kind(name); !ok || got != want {
			t.Fatalf("%s kind = %s, ok %v; want %s", name, got, ok, want)
		}
	}
}

func TestParseSymbolVector(t *testing.T) {
	query := mustParse(t, "select syms:`AAPL`MSFT from trades")

	if len(query.Columns) != 1 || query.Columns[0].Name != "syms" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	vector, ok := query.Columns[0].Expr.(Vector)
	if !ok || len(vector.Items) != 2 {
		t.Fatalf("symbol vector = %#v", query.Columns[0].Expr)
	}
	if first, ok := vector.Items[0].(Symbol); !ok || first.Name != "AAPL" {
		t.Fatalf("first symbol = %#v", vector.Items[0])
	}
	if second, ok := vector.Items[1].(Symbol); !ok || second.Name != "MSFT" {
		t.Fatalf("second symbol = %#v", vector.Items[1])
	}
}

func TestParseDictLiteral(t *testing.T) {
	query := mustParse(t, "select mapping:`AAPL`MSFT!10 20 from trades")

	dict, ok := query.Columns[0].Expr.(DictExpr)
	if !ok {
		t.Fatalf("dict = %#v", query.Columns[0].Expr)
	}
	keys, ok := dict.Keys.(Vector)
	if !ok || len(keys.Items) != 2 {
		t.Fatalf("dict keys = %#v", dict.Keys)
	}
	values, ok := dict.Values.(Vector)
	if !ok || len(values.Items) != 2 {
		t.Fatalf("dict values = %#v", dict.Values)
	}
	if key, ok := keys.Items[0].(Symbol); !ok || key.Name != "AAPL" {
		t.Fatalf("first key = %#v", keys.Items[0])
	}
	if value, ok := values.Items[1].(Number); !ok || value.Text != "20" {
		t.Fatalf("second value = %#v", values.Items[1])
	}
}

func TestParseListIndexExpression(t *testing.T) {
	query := mustParse(t, "select picked:(10 20 30)[1] from trades")

	if len(query.Columns) != 1 || query.Columns[0].Name != "picked" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	index, ok := query.Columns[0].Expr.(IndexExpr)
	if !ok {
		t.Fatalf("index expr = %#v", query.Columns[0].Expr)
	}
	vector, ok := index.Expr.(Vector)
	if !ok || len(vector.Items) != 3 {
		t.Fatalf("indexed expr = %#v", index.Expr)
	}
	if item, ok := index.Index.(Number); !ok || item.Text != "1" {
		t.Fatalf("index = %#v", index.Index)
	}
}

func TestParseFlipTableLiteralFrom(t *testing.T) {
	query := mustParse(t, "select sym,price from ([] sym:`AAPL`MSFT; price:10 20)")

	if query.From != "" {
		t.Fatalf("from = %q, want table literal", query.From)
	}
	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v", query.FromExpr)
	}
	if len(flip.Columns) != 2 || flip.Columns[0].Name != "sym" || flip.Columns[1].Name != "price" {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	syms, ok := flip.Columns[0].Expr.(Vector)
	if !ok || len(syms.Items) != 2 {
		t.Fatalf("sym column = %#v", flip.Columns[0].Expr)
	}
	prices, ok := flip.Columns[1].Expr.(Vector)
	if !ok || len(prices.Items) != 2 {
		t.Fatalf("price column = %#v", flip.Columns[1].Expr)
	}
}

func TestParseNativeFlipDictTableLiteralFrom(t *testing.T) {
	query := mustParse(t, "select sym,price from flip `sym`price!(`AAPL`MSFT;10 20)")

	if query.From != "" {
		t.Fatalf("from = %q, want table literal", query.From)
	}
	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v, want Flip", query.FromExpr)
	}
	if len(flip.Columns) != 2 || flip.Columns[0].Name != "sym" || flip.Columns[1].Name != "price" {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	syms, ok := flip.Columns[0].Expr.(Vector)
	if !ok || len(syms.Items) != 2 {
		t.Fatalf("sym column = %#v", flip.Columns[0].Expr)
	}
	prices, ok := flip.Columns[1].Expr.(Vector)
	if !ok || len(prices.Items) != 2 {
		t.Fatalf("price column = %#v", flip.Columns[1].Expr)
	}
}

func TestParseNativeFlipDictRejectsMismatchedShape(t *testing.T) {
	for _, src := range []string{
		"select * from flip `sym`price!(`AAPL`MSFT)",
		"select * from flip sym!10 20",
	} {
		if _, err := Parse(src); err == nil {
			t.Fatalf("Parse(%q) returned nil error", src)
		}
	}
}

func TestParseFlipTableLiteralBoolVector(t *testing.T) {
	query := mustParse(t, "select sym,active from ([] sym:`AAPL`MSFT; active:true false)")
	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v, want Flip", query.FromExpr)
	}
	if len(flip.Columns) != 2 || flip.Columns[1].Name != "active" {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	active, ok := flip.Columns[1].Expr.(Vector)
	if !ok || len(active.Items) != 2 {
		t.Fatalf("active column = %#v", flip.Columns[1].Expr)
	}
	if first, ok := active.Items[0].(Bool); !ok || !first.Value {
		t.Fatalf("active[0] = %#v, want true", active.Items[0])
	}
	if second, ok := active.Items[1].(Bool); !ok || second.Value {
		t.Fatalf("active[1] = %#v, want false", active.Items[1])
	}
}

func TestParseFlipTableLiteralTemporalTypedNullVectors(t *testing.T) {
	src := "select sym,d,t,ts,qty,px from ([] sym:`AAPL`MSFT; d:2026.06.06 0Nd; t:09:30:00.250 0Nt; ts:2026.06.06D09:30:00.250 0Np; qty:10 0Ni; px:100.5 0N)"
	query, err := Parse(src)
	if err != nil {
		tokens, lexErr := lex(src)
		if lexErr == nil {
			t.Logf("tokens = %#v", tokens)
		}
		t.Fatalf("Parse(%q) returned error: %v", src, err)
	}
	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v, want Flip", query.FromExpr)
	}
	if len(flip.Columns) != 6 {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	for i, name := range []string{"sym", "d", "t", "ts", "qty", "px"} {
		if flip.Columns[i].Name != name {
			t.Fatalf("column %d name = %q, want %q", i, flip.Columns[i].Name, name)
		}
		vector, ok := flip.Columns[i].Expr.(Vector)
		if !ok || len(vector.Items) != 2 {
			t.Fatalf("column %s expr = %#v, want two-item vector", name, flip.Columns[i].Expr)
		}
	}
}

func TestParseKeyedTableLiteralFrom(t *testing.T) {
	query := mustParse(t, "select sym,price from ([sym:`AAPL`MSFT] price:10 20; size:100 200)")

	if query.From != "" {
		t.Fatalf("from = %q, want table literal", query.From)
	}
	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v", query.FromExpr)
	}
	if len(flip.Keys) != 1 || flip.Keys[0].Name != "sym" {
		t.Fatalf("flip keys = %#v", flip.Keys)
	}
	if len(flip.Columns) != 2 || flip.Columns[0].Name != "price" || flip.Columns[1].Name != "size" {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	syms, ok := flip.Keys[0].Expr.(Vector)
	if !ok || len(syms.Items) != 2 {
		t.Fatalf("sym key column = %#v", flip.Keys[0].Expr)
	}
	prices, ok := flip.Columns[0].Expr.(Vector)
	if !ok || len(prices.Items) != 2 {
		t.Fatalf("price column = %#v", flip.Columns[0].Expr)
	}
}

func TestParseKeyedTableLiteralPreservesKeyAndValueColumnOrder(t *testing.T) {
	query := mustParse(t, "select venue,sym,price,size from ([venue:`XNYS`XNAS; sym:`AAPL`MSFT] price:100.5 80.25; size:10 20)")

	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v, want Flip", query.FromExpr)
	}
	if len(flip.Keys) != 2 || flip.Keys[0].Name != "venue" || flip.Keys[1].Name != "sym" {
		t.Fatalf("flip keys = %#v", flip.Keys)
	}
	if len(flip.Columns) != 2 || flip.Columns[0].Name != "price" || flip.Columns[1].Name != "size" {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	for i, column := range append(append([]Column(nil), flip.Keys...), flip.Columns...) {
		vector, ok := column.Expr.(Vector)
		if !ok || len(vector.Items) != 2 {
			t.Fatalf("ordered column %d %q expr = %#v, want two-item vector", i, column.Name, column.Expr)
		}
	}
}

func TestParseNativeTableLiteralAllowsWhitespaceAndTrailingSemicolons(t *testing.T) {
	query := mustParse(t, "select venue,sym,price,size from ( [ venue:`XNYS`XNAS; sym:`AAPL`MSFT; ] price:100 101; size:10 20; )")

	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v, want Flip", query.FromExpr)
	}
	if len(flip.Keys) != 2 || flip.Keys[0].Name != "venue" || flip.Keys[1].Name != "sym" {
		t.Fatalf("flip keys = %#v", flip.Keys)
	}
	if len(flip.Columns) != 2 || flip.Columns[0].Name != "price" || flip.Columns[1].Name != "size" {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
}

func TestParseKeyedTableLiteralDiagnostics(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{
			src:  "select * from ([sym:`AAPL`MSFT; `venue:1 2] price:10 20)",
			want: "expected keyed table key assignment",
		},
		{
			src:  "select * from ([sym:`AAPL`MSFT price:10 20)",
			want: "expected ; or closing delimiter after keyed table key",
		},
		{
			src:  "select * from ([sym:`AAPL`MSFT] `price:10 20)",
			want: "expected table column assignment",
		},
		{
			src:  "select * from ([] price:10 20 size:100 200)",
			want: "expected ; or closing delimiter after table column",
		},
	}
	for _, tt := range tests {
		_, err := Parse(tt.src)
		if err == nil {
			t.Fatalf("Parse(%q) returned nil error", tt.src)
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("Parse(%q) error = %q, want contains %q", tt.src, err.Error(), tt.want)
		}
	}
}

func TestParseSelectAllFromKeyedTableLiteralWithTrailingClauses(t *testing.T) {
	query := mustParse(t, "select * from ([sym:`AAPL`MSFT] price:10 20; size:100 200) where sym=`AAPL order by price desc limit 1")

	if query.From != "" {
		t.Fatalf("from = %q, want table literal", query.From)
	}
	if len(query.Columns) != 1 || query.Columns[0].Name != "*" {
		t.Fatalf("columns = %#v", query.Columns)
	}
	if _, ok := query.Columns[0].Expr.(AllColumns); !ok {
		t.Fatalf("column expr = %#v, want AllColumns", query.Columns[0].Expr)
	}
	flip, ok := query.FromExpr.(Flip)
	if !ok {
		t.Fatalf("from expr = %#v", query.FromExpr)
	}
	if len(flip.Keys) != 1 || flip.Keys[0].Name != "sym" {
		t.Fatalf("flip keys = %#v", flip.Keys)
	}
	if _, ok := query.Where.(Binary); !ok {
		t.Fatalf("where = %#v", query.Where)
	}
	if len(query.OrderBy) != 1 || query.OrderBy[0].Column != "price" || !query.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", query.OrderBy)
	}
	if query.Limit == nil || *query.Limit != 1 {
		t.Fatalf("limit = %#v", query.Limit)
	}
}

func TestLowerSelectPlan(t *testing.T) {
	query := mustParse(t, "select notional:sum price*size, fills:count i by sym from trades where price>100")
	plan, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}

	if plan.Op != SelectQuery || plan.Source != "trades" || plan.Original != query {
		t.Fatalf("plan header = %#v", plan)
	}
	if len(plan.Plan.Aggregates) != 2 || plan.Plan.Aggregates[0].Name != "notional" || plan.Plan.Aggregates[1].Name != "fills" {
		t.Fatalf("plan aggregates = %#v", plan.Plan.Aggregates)
	}
	if len(plan.Plan.ByExprs) != 1 || plan.Plan.ByExprs[0].Name != "sym" {
		t.Fatalf("plan group by = %#v", plan.Plan.ByExprs)
	}
	if plan.Plan.Where == nil {
		t.Fatalf("plan filter is nil")
	}
}

func TestLowerOrderLimitPlan(t *testing.T) {
	query := mustParse(t, "select sym,price,size from trades where size>=10 order by price desc limit 2")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}

	if len(lowered.Plan.OrderBy) != 1 || lowered.Plan.OrderBy[0].Column != "price" || !lowered.Plan.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", lowered.Plan.OrderBy)
	}
	if lowered.Plan.LimitN != 2 {
		t.Fatalf("limit = %d, want 2", lowered.Plan.LimitN)
	}
}

func TestLowerDistinctAndTakePlan(t *testing.T) {
	query := mustParse(t, "select distinct sym,price from trades order by price desc take 2")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}

	if !lowered.Distinct {
		t.Fatalf("distinct = false")
	}
	if lowered.Plan.LimitN != 2 {
		t.Fatalf("limit = %d, want 2", lowered.Plan.LimitN)
	}
	if len(lowered.Plan.Select) != 2 || lowered.Plan.Select[0].Name != "sym" || lowered.Plan.Select[1].Name != "price" {
		t.Fatalf("select = %#v", lowered.Plan.Select)
	}
}

func TestLowerSelectByWithoutProjectionUsesDistinctKeyProjection(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select by sym,bucket from trades order by sym asc"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if !lowered.Distinct || !lowered.Plan.Distinct {
		t.Fatalf("distinct = %v / %v, want true", lowered.Distinct, lowered.Plan.Distinct)
	}
	if len(lowered.Plan.ByExprs) != 0 || len(lowered.Plan.Aggregates) != 0 {
		t.Fatalf("grouped fields = by %#v aggregates %#v, want projection-only distinct", lowered.Plan.ByExprs, lowered.Plan.Aggregates)
	}
	if len(lowered.Plan.Select) != 2 || lowered.Plan.Select[0].Name != "sym" || lowered.Plan.Select[1].Name != "bucket" {
		t.Fatalf("select = %#v", lowered.Plan.Select)
	}
	if len(lowered.Plan.OrderBy) != 1 || lowered.Plan.OrderBy[0].Column != "sym" {
		t.Fatalf("order by = %#v", lowered.Plan.OrderBy)
	}
}

func TestLowerSelectByWithoutProjectionFromKeyedLiteralExecutes(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select by sym,bucket from ([sym:`AAPL`AAPL`MSFT; bucket:`m1`m1`m2] px:100 101 80; size:10 11 20) where px>=100,size>=10 order by sym asc,bucket asc take 1"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if !lowered.Distinct || !lowered.Plan.Distinct {
		t.Fatalf("distinct = %v / %v, want true", lowered.Distinct, lowered.Plan.Distinct)
	}
	if lowered.Frame == nil {
		t.Fatalf("lowered frame is nil")
	}
	if len(lowered.Plan.Select) != 2 || lowered.Plan.Select[0].Name != "sym" || lowered.Plan.Select[1].Name != "bucket" {
		t.Fatalf("select = %#v", lowered.Plan.Select)
	}
	if lowered.Plan.LimitN != 1 {
		t.Fatalf("limit = %d, want 1", lowered.Plan.LimitN)
	}
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 1 {
		t.Fatalf("out len = %d, want 1", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("AAPL"))
	assertColumnValue(t, out, "bucket", 0, data.Symbol("m1"))
}

func TestLowerInnerJoinPlan(t *testing.T) {
	query := mustParse(t, "select sym,price,bid from trades join quotes on sym")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "inner" || lowered.Join.Right != "quotes" {
		t.Fatalf("join = %#v", lowered.Join)
	}
	if len(lowered.Join.Keys) != 1 || lowered.Join.Keys[0].Left != "sym" || lowered.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", lowered.Join.Keys)
	}
}

func TestLowerInnerJoinAliasPlan(t *testing.T) {
	query := mustParse(t, "select sym,price,bid from trades inner join quotes on sym")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "inner" || lowered.Join.Right != "quotes" {
		t.Fatalf("join = %#v", lowered.Join)
	}
	if len(lowered.Join.Keys) != 1 || lowered.Join.Keys[0].Left != "sym" || lowered.Join.Keys[0].Right != "sym" {
		t.Fatalf("join keys = %#v", lowered.Join.Keys)
	}
}

func TestLowerInnerJoinWithMultipleAliasedKeys(t *testing.T) {
	query := mustParse(t, "select id,value,qty from accounts join fills on id=account_id,venue=exchange")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "inner" || lowered.Join.Right != "fills" {
		t.Fatalf("join = %#v", lowered.Join)
	}
	want := []data.JoinKey{
		{Left: "id", Right: "account_id"},
		{Left: "venue", Right: "exchange"},
	}
	if len(lowered.Join.Keys) != len(want) {
		t.Fatalf("join keys len = %d, want %d: %#v", len(lowered.Join.Keys), len(want), lowered.Join.Keys)
	}
	for i := range want {
		if lowered.Join.Keys[i] != want[i] {
			t.Fatalf("join key %d = %#v, want %#v", i, lowered.Join.Keys[i], want[i])
		}
	}
}

func TestLowerJoinAliasesKeepTrailingClauses(t *testing.T) {
	query := mustParse(t, "select id,value,qty from accounts lj fills on id=account_id,venue=exchange where value>0 order by qty desc take 3")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "left" || lowered.Join.Right != "fills" {
		t.Fatalf("join = %#v", lowered.Join)
	}
	want := []data.JoinKey{
		{Left: "id", Right: "account_id"},
		{Left: "venue", Right: "exchange"},
	}
	if !reflect.DeepEqual(lowered.Join.Keys, want) {
		t.Fatalf("join keys = %#v, want %#v", lowered.Join.Keys, want)
	}
	if lowered.Plan.Where == nil {
		t.Fatalf("where is nil")
	}
	if len(lowered.Plan.OrderBy) != 1 || lowered.Plan.OrderBy[0].Column != "qty" || !lowered.Plan.OrderBy[0].Desc {
		t.Fatalf("order by = %#v", lowered.Plan.OrderBy)
	}
	if lowered.Plan.LimitN != 3 {
		t.Fatalf("limit = %d, want 3", lowered.Plan.LimitN)
	}
}

func TestLowerAsofJoin(t *testing.T) {
	query := mustParse(t, "select sym,ts,price,bid from trades asof join quotes on sym,ts")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "asof" || lowered.Join.Right != "quotes" {
		t.Fatalf("join = %#v", lowered.Join)
	}
	want := []data.JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}}
	if len(lowered.Join.Keys) != len(want) {
		t.Fatalf("join keys len = %d, want %d: %#v", len(lowered.Join.Keys), len(want), lowered.Join.Keys)
	}
	for i := range want {
		if lowered.Join.Keys[i] != want[i] {
			t.Fatalf("join key %d = %#v, want %#v", i, lowered.Join.Keys[i], want[i])
		}
	}
}

func TestLowerAsofJoinVariantAliases(t *testing.T) {
	cases := map[string]string{
		"aj":   "asof",
		"aj0":  "asof0",
		"ajf":  "asof_fill",
		"ajf0": "asof_fill0",
	}
	for keyword, wantKind := range cases {
		lowered, err := Lower(mustParse(t, "select sym,ts,price,bid from trades "+keyword+" quotes on sym,ts"))
		if err != nil {
			t.Fatalf("Lower(%s) returned error: %v", keyword, err)
		}
		if lowered.Join == nil || lowered.Join.Kind != wantKind || lowered.Join.Right != "quotes" {
			t.Fatalf("%s join = %#v", keyword, lowered.Join)
		}
		want := []data.JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}}
		if !reflect.DeepEqual(lowered.Join.Keys, want) {
			t.Fatalf("%s join keys = %#v, want %#v", keyword, lowered.Join.Keys, want)
		}
	}
}

func TestLowerAsofJoinVariantAliasesPreserveVariantKind(t *testing.T) {
	const queryTail = " quotes on sym=ticker,ts=quote_time where bid>0 order by ts desc take 2"
	cases := map[string]string{
		"aj":   "asof",
		"aj0":  "asof0",
		"ajf":  "asof_fill",
		"ajf0": "asof_fill0",
	}
	var baselinePlan *data.QueryPlan
	for keyword, wantKind := range cases {
		lowered, err := Lower(mustParse(t, "select sym,ts,bid from trades "+keyword+queryTail))
		if err != nil {
			t.Fatalf("Lower(%s) returned error: %v", keyword, err)
		}
		if lowered.Join == nil || lowered.Join.Kind != wantKind {
			t.Fatalf("%s join = %#v, want %s", keyword, lowered.Join, wantKind)
		}
		if baselinePlan == nil {
			baselinePlan = &lowered.Plan
			continue
		}
		if !reflect.DeepEqual(lowered.Plan.OrderBy, baselinePlan.OrderBy) || lowered.Plan.LimitN != baselinePlan.LimitN {
			t.Fatalf("%s trailing plan = order %#v limit %d, want order %#v limit %d", keyword, lowered.Plan.OrderBy, lowered.Plan.LimitN, baselinePlan.OrderBy, baselinePlan.LimitN)
		}
	}
}

func TestLowerAsofJoinVariantKinds(t *testing.T) {
	cases := map[string]string{
		"aj":   "asof",
		"aj0":  "asof0",
		"ajf":  "asof_fill",
		"ajf0": "asof_fill0",
	}
	for kind, wantKind := range cases {
		lowered, err := Lower(&Query{
			Kind: SelectQuery,
			Columns: []Column{
				{Name: "sym", Expr: Ident{Name: "sym"}},
			},
			From: "trades",
			Join: &Join{
				Kind:  kind,
				Right: "quotes",
				Keys:  []JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}},
			},
		})
		if err != nil {
			t.Fatalf("Lower join kind %s returned error: %v", kind, err)
		}
		if lowered.Join == nil || lowered.Join.Kind != wantKind {
			t.Fatalf("Lower join kind %s = %#v, want %s", kind, lowered.Join, wantKind)
		}
	}
}

func TestLowerUnionPlusAndWindowJoinPlans(t *testing.T) {
	cases := []struct {
		src      string
		kind     string
		wantKeys []data.JoinKey
	}{
		{"select sym,venue from left uj right on sym", "union", []data.JoinKey{{Left: "sym", Right: "sym"}}},
		{"select sym,qty from left pj right on sym", "plus", []data.JoinKey{{Left: "sym", Right: "sym"}}},
		{"select sym,qty from left wj right on sym,ts", "window", []data.JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}}},
	}
	for _, tc := range cases {
		lowered, err := Lower(mustParse(t, tc.src))
		if err != nil {
			t.Fatalf("Lower(%q) returned error: %v", tc.src, err)
		}
		if lowered.Join == nil || lowered.Join.Kind != tc.kind || lowered.Join.Right != "right" {
			t.Fatalf("Lower(%q) join = %#v, want %s right", tc.src, lowered.Join, tc.kind)
		}
		if !reflect.DeepEqual(lowered.Join.Keys, tc.wantKeys) {
			t.Fatalf("Lower(%q) join keys = %#v", tc.src, lowered.Join.Keys)
		}
	}
}

func TestLowerWindowJoinBoundsAndWJ1(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select sym,ts,bid from trades wj1[-5 0] quotes on sym,ts"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "window1" || lowered.Join.Right != "quotes" {
		t.Fatalf("join = %#v", lowered.Join)
	}
	if !lowered.Join.HasWindow || lowered.Join.WindowLow != int64(-5) || lowered.Join.WindowHigh != int64(0) {
		t.Fatalf("window bounds = has %v low %#v high %#v", lowered.Join.HasWindow, lowered.Join.WindowLow, lowered.Join.WindowHigh)
	}

	timespanLowered, err := Lower(mustParse(t, "select sym,ts,bid from trades wj[-0D00:01:00 0D00:00:00] quotes on sym,ts"))
	if err != nil {
		t.Fatalf("Lower timespan bounds returned error: %v", err)
	}
	if !timespanLowered.Join.HasWindow {
		t.Fatalf("timespan bounds missing: %#v", timespanLowered.Join)
	}
	if got, want := timespanLowered.Join.WindowLow, data.TimespanFromNanos(-60*1_000_000_000); got != want {
		t.Fatalf("timespan low = %#v, want %#v", got, want)
	}
	if got, want := timespanLowered.Join.WindowHigh, data.TimespanFromNanos(0); got != want {
		t.Fatalf("timespan high = %#v, want %#v", got, want)
	}
}

func TestLowerWindowJoinCommaSeparatedBounds(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select sym,ts,bid from trades wj[-5,0] quotes on sym,ts"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Join == nil || lowered.Join.Kind != "window" || !lowered.Join.HasWindow {
		t.Fatalf("join = %#v", lowered.Join)
	}
	if lowered.Join.WindowLow != int64(-5) || lowered.Join.WindowHigh != int64(0) {
		t.Fatalf("window bounds = low %#v high %#v", lowered.Join.WindowLow, lowered.Join.WindowHigh)
	}

	timespanLowered, err := Lower(mustParse(t, "select sym,ts,bid from trades wj[-0D00:01:00,0D00:00:00] quotes on sym,ts"))
	if err != nil {
		t.Fatalf("Lower timespan bounds returned error: %v", err)
	}
	if got, want := timespanLowered.Join.WindowLow, data.TimespanFromNanos(-60*1_000_000_000); got != want {
		t.Fatalf("timespan low = %#v, want %#v", got, want)
	}
	if got, want := timespanLowered.Join.WindowHigh, data.TimespanFromNanos(0); got != want {
		t.Fatalf("timespan high = %#v, want %#v", got, want)
	}
}

func TestLowerAsofAndWindowAliasedPartitionAndTimeKeys(t *testing.T) {
	for _, src := range []string{
		"select sym,ts,bid from trades aj quotes on sym=ticker,ts=quote_time where bid>0 order by sym asc take 2",
		"select sym,ts,bid from trades aj0 quotes on sym=ticker,ts=quote_time where bid>0 order by sym asc take 2",
		"select sym,ts,bid from trades ajf quotes on sym=ticker,ts=quote_time where bid>0 order by sym asc take 2",
		"select sym,ts,bid from trades ajf0 quotes on sym=ticker,ts=quote_time where bid>0 order by sym asc take 2",
		"select sym,ts,bid from trades wj[-0D00:01:00,0D00:00:00] quotes on sym=ticker,ts=quote_time where bid>0 order by sym asc take 2",
		"select sym,ts,bid from trades wj1[-0D00:01:00,0D00:00:00] quotes on sym=ticker,ts=quote_time where bid>0 order by sym asc take 2",
	} {
		lowered, err := Lower(mustParse(t, src))
		if err != nil {
			t.Fatalf("Lower(%q) returned error: %v", src, err)
		}
		if lowered.Join == nil {
			t.Fatalf("Lower(%q) join is nil", src)
		}
		wantKeys := []data.JoinKey{{Left: "sym", Right: "ticker"}, {Left: "ts", Right: "quote_time"}}
		if !reflect.DeepEqual(lowered.Join.Keys, wantKeys) {
			t.Fatalf("Lower(%q) join keys = %#v, want %#v", src, lowered.Join.Keys, wantKeys)
		}
		if lowered.Plan.Where == nil || len(lowered.Plan.OrderBy) != 1 || lowered.Plan.LimitN != 2 {
			t.Fatalf("Lower(%q) trailing clauses not preserved: where=%#v order=%#v limit=%d", src, lowered.Plan.Where, lowered.Plan.OrderBy, lowered.Plan.LimitN)
		}
	}
}

func TestLowerRejectsWindowBoundsOnNonWindowJoin(t *testing.T) {
	_, err := Lower(&Query{
		Kind: SelectQuery,
		Columns: []Column{
			{Name: "sym", Expr: Ident{Name: "sym"}},
		},
		From: "trades",
		Join: &Join{
			Kind:  "asof",
			Right: "quotes",
			Keys:  []JoinKey{{Left: "sym", Right: "sym"}, {Left: "ts", Right: "ts"}},
			Window: &WindowBounds{
				Low:  Number{Text: "-5"},
				High: Number{Text: "0"},
			},
		},
	})
	if err == nil {
		t.Fatal("Lower accepted window bounds on asof join")
	}
	if !strings.Contains(err.Error(), "q window bounds are only valid for window joins") {
		t.Fatalf("Lower error = %q", err.Error())
	}
}

func TestLowerXbar(t *testing.T) {
	query := mustParse(t, "select qty:sum size by bucket:xbar 10 ts from trades")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if len(lowered.Plan.ByExprs) != 1 || lowered.Plan.ByExprs[0].Name != "bucket" {
		t.Fatalf("by = %#v", lowered.Plan.ByExprs)
	}
	if len(lowered.Plan.Aggregates) != 1 || lowered.Plan.Aggregates[0].Name != "qty" {
		t.Fatalf("aggregates = %#v", lowered.Plan.Aggregates)
	}
	if _, ok := lowered.Plan.ByExprs[0].Expr.(data.BucketFloorExpr); !ok {
		t.Fatalf("bucket expr = %#v", lowered.Plan.ByExprs[0].Expr)
	}
}

func TestLowerXbarComputedExpressionExecutes(t *testing.T) {
	query := mustParse(t, "select n:count i by bucket:xbar 10 price+size from trades order by bucket asc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	bucket, ok := lowered.Plan.ByExprs[0].Expr.(data.BucketFloorExpr)
	if !ok {
		t.Fatalf("bucket expr = %#v", lowered.Plan.ByExprs[0].Expr)
	}
	if _, ok := bucket.Expr.(data.Binary); !ok {
		t.Fatalf("bucket inner expr = %#v, want binary expression", bucket.Expr)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("price", []any{11, 18, 30}),
		data.NewColumn("size", []any{1, 4, 5}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertQColumnValues(t, out, "bucket", []any{10.0, 20.0, 30.0})
	assertQColumnValues(t, out, "n", []any{int64(1), int64(1), int64(1)})
}

func TestLowerXbarTemporalBucketExecutesWithNulls(t *testing.T) {
	query := mustParse(t, "select n:count size by bucket:xbar 0D00:00:00.000001 ts from trades")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("ts", []any{
			nil,
			data.TimestampFromUnixNanos(1),
			data.TimestampFromUnixNanos(999),
			data.TimestampFromUnixNanos(1_000),
		}),
		data.NewColumn("size", []any{10, 20, 30, 40}),
	)

	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}

	assertQColumnValues(t, out, "bucket", []any{data.NullValue, data.Timestamp(0), data.Timestamp(1_000)})
	assertQColumnValues(t, out, "n", []any{int64(1), int64(2), int64(1)})
	if kind, ok := out.Schema().Kind("bucket"); !ok || kind != data.KindTimestamp {
		t.Fatalf("bucket kind = %s, ok %v; want %s", kind, ok, data.KindTimestamp)
	}
}

func TestLowerKeyedTableLiteralSource(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select sym,price from ([sym:`AAPL`MSFT] price:10.5 20.5) where price>11"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Frame == nil {
		t.Fatalf("lowered frame is nil")
	}
	names := lowered.Frame.Schema().Names()
	if len(names) != 2 || names[0] != "sym" || names[1] != "price" {
		t.Fatalf("schema names = %#v", names)
	}
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 1 {
		t.Fatalf("out len = %d, want 1", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("MSFT"))
	assertColumnValue(t, out, "price", 0, 20.5)
}

func TestLowerKeyedTableLiteralPreservesMultiKeyColumnOrder(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select * from ([venue:`XNYS`XNAS; sym:`AAPL`MSFT] price:100.5 80.25; size:10 20)"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Frame == nil {
		t.Fatalf("lowered frame is nil")
	}
	names := lowered.Frame.Schema().Names()
	want := []data.Symbol{"venue", "sym", "price", "size"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("literal frame schema names = %#v, want %#v", names, want)
	}
	if !reflect.DeepEqual(lowered.LiteralKeys, []data.Symbol{"venue", "sym"}) {
		t.Fatalf("literal keys = %#v", lowered.LiteralKeys)
	}
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if !reflect.DeepEqual(out.Schema().Names(), want) {
		t.Fatalf("exec schema names = %#v, want %#v", out.Schema().Names(), want)
	}
	assertColumnValue(t, out, "venue", 0, data.Symbol("XNYS"))
	assertColumnValue(t, out, "sym", 1, data.Symbol("MSFT"))
	assertColumnValue(t, out, "price", 0, 100.5)
	assertColumnValue(t, out, "size", 1, int64(20))
}

func TestLowerTableLiteralSupportsNegativeStaticVectors(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select * from ([] sym:`A`B`C; offset:(0.5;1.0;0-2.0); qty:(10;20;0-30))"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.Frame == nil {
		t.Fatalf("lowered frame is nil")
	}
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	assertColumnValue(t, out, "offset", 0, 0.5)
	assertColumnValue(t, out, "offset", 2, -2.0)
	assertColumnValue(t, out, "qty", 0, int64(10))
	assertColumnValue(t, out, "qty", 2, int64(-30))
}

func TestLowerSelectAllKeyedTableLiteralPreservesKeyColumns(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select * from ([sym:`AAPL`MSFT] price:10.5 20.5; size:100 200) where sym=`AAPL"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	names := out.Schema().Names()
	wantNames := []data.Symbol{"sym", "price", "size"}
	if !reflect.DeepEqual(names, wantNames) {
		t.Fatalf("schema names = %#v, want %#v", names, wantNames)
	}
	if out.Len() != 1 {
		t.Fatalf("out len = %d, want 1", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("AAPL"))
	assertColumnValue(t, out, "price", 0, 10.5)
	assertColumnValue(t, out, "size", 0, int64(100))
}

func TestLowerOmittedProjectionTableLiteralExpandsAllColumns(t *testing.T) {
	tests := []string{
		"select from ([sym:`AAPL`MSFT] price:10.5 20.5; size:100 200) where sym=`AAPL",
		"exec from ([sym:`AAPL`MSFT] price:10.5 20.5; size:100 200) where sym=`AAPL",
	}
	for _, src := range tests {
		lowered, err := Lower(mustParse(t, src))
		if err != nil {
			t.Fatalf("Lower(%q) returned error: %v", src, err)
		}
		out, err := lowered.Plan.Exec()
		if err != nil {
			t.Fatalf("Exec(%q) returned error: %v", src, err)
		}
		names := out.Schema().Names()
		wantNames := []data.Symbol{"sym", "price", "size"}
		if !reflect.DeepEqual(names, wantNames) {
			t.Fatalf("%q schema names = %#v, want %#v", src, names, wantNames)
		}
		if out.Len() != 1 {
			t.Fatalf("%q out len = %d, want 1", src, out.Len())
		}
		assertColumnValue(t, out, "sym", 0, data.Symbol("AAPL"))
		assertColumnValue(t, out, "price", 0, 10.5)
		assertColumnValue(t, out, "size", 0, int64(100))
	}
}

func TestLowerUpdateMutationPlan(t *testing.T) {
	query := mustParse(t, "update price:price*1.1,size:size+1 from trades where sym=`AAPL")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}

	if lowered.Op != UpdateQuery || lowered.Source != "trades" || lowered.Original != query {
		t.Fatalf("lowered header = %#v", lowered)
	}
	if lowered.Mutation == nil || lowered.Mutation.Kind != UpdateQuery {
		t.Fatalf("mutation = %#v", lowered.Mutation)
	}
	if lowered.Mutation.Where == nil {
		t.Fatalf("mutation where is nil")
	}
	if len(lowered.Mutation.Assignments) != 2 {
		t.Fatalf("assignments = %#v", lowered.Mutation.Assignments)
	}
	if lowered.Mutation.Assignments[0].Name != "price" || lowered.Mutation.Assignments[1].Name != "size" {
		t.Fatalf("assignment names = %#v", lowered.Mutation.Assignments)
	}
	if _, ok := lowered.Mutation.Assignments[0].Expr.(data.Binary); !ok {
		t.Fatalf("price assignment expr = %#v", lowered.Mutation.Assignments[0].Expr)
	}
}

func TestLowerSignedLiteralPredicatesAndMutation(t *testing.T) {
	lowered, err := Lower(mustParse(t, "select price from trades where price within -5 0"))
	if err != nil {
		t.Fatalf("Lower signed within returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t, data.NewColumn("price", []any{-10, -5, -1, 0, 1}))
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec signed within returned error: %v", err)
	}
	if out.Len() != 3 {
		t.Fatalf("out len = %d, want 3", out.Len())
	}
	assertColumnValue(t, out, "price", 0, int64(-5))
	assertColumnValue(t, out, "price", 2, int64(0))

	update, err := Lower(mustParse(t, "update price:-1 from trades where price>-5"))
	if err != nil {
		t.Fatalf("Lower signed update returned error: %v", err)
	}
	if update.Mutation == nil || len(update.Mutation.Assignments) != 1 {
		t.Fatalf("mutation = %#v", update.Mutation)
	}
	if _, ok := update.Mutation.Assignments[0].Expr.(data.Binary); !ok {
		t.Fatalf("assignment expr = %#v, want lowered negative literal expression", update.Mutation.Assignments[0].Expr)
	}
	if update.Mutation.Where == nil {
		t.Fatalf("mutation where is nil")
	}
}

func TestLowerDeleteMutationPlans(t *testing.T) {
	rowDelete, err := Lower(mustParse(t, "delete from trades where price<100"))
	if err != nil {
		t.Fatalf("Lower row delete returned error: %v", err)
	}
	if rowDelete.Mutation == nil || rowDelete.Mutation.Kind != DeleteQuery {
		t.Fatalf("row delete mutation = %#v", rowDelete.Mutation)
	}
	if rowDelete.Mutation.Where == nil {
		t.Fatalf("row delete where is nil")
	}
	if len(rowDelete.Mutation.DeleteColumns) != 0 {
		t.Fatalf("row delete columns = %#v", rowDelete.Mutation.DeleteColumns)
	}

	columnDelete, err := Lower(mustParse(t, "delete price,size from trades"))
	if err != nil {
		t.Fatalf("Lower column delete returned error: %v", err)
	}
	if columnDelete.Mutation == nil || columnDelete.Mutation.Kind != DeleteQuery {
		t.Fatalf("column delete mutation = %#v", columnDelete.Mutation)
	}
	if len(columnDelete.Mutation.DeleteColumns) != 2 || columnDelete.Mutation.DeleteColumns[0] != "price" || columnDelete.Mutation.DeleteColumns[1] != "size" {
		t.Fatalf("delete columns = %#v", columnDelete.Mutation.DeleteColumns)
	}
}

func TestLowerInsertUpsertMutationPlans(t *testing.T) {
	insert, err := Lower(mustParse(t, "insert into trades (sym,price) values (`AAPL,100)"))
	if err != nil {
		t.Fatalf("Lower insert returned error: %v", err)
	}
	if insert.Mutation == nil || insert.Mutation.Kind != InsertQuery {
		t.Fatalf("insert mutation = %#v", insert.Mutation)
	}
	if len(insert.Mutation.InsertColumns) != 2 || insert.Mutation.InsertColumns[0] != "sym" || insert.Mutation.InsertColumns[1] != "price" {
		t.Fatalf("insert columns = %#v", insert.Mutation.InsertColumns)
	}
	if len(insert.Mutation.InsertValues) != 2 || insert.Mutation.InsertValues[0].Value != data.Symbol("AAPL") || insert.Mutation.InsertValues[1].Value != int64(100) {
		t.Fatalf("insert values = %#v", insert.Mutation.InsertValues)
	}

	upsert, err := Lower(mustParse(t, "upsert into trades values (`MSFT,101)"))
	if err != nil {
		t.Fatalf("Lower upsert returned error: %v", err)
	}
	if upsert.Mutation == nil || upsert.Mutation.Kind != UpsertQuery {
		t.Fatalf("upsert mutation = %#v", upsert.Mutation)
	}
	if len(upsert.Mutation.InsertColumns) != 0 || len(upsert.Mutation.InsertValues) != 2 {
		t.Fatalf("upsert payload = columns %#v values %#v", upsert.Mutation.InsertColumns, upsert.Mutation.InsertValues)
	}
}

func TestLowerUpdateDeleteTableLiteralMutationSource(t *testing.T) {
	updated, err := Lower(mustParse(t, "update px:px+1 from ([] sym:`AAPL`MSFT; px:10 20)"))
	if err != nil {
		t.Fatalf("Lower update returned error: %v", err)
	}
	if updated.Frame == nil {
		t.Fatalf("updated frame is nil")
	}
	if updated.Mutation == nil || updated.Mutation.Kind != UpdateQuery {
		t.Fatalf("updated mutation = %#v", updated.Mutation)
	}
	assignments := make(map[data.Symbol]data.Expr, len(updated.Mutation.Assignments))
	for _, assign := range updated.Mutation.Assignments {
		assignments[assign.Name] = assign.Expr
	}
	updatedFrame, err := data.UpdateWhere(*updated.Frame, updated.Mutation.Where, assignments)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	assertQColumnValues(t, updatedFrame, "sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT")})
	assertQColumnValues(t, updatedFrame, "px", []any{int64(11), int64(21)})

	deleted, err := Lower(mustParse(t, "delete from ([] sym:`AAPL`MSFT; px:10 20) where sym=`MSFT"))
	if err != nil {
		t.Fatalf("Lower delete returned error: %v", err)
	}
	if deleted.Frame == nil {
		t.Fatalf("deleted frame is nil")
	}
	if deleted.Mutation == nil || deleted.Mutation.Kind != DeleteQuery {
		t.Fatalf("deleted mutation = %#v", deleted.Mutation)
	}
	deletedFrame, err := data.DeleteWhere(*deleted.Frame, deleted.Mutation.Where)
	if err != nil {
		t.Fatalf("DeleteWhere returned error: %v", err)
	}
	assertQColumnValues(t, deletedFrame, "sym", []any{data.Symbol("AAPL")})
	assertQColumnValues(t, deletedFrame, "px", []any{int64(10)})
}

func TestLowerUpdateDeleteKeyedTableLiteralMutationSource(t *testing.T) {
	updated, err := Lower(mustParse(t, "update price:price+1 from ([sym:`AAPL`MSFT] price:100 80; size:10 20) where sym=`AAPL"))
	if err != nil {
		t.Fatalf("Lower update returned error: %v", err)
	}
	if updated.Frame == nil {
		t.Fatalf("updated frame is nil")
	}
	if got, want := updated.LiteralKeys, []data.Symbol{"sym"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("updated literal keys = %#v, want %#v", got, want)
	}
	if updated.Mutation == nil || updated.Mutation.Kind != UpdateQuery {
		t.Fatalf("updated mutation = %#v", updated.Mutation)
	}
	assignments := make(map[data.Symbol]data.Expr, len(updated.Mutation.Assignments))
	for _, assign := range updated.Mutation.Assignments {
		assignments[assign.Name] = assign.Expr
	}
	updatedFrame, err := data.UpdateWhere(*updated.Frame, updated.Mutation.Where, assignments)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	keyedUpdated, err := data.KeyBy(updatedFrame, updated.LiteralKeys...)
	if err != nil {
		t.Fatalf("KeyBy update returned error: %v", err)
	}
	assertQColumnValues(t, keyedUpdated.Frame(), "sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT")})
	assertQColumnValues(t, keyedUpdated.Frame(), "price", []any{int64(101), int64(80)})

	deleted, err := Lower(mustParse(t, "delete from ([sym:`AAPL`MSFT] price:100 80; size:10 20) where sym=`MSFT"))
	if err != nil {
		t.Fatalf("Lower delete returned error: %v", err)
	}
	if got, want := deleted.LiteralKeys, []data.Symbol{"sym"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("deleted literal keys = %#v, want %#v", got, want)
	}
	deletedFrame, err := data.DeleteWhere(*deleted.Frame, deleted.Mutation.Where)
	if err != nil {
		t.Fatalf("DeleteWhere returned error: %v", err)
	}
	keyedDeleted, err := data.KeyBy(deletedFrame, deleted.LiteralKeys...)
	if err != nil {
		t.Fatalf("KeyBy delete returned error: %v", err)
	}
	assertQColumnValues(t, keyedDeleted.Frame(), "sym", []any{data.Symbol("AAPL")})
	assertQColumnValues(t, keyedDeleted.Frame(), "price", []any{int64(100)})
}

func TestLowerRejectsInvalidMutationShapes(t *testing.T) {
	for _, src := range []string{
		"update price:sum size from trades",
		"delete px:price from trades",
		"delete price*size from trades",
	} {
		if _, err := Lower(mustParse(t, src)); err == nil {
			t.Fatalf("Lower(%q) returned nil error", src)
		}
	}
}

func TestLoweredQSQLExecutesAgainstDataFrame(t *testing.T) {
	query := mustParse(t, "select notional:sum price*size, fills:count i by sym from trades where price>100")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	frame := mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got := out.Len(); got != 1 {
		t.Fatalf("out len = %d, want 1", got)
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("AAPL"))
	assertColumnValue(t, out, "notional", 0, 3025.0)
	assertColumnValue(t, out, "fills", 0, int64(2))
}

func TestLoweredQSQLReportedRankAndXbarFormsExecute(t *testing.T) {
	frame := mustFrame(t,
		data.NewColumn("px", []any{30, 10, 20, 10, 40}),
	)

	ranked, err := Lower(mustParse(t, "select r:rank px from t"))
	if err != nil {
		t.Fatalf("Lower rank projection returned error: %v", err)
	}
	ranked.Plan.Source = frame
	rankedOut, err := ranked.Plan.Exec()
	if err != nil {
		t.Fatalf("rank projection Exec returned error: %v", err)
	}
	assertQColumnValues(t, rankedOut, "r", []any{int64(3), int64(0), int64(2), int64(1), int64(4)})

	bucketed, err := Lower(mustParse(t, "select c:count i by g:2 xbar px from t"))
	if err != nil {
		t.Fatalf("Lower xbar by clause returned error: %v", err)
	}
	bucketed.Plan.Source = frame
	bucketedOut, err := bucketed.Plan.Exec()
	if err != nil {
		t.Fatalf("xbar by clause Exec returned error: %v", err)
	}
	assertQColumnValues(t, bucketedOut, "g", []any{int64(30), int64(10), int64(20), int64(40)})
	assertQColumnValues(t, bucketedOut, "c", []any{int64(1), int64(2), int64(1), int64(1)})
}

func TestLowerQSQLConditionalSelectAndUpdate(t *testing.T) {
	frame := mustFrame(t,
		data.NewColumn("side", []any{data.Symbol("buy"), data.Symbol("sell"), data.Symbol("buy")}),
		data.NewColumn("price", []any{100.5, 99.5, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
		data.NewColumn("arrival_mid", []any{100.0, 100.0, 100.0}),
	)

	selected, err := Lower(mustParse(t, "select side,signed_qty:?[side=`buy;size;0-size],slip:?[side=`buy;price-arrival_mid;arrival_mid-price] from trades order by side asc"))
	if err != nil {
		t.Fatalf("Lower select returned error: %v", err)
	}
	selected.Plan.Source = frame
	out, err := selected.Plan.Exec()
	if err != nil {
		t.Fatalf("conditional select Exec returned error: %v", err)
	}
	assertQColumnValues(t, out, "signed_qty", []any{10.0, 20.0, -15.0})
	assertQColumnValues(t, out, "slip", []any{0.5, 1.0, 0.5})

	updated, err := Lower(mustParse(t, "update signed_qty:?[side=`buy;size;0-size] from trades"))
	if err != nil {
		t.Fatalf("Lower update returned error: %v", err)
	}
	assignments := make(map[data.Symbol]data.Expr, len(updated.Mutation.Assignments))
	for _, assign := range updated.Mutation.Assignments {
		assignments[assign.Name] = assign.Expr
	}
	updatedFrame, err := data.UpdateWhere(frame, updated.Mutation.Where, assignments)
	if err != nil {
		t.Fatalf("conditional update returned error: %v", err)
	}
	assertQColumnValues(t, updatedFrame, "signed_qty", []any{10.0, -15.0, 20.0})
}

func TestLoweredQSQLXbarTemporalIntervalUsesColumnKind(t *testing.T) {
	query := mustParse(t, "select qty:sum size by bucket:xbar 00:01 ts from trades order by bucket asc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	frame := mustFrame(t,
		data.Column{Name: "ts", Data: data.NewSecond([]data.Second{
			data.SecondFromSeconds(34_215),
			data.SecondFromSeconds(34_259),
			data.SecondFromSeconds(34_261),
		})},
		data.NewColumn("size", []any{10, 20, 30}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got := out.Len(); got != 2 {
		t.Fatalf("out len = %d, want 2", got)
	}
	assertColumnValue(t, out, "bucket", 0, data.SecondFromSeconds(34_200))
	assertColumnValue(t, out, "qty", 0, 30.0)
	assertColumnValue(t, out, "bucket", 1, data.SecondFromSeconds(34_260))
	assertColumnValue(t, out, "qty", 1, 30.0)
}

func TestLoweredQSQLXbarTemporalUpdateSelectOrder(t *testing.T) {
	update := mustParse(t, "update bucket:xbar 0D00:01:00 ts from trades")
	loweredUpdate, err := Lower(update)
	if err != nil {
		t.Fatalf("Lower update returned error: %v", err)
	}
	if len(loweredUpdate.Mutation.Assignments) != 1 {
		t.Fatalf("assignments = %#v", loweredUpdate.Mutation.Assignments)
	}
	if _, ok := loweredUpdate.Mutation.Assignments[0].Expr.(data.BucketFloorExpr); !ok {
		t.Fatalf("update bucket expr = %#v, want BucketFloorExpr", loweredUpdate.Mutation.Assignments[0].Expr)
	}

	frame := mustFrame(t,
		data.Column{Name: "ts", Data: data.NewTimestamp([]data.Timestamp{
			data.TimestampFromUnixNanos(59_999_999_999),
			data.TimestampFromUnixNanos(60_000_000_000),
			data.TimestampFromUnixNanos(119_999_999_999),
		})},
		data.NewColumn("price", []any{12.0, 10.0, 11.0}),
	)
	assignments := map[data.Symbol]data.Expr{
		loweredUpdate.Mutation.Assignments[0].Name: loweredUpdate.Mutation.Assignments[0].Expr,
	}
	updated, err := data.UpdateWhere(frame, loweredUpdate.Mutation.Where, assignments)
	if err != nil {
		t.Fatalf("UpdateWhere returned error: %v", err)
	}
	if kind, ok := updated.Schema().Kind("bucket"); !ok || kind != data.KindTimestamp {
		t.Fatalf("bucket kind = %s, ok %v; want %s", kind, ok, data.KindTimestamp)
	}
	assertQColumnValues(t, updated, "bucket", []any{
		data.TimestampFromUnixNanos(0),
		data.TimestampFromUnixNanos(60_000_000_000),
		data.TimestampFromUnixNanos(60_000_000_000),
	})

	selectQuery := mustParse(t, "select ts,bucket,price from trades where bucket>=1970.01.01D00:01:00 order by xbar 0D00:01:00 ts asc,price asc")
	loweredSelect, err := Lower(selectQuery)
	if err != nil {
		t.Fatalf("Lower select returned error: %v", err)
	}
	if len(loweredSelect.Plan.OrderBy) != 2 {
		t.Fatalf("order by = %#v", loweredSelect.Plan.OrderBy)
	}
	if len(loweredSelect.HiddenCols) != 1 {
		t.Fatalf("hidden cols = %#v, want one xbar order expression", loweredSelect.HiddenCols)
	}
	loweredSelect.Plan.Source = updated
	out, err := loweredSelect.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec select returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertQColumnValues(t, out, "price", []any{10.0, 11.0})
	assertQColumnValues(t, out, "bucket", []any{
		data.TimestampFromUnixNanos(60_000_000_000),
		data.TimestampFromUnixNanos(60_000_000_000),
	})
}

func TestLowerTimeSeriesProjectionCallsToDataExpressions(t *testing.T) {
	prefix, err := Lower(mustParse(t, "select p:prev price from trades"))
	if err != nil {
		t.Fatalf("Lower prefix returned error: %v", err)
	}
	if len(prefix.Plan.Select) != 1 {
		t.Fatalf("prefix select = %#v", prefix.Plan.Select)
	}
	transform, ok := prefix.Plan.Select[0].Expr.(data.VectorTransformExpr)
	if !ok || transform.Func != "prev" {
		t.Fatalf("prefix expr = %#v, want prev vector transform", prefix.Plan.Select[0].Expr)
	}

	absProjection, err := Lower(mustParse(t, "select abs_qty:abs signed_qty from trades where (abs signed_qty)>20 order by abs signed_qty desc"))
	if err != nil {
		t.Fatalf("Lower abs returned error: %v", err)
	}
	transform, ok = absProjection.Plan.Select[0].Expr.(data.VectorTransformExpr)
	if !ok || transform.Func != "abs" {
		t.Fatalf("abs projection expr = %#v, want abs vector transform", absProjection.Plan.Select[0].Expr)
	}
	where, ok := absProjection.Plan.Where.(data.Binary)
	if !ok || where.Op != data.OpGT {
		t.Fatalf("abs where = %#v, want comparison", absProjection.Plan.Where)
	}
	if _, ok := where.Left.(data.VectorTransformExpr); !ok {
		t.Fatalf("abs where left = %#v, want abs vector transform", where.Left)
	}
	if len(absProjection.Plan.OrderBy) != 1 || absProjection.Plan.OrderBy[0].Column == "" {
		t.Fatalf("abs order by = %#v", absProjection.Plan.OrderBy)
	}

	for _, query := range []struct {
		sql  string
		want string
	}{
		{sql: "select r:rank qty from trades", want: "rank"},
		{sql: "select neg_qty:neg signed_qty from trades", want: "neg"},
		{sql: "select root_qty:sqrt qty from trades", want: "sqrt"},
		{sql: "select log_qty:log qty from trades", want: "log"},
		{sql: "select exp_qty:exp qty from trades", want: "exp"},
		{sql: "select sin_qty:sin qty from trades", want: "sin"},
		{sql: "select cos_qty:cos qty from trades", want: "cos"},
		{sql: "select tan_qty:tan qty from trades", want: "tan"},
		{sql: "select asin_qty:asin qty from trades", want: "asin"},
		{sql: "select acos_qty:acos qty from trades", want: "acos"},
		{sql: "select atan_qty:atan qty from trades", want: "atan"},
		{sql: "select inv_qty:reciprocal qty from trades", want: "reciprocal"},
		{sql: "select dir:signum signed_qty from trades", want: "signum"},
		{sql: "select bucket:floor price from trades", want: "floor"},
		{sql: "select bucket:ceiling price from trades", want: "ceiling"},
		{sql: "select missing:null price from trades", want: "null"},
	} {
		lowered, err := Lower(mustParse(t, query.sql))
		if err != nil {
			t.Fatalf("Lower %q returned error: %v", query.sql, err)
		}
		transform, ok := lowered.Plan.Select[0].Expr.(data.VectorTransformExpr)
		if !ok || transform.Func != query.want {
			t.Fatalf("%q expr = %#v, want %s vector transform", query.sql, lowered.Plan.Select[0].Expr, query.want)
		}
	}

	xprev, err := Lower(mustParse(t, "select xp:2 xprev price from trades"))
	if err != nil {
		t.Fatalf("Lower xprev returned error: %v", err)
	}
	transform, ok = xprev.Plan.Select[0].Expr.(data.VectorTransformExpr)
	if !ok || transform.Func != "xprev" || transform.Arg == nil {
		t.Fatalf("xprev expr = %#v, want xprev vector transform with arg", xprev.Plan.Select[0].Expr)
	}

	xbar, err := Lower(mustParse(t, "select qty by g:2 xbar price from trades"))
	if err != nil {
		t.Fatalf("Lower xbar returned error: %v", err)
	}
	bucket, ok := xbar.Plan.ByExprs[0].Expr.(data.BucketFloorExpr)
	if !ok || bucket.Interval != int64(2) {
		t.Fatalf("xbar by expr = %#v, want bucket floor interval 2", xbar.Plan.ByExprs[0].Expr)
	}

	moving, err := Lower(mustParse(t, "select ma:3 mavg price from trades"))
	if err != nil {
		t.Fatalf("Lower moving returned error: %v", err)
	}
	listAgg, ok := moving.Plan.Select[0].Expr.(data.ListAggregateExpr)
	if !ok || listAgg.Func != "avg" {
		t.Fatalf("moving expr = %#v, want avg list aggregate", moving.Plan.Select[0].Expr)
	}
	window, ok := listAgg.Expr.(data.VectorTransformExpr)
	if !ok || window.Func != "moving" || window.Arg == nil {
		t.Fatalf("moving window expr = %#v, want moving vector transform with arg", listAgg.Expr)
	}
}

func TestLowerPercentDivideToDataOpDiv(t *testing.T) {
	query := mustParse(t, "select ratio:price%qty from trades where (price%qty)>10 order by price%qty desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower failed: %v", err)
	}
	if len(lowered.Plan.Select) != 2 {
		t.Fatalf("select = %#v, want visible projection plus hidden order projection", lowered.Plan.Select)
	}
	projection, ok := lowered.Plan.Select[0].Expr.(data.Binary)
	if !ok || projection.Op != data.OpDiv {
		t.Fatalf("projection expr = %#v, want OpDiv", lowered.Plan.Select[0].Expr)
	}
	where, ok := lowered.Plan.Where.(data.Binary)
	if !ok || where.Op != data.OpGT {
		t.Fatalf("where = %#v, want > Binary", lowered.Plan.Where)
	}
	whereLeft, ok := where.Left.(data.Binary)
	if !ok || whereLeft.Op != data.OpDiv {
		t.Fatalf("where left = %#v, want OpDiv", where.Left)
	}
	orderExpr, ok := lowered.Plan.Select[1].Expr.(data.Binary)
	if !ok || orderExpr.Op != data.OpDiv {
		t.Fatalf("order hidden expr = %#v, want OpDiv", lowered.Plan.Select[1].Expr)
	}

	update := mustParse(t, "update ratio:price%qty from trades where (price%qty)>10")
	loweredUpdate, err := Lower(update)
	if err != nil {
		t.Fatalf("Lower update failed: %v", err)
	}
	if len(loweredUpdate.Mutation.Assignments) != 1 {
		t.Fatalf("assignments = %#v", loweredUpdate.Mutation.Assignments)
	}
	assignment, ok := loweredUpdate.Mutation.Assignments[0].Expr.(data.Binary)
	if !ok || assignment.Op != data.OpDiv {
		t.Fatalf("update assignment = %#v, want OpDiv", loweredUpdate.Mutation.Assignments[0].Expr)
	}
	updateWhere, ok := loweredUpdate.Mutation.Where.(data.Binary)
	if !ok || updateWhere.Op != data.OpGT {
		t.Fatalf("update where = %#v, want > Binary", loweredUpdate.Mutation.Where)
	}
	updateWhereLeft, ok := updateWhere.Left.(data.Binary)
	if !ok || updateWhereLeft.Op != data.OpDiv {
		t.Fatalf("update where left = %#v, want OpDiv", updateWhere.Left)
	}
}

func TestLoweredGroupedAnalyticsProjectionExecutesByGroup(t *testing.T) {
	query := mustParse(t, "select d:deltas price,r:ratios price,m:3 mavg price,x:2 xprev price by sym from trades order by sym asc,ts asc take 5")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if len(lowered.Plan.ByExprs) != 1 || lowered.Plan.ByExprs[0].Name != "sym" {
		t.Fatalf("by exprs = %#v", lowered.Plan.ByExprs)
	}
	selectItem := func(name data.Symbol) data.SelectItem {
		t.Helper()
		for _, item := range lowered.Plan.Select {
			if item.Name == name {
				return item
			}
		}
		t.Fatalf("missing select item %q in %#v", name, lowered.Plan.Select)
		return data.SelectItem{}
	}
	for _, column := range []struct {
		symbol data.Symbol
		fn     string
	}{
		{symbol: "d", fn: "deltas"},
		{symbol: "r", fn: "ratios"},
		{symbol: "x", fn: "xprev"},
	} {
		item := selectItem(column.symbol)
		transform, ok := item.Expr.(data.VectorTransformExpr)
		if !ok || transform.Func != column.fn {
			t.Fatalf("%s expr = %#v, want %s vector transform", column.symbol, item.Expr, column.fn)
		}
	}
	mavg := selectItem("m")
	listAgg, ok := mavg.Expr.(data.ListAggregateExpr)
	if !ok || listAgg.Func != "avg" {
		t.Fatalf("m expr = %#v, want avg list aggregate", mavg.Expr)
	}
	window, ok := listAgg.Expr.(data.VectorTransformExpr)
	if !ok || window.Func != "moving" || window.Arg == nil {
		t.Fatalf("m window expr = %#v, want moving vector transform with arg", listAgg.Expr)
	}

	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL"), data.Symbol("MSFT")}),
		data.NewColumn("ts", []any{int64(3), int64(2), int64(1), int64(1), int64(2), int64(3)}),
		data.NewColumn("price", []any{30, 20, 10, 10, 20, 30}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 5 {
		t.Fatalf("out len = %d, want 5", out.Len())
	}
	assertQColumnValues(t, out, "d", []any{10.0, 10.0, 10.0, 10.0, 10.0})
	assertQColumnValues(t, out, "r", []any{data.NullValue, 2.0, 1.5, data.NullValue, 2.0})
	assertQColumnValues(t, out, "x", []any{data.NullValue, data.NullValue, int64(10), data.NullValue, data.NullValue})
}

func TestLoweredTimeSeriesVectorWhereAndOrderExecute(t *testing.T) {
	query := mustParse(t, "select sym,price,p:prev price,d:deltas price from trades where (deltas price)>0 order by deltas price desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("AAPL"), data.Symbol("AAPL"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{10, 12, 11, 15}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 3 {
		t.Fatalf("out len = %d, want 3", out.Len())
	}
	assertQColumnValues(t, out, "price", []any{int64(10), int64(15), int64(12)})
	assertQColumnValues(t, out, "p", []any{data.NullValue, int64(12), int64(10)})
	assertQColumnValues(t, out, "d", []any{10.0, 3.0, 2.0})
}

func TestLoweredAbsProjectionWhereAndOrderExecute(t *testing.T) {
	query := mustParse(t, "select sym,abs_qty:abs signed_qty from trades where (abs signed_qty)>20 order by abs signed_qty desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA"), data.Symbol("TSLA")}),
		data.NewColumn("signed_qty", []any{int64(40), int64(-25), int64(10), nil}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("AAPL"))
	assertColumnValue(t, out, "abs_qty", 0, 40.0)
	assertColumnValue(t, out, "sym", 1, data.Symbol("MSFT"))
	assertColumnValue(t, out, "abs_qty", 1, 25.0)
}

func TestLoweredNumericUnaryProjectionWhereAndOrderExecute(t *testing.T) {
	query := mustParse(t, "select sym,neg_qty:neg signed_qty,root_qty:sqrt qty,log_qty:log qty,exp_qty:exp qty,inv_qty:reciprocal qty from trades where (sqrt qty)>=3 order by neg signed_qty asc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA"), data.Symbol("TSLA")}),
		data.NewColumn("signed_qty", []any{int64(5), int64(-9), int64(16), nil}),
		data.NewColumn("qty", []any{float64(4), float64(9), float64(16), nil}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("NVDA"))
	assertColumnValue(t, out, "neg_qty", 0, -16.0)
	assertColumnValue(t, out, "root_qty", 0, 4.0)
	assertColumnValue(t, out, "log_qty", 0, math.Log(16))
	assertColumnValue(t, out, "exp_qty", 0, math.Exp(16))
	assertColumnValue(t, out, "inv_qty", 0, 1.0/16.0)
	assertColumnValue(t, out, "sym", 1, data.Symbol("MSFT"))
	assertColumnValue(t, out, "neg_qty", 1, 9.0)
	assertColumnValue(t, out, "root_qty", 1, 3.0)
	assertColumnValue(t, out, "log_qty", 1, math.Log(9))
	assertColumnValue(t, out, "exp_qty", 1, math.Exp(9))
	assertColumnValue(t, out, "inv_qty", 1, 1.0/9.0)
}

func TestLoweredTrigUnaryProjectionWhereAndOrderExecute(t *testing.T) {
	query := mustParse(t, "select sym,s:sin theta,c:cos theta,t:tan theta,a:asin ratio,ac:acos ratio,at:atan theta from trades where (cos theta)>0 order by sin theta desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA")}),
		data.NewColumn("theta", []any{float64(0), math.Pi / 6, math.Pi}),
		data.NewColumn("ratio", []any{float64(0), float64(0.5), float64(1)}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("MSFT"))
	assertFloatNear(t, out, "s", 0, math.Sin(math.Pi/6))
	assertFloatNear(t, out, "c", 0, math.Cos(math.Pi/6))
	assertFloatNear(t, out, "t", 0, math.Tan(math.Pi/6))
	assertFloatNear(t, out, "a", 0, math.Asin(0.5))
	assertFloatNear(t, out, "ac", 0, math.Acos(0.5))
	assertFloatNear(t, out, "at", 0, math.Atan(math.Pi/6))
	assertColumnValue(t, out, "sym", 1, data.Symbol("AAPL"))
	assertFloatNear(t, out, "s", 1, 0)
	assertFloatNear(t, out, "c", 1, 1)
	assertFloatNear(t, out, "t", 1, 0)
	assertFloatNear(t, out, "a", 1, 0)
	assertFloatNear(t, out, "ac", 1, math.Pi/2)
	assertFloatNear(t, out, "at", 1, 0)
}

func TestLoweredRoundingUnaryProjectionWhereAndOrderExecute(t *testing.T) {
	query := mustParse(t, "select sym,dir:signum signed_qty,floor_qty:floor qty,ceil_qty:ceiling qty from trades where (ceiling qty)>=10 order by floor qty desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA"), data.Symbol("TSLA")}),
		data.NewColumn("signed_qty", []any{int64(5), int64(-9), int64(16), nil}),
		data.NewColumn("qty", []any{float64(4.2), float64(9.1), float64(16.8), nil}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("NVDA"))
	assertColumnValue(t, out, "dir", 0, 1.0)
	assertColumnValue(t, out, "floor_qty", 0, 16.0)
	assertColumnValue(t, out, "ceil_qty", 0, 17.0)
	assertColumnValue(t, out, "sym", 1, data.Symbol("MSFT"))
	assertColumnValue(t, out, "dir", 1, -1.0)
	assertColumnValue(t, out, "floor_qty", 1, 9.0)
	assertColumnValue(t, out, "ceil_qty", 1, 10.0)
}

func TestLoweredNullPrefixProjectionWhereAndOrderExecute(t *testing.T) {
	query := mustParse(t, "select sym,missing:null price from trades where null price order by sym desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA"), data.Symbol("TSLA")}),
		data.NewColumn("price", []any{100.0, nil, 250.0, nil}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("out len = %d, want 2", out.Len())
	}
	assertColumnValue(t, out, "sym", 0, data.Symbol("TSLA"))
	assertColumnValue(t, out, "missing", 0, true)
	assertColumnValue(t, out, "sym", 1, data.Symbol("MSFT"))
	assertColumnValue(t, out, "missing", 1, true)
}

func TestLoweredOrderLimitExecutesAgainstDataFrame(t *testing.T) {
	query := mustParse(t, "select sym,price,size from trades where size>=10 order by price desc limit 2")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	frame := mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got := out.Len(); got != 2 {
		t.Fatalf("out len = %d, want 2", got)
	}
	assertColumnValue(t, out, "price", 0, 101.0)
	assertColumnValue(t, out, "sym", 1, data.Symbol("AAPL"))
}

func TestLoweredOrderByComputedProjectionExecutesAgainstDataFrame(t *testing.T) {
	query := mustParse(t, "select price*size from trades order by price*size desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	frame := mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got := out.Len(); got != 3 {
		t.Fatalf("out len = %d, want 3", got)
	}
	assertColumnValue(t, out, "price*size", 0, 2020.0)
	assertColumnValue(t, out, "price*size", 1, 1350.0)
	assertColumnValue(t, out, "price*size", 2, 1005.0)
}

func TestLoweredOrderByComputedProjectionAliasExecutesAgainstDataFrame(t *testing.T) {
	query := mustParse(t, "select notional:price*size from trades order by notional desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	frame := mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if got := out.Len(); got != 3 {
		t.Fatalf("out len = %d, want 3", got)
	}
	assertColumnValue(t, out, "notional", 0, 2020.0)
	assertColumnValue(t, out, "notional", 2, 1005.0)
}

func TestLoweredOrderByUnprojectedSourceColumnPreProjects(t *testing.T) {
	query := mustParse(t, "select price from trades order by size desc")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if !lowered.Plan.PreProjectOrder {
		t.Fatalf("PreProjectOrder = false")
	}
	if len(lowered.HiddenCols) != 0 {
		t.Fatalf("hidden cols = %#v, want none for source-column ordering", lowered.HiddenCols)
	}
	frame := mustFrame(t,
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	names := out.Schema().Names()
	if len(names) != 1 || names[0] != "price" {
		t.Fatalf("schema names = %#v, want only price", names)
	}
	assertColumnValue(t, out, "price", 0, 101.0)
	assertColumnValue(t, out, "price", 2, 100.5)
}

func TestLoweredExecProjectionKeepsColumnNames(t *testing.T) {
	query := mustParse(t, "exec price,size from trades where sym=`AAPL")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	frame := mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
		data.NewColumn("size", []any{10, 15, 20}),
	)
	lowered.Plan.Source = frame
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	names := out.Schema().Names()
	if len(names) != 2 || names[0] != "price" || names[1] != "size" {
		t.Fatalf("schema names = %#v", names)
	}
}

func TestLowerExecDictExpressionProjectsKeyAndValueColumns(t *testing.T) {
	lowered, err := Lower(mustParse(t, "exec sym!price from trades"))
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	if lowered.ExecDict == nil {
		t.Fatalf("ExecDict = nil")
	}
	if lowered.ExecDict.KeyName == "" || lowered.ExecDict.ValueName == "" {
		t.Fatalf("ExecDict names = %#v", lowered.ExecDict)
	}
	if len(lowered.Plan.Select) != 2 {
		t.Fatalf("select = %#v, want key/value projections", lowered.Plan.Select)
	}
	if lowered.Plan.Select[0].Name != lowered.ExecDict.KeyName || lowered.Plan.Select[1].Name != lowered.ExecDict.ValueName {
		t.Fatalf("select names = %#v, exec dict = %#v", lowered.Plan.Select, lowered.ExecDict)
	}
}

func TestLowerRejectsAmbiguousDictionaryProjectionShapes(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"exec sym!price,size from trades", `q exec dictionary projection "sym!price" must be the only projection`},
		{"exec sym!price by sym from trades", "q exec dictionary projection does not support by"},
		{"select sym!price from trades", `q dictionary projection "sym!price" is only valid for exec`},
	}
	for _, tt := range tests {
		_, err := Lower(mustParse(t, tt.src))
		if err == nil {
			t.Fatalf("Lower(%q) returned nil error", tt.src)
		}
		if !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("Lower(%q) error = %q, want contains %q", tt.src, err.Error(), tt.want)
		}
	}
}

func TestLoweredWhereScalarComparisonsExecute(t *testing.T) {
	frame := mustFrame(t,
		data.NewColumn("name", []any{"alpha", "beta", "alpha"}),
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("AAPL")}),
		data.NewColumn("active", []any{true, false, true}),
		data.NewColumn("note", []any{"set", nil, nil}),
		data.NewColumn("price", []any{100.5, 90.0, 101.0}),
	)
	tests := []struct {
		src  string
		want int
	}{
		{`select price from trades where name="alpha"`, 2},
		{`select price from trades where sym=` + "`AAPL", 2},
		{`select price from trades where active=true`, 2},
		{`select price from trades where note=null`, 2},
		{`select price from trades where sym<>` + "`AAPL", 1},
	}
	for _, tt := range tests {
		lowered, err := Lower(mustParse(t, tt.src))
		if err != nil {
			t.Fatalf("Lower(%q) returned error: %v", tt.src, err)
		}
		lowered.Plan.Source = frame
		out, err := lowered.Plan.Exec()
		if err != nil {
			t.Fatalf("Exec(%q) returned error: %v", tt.src, err)
		}
		if out.Len() != tt.want {
			t.Fatalf("%s len = %d, want %d", tt.src, out.Len(), tt.want)
		}
	}
}

func TestLoweredWhereCommaFormExecutesAsAnd(t *testing.T) {
	query := mustParse(t, "select sym,price from trades where sym=`AAPL,price>=100")
	lowered, err := Lower(query)
	if err != nil {
		t.Fatalf("Lower returned error: %v", err)
	}
	lowered.Plan.Source = mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("AAPL"), data.Symbol("MSFT")}),
		data.NewColumn("price", []any{90, 110, 120}),
	)
	out, err := lowered.Plan.Exec()
	if err != nil {
		t.Fatalf("Exec returned error: %v", err)
	}
	if out.Len() != 1 {
		t.Fatalf("out len = %d, want 1", out.Len())
	}
	assertColumnValue(t, out, "price", 0, int64(110))
}

func TestLoweredQSQLLogicalSymbolOperatorsExecute(t *testing.T) {
	frame := mustFrame(t,
		data.NewColumn("sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT"), data.Symbol("NVDA"), data.Symbol("TSLA")}),
		data.NewColumn("price", []any{101, 99, 103, 88}),
		data.NewColumn("size", []any{12, 20, 5, 7}),
		data.NewColumn("flagged", []any{false, true, false, false}),
		data.NewColumn("low_battery", []any{false, false, true, false}),
	)

	selected, err := Lower(mustParse(t, "select sym,keep:flagged|low_battery from trades where ((price>100)&(size>10))|flagged order by sym asc"))
	if err != nil {
		t.Fatalf("Lower select returned error: %v", err)
	}
	selected.Plan.Source = frame
	out, err := selected.Plan.Exec()
	if err != nil {
		t.Fatalf("select Exec returned error: %v", err)
	}
	if out.Len() != 2 {
		t.Fatalf("select len = %d, want 2", out.Len())
	}
	assertQColumnValues(t, out, "sym", []any{data.Symbol("AAPL"), data.Symbol("MSFT")})
	assertQColumnValues(t, out, "keep", []any{false, true})

	updated, err := Lower(mustParse(t, "update keep:flagged|low_battery from trades where ((price>100)&(size>10))|flagged"))
	if err != nil {
		t.Fatalf("Lower update returned error: %v", err)
	}
	assignments := make(map[data.Symbol]data.Expr, len(updated.Mutation.Assignments))
	for _, assign := range updated.Mutation.Assignments {
		assignments[assign.Name] = assign.Expr
	}
	updatedFrame, err := data.UpdateWhere(frame, updated.Mutation.Where, assignments)
	if err != nil {
		t.Fatalf("update Exec returned error: %v", err)
	}
	assertQColumnValues(t, updatedFrame, "keep", []any{false, true, data.NullValue, data.NullValue})
}

func TestValueASTSkeleton(t *testing.T) {
	flip := Flip{Columns: []Column{
		{Name: "sym", Expr: Vector{Items: []Expr{Symbol{Name: "AAPL"}, Symbol{Name: "MSFT"}}}},
		{Name: "price", Expr: Vector{Items: []Expr{Number{Text: "101"}, Number{Text: "102"}}}},
	}}

	if len(flip.Columns) != 2 {
		t.Fatalf("flip columns = %#v", flip.Columns)
	}
	syms, ok := flip.Columns[0].Expr.(Vector)
	if !ok || len(syms.Items) != 2 {
		t.Fatalf("symbol vector = %#v", flip.Columns[0].Expr)
	}
	if sym, ok := syms.Items[0].(Symbol); !ok || sym.Name != "AAPL" {
		t.Fatalf("first symbol = %#v", syms.Items[0])
	}
}

func mustParse(t *testing.T, src string) *Query {
	t.Helper()
	query, err := Parse(src)
	if err != nil {
		t.Fatalf("Parse(%q) returned error: %v", src, err)
	}
	return query
}

func mustFrame(t *testing.T, cols ...data.Column) data.Frame {
	t.Helper()
	frame, err := data.NewFrame(cols...)
	if err != nil {
		t.Fatalf("NewFrame returned error: %v", err)
	}
	return frame
}

func assertColumnValue(t *testing.T, frame data.Frame, name data.Symbol, row int, want any) {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	got, ok := col.At(row)
	if !ok {
		t.Fatalf("missing row %d in column %q", row, name)
	}
	if got != want {
		t.Fatalf("%s[%d] = %#v, want %#v", name, row, got, want)
	}
}

func assertFloatNear(t *testing.T, frame data.Frame, name data.Symbol, row int, want float64) {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	got, ok := col.At(row)
	if !ok {
		t.Fatalf("missing row %d in column %q", row, name)
	}
	value, ok := got.(float64)
	if !ok {
		t.Fatalf("%s[%d] = %#v, want float64 near %v", name, row, got, want)
	}
	if math.Abs(value-want) > 1e-12 {
		t.Fatalf("%s[%d] = %.17g, want %.17g", name, row, value, want)
	}
}

func assertQColumnValues(t *testing.T, frame data.Frame, name data.Symbol, want []any) {
	t.Helper()
	col, ok := frame.Column(name)
	if !ok {
		t.Fatalf("missing column %q", name)
	}
	if got := col.Values(); !reflect.DeepEqual(got, want) {
		t.Fatalf("column %q = %#v, want %#v", name, got, want)
	}
}
