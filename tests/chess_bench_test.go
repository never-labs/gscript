package tests_test

import (
	"os"
	"path/filepath"
	"testing"

	gs "github.com/never-labs/gscript"
)

// BenchmarkChessAI runs the chess_bench.gs Xiangqi AI benchmark script using the VM.
func BenchmarkChessAI(b *testing.B) {
	chessBenchPath, err := filepath.Abs(filepath.Join("..", "examples", "game_engine", "chess_bench.gs"))
	if err != nil {
		b.Fatalf("failed to resolve chess_bench.gs path: %v", err)
	}

	for i := 0; i < b.N; i++ {
		vm := gs.New(gs.WithVM(), gs.WithPrint(func(args ...interface{}) {
			// Suppress output during benchmark
		}))
		if err := vm.ExecFile(chessBenchPath); err != nil {
			b.Fatalf("chess_bench.gs execution error: %v", err)
		}
	}
}

// TestChessAI_Completes verifies that the chess AI benchmark script runs to
// completion without error using the VM.
func TestChessAI_Completes(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chess AI benchmark in short mode")
	}
	if os.Getenv("GSCRIPT_RUN_CHESS_BENCH_TEST") != "1" {
		t.Skip("set GSCRIPT_RUN_CHESS_BENCH_TEST=1 to run the long chess benchmark completion test")
	}

	chessBenchPath, err := filepath.Abs(filepath.Join("..", "examples", "game_engine", "chess_bench.gs"))
	if err != nil {
		t.Fatalf("failed to resolve chess_bench.gs path: %v", err)
	}

	var lines []string
	vm := gs.New(gs.WithVM(), gs.WithPrint(func(args ...interface{}) {
		// Capture output to verify something was printed
		if len(args) > 0 {
			s, ok := args[0].(string)
			if ok {
				lines = append(lines, s)
			}
		}
	}))

	if err := vm.ExecFile(chessBenchPath); err != nil {
		t.Fatalf("chess_bench.gs execution error: %v", err)
	}

	if len(lines) == 0 {
		t.Error("expected output from chess_bench.gs, got none")
	}

	// Verify the script printed a completion message
	found := false
	for _, line := range lines {
		if line == "Benchmark complete." {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Benchmark complete.' in output")
	}
}

// TestChessAI_TreeWalker is skipped: the chess AI benchmark searches depth 1-8
// which takes >10 minutes on the tree-walker interpreter. Tree-walker correctness
// is covered by the other interpreter test suites (lexer, parser, runtime).
func TestChessAI_TreeWalker(t *testing.T) {
	t.Skip("chess AI benchmark is too slow for tree-walker; covered by VM test")
}
