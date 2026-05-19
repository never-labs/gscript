# process

The `process` library provides functions for running external commands and interacting with the operating system process environment.

## Functions

### process.run(cmd [, opts]) -> table

Run an external command and return a result table with `{ok, stdout, stderr, code}`.

`cmd` can be a string (split by spaces) or a table of arguments.

Options table:
- `stdin` (string) -- string to pass as standard input
- `env` (table) -- additional environment variables `{KEY: "value"}`
- `dir` (string) -- working directory
- `timeout` (number) -- timeout in seconds

```
result := process.run("echo hello")
-- result.ok == true
-- result.stdout == "hello\n"
-- result.stderr == ""
-- result.code == 0

result := process.run({"ls", "-la"})
result := process.run("cat", {stdin: "hello"})
```

### process.exec(cmd, ...) -> string [, error]

Run a command with arguments and return stdout as a string. On failure, returns `nil, "error message"`.

```
out := process.exec("echo", "hello")
-- out == "hello\n"

out, err := process.exec("nonexistent")
-- out == nil, err == "exec: ..."
```

### process.shell(cmd) -> table

Run a command via `/bin/sh -c` and return a result table with `{ok, stdout, stderr, code}`.

```
result := process.shell("echo hello && echo world")
-- result.ok == true
-- result.stdout contains "hello" and "world"
```

### process.which(name) -> string | nil

Find an executable in PATH. Returns the full path or nil if not found.

```
path := process.which("ls")
-- path == "/bin/ls" (or similar)

path := process.which("nonexistent")
-- path == nil
```

### process.pid() -> int

Return the current process ID.

```
pid := process.pid()
```

### process.env() -> table

Return a table of all environment variables as key-value pairs.

```
env := process.env()
path := env.PATH
```

### process.args() -> table

Return the current script argument table.

When the interpreter has script arguments configured, index `0` contains the
entry script name/path and indexes `1..n` contain the user arguments. If no
interpreter arguments are configured, this falls back to the host process
`os.Args` using the same zero-based convention.

```
args := process.args()
print(args[0])   -- script file or host argv[0]
print(args[1])   -- first script argument
```

### process.entry() -> table

Return entrypoint metadata for the current script:

- `file` -- current entry script path/name, or `nil` if none is configured
- `dir` -- current script directory used for relative script/module loading
- `args` -- the same table returned by `process.args()`

```
entry := process.entry()
print(entry.file)
print(entry.dir)
print(entry.args[1])
```

### process.setArgs(script, args...)

Set the interpreter-backed script argument list. This is mainly useful for
embedders and tests that need to emulate command-line execution.

```
process.setArgs("tool.gs", "build", "--fast")
args := process.args()
-- args[0] == "tool.gs"
-- args[1] == "build"
-- args[2] == "--fast"
```

### process.exit([code])

Signal host-controlled process termination. The CLI turns this into an OS exit
status; embedders can catch the process-exit error instead of terminating.

`code` defaults to `0`. A numeric code is used directly. Boolean `false` maps to
exit code `1`; boolean `true` maps to `0`.

```
process.exit(0)
process.exit(false)  -- exit code 1
```
