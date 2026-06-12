package q

// Golden + consistency guard for the four source-eligibility predicates:
//
//	EvalSourceCacheable          — bind/session/JIT value-memo and global plan gate
//	qEvalConstantStatementSource — per-session constant-value memoization
//	qScriptBindingPlanTextEligible — cached script binding plans
//	cachedValueExprTextEligible  — cached compiled value expressions
//
// The golden file pins each predicate's verdict on a curated statement corpus
// so refactors of the shared verb-metadata table cannot silently change which
// sources are memoized. Regenerate with:
//
//	UPDATE_SOURCE_ELIGIBILITY_GOLDEN=1 go test ./internal/stdlib/lib/q -run TestSourceEligibilityGolden
import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sourceEligibilityCorpus is a curated cross-section of q.eval statements:
// deterministic arithmetic/vector/verb forms, adverbs, lambdas, assignments,
// dict/table forms, random (roll/deal/rand) forms, stateful workspace forms
// (insert/upsert/get/set/hopen/system/backslash commands), clock reads (.z.*),
// and textual edge cases that pin the historical scanning quirks.
var sourceEligibilityCorpus = []string{
	// Deterministic arithmetic and vector forms.
	"1+2",
	"2*3+4",
	"1 2 3+4 5 6",
	"til 10",
	"sum til 100",
	"avg til 8",
	"sums til 16",
	"+/til 10",
	"+\\til 10",
	"(+/)til 10",
	"max 3 1 2",
	"min 3 1 2",
	"med 1 2 3 4",
	"dev til 10",
	"var til 10",
	"sqrt 16",
	"exp 1",
	"log 2.718",
	"abs -5",
	"signum -3 0 7",
	"floor 3.7",
	"ceiling 3.2",
	"reciprocal 4",
	"2 xexp 10",
	"2 xlog 8",
	"1.5*til 6",
	"100%7",
	"7 mod 3",
	"7 div 3",
	"neg til 5",
	"reverse til 5",
	"distinct 1 2 2 3",
	"asc 3 1 2",
	"desc 3 1 2",
	"iasc 3 1 2",
	"rank 30 10 20",
	"deltas 1 3 6",
	"ratios 1 2 4 8",
	"fills 1 0N 3",
	"count til 7",
	"first 1 2 3",
	"last 1 2 3",
	"3#til 10",
	"-3#til 10",
	"1_til 5",
	"til 10 within 2 6",
	"1 2 3 in 2 3",
	"2 4 6 bin 5",
	"3 xbar til 10",
	"3 msum til 10",
	"3 mavg til 10",
	"1 xprev til 5",
	"1 rotate til 5",
	"2 cut til 6",
	"0 2 sublist til 10",
	"1 2 cross 3 4",
	"1 2 3 union 3 4",
	"1 2 3 inter 2 3",
	"1 2 3 except 2",
	"all 1 1 1",
	"any 0 0 1",
	"not 0 1",
	"null 0N 1",
	"where 0 1 1 0",
	"group 1 1 2",
	"string 123",
	"upper \"abc\"",
	"lower \"ABC\"",
	"type 1 2 3",
	"enlist 5",
	"raze (1 2;3 4)",
	// Adverbs, lambdas, assignments, apply forms.
	"{x*x}[4]",
	"{x+y}[3;4]",
	"{x+y}/[1 2 3]",
	"f:{x+1};f 3",
	"x:5",
	"x:til 10;sum x",
	"a:1;b:2;a+b",
	"x:1 2 3;x[0]",
	"$[1b;1;2]",
	"@[1 2 3;0;:;9]",
	// Dict / table forms.
	"`a`b!1 2",
	"d:`a`b!1 2;d`a",
	"([] a:1 2;b:3 4)",
	"t:([] a:1 2;b:3 4);count t",
	"?[([] a:1 2);();0b;()]",
	// Symbols and strings.
	"`a`b`c",
	"count `a`b",
	"\"hello\"",
	"`sym=`sym",
	// Random forms: roll, deal, rand (and the conservative find-shaped scans).
	"5?10",
	"-5?10",
	"rand 10",
	"rand 1f",
	"n?10",
	"x:5?10",
	"20?1000000",
	"1 2 3?2",
	"x:1 2 3;x?2",
	"2?`a`b`c",
	"-3?5",
	// Stateful workspace / system / IPC / filesystem forms.
	"`a set 5",
	"get `a",
	"x:get `a",
	"system \"ls\"",
	"\\ts 1+1",
	"\\l script.q",
	"hopen `:localhost:5001",
	"hsym `localhost",
	"t:([] a:1 2);`t insert (5;6)",
	"`t upsert (1;2)",
	"x insert (1;2)",
	"tab upsert row",
	// Clock reads.
	".z.p",
	".z.d",
	"x:.z.p",
	// Textual edge cases pinning historical scan quirks.
	"offset + 1",
	"target 5",
	"xinserted:1",
	"\"insert\"",
	"`insert",
	"set",
	"get",
	"",
	"   ",
	"0W",
	"0N",
	"2024.01.01",
	"09:30:00",
	"1#2",
}

