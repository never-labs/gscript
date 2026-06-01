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

## Keywords

The following keywords are reserved and may not be used as identifiers.

```text
func return if else elseif for range break continue in var go chan defer const goto
true false nil
```

AI-native words such as `agent`, `models`, `tool`, `turn`, `messages`, `flow`,
and `budget` are contextual syntax words. They remain ordinary identifiers
outside the grammar positions that use them.

## Literals

Numeric literals support decimal integers and floats plus `0x`, `0b`, and `0o`
integer prefixes. Underscores may separate digits where accepted by the lexer.

String literals are quoted strings with escapes or raw backtick strings.
Strings are byte strings; UTF-8 interpretation is provided by library helpers.

Boolean and nil literals are `true`, `false`, and `nil`.

## Operators And Punctuation

The stable operator and punctuation tokens are:

```text
+ -- + - * / % ** .. # !
 += -= *= /= %= =
 := == != < <= > >=
 && || & | ^ &^ << >>
 <- ... . , ; : ( ) [ ] { }
```

The parser may reject a token sequence even when each token is lexically valid.
