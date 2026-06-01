# Tables And Metatables

Tables are mutable identity-bearing maps. Keys and values are runtime values.
Implementations may use optimized array, record, or typed layouts when this
does not change observable behavior.

Table constructors create fresh table identities. List-style fields are assigned
using 1-based integer sequence keys. Keyed fields assign the specified key.

```leia
t := { "first", "second", name: "Ada" }
t[1]        // "first"
t.name      // "Ada"
t["name"]   // "Ada"
```

Assigning `nil` to a table field removes that field for ordinary table lookup.
Tables compare by identity unless a metatable supplies comparison behavior.

Raw helpers bypass metamethods by contract. Non-raw operations may consult
metatables when the runtime supports the corresponding metamethod.

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
up by raw string key in the value's metatable. Binary operators first try the
left operand's metamethod and then the right operand's metamethod when needed.
Each metamethod receives the listed arguments; unless otherwise stated, only its
first return value is used.

| Metamethod | Trigger | Arguments | Result contract |
| --- | --- | --- | --- |
| `__index` | `t[k]` or `t.k` when `t` has no raw key `k`. Strings use a standard-library `__index` table for methods. | If function: `(t, k)`. If table: lookup continues in that table with key `k`. | Function result or redirected table lookup result. Missing chains produce `nil`; excessive cycles raise a runtime error. |
| `__newindex` | `t[k] = v` when `t` has no raw key `k`. | If function: `(t, k, v)`. If table: assignment continues in that table. | Return values are ignored. Existing raw keys are updated directly. |
| `__call` | Calling a non-function table value, `t(...)`. | `(t, ...)` | All return values become the call result. |
| `__add`, `__sub`, `__mul`, `__div`, `__mod`, `__pow` | `+`, `-`, `*`, `/`, `%`, `**` when the primitive numeric operation is not applicable. | `(left, right)` | First return value is the operator result. |
| `__unm` | Unary `-x` when primitive numeric negation is not applicable. | `(x)` | First return value is the operator result. |
| `__concat` | `left .. right` when primitive string/number concatenation is not applicable. | `(left, right)` | First return value is the concatenation result. |
| `__len` | `#x` for tables or other values with length behavior. | `(x)` | First return value is the length result. Library APIs that require an integer length may reject non-integer or negative results. |
| `__eq` | `left == right` or `left != right` for identity-bearing values that are not raw-equal. | `(left, right)` | Truthiness of the first return value determines equality; `!=` negates it. |
| `__lt` | `left < right`, and reversed `>` forms. | `(left, right)` for `<`; operands are reversed for `>`. | Truthiness of the first return value determines the comparison. |
| `__le` | `left <= right`, and reversed `>=` forms. | `(left, right)` for `<=`; operands are reversed for `>=`. | Truthiness of the first return value determines the comparison. |
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
