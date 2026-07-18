//go:build leia_q

package benchmarks

// Coverage gate for q.eval verbs: the set of supported verbs is extracted
// mechanically from the dispatch switches in internal/stdlib/lib/q/eval.go
// (lookupDyadicVerb, lookupDyadicVerbFunc, lookupUnaryVerb), so any verb added
// to eval.go immediately requires benchmark evidence in qEvalVectorCases or an
// explicit entry in the exclusion/backlog maps below. Non-verb special forms
// come from stdq.SupportedEvalForms() and are matched through per-form
// evidence patterns.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

// ---------------------------------------------------------------------------
// Exclusions and backlogs (shrink-only: max counts are constants, stale
// entries fail the gate).
// ---------------------------------------------------------------------------

// qEvalVerbExclusions lists extracted verbs that intentionally have no
// qEvalVectorCases evidence because they are exercised by dedicated suites or
// are non-performance-relevant aliases. Reason strings must say where the
// coverage lives.
const qEvalVerbExclusionsMax = 4

var qEvalVerbExclusions = map[string]string{
	// Currently empty: every extracted verb either has qEvalVectorCases
	// evidence or sits in qEvalVerbCoverageBacklog. Add entries here only for
	// verbs whose performance coverage lives in a dedicated suite elsewhere,
	// with a reason naming that suite.
}

// qEvalVerbCoverageBacklog was the temporary Phase 2 work list. It is now
// empty and must stay empty: a verb added to eval.go without benchmark
// evidence fails the gate immediately (the max guards re-growth).
const qEvalVerbCoverageBacklogMax = 0

var qEvalVerbCoverageBacklog = map[string]string{}

// qEvalRequiredComboShapes are cross-cutting combination axes every q.eval
// benchmark suite must eventually carry as tc.tags entries, with at least
// qEvalComboMinCases cases per tag.
const qEvalComboMinCases = 3

var qEvalRequiredComboShapes = []string{
	"combo:producer-transform-reducer:depth3",
	"combo:mixed-type-arith",
	"combo:null-mixed-pipeline",
	"combo:nested-adverb",
	"combo:where-compound-predicate",
	"combo:dict-table-chain",
	"combo:adverb-over-apply-index",
}

const qEvalComboBacklogMax = 0

var qEvalComboBacklog = map[string]string{}

const qEvalFormBacklogMax = 0

var qEvalFormBacklog = map[string]string{}

// ---------------------------------------------------------------------------
// Verb extraction from eval.go via go/ast.
// ---------------------------------------------------------------------------

var qEvalLookupFuncFloors = map[string]int{
	"lookupDyadicVerb":     15,
	"lookupDyadicVerbFunc": 35,
	"lookupUnaryVerb":      60,
}

func qEvalSourcePath(t *testing.T) string {
	t.Helper()
	candidates := make([]string, 0, 2)
	if _, thisFile, _, ok := runtime.Caller(0); ok {
		candidates = append(candidates, filepath.Join(filepath.Dir(thisFile), "..", "internal", "stdlib", "lib", "q", "eval.go"))
	}
	candidates = append(candidates, filepath.Join("..", "internal", "stdlib", "lib", "q", "eval.go"))
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Fatalf("cannot locate internal/stdlib/lib/q/eval.go; tried %v", candidates)
	return ""
}

// qEvalExtractVerbTables parses eval.go and returns, per lookup function, the
// sorted deduplicated string literals collected from the case clauses of the
// switch statement(s) dispatching on the verb parameter.
func qEvalExtractVerbTables(t *testing.T) map[string][]string {
	t.Helper()
	path := qEvalSourcePath(t)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	tables := make(map[string][]string, len(qEvalLookupFuncFloors))
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		floor, want := qEvalLookupFuncFloors[fn.Name.Name]
		if !want {
			continue
		}
		param := qEvalVerbParamName(t, fn)
		verbs := qEvalCollectSwitchVerbs(fn.Body, param)
		if len(verbs) < floor {
			t.Fatalf("%s: extracted %d verbs, below robustness floor %d; eval.go dispatch structure may have changed", fn.Name.Name, len(verbs), floor)
		}
		tables[fn.Name.Name] = verbs
	}
	for name := range qEvalLookupFuncFloors {
		if _, ok := tables[name]; !ok {
			t.Fatalf("function %s not found in %s; eval.go dispatch structure may have changed", name, path)
		}
	}
	return tables
}

