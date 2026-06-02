# Values And Types

Leia is dynamically typed. Types are properties of runtime values, not declared
static variable slots.

Stable value categories are:

- nil;
- booleans;
- numbers;
- strings;
- tables;
- functions;
- coroutines;
- channels;
- host-backed values represented through tables or native functions.

The v1.0 value model has three observable attributes:

1. category, reported by `type`;
2. representation details explicitly exposed by a stable API, such as
   `math.type` for numbers;
3. identity, only for tables, functions, coroutines, channels, and documented
   host-backed values.

Variables, parameters, table fields, return slots, channel messages, and
protected-call result slots hold values. Assignment copies the value reference,
not the contents of an identity-bearing value. Reassigning a variable does not
mutate the old value; mutating an aliased table does.

```leia run all
a := {count: 1}
b := a
b.count = 2
assert(a.count == 2)
assert(a == b)

b = {count: 3}
assert(a.count == 2)
assert(b.count == 3)
assert(a != b)

x := 1
y := x
y = 2
assert(x == 1)
assert(y == 2)
```

Only `nil` and `false` are falsy. Numbers, including `0`, empty strings, empty
tables, functions, coroutines, and channels are truthy.

```leia
if 0 {
    print("numbers are truthy")
}
if "" {
    print("empty strings are truthy")
}
```

Numbers have one script-visible category, `number`, with integer and
floating-point subtypes observable through `math.type`. The subtype is part of
the v1.0 observable contract for `math.type`, `tostring`, table-key behavior,
and host APIs that expose value kinds.

Integer literals may be decimal or use the `0x`, `0b`, and `0o` prefixes
specified in [Lexical Elements](lexical.md). The portable integer subtype range
for ordinary values is signed 48-bit two's-complement:
`-140737488355328` through `140737488355327`. Values outside that range are
represented as floats by the current runtime value model. Portable programs
must not depend on integer subtype preservation outside that range unless a
module contract says so. The unary `-` operator is not part of a decimal
literal; a negative boundary value can be produced by arithmetic even when the
corresponding positive magnitude would already be a float.

Floating-point literals use IEEE-754 binary64 semantics. Decimal and exponent
forms such as `1.25` and `1e3` are accepted where the lexer recognizes them.
Finite binary64 values have 53 bits of integer precision.
`math.huge` is positive infinity; negative infinity is `-math.huge`; NaN is
produced by math operations such as `math.sqrt(-1)`.

Arithmetic may preserve integer representation when both operands are integers
and the result fits the integer subtype range. Integer overflow promotes the
result to float. Mixed integer/float arithmetic produces float. Division
produces float. Exact float-to-integer conversion is exposed by
`math.tointeger`; conversion succeeds only for finite integral values in the
host integer range, and the resulting value still obeys the ordinary runtime
subtype range. JIT raw integer representations are not observable.

Numeric equality compares numeric values, so `1 == 1.0` is true even though
`math.type(1)` and `math.type(1.0)` differ. Ordered numeric comparison converts
both operands to their numeric value. NaN is unordered: it is not equal to
itself, and all primitive ordered comparisons involving NaN are false. Positive
and negative infinity compare according to IEEE-754 ordering.

Other primitive equality compares booleans and strings by value; `nil` is equal
only to `nil`. Strings order lexicographically by byte sequence. Values of other
categories do not have primitive ordering. Identity-bearing values compare by
identity unless metatable or host contracts say otherwise.

The primitive equality relation is total and never raises an error. Primitive
ordering is partial: it is defined only for numeric operands and for string
operands, plus any metatable or host comparison contract documented elsewhere.
Ordering values from unsupported categories raises a runtime error.

```leia run all
assert(nil == nil)
assert(true != false)
assert("a" < "b")
assert(2 < 3.5)

ok := pcall(func() {
    return {} < {}
})
assert(!ok)
```

Primitive string conversion for numbers is stable: integer values format as
decimal without a suffix; floats use binary64 shortest-round-trip formatting,
with a `.0` suffix for finite whole-number floats so their subtype remains
visible. NaN stringifies as `NaN`, positive infinity as `+Inf`, and negative
infinity as `-Inf`.

```leia run all
assert(type(1) == "number")
assert(math.type(1) == "integer")
assert(math.type(1.5) == "float")
assert(0xff == 255)
assert(0b1010 == 10)
assert(0o755 == 493)
assert(0.2e2 == 20)
assert(1 == 1.0)
assert(math.type(1.0) == "float")
assert(tostring(1) == "1")
assert(tostring(1.0) == "1.0")

maxPortableInt := 140737488355327
minPortableInt := -140737488355327 - 1
assert(math.type(maxPortableInt) == "integer")
assert(math.type(minPortableInt) == "integer")
assert(math.type(maxPortableInt + 1) == "float")
assert(math.tointeger(1.0) == 1)
assert(math.tointeger(1.5) == nil)
```

