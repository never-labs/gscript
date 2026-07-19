//go:build darwin && arm64

package methodjit

import (
	"testing"

	"github.com/never-labs/leia/internal/runtime"
	"github.com/never-labs/leia/internal/testutil/vmtest"
	"github.com/never-labs/leia/internal/vm"
)

func executeTier1GenericForSource(t *testing.T, src string) runtime.Value {
	t.Helper()
	top := compileTop(t, src)
	jitVM := vm.New(vmtest.NewInterpreterGlobals())
	t.Cleanup(jitVM.Close)
	jitVM.SetMethodJIT(NewBaselineJITEngine())
	if _, err := jitVM.Execute(top); err != nil {
		t.Fatalf("Tier 1 execute: %v", err)
	}
	return jitVM.GetGlobal("checksum")
}

func TestTier1StdIPairsFastPathSemantics(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want int64
	}{
		{
			name: "dense integer array",
			src: `
func run() {
  t := {10, 20, 30, 40}
  sum := 0
  for i, v := range ipairs(t) { sum = sum + i + v }
  return sum
}
checksum := run()
`,
			want: 110,
		},
		{
			name: "nil hole terminates",
			src: `
func run() {
  t := {10, 20, 30}
  t[2] = nil
  sum := 0
  for i, v := range ipairs(t) { sum = sum + i + v }
  return sum
}
checksum := run()
`,
			want: 11,
		},
		{
			name: "metatable index remains observable",
			src: `
func run() {
  t := {10, 20}
  setmetatable(t, {__index: func(_, k) {
    if k == 3 { return 30 }
    return nil
  }})
  sum := 0
  for i, v := range ipairs(t) { sum = sum + i + v }
  return sum
}
checksum := run()
`,
			want: 66,
		},
		{
			name: "custom iterator falls back",
			src: `
func run() {
  func iter(limit, i) {
    i = i + 1
    if i > limit { return nil }
    return i, i * 3
  }
  func source() { return iter, 4, 0 }
  sum := 0
  for i, v := range source() { sum = sum + i + v }
  return sum
}
checksum := run()
`,
			want: 40,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := executeTier1GenericForSource(t, tt.src)
			if !got.IsInt() || got.Int() != tt.want {
				t.Fatalf("checksum = %v, want %d", got, tt.want)
			}
		})
	}
}

func TestTier1StdIPairsFastPathAvoidsPerIterationNativeCalls(t *testing.T) {
	stats := runtime.EnableRuntimePathStats()
	t.Cleanup(runtime.DisableRuntimePathStats)

	got := executeTier1GenericForSource(t, `
func run(n) {
  t := {}
  for i := 1; i <= n; i++ { t[i] = i }
  sum := 0
  for i, v := range ipairs(t) { sum = sum + i + v }
  return sum
}
checksum := run(20000)
`)
	if !got.IsInt() || got.Int() != 400020000 {
		t.Fatalf("checksum = %v, want 400020000", got)
	}

	var iteratorFast uint64
	for _, row := range stats.Snapshot().NativeCall.PerBuiltin {
		if row.Name == "ipairs_iterator" {
			iteratorFast += row.Fast
		}
	}
	if iteratorFast > 1 {
		t.Fatalf("ipairs_iterator native fast calls = %d, want at most one boundary fallback", iteratorFast)
	}
}