func qEvalVerbParamName(t *testing.T, fn *ast.FuncDecl) string {
	t.Helper()
	params := fn.Type.Params
	if params == nil || len(params.List) == 0 || len(params.List[0].Names) == 0 {
		t.Fatalf("%s: expected a named verb parameter", fn.Name.Name)
	}
	return params.List[0].Names[0].Name
}

// qEvalCollectSwitchVerbs walks body and collects string literals from case
// clauses of switch statements whose tag expression references the verb
// parameter (covers both `switch verb` and `switch strings.TrimSpace(verb)`).
func qEvalCollectSwitchVerbs(body *ast.BlockStmt, param string) []string {
	seen := make(map[string]bool)
	ast.Inspect(body, func(n ast.Node) bool {
		sw, ok := n.(*ast.SwitchStmt)
		if !ok || sw.Tag == nil || !qEvalExprMentionsIdent(sw.Tag, param) {
			return true
		}
		for _, stmt := range sw.Body.List {
			clause, ok := stmt.(*ast.CaseClause)
			if !ok {
				continue
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					continue
				}
				value, err := strconv.Unquote(lit.Value)
				if err != nil {
					continue
				}
				seen[value] = true
			}
		}
		return true
	})
	verbs := make([]string, 0, len(seen))
	for verb := range seen {
		verbs = append(verbs, verb)
	}
	sort.Strings(verbs)
	return verbs
}

