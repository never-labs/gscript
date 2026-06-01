//go:build darwin && arm64

package methodjit

import "testing"

func TestTier1_ArithmeticNumericStringFallbackMatchesVM(t *testing.T) {
	compareVMvsJIT(t, `
result := 0
for i := 1; i <= 50; i++ {
    result = result + ("5" + 3)
    result = result + ("5.5" * 2)
}
`, "result")
}

func TestTier1_ArithmeticInvalidStringFallbackMatchesVM(t *testing.T) {
	compareVMvsJIT(t, `
result := "missing"
for i := 1; i <= 50; i++ {
    ok, err := pcall(func() {
        return "x" + 3
    })
    if ok {
        result = "wrong"
        break
    }
    result = "ok"
}
`, "result")
}
