# Shell and Host Automation

## Goal

Leia should replace many shell scripts without becoming a shell. It should keep
normal structured programming while making commands, file selection, and host
automation first-class and concise.

## Core Forms

```leia
out := $`git status --short`
out := sh`git status --short`
out := cmd`git status --short`
files := glob`**/*.leia`
p := path`games/stonebridge/main.leia`
```

`$` is the short form for shell-compatible execution. `sh` is explicit.
`cmd` is argv-safe and should avoid shell interpretation.

## Command Result

Commands return a structured result:

```leia
{
    ok: true,
    code: 0,
    text: "...",
    stdout: "...",
    stderr: "...",
    lines: ["..."],
}
```

The primary human-facing output field should be `text`. `stdout` is retained
for explicit process semantics.

## Fail-Fast

```leia
sh!`go test ./...`
$!`git push`
cmd!`git commit -m ${message}`
```

Without `!`, command failure returns `ok: false`. With `!`, command failure
raises a Leia error.

## Shell-Compatible vs Argv-Safe

`sh` supports shell syntax:

```leia
sh`rg "agent" **/*.leia | head -20`
sh`printf ok > out.txt`
```

`cmd` is for safe command invocation:

```leia
branch := "main"
cmd`git checkout ${branch}`
```

Interpolated values in `cmd` must be treated as argument values, not shell text.

## Glob

```leia
files := glob`**/*.leia`
```

`glob` is the primary user-facing file matching style. A library function may
exist, but the main Leia style should not be `fs.glob("...")`.

Glob results should be deterministic:

- return an array of paths;
- sort paths by default;
- support include/exclude patterns;
- respect filesystem capability restrictions.

Multi-line glob form:

```leia
files := glob`
    **/*.leia
    !vendor/**
    !persistence/**
`
```

## Path Values

`path` should create a path value or normalized path string:

```leia
root := path`games/stonebridge`
main := root / "main.leia"
```

Path composition may use metamethods, but must remain predictable and portable.

## Scoped Environment and Directory

Desired high-level forms:

```leia
cd path`games/stonebridge` {
    sh!`leia run main.leia`
}

env {
    STONEBRIDGE_FAST: "1"
} {
    sh!`leia run main.leia`
}
```

These are scoped changes. They must not leak into the caller after the block.

## Streaming

Long-running commands need streaming output:

```leia
proc := cmd.start("leia", ["run", "main.leia"])
for line := range proc.stdout {
    print("[game]", line)
}
code := proc.wait()
```

The high-level command dialect may later expose streaming directly, but the
first requirement is a clear result model for ordinary commands.

## Composition

Shell results should compose with pipeline and data APIs:

```leia
errors := sh`rg "ERROR" logs/*.txt`
    |> .lines
    |> filter(fn(line) { return string.contains(line, "timeout") })
```

## Non-Goals

- No implicit bare command syntax such as `git status`.
- No shell-like global mutable environment by default.
- No silent command failure.
