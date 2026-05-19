print("case:vararg_tail_missing_args")

local function f (a, b, c)
  return c, b
end

local function g ()
  return f(1, 2)
end

local a, b = g()
assert(a == nil and b == 2)

print("ok")
