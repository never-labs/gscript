//go:build darwin && arm64

package benchmarks

import (
	"testing"

	stdq "github.com/never-labs/leia/internal/stdlib/lib/q"
)

// TestQEvalCompiledStatementDifferential is the differential oracle for
// statement compilation: every qEvalVectorCases expression runs statement by
// statement on a fresh state, and every statement the compiler can represent
// is evaluated through BOTH the compiled route and the string evaluator with
// identical session state. Results must match exactly (q ~ semantics);
// compiled errors that surface must carry the identical message. This pins
// the contract that compiling a statement never changes what users see.
func TestQEvalCompiledStatementDifferential(t *testing.T) {
	totalStatements := 0
	compiledStatements := 0
	for _, tc := range qEvalVectorCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			state := stdq.NewEvalState(nil)
			records, err := state.EvalScriptCompiledDifferential(tc.expr(qEvalVectorRows))
			if err != nil {
				t.Fatalf("script error: %v", err)
			}
			for _, record := range records {
				totalStatements++
				if record.Compiled {
					compiledStatements++
				}
				if !record.Match {
					t.Errorf("compiled/string divergence on %q (compiledErr=%v)", record.Statement, record.CompiledErr)
				}
			}
		})
	}
	t.Logf("differential coverage: %d/%d statements compiled", compiledStatements, totalStatements)
}
