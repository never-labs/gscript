print("case:nextvar_next_function_identity_more")

assert(next{} == next{})
assert(type(next) == "function")
local t = {a = 1, b = 2}
local k, v = next(t)
assert((k == "a" or k == "b") and (v == 1 or v == 2))
local k2, v2 = next(t, k)
if k2 ~= nil then
  assert(k2 ~= k)
  assert((k2 == "a" or k2 == "b") and (v2 == 1 or v2 == 2))
end
assert(next({}) == nil)

print("ok")