```leia run all
nan := math.sqrt(-1)
assert(math.isnan(nan))
assert(math.isinf(math.huge))
assert(math.isinf(-math.huge))
assert(nan != nan)
assert(!(nan == nan))
assert(!(nan < 0))
assert(!(nan <= 0))
assert(-math.huge < 0)
assert(math.huge > 0)
assert(tostring(nan) == "NaN")
assert(tostring(math.huge) == "+Inf")
assert(tostring(-math.huge) == "-Inf")
```

Strings are immutable byte strings. String operations produce new strings or
views specified by their library contract; they do not mutate existing string
values. Library functions may interpret strings as UTF-8, paths, JSON, or
protocol data when their module contract says so.

```leia run all
s := "abc"
t := s .. "d"
assert(s == "abc")
assert(t == "abcd")
assert(#"A" == 1)
```

Tables are mutable identity-bearing key/value objects. Raw table equality is
identity equality. Table keys may be any non-`nil` runtime value. Assigning
`nil` removes a key. Arrays, records, dense vectors, matrices, and SOA layouts
are optimized representations or standard library structures unless a future
spec promotes them to primitive value categories.

Value transport does not deep-copy tables, functions, channels, or coroutines.
A module, host API, or standard-library function may explicitly document that it
clones or serializes a value; absent that contract, ordinary argument passing,
returns, table storage, and channel transfer preserve identity.

The v1.0 numeric table-key contract follows the current runtime exactly. An
integer key is stored as an integer key. A float key whose value is finite and
integral is normalized to the corresponding integer key when written. Ordinary
float-key lookup is not normalized before lookup. Therefore `1 == 1.0` is true,
but the table hash slot used for a stored integer key is not found by an
ordinary `1.0` lookup; `t[1]` and `t[1.0]` do not behave as interchangeable
lookup expressions. Non-integral float keys are stored and looked up as float
keys. NaN may be used as a raw key by the current runtime, but it still does
not compare equal with `==`; portable programs should avoid NaN table keys
unless a module explicitly documents that convention.

```leia run all
t := {}
t[1] = "int"
assert(1 == 1.0)
assert(t[1] == "int")
assert(t[1.0] == nil)

t[1.0] = "written as int"
assert(t[1] == "written as int")
assert(t[1.0] == nil)

t[1.5] = "float"
assert(t[1.5] == "float")
assert(t[1] != t[1.5])
```

Functions are callable identity-bearing values. Script functions close over
their lexical environment; each evaluation of a function expression creates a
distinct function identity even if it closes over equal values or the same
source body. Aliasing preserves identity. Host functions are also functions:
their identity is the host-provided callable object, not the display name
returned by `tostring` and not structural equivalence of native code. Host
functions and script functions share script-visible call, argument adjustment,
multi-return, and protected-call semantics, but may differ in performance,
resource accounting, and recoverable host error behavior. Functions compare by
identity unless a host-backed value documents a narrower comparison rule.

```leia run all
func makeCounter() {
    n := 0
    return func() {
        n = n + 1
        return n
    }
}

counter := makeCounter()
same := counter
other := makeCounter()
assert(counter == same)
assert(counter != other)
assert(counter() == 1)
assert(counter() == 2)

host := tostring
assert(host == tostring)
assert(host != print)
```

Channels are identity-bearing synchronization values created by the runtime or
host libraries. Leia v1.0 has one script-visible channel category,
`type(ch) == "channel"`; it does not have distinct send-only, receive-only, or
direction-parameterized channel types. Host APIs may document directional
capabilities, but those capabilities are not separate language-level value
types.

Channel sends and receives transfer ordinary Leia values without copying
table/function/channel identity. Equality compares channel identity. A receive
from a closed channel yields the channel contract's closed result; sends to a
closed channel raise a runtime error unless protected.

```leia run all
ch := make(chan, 1)
same := ch
other := make(chan, 1)
assert(type(ch) == "channel")
assert(ch == same)
assert(ch != other)

box := {value: 7}
ch <- box
received := <-ch
assert(received == box)
received.value = 8
assert(box.value == 8)
```

The `type` function reports the stable script-visible category for ordinary
values.

```leia
type(nil)     // "nil"
type(true)    // "boolean"
type(1)       // "number"
type("x")     // "string"
type({})      // "table"
type(func() {}) // "function"
```

```leia run all
assert(type(nil) == "nil")
assert(type(true) == "boolean")
assert(type(1) == "number")
assert(type("x") == "string")
assert(type({}) == "table")
assert(type(func() {}) == "function")
```