func qEvalExprMentionsIdent(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && ident.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

func qEvalVerbUnion(tables map[string][]string) []string {
	seen := make(map[string]bool)
	for _, verbs := range tables {
		for _, verb := range verbs {
			seen[verb] = true
		}
	}
	union := make([]string, 0, len(seen))
	for verb := range seen {
		union = append(union, verb)
	}
	sort.Strings(union)
	return union
}

// ---------------------------------------------------------------------------
// Evidence matching against rendered benchmark expressions.
// ---------------------------------------------------------------------------

var (
	qEvalWordVerbRE  = regexp.MustCompile(`^[a-z][a-z0-9]*$`)
	qEvalWordTokenRE = regexp.MustCompile(`[a-zA-Z][a-zA-Z0-9]*`)
)

// qEvalStripQLiterals blanks out q string literal contents ("...", honoring
// backslash escapes) and backtick symbol names (`AAPL, `sym.ns, `:path) so
// neither can produce false verb matches. Structural characters outside
// literals are preserved byte-for-byte.
func qEvalStripQLiterals(src string) string {
	out := make([]byte, 0, len(src))
	for i := 0; i < len(src); i++ {
		ch := src[i]
		switch ch {
		case '"':
			out = append(out, ' ')
			for i++; i < len(src); i++ {
				out = append(out, ' ')
				if src[i] == '\\' {
					i++
					if i < len(src) {
						out = append(out, ' ')
					}
					continue
				}
				if src[i] == '"' {
					break
				}
			}
		case '`':
			out = append(out, ' ')
			for i+1 < len(src) && qEvalIsSymbolNameByte(src[i+1]) {
				out = append(out, ' ')
				i++
			}
		default:
			out = append(out, ch)
		}
	}
	return string(out)
}

func qEvalIsSymbolNameByte(ch byte) bool {
	switch {
	case ch >= 'a' && ch <= 'z', ch >= 'A' && ch <= 'Z', ch >= '0' && ch <= '9':
		return true
	case ch == '.' || ch == '_' || ch == ':' || ch == '/':
		return true
	default:
		return false
	}
}

// qEvalVerbEvidence is a pre-tokenized rendered benchmark expression: word
// verbs match against whole-identifier tokens; symbol operator verbs match as
// raw substrings of the literal-stripped text.
type qEvalVerbEvidence struct {
	stripped string
	words    map[string]bool
}

func qEvalNewVerbEvidence(expr string) qEvalVerbEvidence {
	stripped := qEvalStripQLiterals(expr)
	words := make(map[string]bool)
	for _, token := range qEvalWordTokenRE.FindAllString(stripped, -1) {
		words[token] = true
	}
	return qEvalVerbEvidence{stripped: stripped, words: words}
}

func (e qEvalVerbEvidence) matchesVerb(verb string) bool {
	if qEvalWordVerbRE.MatchString(verb) {
		return e.words[verb]
	}
	return strings.Contains(e.stripped, verb)
}

func qEvalRenderedCaseEvidence() []qEvalVerbEvidence {
	evidence := make([]qEvalVerbEvidence, 0, len(qEvalVectorCases))
	for _, tc := range qEvalVectorCases {
		evidence = append(evidence, qEvalNewVerbEvidence(tc.expr(qEvalVectorRows)))
	}
	return evidence
}

func qEvalAnyEvidenceMatchesVerb(evidence []qEvalVerbEvidence, verb string) bool {
	for _, e := range evidence {
		if e.matchesVerb(verb) {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Special-form evidence patterns.
// ---------------------------------------------------------------------------

type qEvalFormPattern struct {
	kind    string // "word", "raw", or "regex"
	pattern string
}

// qEvalFormEvidence maps each stdq.SupportedEvalForms() name to syntax
// patterns that count as benchmark evidence in a rendered expression
// (matched against literal-stripped text).
var qEvalFormEvidence = map[string][]qEvalFormPattern{
	"if":            {{kind: "word", pattern: "if"}},
	"do":            {{kind: "word", pattern: "do"}},
	"while":         {{kind: "word", pattern: "while"}},
	"cond":          {{kind: "raw", pattern: "$["}, {kind: "raw", pattern: "?["}},
	"each":          {{kind: "word", pattern: "each"}, {kind: "regex", pattern: `'($|[^:])`}},
	"each-prior":    {{kind: "raw", pattern: "':"}},
	"each-left":     {{kind: "raw", pattern: `\:`}},
	"each-right":    {{kind: "raw", pattern: "/:"}},
	"over":          {{kind: "regex", pattern: `/($|[^:])`}},
	"scan":          {{kind: "regex", pattern: `\\($|[^:])`}},
	"apply-at":      {{kind: "raw", pattern: "@["}},
	"apply-dot":     {{kind: "raw", pattern: ".["}},
	"dict-literal":  {{kind: "raw", pattern: "!"}},
	"table-literal": {{kind: "raw", pattern: "(["}},
	"fby":           {{kind: "word", pattern: "fby"}},
	"projection":    {{kind: "regex", pattern: `;\s*\]|\[\s*;`}},
	"composition":   {{kind: "regex", pattern: `\((count|first|last|sum|avg|reverse|raze|distinct|neg|abs|sums|where)\s+(count|first|last|sum|avg|reverse|raze|distinct|neg|abs|sums|where)\)`}},
}

func qEvalAnyEvidenceMatchesForm(t *testing.T, evidence []qEvalVerbEvidence, patterns []qEvalFormPattern) bool {
	t.Helper()
	for _, p := range patterns {
		var match func(e qEvalVerbEvidence) bool
		switch p.kind {
		case "word":
			match = func(e qEvalVerbEvidence) bool { return e.words[p.pattern] }
		case "raw":
			match = func(e qEvalVerbEvidence) bool { return strings.Contains(e.stripped, p.pattern) }
		case "regex":
			re := regexp.MustCompile(p.pattern)
			match = func(e qEvalVerbEvidence) bool { return re.MatchString(e.stripped) }
		default:
			t.Fatalf("unknown form pattern kind %q", p.kind)
		}
		for _, e := range evidence {
			if match(e) {
				return true
			}
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Tests.
// ---------------------------------------------------------------------------

func TestQEvalVerbMatcher(t *testing.T) {
	cases := []struct {
		name  string
		expr  string
		verb  string
		match bool
	}{
		{"InMatchesWordUse", "x:1 2 3;y:x in 1 2", "in", true},
		{"InNotInsideIntersect", "x:1 2 3;y:x intersect 1 2", "in", false},
		{"InNotInsideStringLiteral", `x like "init"`, "in", false},
		{"InNotInsideBacktickSymbol", "x:`init`intra;count x", "in", false},
		{"SumMatchesWordUse", "x:til 8;sum x", "sum", true},
		{"SumNotInsideSums", "x:til 8;sums x", "sum", false},
		{"SumNotInsideMsum", "x:til 8;3 msum x", "sum", false},
		{"MsumMatches", "x:til 8;3 msum x", "msum", true},
		{"PlusMatchesOperator", "x:til 8;+/x", "+", true},
		{"PlusNotInsideStringLiteral", `x:"a+b";count x`, "+", false},
		{"MinNotInsideBacktickSymbol", "x:`min`max;count x", "min", false},
		{"MinMatchesWordUse", "1 min 2", "min", true},
		{"CompositeNotEqual", "x:til 8;sum x<>3", "<>", true},
		{"LessMatchesInsideComposite", "x:til 8;sum x<=3", "<", true},
		{"BracketMatches", "x:til 8;x[3]", "[", true},
		{"WordNotInsideCamelIdent", "mySumTotal:3;mySumTotal", "sum", false},
		{"TildeNotInsideString", `x like "a~b"`, "~", false},
		{"HsymStripped", "x:`:tmp.bin;1", "bin", false},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			evidence := qEvalNewVerbEvidence(tc.expr)
			if got := evidence.matchesVerb(tc.verb); got != tc.match {
				t.Fatalf("matchesVerb(%q) on %q = %v, want %v (stripped=%q)", tc.verb, tc.expr, got, tc.match, evidence.stripped)
			}
		})
	}
}

func TestQEvalVectorVerbRegistryCoverage(t *testing.T) {
	tables := qEvalExtractVerbTables(t)
	for name, verbs := range tables {
		t.Logf("%s: %d verbs extracted", name, len(verbs))
	}
	union := qEvalVerbUnion(tables)
	if len(union) == 0 {
		t.Fatal("no verbs extracted from eval.go")
	}
	verbSet := make(map[string]bool, len(union))
	for _, verb := range union {
		verbSet[verb] = true
	}

	if len(qEvalVerbExclusions) > qEvalVerbExclusionsMax {
		t.Fatalf("qEvalVerbExclusions has %d entries, exceeding shrink-only max %d", len(qEvalVerbExclusions), qEvalVerbExclusionsMax)
	}
	if len(qEvalVerbCoverageBacklog) > qEvalVerbCoverageBacklogMax {
		t.Fatalf("qEvalVerbCoverageBacklog has %d entries, exceeding shrink-only max %d", len(qEvalVerbCoverageBacklog), qEvalVerbCoverageBacklogMax)
	}
	for verb := range qEvalVerbExclusions {
		if !verbSet[verb] {
			t.Errorf("qEvalVerbExclusions has stale entry %q: not an extracted verb", verb)
		}
		if _, dup := qEvalVerbCoverageBacklog[verb]; dup {
			t.Errorf("verb %q appears in both qEvalVerbExclusions and qEvalVerbCoverageBacklog", verb)
		}
	}
	for verb := range qEvalVerbCoverageBacklog {
		if !verbSet[verb] {
			t.Errorf("qEvalVerbCoverageBacklog has stale entry %q: not an extracted verb", verb)
		}
	}

	evidence := qEvalRenderedCaseEvidence()
	var uncovered []string
	for _, verb := range union {
		covered := qEvalAnyEvidenceMatchesVerb(evidence, verb)
		_, excluded := qEvalVerbExclusions[verb]
		_, backlogged := qEvalVerbCoverageBacklog[verb]
		switch {
		case covered && excluded:
			t.Errorf("verb %q is covered by qEvalVectorCases but still listed in qEvalVerbExclusions; remove the stale exclusion", verb)
		case covered && backlogged:
			t.Errorf("verb %q is covered by qEvalVectorCases but still listed in qEvalVerbCoverageBacklog; remove the stale backlog entry", verb)
		case !covered && !excluded && !backlogged:
			uncovered = append(uncovered, verb)
		}
	}
	if len(uncovered) > 0 {
		t.Errorf("q.eval verbs without benchmark evidence (add a qEvalVectorCases case, or an exclusion/backlog entry with reason): %s", strings.Join(uncovered, ", "))
	}

	// Special forms: every SupportedEvalForms entry needs pattern evidence or
	// a backlog entry.
	forms := stdq.SupportedEvalForms()
	if len(qEvalFormBacklog) > qEvalFormBacklogMax {
		t.Fatalf("qEvalFormBacklog has %d entries, exceeding shrink-only max %d", len(qEvalFormBacklog), qEvalFormBacklogMax)
	}
	formSet := make(map[string]bool, len(forms))
	for _, form := range forms {
		formSet[form] = true
	}
	for form := range qEvalFormBacklog {
		if !formSet[form] {
			t.Errorf("qEvalFormBacklog has stale entry %q: not in SupportedEvalForms", form)
		}
	}
	for form := range qEvalFormEvidence {
		if !formSet[form] {
			t.Errorf("qEvalFormEvidence has stale entry %q: not in SupportedEvalForms", form)
		}
	}
	var uncoveredForms []string
	for _, form := range forms {
		patterns, ok := qEvalFormEvidence[form]
		if !ok {
			t.Errorf("form %q from SupportedEvalForms has no evidence pattern in qEvalFormEvidence", form)
			continue
		}
		covered := qEvalAnyEvidenceMatchesForm(t, evidence, patterns)
		_, backlogged := qEvalFormBacklog[form]
		switch {
		case covered && backlogged:
			t.Errorf("form %q is covered by qEvalVectorCases but still listed in qEvalFormBacklog; remove the stale backlog entry", form)
		case !covered && !backlogged:
			uncoveredForms = append(uncoveredForms, form)
		}
	}
	if len(uncoveredForms) > 0 {
		t.Errorf("q.eval special forms without benchmark evidence (add a case or a qEvalFormBacklog entry): %s", strings.Join(uncoveredForms, ", "))
	}
}

func TestQEvalVectorComboShapeCoverage(t *testing.T) {
	if len(qEvalComboBacklog) > qEvalComboBacklogMax {
		t.Fatalf("qEvalComboBacklog has %d entries, exceeding shrink-only max %d", len(qEvalComboBacklog), qEvalComboBacklogMax)
	}
	required := make(map[string]bool, len(qEvalRequiredComboShapes))
	for _, shape := range qEvalRequiredComboShapes {
		required[shape] = true
	}
	for shape := range qEvalComboBacklog {
		if !required[shape] {
			t.Errorf("qEvalComboBacklog has stale entry %q: not in qEvalRequiredComboShapes", shape)
		}
	}
	counts := make(map[string]int)
	for _, tc := range qEvalVectorCases {
		for _, tag := range tc.tags {
			if required[tag] {
				counts[tag]++
			}
		}
	}
	var missing []string
	for _, shape := range qEvalRequiredComboShapes {
		covered := counts[shape] >= qEvalComboMinCases
		_, backlogged := qEvalComboBacklog[shape]
		switch {
		case covered && backlogged:
			t.Errorf("combo shape %q has %d cases (>= %d) but is still listed in qEvalComboBacklog; remove the stale backlog entry", shape, counts[shape], qEvalComboMinCases)
		case !covered && !backlogged:
			missing = append(missing, fmt.Sprintf("%s (have %d, need %d)", shape, counts[shape], qEvalComboMinCases))
		}
	}
	if len(missing) > 0 {
		t.Errorf("combo shape coverage missing: %s", strings.Join(missing, ", "))
	}
}
