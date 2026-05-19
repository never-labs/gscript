print("case:nextvar_next_function_identity_more")

assert(next({}) == next({}))
assert(type(next) == "function")
t := {a: 1, b: 2}
k, v := next(t)
assert((k == "a" || k == "b") && (v == 1 || v == 2))
k2, v2 := next(t, k)
if k2 != nil {
  assert(k2 != k)
  assert((k2 == "a" || k2 == "b") && (v2 == 1 || v2 == 2))
}
assert(next({}) == nil)

print("ok")
