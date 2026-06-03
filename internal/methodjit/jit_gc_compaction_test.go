//go:build darwin && arm64

package methodjit

import "testing"

func TestJITCollectGarbageDefersRootCompaction(t *testing.T) {
	src := `
func touch(tbl) {
    x := tbl.value
    collectgarbage("collect")
    return x + tbl.value
}

result := 0
for i := 1; i <= 2500; i++ {
    t := {value: i}
    result = touch(t)
}
`
	globals := runVMFullWithJIT(t, src)
	result := globals["result"]
	if !result.IsInt() || result.Int() != 5000 {
		t.Fatalf("result = %v (%s), want int 5000", result, result.TypeName())
	}
}
