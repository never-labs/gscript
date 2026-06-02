# Tables And Metatables

Tables are mutable identity-bearing maps. Keys and values are runtime values.
Implementations may use optimized array, record, or typed layouts when this
does not change observable behavior.

Table constructors create fresh table identities. List-style fields are assigned
using 1-based integer sequence keys. Keyed fields assign the specified key.
The v1.0 stable constructor subset is simple list fields, named fields, and
explicit keyed fields that do not depend on an interleaving order between those
forms. Portable programs must not depend on duplicate constructor fields that
write the same normalized key, or on list-index assignment after interleaved
keyed fields; use explicit assignments when that order matters. A constructor
expression itself never reuses an existing table identity.

```leia run all
t := { "first", "second", name: "Ada" }
assert(t[1] == "first")
assert(t.name == "Ada")
assert(t["name"] == "Ada")
```

```leia run all
a := {"x", "y", name: "Ada"}
b := {"x", "y", name: "Ada"}
assert(a != b)
assert(a[1] == "x")
assert(a[2] == "y")
assert(a.name == "Ada")

a[3] = "third"
assert(a[3] == "third")
```

Assigning `nil` to a table field removes that field for ordinary table lookup.
Tables compare by identity unless a metatable supplies comparison behavior.

Raw table operations are the primitive map operations:

- `rawget(t, k)` returns the stored value for key `k`, or `nil` if absent;
- `rawset(t, k, v)` stores `v` at key `k` and returns `t`;
- `rawset(t, k, nil)` removes key `k`;
- `rawequal(a, b)` compares primitive equality without invoking `__eq`;
- `rawlen(x)` returns the primitive length for strings and tables.

Raw helpers bypass metamethods by contract. Non-raw operations may consult
metatables when the corresponding operation names an event below.

```leia run all
t := {}
assert(rawset(t, "x", 1) == t)
assert(rawget(t, "x") == 1)
t.x = nil
assert(rawget(t, "x") == nil)

a := {}
b := {}
assert(rawequal(a, a))
assert(!rawequal(a, b))
```

```leia run all
log := {}
t := setmetatable({present: 11}, {
    __index: func(_, key) {
        log[#log + 1] = "index:" .. key
        return "fallback"
    },
    __newindex: func(_, key, value) {
        log[#log + 1] = "new:" .. key
    },
})

assert(t.present == 11)
assert(t.missing == "fallback")
assert(log[1] == "index:missing")
assert(rawget(t, "missing") == nil)

t.created = 12
assert(log[2] == "new:created")
assert(rawget(t, "created") == nil)
rawset(t, "created", 13)
assert(rawget(t, "created") == 13)
```

Stable table/metamethod behavior is defined by the events below. A metamethod
is looked up by raw string key in the value's metatable. The lookup never
invokes `__index` on the metatable itself. Each metamethod receives the listed
arguments; unless otherwise stated, only its first return value is used.

Only table values have user-controlled metatables in the v1.0 stable contract.
Strings have implementation-provided method lookup through `__index`, but
portable programs must not assume that `setmetatable` can change string
behavior.

For a binary event named `event`, Leia uses this lookup algorithm after any
primitive operation has failed:

1. If the left operand is a table and its metatable has a non-`nil` raw field named `event`, use that value.
2. Otherwise, if the right operand is a table and its metatable has a non-`nil` raw field named `event`, use that value.
3. Otherwise, the operation raises the normal type error for that operator.

The chosen binary metamethod is called as `(left, right)` even when it came from
the right operand. Operators `>` and `>=` are normalized before lookup:
`left > right` dispatches as `right < left`, and `left >= right` dispatches as
`right <= left`. Leia does not require the two operands to share the same
metatable or metamethod function.

