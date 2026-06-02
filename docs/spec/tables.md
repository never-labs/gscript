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

```leia
t := { "first", "second", name: "Ada" }
t[1]        // "first"
t.name      // "Ada"
t["name"]   // "Ada"
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

```leia
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

t.missing        // "fallback"
rawget(t, "missing") // nil

t.created = 12
rawget(t, "created") // nil; __newindex handled the write
rawset(t, "created", 13)
rawget(t, "created") // 13
```

Stable metatable behavior is defined by the events below. A metamethod is looked
up by raw string key in the value's metatable. The lookup never invokes
`__index` on the metatable itself. Each metamethod receives the listed
arguments; unless otherwise stated, only its first return value is used.

For a binary event named `event`, Leia uses this lookup algorithm after any
primitive operation has failed:

1. If the left operand is a table and its metatable has a non-`nil` raw field named `event`, use that value.
2. Otherwise, if the right operand is a table and its metatable has a non-`nil` raw field named `event`, use that value.
3. Otherwise, the operation raises the normal type error for that operator.

The chosen metamethod is called as `(left, right)` even when it came from the
right operand. Operators `>` and `>=` are normalized before lookup: `left > right`
dispatches as `right < left`, and `left >= right` dispatches as `right <= left`.
Leia does not require the two operands to share the same metatable or the same
metamethod function.

| Metamethod | Trigger | Arguments | Result contract |
| --- | --- | --- | --- |
| `__index` | `t[k]` or `t.k` when `t` has no raw key `k`. Strings use a standard-library `__index` table for methods. | If function: `(t, k)`. If table: lookup continues in that table with key `k`. | Function result or redirected table lookup result. Missing chains produce `nil`; chains deeper than 50 redirects raise a runtime error. |
| `__newindex` | `t[k] = v` when `t` has no raw key `k`. | If function: `(t, k, v)`. If table: assignment continues in that table. | Return values are ignored. Existing raw keys are updated directly. Chains deeper than 50 redirects raise a runtime error. |
| `__call` | Calling a non-function table value, `t(...)`. | `(t, ...)` | All return values become the call result. |
| `__add`, `__sub`, `__mul`, `__div`, `__mod`, `__pow` | `+`, `-`, `*`, `/`, `%`, `**` when the primitive numeric operation is not applicable. | `(left, right)` | First return value is the operator result. |
| `__unm` | Unary `-x` when primitive numeric negation is not applicable. | `(x)` | First return value is the operator result. |
| `__concat` | `left .. right` when primitive string/number concatenation is not applicable. | `(left, right)` | First return value is the concatenation result. |
| `__len` | `#x` for tables or other values with length behavior. | `(x)` | First return value is the length result. Library APIs that require an integer length may reject non-integer or negative results. |
| `__eq` | `left == right` or `left != right` when both operands are tables and are not the same table identity. | `(left, right)` | Truthiness of the first return value determines equality; `!=` negates it. Primitive values and same-identity tables use raw equality and do not call `__eq`. |
| `__lt` | `left < right`, and reversed `>` forms. | `(left, right)` for `<`; operands are reversed for `>`. | Truthiness of the first return value determines the comparison. |
| `__le` | `left <= right`, and reversed `>=` forms. | `(left, right)` for `<=`; operands are reversed for `>=`. | Truthiness of the first return value determines the comparison. There is no fallback to `__lt`; if neither operand supplies `__le`, the operation raises a comparison error. |
| `__pairs` | `pairs(x)` when `x` has this metamethod. | `(x)` | Must return iterator function, state, and initial control value. |
| `__tostring` | `tostring(x)` for a table with this metamethod. | `(x)` | Must return a string; other results raise a runtime error. |
| `__name` | `tostring(x)` fallback for a table with no `__tostring`. | Not called; read as a string field. | Used as a type-name prefix in the fallback string form. |
| `__metatable` | `getmetatable(x)` and `setmetatable(x, mt)` for protected tables. | Not called; read as a field. | `getmetatable` returns this value; attempts to change the protected metatable raise a runtime error. |

Metamethods for integer floor division, bitwise operators, finalizers, weak
tables, binary chunks, and Lua debug-slot protocols are not v1.0 stable
contract unless a later spec revision names them explicitly.

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

```leia
t := setmetatable({}, { __len: func(_) { return 99 } })
#t        // 99
rawlen(t) // 0
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
