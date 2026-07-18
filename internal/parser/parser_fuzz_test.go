package parser

import (
	"testing"

	"github.com/never-labs/leia/internal/lexer"
)

func FuzzParseSourceDoesNotPanic(f *testing.F) {
	seeds := []string{
		"",
		"answer := 6 * 7",
		"func add(a, b) { return a + b }\nresult := add(1, 2)",
		"for i := 0; i < 3; i++ { if i == 1 { continue } }",
		`config := {name: "Leia", enabled: true, values: {1, 2, 3}}`,
		"import \"math\" as math\nvalue := math.abs(-1)",
		"select { case value := <-input: return value; default: return nil }",
		"message := prompt`hello ${name}`",
		"name := \"Leia 世界\"",
		"\x00\xff\xfe",
		"func broken(",
		"value := {key:",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		tokens, err := lexer.New(source).Tokenize()
		if err != nil {
			return
		}
		_, _ = New(tokens).Parse()
	})
}
