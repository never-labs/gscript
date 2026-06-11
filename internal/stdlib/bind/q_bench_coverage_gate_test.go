package bind

// Source-derived qSQL benchmark coverage gate, mirroring the q.eval verb gate
// (benchmarks/q_eval_verb_coverage_test.go). The query-kind, join-kind, and
// aggregate surfaces are extracted mechanically from internal/stdlib/lib/q via
// go/ast, so any kind added to the parser/lowerer immediately requires a
// benchmark case in qSQLMatrixBenchCases (or a shrink-only backlog entry with
// a written justification).

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Backlogs (shrink-only).
// ---------------------------------------------------------------------------

// qSQLJoinKindBacklog must stay empty: every join kind the parser accepts has
// an executable benchmark case.
const qSQLJoinKindBacklogMax = 0

var qSQLJoinKindBacklog = map[string]string{}

// qSQLQueryKindBacklog must stay empty: every QueryKind constant has an
// executable benchmark case.
const qSQLQueryKindBacklogMax = 0

var qSQLQueryKindBacklog = map[string]string{}

// qSQLAggregateBacklog lists aggregates accepted by lower.go that cannot be
// covered with a working qSQL query today. Entries need a justification and
// the list may only shrink.
const qSQLAggregateBacklogMax = 1

var qSQLAggregateBacklog = map[string]string{
	"wavg": "qSQL lowering builds data.Aggregate without a Weight expression (Call carries a single argument), so execution fails with 'aggregate wavg expects a weight expression'; cover once the parser/lowerer grows a weighted-aggregate form",
}

// ---------------------------------------------------------------------------
// Source extraction via go/ast.
// ---------------------------------------------------------------------------

func qSQLSourceFile(t *testing.T, name string) string {
	t.Helper()
	candidates := make([]string, 0, 2)
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(thisFile), "..", "lib", "q", name))
	}
	candidates = append(candidates, filepath.Join("..", "lib", "q", name))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Fatalf("cannot locate internal/stdlib/lib/q/%s; tried %v", name, candidates)
	return ""
}

func qSQLParseSourceFile(t *testing.T, name string) *ast.File {
	t.Helper()
	path := qSQLSourceFile(t, name)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return file
}

// qSQLExtractQueryKinds collects the string values of the QueryKind constant
// block in ast.go.
func qSQLExtractQueryKinds(t *testing.T) []string {
	t.Helper()
	file := qSQLParseSourceFile(t, "ast.go")
	seen := make(map[string]bool)
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			ident, ok := value.Type.(*ast.Ident)
			if !ok || ident.Name != "QueryKind" {
				continue
			}
			for _, expr := range value.Values {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				kind, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				seen[kind] = true
			}
		}
	}
	kinds := make([]string, 0, len(seen))
	for kind := range seen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	const floor = 6
	if len(kinds) < floor {
		t.Fatalf("extracted %d QueryKind constants from ast.go, below robustness floor %d; the const block structure may have changed", len(kinds), floor)
	}
	return kinds
}

// qSQLExtractJoinKinds collects the join-kind string literals passed to
// p.parseJoin(...) inside parseOptionalJoin in parser.go.
func qSQLExtractJoinKinds(t *testing.T) []string {
	t.Helper()
	file := qSQLParseSourceFile(t, "parser.go")
	seen := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "parseOptionalJoin" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "parseJoin" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			kind, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			seen[kind] = true
			return true
		})
	}
	kinds := make([]string, 0, len(seen))
	for kind := range seen {
		kinds = append(kinds, kind)
	}
	sort.Strings(kinds)
	const floor = 8
	if len(kinds) < floor {
		t.Fatalf("extracted %d join kinds from parser.go parseOptionalJoin, below robustness floor %d; the dispatch structure may have changed", len(kinds), floor)
	}
	return kinds
}

// qSQLExtractAggregates collects the case-clause string literals of the
// isAggregate switch in lower.go.
func qSQLExtractAggregates(t *testing.T) []string {
	t.Helper()
	file := qSQLParseSourceFile(t, "lower.go")
	seen := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "isAggregate" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				name, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				seen[name] = true
			}
			return true
		})
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	const floor = 10
	if len(names) < floor {
		t.Fatalf("extracted %d aggregates from lower.go isAggregate, below robustness floor %d; the switch structure may have changed", len(names), floor)
	}
	return names
}

// ---------------------------------------------------------------------------
// Gate.
// ---------------------------------------------------------------------------

// qSQLAllBenchmarkCaseTags returns the union of tags across the legacy
// hand-written qSQL benchmark list and the matrix case table.
func qSQLAllBenchmarkCaseTags() map[string][]string {
	covered := make(map[string][]string)
	for _, tc := range qSQLBenchmarkCases {
		for _, tag := range tc.tags {
			covered[tag] = append(covered[tag], tc.name)
		}
	}
	for _, tc := range qSQLMatrixBenchCases {
		for _, tag := range tc.tags {
			covered[tag] = append(covered[tag], tc.name)
		}
	}
	return covered
}

func qSQLCheckSurfaceCoverage(t *testing.T, surface string, kinds []string, tagPrefix string, backlog map[string]string, backlogMax int) {
	t.Helper()
	if len(backlog) > backlogMax {
		t.Fatalf("%s backlog has %d entries, exceeding shrink-only max %d", surface, len(backlog), backlogMax)
	}
	kindSet := make(map[string]bool, len(kinds))
	for _, kind := range kinds {
		kindSet[kind] = true
	}
	for kind, justification := range backlog {
		if !kindSet[kind] {
			t.Errorf("%s backlog has stale entry %q: not extracted from source", surface, kind)
		}
		if justification == "" {
			t.Errorf("%s backlog entry %q has no justification", surface, kind)
		}
	}
	covered := qSQLAllBenchmarkCaseTags()
	var missing []string
	for _, kind := range kinds {
		tag := tagPrefix + kind
		hasCase := len(covered[tag]) > 0
		_, backlogged := backlog[kind]
		switch {
		case hasCase && backlogged:
			t.Errorf("%s %q is covered by tag %q but still backlogged; remove the stale backlog entry", surface, kind, tag)
		case !hasCase && !backlogged:
			missing = append(missing, kind)
		}
	}
	if len(missing) > 0 {
		t.Errorf("%s without benchmark evidence (add a qSQLMatrixBenchCases case tagged %s<kind>, or a backlog entry with justification): %s",
			surface, tagPrefix, strings.Join(missing, ", "))
	}
}

func TestQSQLBenchmarkCoverageMatchesSourceSurface(t *testing.T) {
	queryKinds := qSQLExtractQueryKinds(t)
	t.Logf("query kinds extracted from ast.go: %v", queryKinds)
	qSQLCheckSurfaceCoverage(t, "qSQL query kind", queryKinds, "query:", qSQLQueryKindBacklog, qSQLQueryKindBacklogMax)

	joinKinds := qSQLExtractJoinKinds(t)
	t.Logf("join kinds extracted from parser.go: %v", joinKinds)
	qSQLCheckSurfaceCoverage(t, "qSQL join kind", joinKinds, "join:", qSQLJoinKindBacklog, qSQLJoinKindBacklogMax)

	aggregates := qSQLExtractAggregates(t)
	t.Logf("aggregates extracted from lower.go: %v", aggregates)
	qSQLCheckSurfaceCoverage(t, "qSQL aggregate", aggregates, "agg:", qSQLAggregateBacklog, qSQLAggregateBacklogMax)
}