// TestSourceEligibilityVerbConsistency asserts the four predicates' verb-
// driven decisions stay mutually consistent on the corpus:
//
//   - a source flagged Random by the shared verb scan is never memoized by
//     any layer (the R9 invariant: roll/deal/rand results must not be value-
//     memoized or folded into cached plans);
//   - a source flagged Stateful or ReadsClock is never value-memoized
//     (EvalSourceCacheable and the constant-statement memo both reject it);
//   - a constant statement is always source-cacheable (the constant memo is
//     the strictest layer).
func TestSourceEligibilityVerbConsistency(t *testing.T) {
	for _, src := range sourceEligibilityCorpus {
		flags := qSourceVerbFlags(src)
		if flags.Random {
			if EvalSourceCacheable(src) {
				t.Errorf("EvalSourceCacheable(%q) = true for a Random-flagged source", src)
			}
			if qEvalConstantStatementSource(src) {
				t.Errorf("qEvalConstantStatementSource(%q) = true for a Random-flagged source", src)
			}
			if qScriptBindingPlanTextEligible(src) {
				t.Errorf("qScriptBindingPlanTextEligible(%q) = true for a Random-flagged source", src)
			}
			if cachedValueExprTextEligible(src) {
				t.Errorf("cachedValueExprTextEligible(%q) = true for a Random-flagged source", src)
			}
		}
		if flags.Stateful || flags.ReadsClock {
			if EvalSourceCacheable(src) {
				t.Errorf("EvalSourceCacheable(%q) = true for a stateful/clock source", src)
			}
			if qEvalConstantStatementSource(src) {
				t.Errorf("qEvalConstantStatementSource(%q) = true for a stateful/clock source", src)
			}
		}
		if qEvalConstantStatementSource(src) && !EvalSourceCacheable(src) {
			t.Errorf("%q is constant-memoizable but not source-cacheable", src)
		}
	}
}

func sourceEligibilityVerdictLine(src string) string {
	return fmt.Sprintf("%t\t%t\t%t\t%t\t%s",
		EvalSourceCacheable(src),
		qEvalConstantStatementSource(src),
		qScriptBindingPlanTextEligible(src),
		cachedValueExprTextEligible(src),
		src)
}

const sourceEligibilityGoldenPath = "testdata/source_eligibility_golden.tsv"
const sourceEligibilityGoldenHeader = "# cacheable\tconstant\tbindingText\tvalueExprText\tsource"

// TestSourceEligibilityGolden asserts the four predicates' verdicts on the
// corpus are byte-identical to the committed golden: refactors of the verb
// metadata table must not change behavior for current sources.
func TestSourceEligibilityGolden(t *testing.T) {
	lines := make([]string, 0, len(sourceEligibilityCorpus)+1)
	lines = append(lines, sourceEligibilityGoldenHeader)
	for _, src := range sourceEligibilityCorpus {
		lines = append(lines, sourceEligibilityVerdictLine(src))
	}
	got := strings.Join(lines, "\n") + "\n"
	if os.Getenv("UPDATE_SOURCE_ELIGIBILITY_GOLDEN") != "" {
		if err := os.MkdirAll(filepath.Dir(sourceEligibilityGoldenPath), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(sourceEligibilityGoldenPath, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("golden updated: %s", sourceEligibilityGoldenPath)
		return
	}
	want, err := os.ReadFile(sourceEligibilityGoldenPath)
	if err != nil {
		t.Fatalf("read golden (regenerate with UPDATE_SOURCE_ELIGIBILITY_GOLDEN=1): %v", err)
	}
	if got == string(want) {
		return
	}
	gotLines := strings.Split(got, "\n")
	wantLines := strings.Split(string(want), "\n")
	for i := 0; i < len(gotLines) && i < len(wantLines); i++ {
		if gotLines[i] != wantLines[i] {
			t.Errorf("verdict drift at golden line %d:\n  want %q\n  got  %q", i+1, wantLines[i], gotLines[i])
		}
	}
	if len(gotLines) != len(wantLines) {
		t.Errorf("golden line count drift: want %d, got %d", len(wantLines), len(gotLines))
	}
}
