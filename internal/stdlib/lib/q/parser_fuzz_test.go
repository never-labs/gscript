package q

// Parser crash-safety fuzzing.
//
// FuzzQParse feeds arbitrary input through every parse-shaped entry point
// that runs before evaluation: statement splitting, the statement compiler
// (which exercises the shared leaf parsers: parseAtomOrVector,
// parseSymbolList, parseTemporalAtomOrVector, ...), and the qSQL parser.
// None of them may panic on any input; parse errors are fine.
//
// Parse->render->parse stability is intentionally not tested: the q AST has
// no renderer.
//
// Known crashers being fixed on a concurrent branch are pinned in
// qParseKnownCrashers; TestQParseKnownCrashersStillCrash demands the entry's
// removal the moment the fix lands (shrink-only), after which the fuzzer
// guards the fix.

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"strings"
	"testing"
	"unicode/utf8"
)

// qParseKnownCrashers maps a representative input to a substring of the panic
// value it currently raises. Inputs whose panic matches an entry are skipped
// by the fuzzer instead of failing; the companion test below fails as soon as
// the crash is fixed, demanding the entry's removal.
var qParseKnownCrashers = map[string]string{}

func qParseKnownCrasherPanic(r any) bool {
	text := panicText(r)
	for _, want := range qParseKnownCrashers {
		if strings.Contains(text, want) {
			return true
		}
	}
	return false
}

func panicText(r any) string {
	return fmt.Sprint(r)
}

// qParseProbe runs every parse entry point on src. It reports the panic
// value, if any.
func qParseProbe(src string) (panicked any) {
	defer func() {
		panicked = recover()
	}()
	for _, stmt := range splitQScriptStatements(src) {
		// compileQStatementExpr (uncached, to keep the process-wide compiled
		// statement cache free of fuzz garbage) walks the same split helpers
		// and leaf parsers the string evaluator uses.
		_ = compileQStatementExpr(stmt)
	}
	_, _ = Parse(src) // qSQL parser
	return nil
}

// qParseSeedCorpus extracts every string literal passed to the assertEval*
// helpers in eval_test.go: a large, realistic corpus of valid q statements.
func qParseSeedCorpus(t interface{ Fatalf(string, ...any) }) []string {
	fset := gotoken.NewFileSet()
	file, err := goparser.ParseFile(fset, "eval_test.go", nil, 0)
	if err != nil {
		t.Fatalf("parse eval_test.go: %v", err)
	}
	var seeds []string
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if !ok || !strings.HasPrefix(ident.Name, "assertEval") || len(call.Args) < 2 {
			return true
		}
		if src, ok := qErrorStringLiteral(call.Args[1]); ok {
			seeds = append(seeds, src)
		}
		return true
	})
	if len(seeds) < 500 {
		t.Fatalf("extracted only %d parser seeds from eval_test.go", len(seeds))
	}
	return seeds
}

// qParseHostileSeeds are hand-written hostile inputs: unterminated literals,
// deep nesting, stray adverbs, malformed temporals, NULs, and the known
// juxtaposition crasher.
var qParseHostileSeeds = []string{
	`1 "hello"`,
	`2 "x" 3`,
	"((((((((((",
	"))))",
	"`",
	"``",
	"`a`",
	"0x",
	"0xzz",
	"1..2",
	";;;",
	"{",
	"}",
	"{}[",
	"{[}",
	`"unterminated`,
	"1 2 3#",
	"#1 2",
	"_1",
	"!@#$%^&*",
	"0N0N",
	"0Nx",
	"2024.13.45",
	"2024.02.30T99:99:99.999",
	"12:99",
	"25:61:61",
	"`a`b!",
	"!`a`b",
	"$[;;]",
	"?[;]",
	"@[;;;]",
	".[;]",
	"+/",
	"\\:",
	"/:",
	"':",
	"a:;b",
	"::",
	"\\",
	"\\p",
	"select from",
	"select a from t where",
	"x y z w",
	"1 2 3 4f 5",
	"1f2",
	"1e999",
	"-",
	"- -",
	"0:",
	"1:",
	"`:path",
	"(;)",
	"(;;)",
	"01b0",
	"101bb",
	"\x00",
	"1 \x00 2",
	"\xff\xfe",
	strings.Repeat("(", 64) + "1" + strings.Repeat(")", 64),
	strings.Repeat("{x+", 32) + "1" + strings.Repeat("}", 32),
	strings.Repeat("`a", 100),
	strings.Repeat("1 ", 200),
}

func FuzzQParse(f *testing.F) {
	for _, seed := range qParseSeedCorpus(f) {
		f.Add(seed)
	}
	for _, seed := range qParseHostileSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		if len(src) > 4096 || !utf8.ValidString(src) {
			t.Skip("out of scope: oversized or invalid UTF-8")
		}
		if r := qParseProbe(src); r != nil {
			if qParseKnownCrasherPanic(r) {
				t.Skipf("known crasher (fix in flight): %v", r)
			}
			t.Fatalf("parser panicked on %q: %v", src, r)
		}
	})
}

// TestQParseKnownCrashersStillCrash keeps qParseKnownCrashers shrink-only:
// the moment a listed input stops panicking, this test fails and demands the
// entry's removal so the fuzzer starts guarding the fix.
func TestQParseKnownCrashersStillCrash(t *testing.T) {
	for src, want := range qParseKnownCrashers {
		r := qParseProbe(src)
		if r == nil {
			t.Errorf("known crasher %q no longer panics: remove it from qParseKnownCrashers (shrink-only) so FuzzQParse guards the fix", src)
			continue
		}
		if !strings.Contains(panicText(r), want) {
			t.Errorf("known crasher %q panics differently\n  want substring %q\n  got            %v", src, want, r)
		}
	}
}

// TestQParseSeedsNoPanic runs the full seed corpus through the parse probe in
// normal CI (fuzz seeds only execute under go test for the fuzz target's own
// package run; this makes the guarantee explicit and fast).
func TestQParseSeedsNoPanic(t *testing.T) {
	seeds := qParseSeedCorpus(t)
	checked := 0
	for _, src := range append(seeds, qParseHostileSeeds...) {
		if r := qParseProbe(src); r != nil {
			if qParseKnownCrasherPanic(r) {
				continue
			}
			t.Errorf("parser panicked on seed %q: %v", src, r)
		}
		checked++
	}
	t.Logf("parser seed corpus: %d inputs, no unexpected panics", checked)
}