| Metamethod | Trigger | Arity | Return contract |
| --- | --- | --- | --- |
| `__index` | `t[k]` or `t.k` when table `t` has no raw key `k`. Strings use an implementation-provided `__index` table for methods. | If function: `(t, k)`. If table: no call; lookup continues in that table with key `k`. | Function result or redirected table lookup result. Missing chains produce `nil`; chains deeper than 50 redirects raise a runtime error. |
| `__newindex` | `t[k] = v` when table `t` has no raw key `k`. | If function: `(t, k, v)`. If table: no call; assignment continues in that table. | Return values are ignored. Existing raw keys are updated directly. Chains deeper than 50 redirects raise a runtime error. |
| `__add` | `left + right` when primitive numeric addition is not applicable. | `(left, right)` | First return value is the operator result. |
| `__sub` | `left - right` when primitive numeric subtraction is not applicable. | `(left, right)` | First return value is the operator result. |
| `__mul` | `left * right` when primitive numeric multiplication is not applicable. | `(left, right)` | First return value is the operator result. |
| `__div` | `left / right` when primitive numeric division is not applicable. | `(left, right)` | First return value is the operator result. |
| `__mod` | `left % right` when primitive numeric modulo is not applicable. | `(left, right)` | First return value is the operator result. |
| `__pow` | `left ** right` when primitive numeric exponentiation is not applicable. | `(left, right)` | First return value is the operator result. |
| `__unm` | Unary `-x` when primitive numeric negation is not applicable. | `(x)` | First return value is the operator result. |
| `__eq` | `left == right` or `left != right` when both operands are tables and are not the same table identity. | `(left, right)` | Truthiness of the first return value determines equality; `!=` negates it. Primitive values and same-identity tables use raw equality and do not call `__eq`. |
| `__lt` | `left < right`, or `left > right` after operand reversal. | `(left, right)` for `<`; `(right, left)` for `>`. | Truthiness of the first return value determines the comparison. |
| `__le` | `left <= right`, or `left >= right` after operand reversal. | `(left, right)` for `<=`; `(right, left)` for `>=`. | Truthiness of the first return value determines the comparison. There is no fallback to `__lt`; if neither operand supplies `__le`, the operation raises a comparison error. |
| `__concat` | `left .. right` when primitive string/number concatenation is not applicable. | `(left, right)` | First return value is the concatenation result. |
| `__len` | `#x` for a table `x`. | `(x)` | First return value is the length result. Library APIs that require an integer length may reject non-integer or negative results. |
| `__call` | Calling a non-function table value, `t(...)`. | `(t, ...)` | All return values become the call result. |
| `__tostring` | `tostring(x)` for a table with this metamethod. | `(x)` | Must return a string; other results raise a runtime error. |
| `__metatable` | `getmetatable(x)` and `setmetatable(x, mt)` for a table whose current metatable has a non-`nil` raw `__metatable` field. | Not called; read as a field. | `getmetatable` returns this value instead of the real metatable. Attempts to change the protected metatable raise a runtime error. |

Raw operations bypass metamethod dispatch by contract:

- `rawget` never invokes `__index`;
- `rawset` never invokes `__newindex` and can create, replace, or remove raw keys directly;
- `rawequal` never invokes `__eq`;
- `rawlen` never invokes `__len`.

`__metatable` protects the metatable slot, not arbitrary table contents. If a
program already holds a reference to the metatable table, ordinary writes to
that table remain ordinary table writes.

The current implementation also supports `__pairs` for `pairs(x)` and `__name`
as a `tostring` fallback prefix. These are implementation extensions, not part
of the v1.0 stable table/metamethod contract above. Metamethods for integer
floor division, bitwise operators, finalizers, weak tables, binary chunks, and
Lua debug-slot protocols are not v1.0 stable contract unless a later spec
revision names them explicitly.

```leia run all
vec := {x: 3}
mt := {
    __add: func(a, b) { return {x: a.x + b.x} },
    __eq: func(a, b) { return a.x == b.x },
    __tostring: func(a) { return "vec(" .. a.x .. ")" },
}
setmetatable(vec, mt)
other := setmetatable({x: 4}, mt)
sum := vec + other
assert(sum.x == 7)
assert(vec == setmetatable({x: 3}, mt))
assert(tostring(vec) == "vec(3)")
```

