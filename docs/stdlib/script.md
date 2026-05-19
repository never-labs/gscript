# script

The `script` library compiles, evaluates, and loads GScript chunks with optional
environment and source configuration.

## Environment Options

Most functions accept an optional configuration value:

- string -- used as `sourceName`
- table with `sourceName` or `source` -- diagnostic source name
- table with `scriptDir` -- directory for relative script/module loading while the chunk runs
- table with `env` -- lexical global environment table
- table with `sandbox = true` -- use `env` without falling back to existing globals

If a table has none of the config keys above, it is treated as the environment
table directly.

`script.env(seed)` and `script.sandbox(seed)` build option tables for common
environment modes. Variables written by the script are copied back to the env
table after execution.

## Functions

### script.env([seed]) -> opts

Return an options table that runs with `seed` over the current globals.

### script.sandbox([seed]) -> opts

Return an options table that runs with only `seed` as its globals.

### script.compile(source [, opts]) -> function

Compile source text and return a callable chunk. Calling the chunk executes the
compiled program and returns top-level return values.

```
fn := script.compile("return a + b", {
    env: {a: 1, b: 2},
    sourceName: "virtual/generated.gs",
})
print(fn())  -- 3
```

### script.eval(source [, opts]) -> values...

Compile and immediately execute source text.

```
name := script.eval("return name", {env: {name: "gscript"}})
```

### script.loadFile(path [, opts]) -> function

Load and compile a file without running it. Relative paths are resolved against
the current script directory when possible. If `scriptDir` is not provided, the
loaded file's directory becomes the chunk's script directory.

### script.runFile(path [, opts]) -> values...

Load and execute a file.

### script.dir() -> string

Return the current script directory used for relative `require`, `loadFile`, and
`runFile` resolution.

### script.setDir(dir) -> oldDir

Set the current script directory and return the previous value.

```
old := script.setDir("plugins")
plugin := script.loadFile("tool.gs", {sourceName: "plugin/tool.gs"})
script.setDir(old)
```
