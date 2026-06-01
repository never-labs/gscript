# binary

The `binary` library packs Leia values into byte strings and unpacks byte
strings back into values.

## Format Strings

Formats are whitespace- or comma-separated tokens. The default byte order is
little-endian.

Byte order tokens:

- `le`, `little`, `littleendian`, or `<`
- `be`, `big`, `bigendian`, or `>`
- prefix form such as `be:u16` or `<u32`

Field tokens:

| Token | Meaning |
|-------|---------|
| `i8`, `int8` | signed 8-bit integer |
| `u8`, `uint8` | unsigned 8-bit integer |
| `i16`, `int16` | signed 16-bit integer |
| `u16`, `uint16` | unsigned 16-bit integer |
| `i32`, `int32` | signed 32-bit integer |
| `u32`, `uint32` | unsigned 32-bit integer |
| `i64`, `int64` | signed 64-bit integer |
| `u64`, `uint64` | unsigned 64-bit integer |
| `f32`, `float32` | IEEE-754 32-bit float |
| `f64`, `float64` | IEEE-754 64-bit float |
| `string`, `str`, `bytes` | u32 length-prefixed byte string |
| `string:N`, `str:N`, `bytes:N` | exactly `N` raw bytes |

## Functions

### binary.pack(format, ...) -> string

Pack one value per field and return the encoded byte string.

```
data := binary.pack("be:u16 i32 f32 string bytes:3", 258, -7, 1.5, "go", "abc")
```

Fixed-size string fields require the input string to contain exactly the
requested number of bytes.

### binary.unpack(format, data [, offset]) -> values..., nextOffset

Unpack fields from `data`. `offset` is 1-based and defaults to `1`. The final
return value is the next 1-based offset after the decoded fields.

```
data := binary.pack("le u16 u32", 513, 16909060)
a, b, next := binary.unpack("le u16 u32", "xx" .. data, 3)
-- a == 513
-- b == 16909060
```

On short input or invalid offset, returns `nil, error`.

### binary.size(format) -> int [, error]

Return the fixed byte size for a format. Formats containing variable-size
`string`, `str`, or `bytes` fields return `nil, error`.

```
binary.size("u16 u32 bytes:3")  -- 9
size, err := binary.size("string")
-- size == nil
```

### string.pack / string.unpack / string.packsize

The `string` namespace also exposes compatibility entry points with the same
Go-style binary format strings:

```
data := string.pack("be:u16 bytes:2", 258, "go")
a, raw, next := string.unpack("be:u16 bytes:2", data)
size := string.packsize("be:u16 bytes:2")
```

These functions intentionally share `binary` formats; they are not Lua
`string.pack` format-string clones.