```leia run all
num_mt := {}
num := func(value) {
    return setmetatable({value: value}, num_mt)
}

num_mt.__add = func(a, b) { return num(a.value + b.value) }
num_mt.__sub = func(a, b) { return num(a.value - b.value) }
num_mt.__mul = func(a, b) { return num(a.value * b.value) }
num_mt.__div = func(a, b) { return num(a.value / b.value) }
num_mt.__mod = func(a, b) { return num(a.value % b.value) }
num_mt.__pow = func(a, b) { return num(a.value ** b.value) }
num_mt.__unm = func(a) { return num(-a.value) }
num_mt.__concat = func(a, b) { return "num:" .. a.value .. ":" .. b.value }

a := num(5)
b := num(2)
assert((a + b).value == 7)
assert((a - b).value == 3)
assert((a * b).value == 10)
assert((a / b).value == 2.5)
assert((a % b).value == 1)
assert((a ** b).value == 25)
assert((-a).value == -5)
assert(a .. b == "num:5:2")
```

```leia run all
callable := setmetatable({base: 10}, {
    __call: func(self, value) {
        return self.base + value, self.base - value
    },
})

sum, diff := callable(3)
assert(sum == 13)
assert(diff == 7)

mt := {__metatable: "locked"}
protected := setmetatable({}, mt)
assert(getmetatable(protected) == "locked")

ok, err := pcall(func() {
    setmetatable(protected, nil)
})
assert(!ok)
assert(type(err) == "string")

mt.__metatable = nil
assert(setmetatable(protected, nil) == protected)
```

```leia run all
left := setmetatable({name: "left"}, {
    __add: func(a, b) { return a.name .. "+" .. b.name },
})
right := setmetatable({name: "right"}, {
    __add: func(a, b) { return a.name .. "->" .. b.name },
})
plain := {name: "plain"}

assert(left + right == "left+right")
assert(plain + right == "plain->right")
```

```leia run all
mt := {}
mt.__eq = func(a, b) { return a.key == b.key }
mt.__lt = func(a, b) { return a.key < b.key }

a := setmetatable({key: 1}, mt)
b := setmetatable({key: 1}, mt)
c := a

assert(a == b)
assert(rawequal(a, c))
assert(!rawequal(a, b))
assert(pcall(func() { return a <= b }) == false)

mt.__le = func(a, b) { return a.key <= b.key }
assert(a <= b)
```

`__index` and `__newindex` table redirects are ordinary table operations on the
redirect target. They can chain through more metatables, and the same raw-key
rule is applied at each hop. A function-valued `__index` or `__newindex` stops
the chain by handling the operation directly.

```leia run all
base := {answer: 42}
middle := setmetatable({}, {__index: base})
obj := setmetatable({}, {__index: middle})
assert(obj.answer == 42)

log := {}
sink := setmetatable({}, {
    __newindex: func(_, key, value) {
        log[#log + 1] = key .. ":" .. value
    },
})
proxy := setmetatable({}, {__newindex: sink})
proxy.event = "saved"
assert(log[1] == "event:saved")
assert(rawget(proxy, "event") == nil)
```

```leia fail all
t := {}
setmetatable(t, {__index: t})
return t.missing
```

```leia fail all
t := {}
setmetatable(t, {__newindex: t})
t.missing = 1
```

The length operator `#x` uses the value's ordinary length behavior and may
consult `__len`. `rawlen(x)` bypasses `__len` for tables and strings.

```leia run all
t := setmetatable({}, { __len: func(_) { return 99 } })
assert(#t == 99)
assert(rawlen(t) == 0)
```

```leia run all
t := setmetatable({}, { __len: func(_) { return 99 } })
assert(#t == 99)
assert(rawlen(t) == 0)
```

Sequence length on sparse tables follows Leia runtime behavior. Programs that
depend on sparse length edge cases should pin that behavior with tests.

Iteration order for hash-like table keys is not a v1.0 stable contract.
Programs may depend on `pairs` visiting each key that remains present during an
ordinary traversal, but not on the order of those visits. `ipairs` is the
portable sequence traversal form for consecutive positive integer keys starting
at `1`; it stops at the first missing or `nil` element.

```leia run all
t := {10, 20, [4]: 40, name: "Ada"}
seen := {}
for _, value := range ipairs(t) {
    seen[#seen + 1] = value
}
assert(#seen == 2)
assert(seen[1] == 10)
assert(seen[2] == 20)

count := 0
for _ := range pairs(t) {
    count = count + 1
}
assert(count == 4)
```
