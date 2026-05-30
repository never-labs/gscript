print("case:calls_type_basic")

assert(type(1 < 2) == "boolean")
assert(type(true) == "boolean" and type(false) == "boolean")
assert(type(nil) == "nil")
assert(type(-3) == "number")
assert(type("x") == "string")
assert(type({}) == "table")
assert(type(type) == "function")
assert(type(assert) == type(print))

print("ok")
