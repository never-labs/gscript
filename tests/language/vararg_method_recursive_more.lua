print("case:vararg_method_recursive_more")

local t = {1, 10}
function t:f(...)
  local arg = table.pack(...)
  return self[...] + arg.n
end

assert(t:f(1, 4) == 3 and t:f(2) == 11)

local function oneless(a, ...) return ... end

local function f(n, a, ...)
  if n == 0 then
    local b, c, d = ...
    return a, b, c, d, oneless(oneless(oneless(...)))
  else
    local b
    n, b, a = n - 1, ..., a
    assert(b == (...))
    return f(n, a, ...)
  end
end

local a, b, c, d, e = assert(f(10, 5, 4, 3, 2, 1))
assert(a == 5 and b == 4 and c == 3 and d == 2 and e == 1)

a, b, c, d, e = f(4)
assert(a == nil and b == nil and c == nil and d == nil and e == nil)

print("ok")
