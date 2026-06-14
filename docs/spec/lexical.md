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

Dialect words such as `q`, `model`, `tool`, `turn`, `agent`, and `evaluate`
are contextual syntax words, not lexical keywords. They scan as identifiers
outside grammar positions that assign them dialect meaning.

```leia run all
agent := "identifier"
tool := "identifier"
turn := 1
model := 2

assert(agent == "identifier")
assert(tool == "identifier")
assert(turn + model == 3)
```

In grammar positions that define tagged dialect forms or evaluate blocks,
contextual words are consumed by that syntax rather than bound as ordinary
identifiers. See [Expressions](expressions.md) for the generic tagged-dialect
boundary and [AI Dialect Syntax](ai-dialect.md) for one standard dialect family.

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

String literals have three untagged source forms. Double-quoted strings process
escapes and support `${expr}` interpolation. Single-quoted strings process
escapes but do not interpolate; `${` has no special meaning inside them.
Backtick raw strings use Go raw-string behavior: they preserve their contents
and do not process escapes or interpolation.

The same raw delimiter syntax is also used by tagged dialect expressions. For
example, a q dialect expression places the raw body immediately after `q`, and a
shell dialect expression places the raw body immediately after `$`. Tagged raw
bodies may use one backtick delimiter or a fenced three-backtick delimiter. Both
tagged forms may contain `${expr}` interpolation tokens that are evaluated by
Leia and encoded according to the dialect contract. The fenced form is for
embedded DSL source that contains single backticks or spans multiple lines.

```ebnf
double_quoted_string = '"' { any_char_except_quote_newline_dollar_backslash
                           | escape
                           | interpolation } '"' ;
single_quoted_string = "'" { any_char_except_single_quote_newline_backslash
                           | escape } "'" ;
raw_string           = short_raw_body | fenced_raw_body ;
short_raw_body       = '`' { any_byte_except_backtick } '`' ;
fenced_raw_body      = '```' { any_byte_except_three_backticks } '```' ;
tagged_raw_body      = tagged_short_raw_body | tagged_fenced_raw_body ;
tagged_short_raw_body = '`' { any_byte_except_backtick_or_interpolation
                            | interpolation } '`' ;
tagged_fenced_raw_body = '```' { any_byte_except_three_backticks_or_interpolation
                               | interpolation } '```' ;
interpolation = "${" expr "}" ;
escape        = backslash ( backslash | '"' | "'" | "a" | "b" | "f" | "n"
                          | "r" | "t" | "v"
                          | "x" hex hex
                          | "u" hex hex hex hex
                          | "U" hex hex hex hex hex hex hex hex
                          | decimal_escape ) ;
decimal_escape = digit [ digit [ digit ] ] ;
hex           = "0".."9" | "A".."F" | "a".."f" ;
backslash     = "\\" ;
```

Stable quoted-string escapes are `\\`, `\"`, `\'`, `\a`, `\b`, `\f`, `\n`,
`\r`, `\t`, `\v`, `\xNN`, `\uNNNN`, `\UNNNNNNNN`, and decimal byte escapes
from `\0` through `\255`. Hex and Unicode escapes require exactly the specified
number of hex digits. Unicode escapes must encode a valid Unicode scalar value.

Strings are byte strings; UTF-8 interpretation is provided by library helpers.
`"\uNNNN"` and `"\UNNNNNNNN"` append the UTF-8 bytes for the escaped rune.

Untagged raw strings preserve every byte between the delimiters. Tagged raw
bodies preserve uninterpolated bytes exactly and replace `${expr}` segments
with dialect-encoded values. A short raw body cannot contain a backtick. A
fenced raw body can contain single or double backticks, but not three
consecutive backticks. Quoted strings may not contain an unescaped newline.

```leia run all
quoted := "line\n"
literal := 'line\n'
raw := `line\n`
name := "Leia"
interpolated := "hello ${name}"
literal_interp := 'hello ${name}'
unicode := "\u0041\U00000042"
byte_escape := "\065"

assert(#quoted == 5)
assert(#literal == 5)
assert(#raw == 6)
assert(interpolated == "hello Leia")
assert(literal_interp == 'hello ${name}')
assert(unicode == "AB")
assert(byte_escape == "A")
```

The first two strings contain a newline byte because quoted strings process
escapes. The raw string contains the two bytes backslash and `n`. The
double-quoted string interpolates `name`; the single-quoted spelling contains
the literal bytes `${name}`.

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
names and their semantic effects are specified by the directive and dialect
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
