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
floating-point subtypes observable through `math.type`.

Integer literals may be decimal or use the `0x`, `0b`, and `0o` prefixes
specified in [Lexical Elements](lexical.md). The portable exact integer range
for ordinary values is at least signed 48-bit two's-complement:
`-140737488355328` through `140737488355327`. Implementations may preserve
larger integer values in host APIs, libraries, or optimized paths, but portable
programs must not depend on exact integer subtype preservation outside that
range unless a module contract says so.

Floating-point literals use IEEE-754 binary64 semantics. Decimal and exponent
forms such as `1.25`, `.5`, `1.`, and `1e3` are accepted where the lexer
recognizes them. Finite binary64 values have 53 bits of integer precision;
`math.huge`, infinities, and NaN follow the runtime math library contract.

Arithmetic may preserve integer representation when exact and fall back to
floating-point or boxed runtime operations. JIT raw integer representations are
not observable.

```leia run all
assert(type(1) == "number")
assert(math.type(1) == "integer")
assert(math.type(1.5) == "float")
assert(0xff == 255)
assert(0b1010 == 10)
assert(0o755 == 493)
assert(0.2e2 == 20)
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

Tables are mutable identity-bearing key/value objects. Arrays, records, dense
vectors, matrices, and SOA layouts are optimized representations or standard
library structures unless a future spec promotes them to primitive value
categories.

Functions are callable identity-bearing values. Script functions close over
their lexical environment. Host functions and script functions share
script-visible call, argument adjustment, multi-return, and protected-call
semantics, but may differ in performance, resource accounting, and recoverable
host error behavior. Functions compare by identity unless a host-backed value
documents a narrower comparison rule.

Channels are identity-bearing synchronization values created by the runtime or
host libraries. Channel sends and receives transfer ordinary Leia values
without copying table/function/channel identity. Equality compares channel
identity. A receive from a closed channel yields the channel contract's closed
result; sends to a closed channel raise a runtime error unless protected.

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
