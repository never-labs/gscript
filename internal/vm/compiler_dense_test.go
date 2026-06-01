package vm

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestCompilerDenseLiteralLowersToArrayConstructors(t *testing.T) {
	globals := compileAndRun(t, `
xs := []f64{1, 2, 3}
ids := [3]i64{101, 102, 103}
mask := []bool{true, false, true}
points := soa.zip({x: xs, id: ids, mask: mask})
soa.addScaled(points, "x", "id", 0.5)
sum := soa.sum(points, "x")
`)
	sum := globals["sum"]
	if !sum.IsFloat() || sum.Float() != 159 {
		t.Fatalf("sum = %s, want 159", sum.String())
	}
	points := globals["points"]
	if !points.IsSoA() {
		t.Fatalf("points = %s, want soa", points.String())
	}
	col, ok := points.SoA().Column("mask")
	if !ok {
		t.Fatal("missing mask column")
	}
	if col.DType() != runtime.DenseArrayBool {
		t.Fatalf("mask dtype = %v, want bool", col.DType())
	}
}

func TestCompilerDenseLiteralRejectsLengthMismatch(t *testing.T) {
	err := compileOrRunError(t, `xs := [2]f64{1, 2, 3}`)
	if err == nil {
		t.Fatal("expected dense literal length mismatch")
	}
}
