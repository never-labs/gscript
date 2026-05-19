print("case:calls_tail_varargs")

local X, Y, A
local function sink (x, y, ...)
  X = x
  Y = y
  A = {...}
end

local function forward (...)
  return sink(...)
end

local a, b, c = forward()
assert(X == nil and Y == nil and #A == 0)

print("ok")
