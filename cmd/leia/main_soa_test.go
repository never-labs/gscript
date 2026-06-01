package main

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
)

func TestSoADirectAccessRunsInInterpreterAndVM(t *testing.T) {
	source := `
points := soa.zip({
    x: []f64{1, 2, 3},
    y: []f64{10, 20, 30},
    id: []i64{101, 102, 103},
})
xcol := points.x
row := points[2]
row.x = 42
points[2] = row
points.y = []f64{100, 200, 300}
points.z = []i64{7, 8, 9}
assert(xcol[2] == 42)
assert(points.x[2] == 42)
assert(points["x"][3] == 3)
assert(points[2].x == 42)
assert(points.y[3] == 300)
assert(points.z[1] == 7)
assert(points.missing == nil)
`
	for _, tc := range []struct {
		name string
		run  func(*runtime.Interpreter, string) error
	}{
		{name: "interpreter", run: runString},
		{name: "bytecode", run: func(interp *runtime.Interpreter, src string) error {
			return runStringVM(interp, src, false, false, jitCLIOptions{})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			interp := newCLIInterpreter()
			if err := tc.run(interp, source); err != nil {
				t.Fatalf("run: %v", err)
			}
		})
	}
}
