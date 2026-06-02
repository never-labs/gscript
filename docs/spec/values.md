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

## Assignment, Identity, And Aliasing

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

## Truthiness

Only `nil` and `false` are falsy. Numbers, including `0`, empty strings, empty
tables, functions, coroutines, and channels are truthy.

```leia run all
seenZero := false
seenEmpty := false
if 0 {
    seenZero = true
}
if "" {
    seenEmpty = true
}
assert(seenZero)
assert(seenEmpty)
```

## Numbers

Numbers have one script-visible category, `number`, with integer and
floating-point subtypes observable through `math.type`. The subtype is part of
the v1.0 observable contract for `math.type`, `tostring`, table-key behavior,
and host APIs that expose value kinds. The subtype is a property of the runtime
value, not of a variable or table slot.

Numeric literal syntax determines the initial runtime subtype. An integer
literal token is a decimal numeral without a decimal point or exponent, or a
prefixed numeral using `0x`, `0b`, or `0o` as specified in
[Lexical Elements](lexical.md). An integer literal denotes an integer value
when its mathematical value is in the portable integer subtype range; otherwise
it denotes the nearest representable float selected by the current runtime
conversion. A floating-point literal token has a decimal point or exponent and
always denotes a float value, even when its mathematical value is integral.
The unary `-` operator is not part of any literal token.

The portable integer subtype range for ordinary values is signed 48-bit
two's-complement: `-140737488355328` through `140737488355327`. Portable
programs may rely on integer subtype preservation inside that closed range.
Portable programs must not rely on integer subtype preservation outside that
range unless a module or host contract explicitly promises a wider range.
A negative boundary value can be produced by arithmetic even when the
corresponding positive source literal would be outside the range.

Float values use IEEE-754 binary64 semantics. Finite binary64 values have
53 bits of integer precision, so every integer whose magnitude is at most
`9007199254740992` is exactly representable as a float; not every larger
integer is. Decimal conversion for float literals is implementation-defined
only within the ordinary binary64 nearest-representable result. `math.huge` is
positive infinity; negative infinity is `-math.huge`; NaN is produced by math
operations such as `math.sqrt(-1)`.

Arithmetic may preserve integer representation when all operands that determine
the result are integers and the result fits the integer subtype range. Integer
overflow promotes the result to float. Mixed integer/float arithmetic produces
float. Division produces float. Exact float-to-integer conversion is exposed by
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
assert(math.type(1e3) == "float")
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
assert(9007199254740992.0 == 9007199254740992.0 + 1.0)
assert(9007199254740992.0 + 2.0 == 9007199254740994.0)

fromFloat := math.tointeger(140737488355327.0)
assert(fromFloat == 140737488355327)
assert(math.type(fromFloat) == "integer")
assert(math.tointeger(math.huge) == nil)
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

## Strings

Strings are immutable byte strings. A string value is its byte sequence; strings
do not have script-visible identity separate from that sequence. Equality and
ordering compare byte sequences, and `#s` reports the byte length. Assignment,
argument passing, return, table storage, and channel transfer of a string value
cannot create aliases through which the original bytes can be mutated. String
operations produce new strings or views specified by their library contract;
they do not mutate existing string values. Library functions may interpret
strings as UTF-8, paths, JSON, or protocol data when their module contract says
so.

```leia run all
s := "abc"
alias := s
t := s .. "d"
assert(s == "abc")
assert(alias == "abc")
assert(t == "abcd")
assert(#"A" == 1)
assert(string.sub(t, 1, 3) == s)
```

## Tables

Tables are mutable identity-bearing key/value objects. Raw table equality is
identity equality. Table keys may be any non-`nil` runtime value. Assigning
`nil` removes a key. Arrays, records, dense vectors, matrices, and SOA layouts
are optimized representations or standard library structures unless a separate
stable spec section explicitly promotes them to primitive value categories.

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

## Functions

Functions are callable identity-bearing values. Script functions close over
their lexical environment; each evaluation of a function expression creates a
distinct function identity even if it closes over equal values or the same
source body. A function declaration initializes its binding with one function
value when the declaration executes. Aliasing preserves identity; copying a
function value into a variable, table field, return slot, or channel message
does not clone the function or its captured environment. Host functions are
also functions: their identity is the host-provided callable object, not the
display name returned by `tostring` and not structural equivalence of native
code. Host functions and script functions share script-visible call, argument
adjustment, multi-return, and protected-call semantics, but may differ in
performance, resource accounting, and recoverable host error behavior.
Functions compare by identity unless a host-backed value documents a narrower
comparison rule.

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
expr1 := func() { return 1 }
expr2 := func() { return 1 }
tableAlias := {fn: expr1}
assert(counter == same)
assert(counter != other)
assert(expr1 != expr2)
assert(tableAlias.fn == expr1)
assert(counter() == 1)
assert(counter() == 2)

host := tostring
assert(host == tostring)
assert(host != print)
```

## Channels

Channels are identity-bearing synchronization values created by the runtime or
host libraries. Leia v1.0 has one script-visible channel category,
`type(ch) == "channel"`; it does not have distinct send-only, receive-only, or
direction-parameterized channel types. Channel direction is not a value subtype,
not part of equality, and not reported by `type`. A host API may document that
a parameter will only be sent to or only received from, but that direction is an
API precondition or capability contract, not a separate language-level value
type and not a castable value form.

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
func id(x) { return x }
fn := id
ch <- box
received := <-ch
assert(received == box)
received.value = 8
assert(box.value == 8)

ch <- fn
receivedFn := <-ch
assert(receivedFn == fn)
assert(receivedFn(9) == 9)
```

## Type Queries

The `type` function reports the stable script-visible category for ordinary
values.

```text
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
