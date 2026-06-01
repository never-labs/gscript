# Source Code Representation

Leia source files are UTF-8 text. Implementations may reject invalid UTF-8 or
the NUL character. Source text is not normalized: distinct Unicode code point
sequences remain distinct strings and comments.

Stable identifiers are ASCII. Non-ASCII code points may appear in comments and
strings. Future revisions may extend identifier characters, but portable source
should use ASCII identifiers.

Whitespace separates tokens but does not by itself terminate statements.
Semicolons are optional separators in the public grammar.

Comments have two forms:

```ebnf
line_comment  = "//" { any_char_except_newline } ;
block_comment = "/*" { any_char } "*/" ;
```

Line comments that begin with `//leia:` before the first parsed token in a file
are file directives. Directives are metadata for tooling, sandbox policy, build
selection, tests, and AI tool declarations. Directives do not execute and do not
grant capabilities by themselves.

Directive syntax and recognized keys are specified in
[File Directives](../reference/directives/index.md).
