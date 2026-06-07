# Dialect System

## Goal

Leia should be a DSL-native scripting language. Shell commands, glob patterns,
regular expressions, JSON, SQL, agents, web flows, tests, and future domain
languages should all use one consistent extension mechanism rather than custom
keywords or unrelated library conventions.

The core syntax is:

```leia
tag`text`
tag!`text`
tag { ... }
```

`tag` identifies a registered dialect. The dialect returns an ordinary Leia
value. The language should not add a new keyword for each domain.

## User-Facing Forms

### Tagged literal

```leia
files := glob`**/*.leia`
pattern := re`\bagent\b`
request := http`GET https://example.com/users/${id}`
```

Tagged literals are for compact text-like domains: shell commands, file globs,
regex, URLs, JSON fragments, SQL, HTTP requests, and other structured strings.

### Fail-fast tagged literal

```leia
sh!`go test ./...`
cmd!`git commit -m ${message}`
```

The `!` form means failure should become a Leia error instead of a normal result
object. The error model should be consistent across dialects.

### Tagged block

```leia
agent {
    model: "glm"
    system: `You are concise.`
    user: `Question: ${question}`
}
```

Tagged blocks are for configuration, workflows, generated documents, tests,
agents, and other multi-field or multi-step domains.

## Interpolation

`${expr}` is the only interpolation form.

```leia
name := "Ada"
text := `hello ${name}`
cmd := sh`git log ${branch} --oneline -5`
```

Backtick strings should support interpolation by default. Double-quoted strings
remain literal unless a future spec explicitly changes them.

Dialect evaluators may treat interpolated values differently:

- shell-like dialects may preserve argument boundaries;
- SQL dialects may preserve parameter values;
- prompt/agent dialects may preserve source metadata;
- text dialects may stringify values.

The common rule is that interpolation evaluates Leia expressions in lexical
scope and produces ordinary values for the dialect to consume.

## Dialect Levels

### Level 1: field block

```leia
turn {
    model: "glm"
    user: `Explain ${topic}`
}
```

Field blocks are close to tagged table literals. They are useful for agent,
turn, model, tool, evaluate, HTTP, mail, and database configurations.

### Level 2: tagged literal

```leia
sh`git status --short`
glob`**/*.leia`
json`{"name": ${name}}`
```

These are compact, high-frequency DSLs.

### Level 3: raw block

```leia
html {
    div class: "card" {
        h1 title
        p body
    }
}
```

Raw blocks are true REBOL-style dialect blocks. The block is domain syntax, not
ordinary Leia statements and not just a table. This is the long-term mechanism
for HTML, parse rules, workflow scripts, code queries, and patch plans.

### Level 4: user-defined raw dialect

Users and packages may define their own dialects, subject to capability and
module rules. This is later-stage functionality and should not compromise the
stability of built-in dialects.

## Requirements

- Dialects must be explicit by tag name.
- Dialects return ordinary Leia values.
- Dialects must not implicitly mutate the caller's lexical scope.
- Dialects must have stable error behavior.
- `tag!` is the uniform fail-fast modifier.
- Dialects that perform side effects must declare capabilities.
- Dialects should compose with pipeline, `try`, destructuring, and tests.

## Non-Goals

- Do not make every identifier a possible command.
- Do not make Leia a shell language.
- Do not introduce domain-specific parser keywords for each built-in domain.
- Do not let user dialects silently override stable built-in dialects.
