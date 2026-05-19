# io

The `io` library provides Lua-style file I/O around GScript file handles.

## Module Functions

### io.write(...)

Write values to the current output stream without adding a newline. Values are
converted with GScript string conversion. Returns `nil` on success or
`nil, error` if the stream cannot be written.

### io.read([format...])

Read from the current input stream. With no format it reads one line.

Formats:

- `"*l"` or `"l"` -- line without the trailing newline
- `"*a"` or `"a"` -- all remaining input
- `"*n"` or `"n"` -- number from the next input line
- integer count -- up to that many bytes

Multiple formats return multiple values.

### io.lines([filename]) -> iterator

Return an iterator over lines from `filename`, or from the current input stream
when no filename is provided. A file opened by `io.lines(filename)` is closed
when iteration reaches EOF.

### io.open(filename [, mode]) -> file [, error]

Open a file and return a file handle. Supported modes are `r`, `w`, `a`, `r+`,
`w+`, `a+`, and the same modes with `b` ignored for binary compatibility.

### io.flush() -> true [, error]

Flush the current output stream.

### io.input([fileOrPath]) -> file [, error]

With no argument, return the current input handle. With a file handle or path,
set the current input stream and return it.

### io.output([fileOrPath]) -> file [, error]

With no argument, return the current output handle. With a file handle or path,
set the current output stream and return it. A path is opened for write,
creating or truncating the file.

### io.type(obj) -> string | nil

Return `"file"` for an open GScript file handle, `"closed file"` for a closed
handle, or `nil` for non-file values.

### io.tmpfile() -> file [, error]

Create a temporary read/write file handle. The backing path is removed
immediately, so it is cleaned up by the OS after the handle is closed.

## File Methods

File handles support method-call style:

```
f := io.open("data.txt", "w+")
f:write("abc")
f:flush()
f:seek("set", 0)
print(f:read(3))      -- "abc"
print(io.type(f))     -- "file"
f:close()
print(io.type(f))     -- "closed file"
```

### file:read([format...])

Read from this file using the same formats as `io.read`.

### file:write(...)

Write values to this file. On success, returns the file handle.

### file:close() -> true [, error]

Close the file handle.

### file:flush() -> true [, error]

Flush the file handle.

### file:seek([whence [, offset]]) -> position [, error]

Seek and return the new zero-based byte position. `whence` is `"set"`, `"cur"`,
or `"end"`; defaults to `"cur"`. `offset` defaults to `0`.

### file:lines() -> iterator

Return an iterator over lines from the current file position.

## Standard Handles

`io.stdin`, `io.stdout`, and `io.stderr` are file handles for the process
standard streams.
