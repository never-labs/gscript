package modules

import (
	"testing"

	"github.com/never-labs/gscript/internal/runtime"
)

func TestMatrixDenseGetSet(t *testing.T) {
	interp := runWithLib(t, `
m := matrix.dense(2, 3)
matrix.setf(m, 1, 2, 7.5)
result := matrix.getf(m, 1, 2)
`, "matrix", BuildMatrix())
	got := interp.GetGlobal("result")
	if !got.IsFloat() || got.Float() != 7.5 {
		t.Fatalf("matrix.getf result = %v, want 7.5", got)
	}
	m := interp.GetGlobal("m")
	if !m.IsTable() || m.Table().DMStride() != 3 {
		t.Fatalf("matrix.dense did not preserve DenseMatrix metadata")
	}
}

func TestMatrixFastPathsInstalled(t *testing.T) {
	lib := BuildMatrix()
	for _, tc := range []struct {
		name string
		ok   func(*runtime.GoFunction) bool
	}{
		{name: "dense", ok: func(gf *runtime.GoFunction) bool { return gf.FastArg2 != nil }},
		{name: "getf", ok: func(gf *runtime.GoFunction) bool { return gf.FastArg3 != nil }},
		{name: "setf", ok: func(gf *runtime.GoFunction) bool { return gf.FastArg4 != nil }},
	} {
		gf := lib.RawGetString(tc.name).GoFunction()
		if gf == nil || gf.Name != "matrix."+tc.name || !tc.ok(gf) {
			t.Fatalf("matrix.%s fast binding missing: %#v", tc.name, gf)
		}
	}
}
