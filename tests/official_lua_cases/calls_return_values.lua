print("case:calls_return_values")

local function f ()
  return 1, 2, 3
end

local a, b, c = f()
assert(a == 1 and b == 2 and c == 3)

local function g ()
  f()
  return
end

assert(g() == nil)

local function h ()
  return nil or f()
end

a, b = h()
assert(a == 1 and b == nil)

print("ok")
