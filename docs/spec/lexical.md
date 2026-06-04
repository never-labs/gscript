# Lexical Elements

Tokens are identifiers, keywords, literals, operators, and punctuation. Whitespace
and comments separate tokens but are not tokens themselves.

Implementations must scan the longest valid token from the current byte position.
For example, `...` is one ellipsis token, not `..` plus `.`, and `<=` is one
comparison token, not `<` plus `=`.

```leia run all
concat := "a" .. "b"
ellipsis_seen := false

func collect(...) {
    ellipsis_seen = true
}

collect(1, 2, 3)

assert(concat == "ab")
assert(ellipsis_seen)
```

The parser may reject a token sequence even when every token is lexically valid.

## Identifiers

Identifiers are ASCII-only. They begin with an ASCII letter or underscore and
continue with ASCII letters, ASCII digits, or underscores.

```ebnf
identifier = ident_start { ident_continue } ;
ident_start = "A".."Z" | "a".."z" | "_" ;
ident_continue = ident_start | "0".."9" ;
```

Identifiers name variables, functions, labels, tools, agents, module aliases,
and fields. Identifiers are case-sensitive. Non-ASCII letters are not identifier
characters in v1.0 source.

```leia run all
name := "leia"
Name := "different binding"
_scratch := 42
agent_name := "summarizer"

assert(name == "leia")
assert(Name == "different binding")
assert(_scratch == 42)
assert(agent_name == "summarizer")
```

The identifiers `name` and `Name` are distinct. A digit may not be the first
character of an identifier.

```leia fail all
1name := "not an identifier"
```

## Keywords

Reserved keywords are recognized only when the complete identifier spelling
matches a keyword. Longer names that contain a keyword remain identifiers, so
`func_name`, `returning`, and `iffoo` are ordinary identifiers.

The following keywords are reserved and may not be used as identifiers:

```text
func return if else elseif for range break continue in var go chan defer const goto
true false nil
```

```leia run all
func_name := "ordinary identifier"
returning := true
iffoo := 7

assert(func_name == "ordinary identifier")
assert(returning == true)
assert(iffoo == 7)
```

```leia fail all
func := "reserved"
```

AI-native words such as `agent`, `models`, `tool`, `turn`, `messages`,
`evaluate`, `case`, `expect`, `flow`, and `budget` are contextual syntax words,
not lexical keywords. They scan as identifiers outside grammar positions that
assign them AI-native meaning.

```leia run all
agent := "identifier"
tool := "identifier"
turn := 1
budget := 2

assert(agent == "identifier")
assert(tool == "identifier")
assert(turn + budget == 3)
```

In grammar positions that define AI-native forms, contextual words are consumed
by that syntax rather than bound as ordinary identifiers. See
[AI-Native Constructs](ai-native.md) for the stable forms.

## Numeric Literals

Numeric literals are unsigned tokens. Unary `+` and `-` are separate operators.
The stable v1.0 numeric forms are decimal integers, decimal floats, and
base-prefixed integers.

```ebnf
number_lit  = decimal_int [ fraction ] [ exponent ]
            | decimal_int exponent
            | based_int ;
decimal_int = digit { [ "_" ] digit } ;
fraction    = "." digit { [ "_" ] digit } ;
exponent    = ( "e" | "E" ) [ "+" | "-" ] digit { [ "_" ] digit } ;
based_int   = "0" ( "x" | "X" ) hex_digits
            | "0" ( "b" | "B" ) bin_digits
            | "0" ( "o" | "O" ) oct_digits ;
hex_digits  = hex_digit { [ "_" ] hex_digit } ;
bin_digits  = bin_digit { [ "_" ] bin_digit } ;
oct_digits  = oct_digit { [ "_" ] oct_digit } ;
hex_digit   = digit | "A".."F" | "a".."f" ;
bin_digit   = "0" | "1" ;
oct_digit   = "0".."7" ;
digit       = "0".."9" ;
```

For valid programs, an underscore may appear only between digits within the
same digit sequence. It may not start or end a number, appear next to `.`,
appear immediately after an exponent marker or exponent sign, or appear twice
in a row. Current implementations may scan some wider base-prefixed spellings
before numeric conversion rejects or accepts them; portable v1.0 source should
use digits valid for the base and the underscore placement above.

