package q

import (
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
	if by, ok := query.By[0].(Ident); !ok || by.Name != "sym" {
		t.Fatalf("by expr = %#v", query.By[0])
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

func TestParseHashTakePrefix(t *testing.T) {
	query := mustParse(t, "2#select sym,price from trades")

	if query.Take == nil || *query.Take != 2 {
		t.Fatalf("take = %#v", query.Take)
	}
	if query.Kind != SelectQuery || query.From != "trades" {
		t.Fatalf("query header = %#v", query)
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
	if len(plan.Plan.By) != 1 || plan.Plan.By[0] != "sym" {
		t.Fatalf("plan group by = %#v", plan.Plan.By)
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
