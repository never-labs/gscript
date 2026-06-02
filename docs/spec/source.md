# Source Code Representation

Leia source files are UTF-8 text. The stable source contract excludes invalid
UTF-8 byte sequences and the NUL character. Implementations may reject either
before lexing and tools may report them as source encoding errors rather than
language syntax errors.

Source text is not normalized. Distinct Unicode code point sequences remain
distinct in comments and string literal contents, and implementations must not
apply Unicode normalization before tokenization or before comparing string
values.

Stable identifiers are ASCII. Non-ASCII code points may appear in comments and
strings. Future revisions may extend identifier characters, but portable source
should use ASCII identifiers.

```leia run all
// Non-ASCII text is stable in comments and strings.
text := "cafe"
assert(text == "cafe")
```

```leia fail all
cafeé := 1
```

Whitespace separates tokens but does not by itself terminate statements.
Semicolons are optional separators in the public grammar.

Comments have two forms:

```ebnf
line_comment  = "//" { any_char_except_newline } ;
block_comment = "/*" { any_char } "*/" ;
```

Line comments that begin with the exact prefix `//leia:` and are attached to the
first parsed token in a file are file directives. A file directive must appear
before imports, declarations, statements, and any non-directive leading comment
that separates it from the first token. `//@leia:`, `/* //leia:... */`, and
`// leia:` are ordinary comments, not file directives.

File directives are metadata for tooling, sandbox policy, build selection, and
tests. They do not execute, import modules, skip tests, grant host capabilities,
enable providers, or change expression and statement semantics by themselves.
Tools and host applications decide how to interpret the metadata they recognize.

The stable file directive kinds are the ones specified in
[File Directives](../reference/directives/index.md): `build`, `test`, `cap`,
and `feature`. Implementations may report malformed arguments, unsupported
argument values, duplicates, or unrecognized `//leia:` names as diagnostics when
a tool or host policy validates directives. Unknown file directive names are not
reserved extension points for user programs: portable source must not depend on
them being accepted, ignored, preserved, rejected, or reported in inspection
output. Regardless of those diagnostics, an unknown file directive must not gain
execution behavior or silently grant capabilities.

```leia run all
//leia:build linux,darwin
//leia:test smoke
//leia:cap fs.read
//leia:feature source-directives
//@leia:build ignored
ran := true
assert(ran)
```

`//leia:` comments outside the file-directive position remain comments for this
chapter. Other chapters may define declaration-local directive comments, such as
AI tool metadata, but those comments are not file directives.
