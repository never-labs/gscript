package bind

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
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

func TestMatrixDenseRejectsInvalidDimensions(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "negative rows",
			src:  `matrix.dense(-1, 2)`,
			want: "matrix.dense: rows and cols must be non-negative",
		},
		{
			name: "negative cols",
			src:  `matrix.dense(1, -2)`,
			want: "matrix.dense: rows and cols must be non-negative",
		},
		{
			name: "float rows",
			src:  `matrix.dense(1.5, 2)`,
			want: "matrix.dense: rows and cols must be integers",
		},
		{
			name: "string cols",
			src:  `matrix.dense(1, "2")`,
			want: "matrix.dense: rows and cols must be integers",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runProgramExpectError(t, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestMatrixGetSetRejectsOutOfRangeIndices(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "get negative row",
			src: `
m := matrix.dense(2, 3)
matrix.getf(m, -1, 0)
`,
			want: "matrix.getf: index out of range",
		},
		{
			name: "get row past end",
			src: `
m := matrix.dense(2, 3)
matrix.getf(m, 2, 0)
`,
			want: "matrix.getf: index out of range",
		},
		{
			name: "get negative col",
			src: `
m := matrix.dense(2, 3)
matrix.getf(m, 0, -1)
`,
			want: "matrix.getf: index out of range",
		},
		{
			name: "get col past end",
			src: `
m := matrix.dense(2, 3)
matrix.getf(m, 0, 3)
`,
			want: "matrix.getf: index out of range",
		},
		{
			name: "set negative row",
			src: `
m := matrix.dense(2, 3)
matrix.setf(m, -1, 0, 1)
`,
			want: "matrix.setf: index out of range",
		},
		{
			name: "set row past end",
			src: `
m := matrix.dense(2, 3)
matrix.setf(m, 2, 0, 1)
`,
			want: "matrix.setf: index out of range",
		},
		{
			name: "set negative col",
			src: `
m := matrix.dense(2, 3)
matrix.setf(m, 0, -1, 1)
`,
			want: "matrix.setf: index out of range",
		},
		{
			name: "set col past end",
			src: `
m := matrix.dense(2, 3)
matrix.setf(m, 0, 3, 1)
`,
			want: "matrix.setf: index out of range",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runProgramExpectError(t, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestMatrixGetSetRejectsNonIntegerIndices(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
		want string
	}{
		{
			name: "get float row",
			src: `
m := matrix.dense(2, 3)
matrix.getf(m, 0.5, 0)
`,
			want: "matrix.getf: row and column must be integers",
		},
		{
			name: "get string col",
			src: `
m := matrix.dense(2, 3)
matrix.getf(m, 0, "1")
`,
			want: "matrix.getf: row and column must be integers",
		},
		{
			name: "set float row",
			src: `
m := matrix.dense(2, 3)
matrix.setf(m, 0.5, 0, 1)
`,
			want: "matrix.setf: row and column must be integers",
		},
		{
			name: "set string col",
			src: `
m := matrix.dense(2, 3)
matrix.setf(m, 0, "1", 1)
`,
			want: "matrix.setf: row and column must be integers",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runProgramExpectError(t, tc.src)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestMatrixSetRejectsNonNumericValues(t *testing.T) {
	for _, tc := range []struct {
		name string
		src  string
	}{
		{
			name: "string",
			src: `
m := matrix.dense(1, 1)
matrix.setf(m, 0, 0, "1")
`,
		},
		{
			name: "bool",
			src: `
m := matrix.dense(1, 1)
matrix.setf(m, 0, 0, true)
`,
		},
		{
			name: "nil",
			src: `
m := matrix.dense(1, 1)
matrix.setf(m, 0, 0, nil)
`,
		},
		{
			name: "table",
			src: `
m := matrix.dense(1, 1)
matrix.setf(m, 0, 0, {})
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := runProgramExpectError(t, tc.src)
			if err == nil || !strings.Contains(err.Error(), "matrix.setf: value must be numeric") {
				t.Fatalf("error = %v, want matrix.setf numeric error", err)
			}
		})
	}
}
