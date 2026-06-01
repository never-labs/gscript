# Leia File Directives

File directives are Go-style line comments at the top of a `.leia` file. They
let tools classify a file without executing it.

Only comments attached to the first parsed token are file directives. Keep them
before imports, declarations, statements, and blank-separated prose comments.

```leia
//leia:build darwin,arm64
//leia:test integration slow
//leia:cap fs.read,net.client
//leia:feature llm
print("ready")
```

The directive prefix is exactly `//leia:`. Java-style or annotation-style forms
such as `//@leia:build` are ignored.

## Directive Kinds

| Directive | Meaning | Argument form |
|---|---|---|
| `//leia:build` | Platform, environment, or feature build selection. | comma or whitespace separated tags |
| `//leia:test` | Test classification for the CLI and release gates. | comma or whitespace separated labels |
| `//leia:cap` | Host capabilities expected by the file. | comma or whitespace separated capability names |
| `//leia:feature` | Language or product feature used by the file. | free text plus parsed words |

Capabilities use the same naming convention as `leia.mod`: letters, digits,
`.`, `-`, `_`, `/`, and `:` are valid capability-name characters.

## Inspection

Use the CLI to inspect the directives the parser sees:

```bash
leia inspect directives path/to/file.leia
leia inspect directives --json path/to/file.leia
```

The JSON output is stable enough for tooling and contains `kind`, `args`,
`text`, `line`, and `column`.

## Scope

File directives are metadata. They do not automatically grant capabilities,
load providers, or skip tests by themselves. Host applications and CLI commands
decide how to interpret them.
