package tests_test

import (
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

// TestChessAI_TreeWalker is skipped: the chess AI benchmark searches depth 1-8
// which takes >10 minutes on the tree-walker interpreter. Tree-walker correctness
// is covered by the other interpreter test suites (lexer, parser, runtime).
func TestChessAI_TreeWalker(t *testing.T) {
	t.Skip("chess AI benchmark is too slow for tree-walker; covered by VM test")
}
