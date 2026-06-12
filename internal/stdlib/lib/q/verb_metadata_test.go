package q

// Completeness gate for the verb metadata table: the set of dispatchable
// verbs is extracted mechanically from the switch statements in eval.go
// (lookupDyadicVerb, lookupDyadicVerbFunc, lookupUnaryVerb), mirroring
// benchmarks/q_eval_verb_coverage_test.go, so a verb added to a dispatch
// switch without a metadata entry fails loudly here.

import (
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"
)

var qVerbMetadataLookupFuncFloors = map[string]int{
	"lookupDyadicVerb":     15,
	"lookupDyadicVerbFunc": 35,
	"lookupUnaryVerb":      50,
}

// qVerbMetadataExtractDispatchVerbs parses eval.go and returns the union of
// string literals collected from the case clauses of switch statements that
// dispatch on the verb parameter of the three lookup functions.
func qVerbMetadataExtractDispatchVerbs(t *testing.T) []string {
	t.Helper()
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate test source file")
	}
	path := filepath.Join(filepath.Dir(thisFile), "eval.go")
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, path, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	seen := make(map[string]bool)
	found := make(map[string]bool)
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*ast.FuncDecl)
		if !isFunc || fn.Body == nil {
			continue
		}
		floor, want := qVerbMetadataLookupFuncFloors[fn.Name.Name]
		if !want {
			continue
		}
		params := fn.Type.Params
		if params == nil || len(params.List) == 0 || len(params.List[0].Names) == 0 {
			t.Fatalf("%s: expected a named verb parameter", fn.Name.Name)
		}
		param := params.List[0].Names[0].Name
		count := 0
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sw, isSwitch := n.(*ast.SwitchStmt)
			if !isSwitch || sw.Tag == nil || !qVerbMetadataExprMentionsIdent(sw.Tag, param) {
				return true
			}
			for _, stmt := range sw.Body.List {
				clause, isClause := stmt.(*ast.CaseClause)
				if !isClause {
					continue
				}
				for _, expr := range clause.List {
					lit, isLit := expr.(*ast.BasicLit)
					if !isLit || lit.Kind != gotoken.STRING {
						continue
					}
					value, err := strconv.Unquote(lit.Value)
					if err != nil {
						continue
					}
					seen[value] = true
					count++
				}
			}
			return true
		})
		if count < floor {
			t.Fatalf("%s: extracted %d verbs, below robustness floor %d; eval.go dispatch structure may have changed", fn.Name.Name, count, floor)
		}
		found[fn.Name.Name] = true
	}
	for name := range qVerbMetadataLookupFuncFloors {
		if !found[name] {
			t.Fatalf("function %s not found in %s; eval.go dispatch structure may have changed", name, path)
		}
	}
	verbs := make([]string, 0, len(seen))
	for verb := range seen {
		verbs = append(verbs, verb)
	}
	sort.Strings(verbs)
	return verbs
}

func qVerbMetadataExprMentionsIdent(expr ast.Expr, name string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if ident, isIdent := n.(*ast.Ident); isIdent && ident.Name == name {
			found = true
			return false
		}
		return !found
	})
	return found
}

// TestQVerbMetadataCompleteness asserts every dispatch-table verb has a verb
// metadata entry, and that the hardcoded qDispatchVerbNames list stays in
// lockstep with the dispatch switches (both directions), so a new verb
// without an explicit classification fails loudly.
func TestQVerbMetadataCompleteness(t *testing.T) {
	extracted := qVerbMetadataExtractDispatchVerbs(t)
	t.Logf("dispatch switches: %d verbs extracted; metadata table: %d entries", len(extracted), len(qVerbMetadataTable))
	extractedSet := make(map[string]bool, len(extracted))
	var missing []string
	for _, verb := range extracted {
		extractedSet[verb] = true
		if _, ok := qVerbMetadataLookup(verb); !ok {
			missing = append(missing, verb)
		}
	}
	if len(missing) > 0 {
		t.Errorf("dispatch verbs without a verb metadata entry (add to qDispatchVerbNames or qVerbMetadataExceptions in verb_metadata.go): %s", strings.Join(missing, ", "))
	}
	// Numeric descriptor-map verbs are merged mechanically; verify anyway so
	// a builder regression cannot drop them.
	for name := range qNumericUnaryVerbDescriptors {
		if _, ok := qVerbMetadataLookup(name); !ok {
			t.Errorf("numeric unary verb %q missing from the metadata table", name)
		}
	}
	for name := range qNumericDyadicFloatVerbDescriptors {
		if _, ok := qVerbMetadataLookup(name); !ok {
			t.Errorf("numeric dyadic verb %q missing from the metadata table", name)
		}
	}
	for name := range qEvalConstantWords {
		if _, ok := qVerbMetadataLookup(name); !ok {
			t.Errorf("constant word %q missing from the metadata table", name)
		}
	}
	// Staleness: every hardcoded dispatch name must still exist in a switch.
	for _, name := range qDispatchVerbNames {
		if !extractedSet[name] {
			t.Errorf("qDispatchVerbNames has stale entry %q: not in any dispatch switch", name)
		}
	}
}

// TestQVerbMetadataInvariants pins the structural invariants of the table:
// Deterministic is exactly the complement of the random/stateful/clock flags,
// and every exception entry is actually flagged.
func TestQVerbMetadataInvariants(t *testing.T) {
	for name, props := range qVerbMetadataTable {
		flagged := props.Stateful || props.ReadsClock || props.Random
		if props.Deterministic == flagged {
			t.Errorf("table entry %q: Deterministic=%t inconsistent with flags %+v", name, props.Deterministic, props)
		}
		if qVerbMemoUnsafe(props) != flagged {
			t.Errorf("table entry %q: qVerbMemoUnsafe disagrees with flags %+v", name, props)
		}
	}
	for name, props := range qVerbMetadataExceptions {
		if !qVerbMemoUnsafe(props) {
			t.Errorf("exception entry %q carries no flag: %+v", name, props)
		}
		got, ok := qVerbMetadataLookup(name)
		if !ok || got != props {
			t.Errorf("exception entry %q not reflected in the table: %+v vs %+v", name, props, got)
		}
		if _, clash := qEvalConstantWords[name]; clash {
			t.Errorf("exception entry %q is also a constant word; the table overlay would mask the exception ordering intent", name)
		}
	}
	// Default for unknown user names is deterministic with no flags.
	if props := qVerbMetadata("someUserName"); !props.Deterministic || qVerbMemoUnsafe(props) {
		t.Errorf("unknown-name default = %+v, want plain deterministic", props)
	}
}
