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

Stable metatable behavior includes indexing, new indexing, call, arithmetic,
comparison, concatenation, and length behavior where covered by conformance
tests and the feature matrix. Exact Lua debug-slot protocols, binary chunks,
and finalizer behavior are not stable Leia promises unless specified
separately.

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
