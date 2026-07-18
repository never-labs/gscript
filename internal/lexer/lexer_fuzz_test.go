package lexer

import "testing"

func FuzzTokenizeDoesNotPanic(f *testing.F) {
	seeds := []string{
		"",
		"value := 42\nvalue += 1",
		"func add(a, b) { return a + b }",
		"if ready { run() } else { retry() }",
		`items := {1, "two", key: true, nested: {nil}}`,
		"// line comment\n/* block comment */\nreturn",
		`text := "escaped\n\t\"text"`,
		"raw := `literal ${text}`",
		"name := \"Leia 世界\"",
		"\x00\xff\xfe",
		`"unterminated`,
		"/* unterminated",
	}
	for _, seed := range seeds {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, source string) {
		_, _ = New(source).Tokenize()
	})
}
