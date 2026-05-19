# bits

The `bits` library provides Go-style 64-bit integer bit operations. Inputs are
converted to integers; results are returned as GScript integers.

## Functions

### bits.and(...) -> int

Bitwise AND. With no arguments, returns `-1`.

### bits.or(...) -> int

Bitwise OR. With no arguments, returns `0`.

### bits.xor(...) -> int

Bitwise XOR. With no arguments, returns `0`.

### bits.not(n) -> int

Bitwise complement.

### bits.shl(n, shift) -> int

Logical left shift. `shift` must be non-negative. Shifts of 64 or more return
`0`.

### bits.shr(n, shift) -> int

Logical right shift through `uint64`. `shift` must be non-negative. Shifts of
64 or more return `0`.

### bits.sar(n, shift) -> int

Arithmetic right shift. `shift` must be non-negative. Shifts of 64 or more
return `-1` for negative inputs and `0` otherwise.

### bits.rotl(n, shift) -> int

Rotate left by `shift` bits.

### bits.rotr(n, shift) -> int

Rotate right by `shift` bits.

### bits.test(n, pos) -> bool

Return whether bit position `pos` is set. Positions are zero-based and must be
in `0..63`.

### bits.set(n, pos) -> int

Set bit `pos`.

### bits.clear(n, pos) -> int

Clear bit `pos`.

### bits.toggle(n, pos) -> int

Flip bit `pos`.

### bits.ones(n) -> int

Return the number of set bits in the `uint64` representation.

### bits.leadingZeros(n) -> int

Return the number of leading zero bits in the `uint64` representation.

### bits.trailingZeros(n) -> int

Return the number of trailing zero bits in the `uint64` representation.

## Expression Syntax

GScript also supports bitwise operators directly in expressions:

| Operator | Meaning |
|----------|---------|
| `&` | bitwise AND |
| `|` | bitwise OR |
| `^` | bitwise XOR; unary `^` is bitwise NOT |
| `&^` | bit clear |
| `<<` | left shift |
| `>>` | right shift |

`|` and `^` bind with additive operators. `&`, `&^`, `<<`, and `>>` bind with
multiplicative operators.

```
mask := (1 << 8) - 1
flags := (flags & mask) | 4
flags = flags &^ 2
flags = ^flags
```
