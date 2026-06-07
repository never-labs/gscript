# Composition Features

## Goal

Leia should make generated and hand-written code shorter without making the
language harder to reason about. The first composition features should improve
data processing, shell automation, AI flows, and error handling.

## Pipeline

```leia
matches := glob`**/*.leia`
    |> filter(fn(path) { return !string.contains(path, "vendor") })
    |> map(fn(path) { return {path: path, text: fs.readfile(path)} })
    |> filter(fn(row) { return string.contains(row.text, "agent") })
```

Desired semantics:

```leia
x |> f
x |> f(a, b)
```

means:

```leia
f(x)
f(x, a, b)
```

Pipelines should work with ordinary functions, dialect results, arrays,
iterators, command results, and data tables.

## Try

```leia
text := try fs.readfile(path)
doc := try json.decode(text)
```

`try` reduces Go-style error boilerplate.

Requirements:

- works with functions returning `(value, err)`;
- if `err` is non-nil, return or raise according to context;
- should not hide errors silently;
- should produce clear stack/source diagnostics.

## Destructuring

```leia
{text, code, ok} := sh`git status --short`
{name, age} := user
[first, second] := rows
```

Requirements:

- table field destructuring;
- array/list destructuring;
- clear behavior for missing fields;
- compatibility with multi-return assignment;
- useful with shell, HTTP, AI, and DB results.

## Optional Chaining and Nil Coalescing

```leia
city := user.profile?.address?.city ?? "unknown"
label := result.value?.label ?? "unclassified"
```

Requirements:

- works with table field/index access;
- short-circuits on nil;
- does not suppress non-nil runtime errors;
- clearly specified truth/nil behavior.

## Comprehensions

```leia
names := [u.name for u in users if u.active]
by_id := {u.id: u for u in users}
```

Requirements:

- array comprehension;
- table/map comprehension;
- optional filter clause;
- lexical scoping rules;
- deterministic iteration when source is ordered.

## Pattern Match

```leia
match event {
case {type: "conversation", actor: a, partner: p}:
    print(a, p)
case {type: "death", actor: name}:
    print("died", name)
default:
    print("other")
}
```

Requirements:

- table patterns;
- literal patterns;
- binding variables;
- optional guards;
- clear shadowing rules.

## Using

```leia
using cd(path`games/stonebridge`) {
    sh!`leia run main.leia`
}

using tempdir() as dir {
    sh!`git clone ${repo} ${dir}`
}
```

Requirements:

- scoped resources;
- cleanup even on error;
- useful for cwd/env/temp files/files;
- should compose with `try`.

## Priority

1. Pipeline
2. Try
3. Destructuring
4. Optional chaining / `??`
5. Comprehensions
6. Pattern match
7. Using