Decimal float forms include `1.25`, `1e3`, `1.25e2`, and `1_2.3_4e5_6`.
A dot belongs to a number only when it is not the start of `..`; therefore
`1..2` scans as `1`, `..`, `2`.

```leia run all
decimal := 1_000_000
hex := 0xff
binary := 0b1010
octal := 0o755
float := 1.25e2
spread := "1" .. "2"

assert(decimal == 1000000)
assert(hex == 255)
assert(binary == 10)
assert(octal == 493)
assert(float == 125)
assert(spread == "12")
```

```leia fail all
bad := 1__2
```

```leia fail all
bad := 1e_2
```

## String Literals

Quoted string literals use double quotes and process escapes. Raw string
literals use backticks and do not process escapes.

```ebnf
quoted_string = '"' { quoted_char | escape } '"' ;
raw_string    = '`' { any_byte_except_backtick } '`' ;
escape        = "\\" | "\"" | "\a" | "\b" | "\f" | "\n" | "\r" | "\t" | "\v"
              | "\x" hex hex
              | "\u" hex hex hex hex
              | "\U" hex hex hex hex hex hex hex hex
              | "\" decimal_escape ;
decimal_escape = digit [ digit [ digit ] ] ;
hex           = "0".."9" | "A".."F" | "a".."f" ;
```

Stable quoted-string escapes are `\\`, `\"`, `\a`, `\b`, `\f`, `\n`, `\r`,
`\t`, `\v`, `\xNN`, `\uNNNN`, `\UNNNNNNNN`, and decimal byte escapes from
`\0` through `\255`. Hex and Unicode escapes require exactly the specified
number of hex digits. Unicode escapes must encode a valid Unicode scalar value.

Strings are byte strings; UTF-8 interpretation is provided by library helpers.
`"\uNNNN"` and `"\UNNNNNNNN"` append the UTF-8 bytes for the escaped rune.

Raw strings may span lines, preserve every byte between the delimiters, and may
not contain a backtick byte. Quoted strings may not contain an unescaped newline.

```leia run all
quoted := "line\n"
raw := `line\n`
unicode := "\u0041\U00000042"
byte_escape := "\065"

assert(#quoted == 5)
assert(#raw == 6)
assert(unicode == "AB")
assert(byte_escape == "A")
```

The first string contains a newline byte. The second contains the two bytes
backslash and `n`.

```leia fail all
bad := "line
break"
```

Boolean and nil literals are `true`, `false`, and `nil`.

## Comments And Directives

Line comments begin with `//` and continue to the end of the line. Block
comments begin with `/*` and end at the next `*/`; block comments do not nest.
Comments are otherwise ignored for parsing.

```leia run all
// A full-line comment.
x := 1 /* inline block comment */ + 2

assert(x == 3)
```

File directives are line comments whose text begins with `leia:` after `//`.
Directives attach to the following token when they are in the leading comment
group for that token; blank-line separation starts a new group. Stable directive
names and their semantic effects are specified by the directive and AI-native
chapters. Lexically, a directive is still a line comment.

```text
// leia:requires docs.read
// leia:cap net.client
func summarize(text) {
    return llm.turn({
        messages: {llm.user(text)}
    })
}
```

```leia fail all
/* block comments do not nest
   /* inner */
*/
x := 1
```

## Operators And Punctuation

The stable operator and punctuation tokens are:

```text
++ -- + - * / % ** .. # !
 += -= *= /=
 = := == != < <= > >=
 && || & | ^ &^ << >>
 <- ... . , ; : ( ) [ ] { }
```

Longest-token scanning applies to this table. For example, `+=` is one token,
`++` is one token, `**` is one token, `...` is one token, and `<-` is one token.

```leia run all
x := 1
x += 2
y := 2 ** 3
bits := (1 << 4) | 3

assert(x == 3)
assert(y == 8)
assert(bits == 19)
```

The parser may reject a token sequence even when each token is lexically valid.
For example, `a + * b` is lexically valid as tokens but invalid as an
expression.

```leia fail all
a := 1 + * 2
```
