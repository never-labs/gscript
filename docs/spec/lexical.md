# Lexical Elements

Tokens are identifiers, keywords, literals, operators, and punctuation.
Implementations must choose the longest valid token when scanning source.

## Identifiers

```ebnf
identifier = ( "A".."Z" | "a".."z" | "_" )
             { "A".."Z" | "a".."z" | "0".."9" | "_" } ;
digit      = "0".."9" ;
```

Identifiers name variables, functions, labels, tools, agents, module aliases,
and fields. Identifiers are case-sensitive.

```leia run all
name := "leia"
Name := "different binding"
_scratch := 42

assert(name == "leia")
assert(Name == "different binding")
assert(_scratch == 42)
```

The identifiers `name` and `Name` are distinct. A digit may not be the first
character of an identifier.

## Keywords

The following keywords are reserved and may not be used as identifiers.

```text
func return if else elseif for range break continue in var go chan defer const goto
true false nil
```

AI-native words such as `agent`, `models`, `tool`, `turn`, `messages`, `flow`,
and `budget` are contextual syntax words. They remain ordinary identifiers
outside the grammar positions that use them.

```leia run all
agent_name := "summarizer"
turn_count := 1

assert(agent_name == "summarizer")
assert(turn_count == 1)
```

In a declaration position, contextual words introduce AI-native syntax:

```leia
agent summarize(text) {
    user: text
}
```

## Literals

Numeric literals support decimal integers and floats plus `0x`, `0b`, and `0o`
integer prefixes. Underscores may separate digits where accepted by the lexer.

```leia run all
decimal := 1000000
hex := 0xff
binary := 0b1010
octal := 0o755
float := 1.25e2

assert(decimal == 1000000)
assert(hex == 255)
assert(binary == 10)
assert(octal == 493)
assert(float == 125)
```

String literals are quoted strings with escapes or raw backtick strings.
Strings are byte strings; UTF-8 interpretation is provided by library helpers.

```leia run all
quoted := "line\n"
raw := `line\n`

assert(#quoted == 5)
assert(#raw == 6)
```

The first string contains a newline byte. The second contains the two bytes
backslash and `n`.

Boolean and nil literals are `true`, `false`, and `nil`.

## Operators And Punctuation

The stable operator and punctuation tokens are:

```text
++ -- + - * / % ** .. # !
 += -= *= /= %= =
 := == != < <= > >=
 && || & | ^ &^ << >>
 <- ... . , ; : ( ) [ ] { }
```

The parser may reject a token sequence even when each token is lexically valid.
For example, `a + * b` is lexically valid as tokens but invalid as an
expression.
