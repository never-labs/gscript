//go:build darwin && arm64

package methodjit

import (
	"strings"
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

func TestMethodJITGenericForFeatureMatrixTier1Correctness(t *testing.T) {
	tests := []struct {
		name string
		src  string
		fn   string
		want runtime.Value
	}{
		{
			name: "pairs table",
			fn:   "sum_pairs",
			want: runtime.IntValue(603),
			src: `
func sum_pairs() {
  t := {a: 100, b: 200, c: 300}
  n := 0
  total := 0
  for k, v := range pairs(t) {
    n = n + 1
    total = total + v
  }
  return total + n
}
`,
		},
		{
			name: "ipairs array",
			fn:   "sum_ipairs",
			want: runtime.IntValue(68),
			src: `
func sum_ipairs() {
  t := {7, 11, 13}
  total := 0
  for i, v := range ipairs(t) {
    total = total + i * v
  }
  return total
}
`,
		},
		{
			name: "explicit next state",
			fn:   "sum_next",
			want: runtime.IntValue(63),
			src: `
func sum_next() {
  t := {x: 10, y: 20, z: 30}
  k := nil
  v := nil
  n := 0
  total := 0
  for {
    k, v = next(t, k)
    if k == nil {
      break
    }
    n = n + 1
    total = total + v
  }
  return total + n
}
`,
		},
		{
			name: "__pairs proxy",
			fn:   "sum_pairs_meta",
			want: runtime.IntValue(114),
			src: `
func proxy_next(state, key) {
  if key == nil {
    return "first", state.first
  }
  if key == "first" {
    return "second", state.second
  }
  return nil
}

func sum_pairs_meta() {
  target := {}
  setmetatable(target, {
    __pairs: func(t) {
      return proxy_next, {first: 40, second: 70}, nil
    },
  })
  n := 0
  total := 0
  for k, v := range pairs(target) {
    n = n + 1
    total = total + v
  }
  return total + n * 2
}
`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wantValues := runGenericForFeatureCase(t, tc.src, tc.fn, false)
			if len(wantValues) != 1 {
				t.Fatalf("VM %s returned %d values: %v", tc.fn, len(wantValues), wantValues)
			}
			assertValuesEqual(t, tc.name+" VM expected value", wantValues[0], tc.want)

			gotValues := runGenericForFeatureCase(t, tc.src, tc.fn, true)
			if len(gotValues) != len(wantValues) {
				t.Fatalf("JIT %s returned %d values, VM returned %d: got=%v want=%v",
					tc.fn, len(gotValues), len(wantValues), gotValues, wantValues)
			}
			for i := range gotValues {
				assertValuesEqual(t, tc.name, gotValues[i], wantValues[i])
			}
		})
	}
}

func TestMethodJITGenericForTier2TForShapeFallsBackSafely(t *testing.T) {
	const src = `
func generic_for_pairs_next(n) {
  t := {a: 1, b: 2, c: 3}
  total := 0
  for i := 1; i <= n; i++ {
    for k, v := range pairs(t) {
      total = total + v
    }
  }
  return total
}
`
	top := compileTop(t, src)
	proto := findProtoByName(top, "generic_for_pairs_next")
	if proto == nil {
		t.Fatal("generic_for_pairs_next proto not found")
	}

	tm := NewTieringManager()
	err := tm.CompileTier2(proto)
	if err == nil {
		t.Fatalf("CompileTier2(generic_for_pairs_next) unexpectedly accepted TFor shape")
	}
	errText := strings.ToLower(err.Error())
	if !strings.Contains(errText, "tfor") && !strings.Contains(errText, "unsupported") {
		t.Fatalf("CompileTier2(generic_for_pairs_next) error = %q, want explicit TFor/unsupported gate", err)
	}
	if proto.EnteredTier2 != 0 {
		t.Fatalf("generic_for_pairs_next EnteredTier2=%d after rejected compile, want 0", proto.EnteredTier2)
	}

	globals := vmtest.NewInterpreterGlobals()
	v := vm.New(globals)
	defer v.Close()
	v.SetMethodJIT(tm)
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("JIT execute top: %v", err)
	}
	fn := v.GetGlobal("generic_for_pairs_next")
	got, err := v.CallValue(fn, []runtime.Value{runtime.IntValue(5)})
	if err != nil {
		t.Fatalf("CallValue(generic_for_pairs_next): %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("generic_for_pairs_next returned %d values: %v", len(got), got)
	}
	assertValuesEqual(t, "tier2 rejected TFor fallback correctness", got[0], runtime.IntValue(30))
	if proto.EnteredTier2 != 0 {
		t.Fatalf("generic_for_pairs_next EnteredTier2=%d after fallback execution, want 0", proto.EnteredTier2)
	}
}

func runGenericForFeatureCase(t *testing.T, src, fnName string, jit bool) []runtime.Value {
	t.Helper()

	top := compileTop(t, src)
	globals := vmtest.NewInterpreterGlobals()
	v := vm.New(globals)
	defer v.Close()
	if jit {
		v.SetMethodJIT(NewTieringManager())
	}
	if _, err := v.Execute(top); err != nil {
		t.Fatalf("execute top: %v", err)
	}
	fn := v.GetGlobal(fnName)
	if fn.IsNil() {
		t.Fatalf("function %q not found in globals", fnName)
	}
	var got []runtime.Value
	var err error
	for i := 0; i < 20; i++ {
		got, err = v.CallValue(fn, nil)
		if err != nil {
			t.Fatalf("CallValue(%s) #%d: %v", fnName, i+1, err)
		}
	}
	return got
}
