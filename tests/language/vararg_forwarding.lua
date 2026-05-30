print("case:vararg_forwarding")

local function vararg (...)
  return {n = select("#", ...), ...}
end

local function first_and_more (a, ...)
  return a, ...
end

local x = vararg(nil, nil)
assert(x.n == 2 and x[1] == nil and x[2] == nil)

local a, b, c = first_and_more(1, 2, 3)
assert(a == 1 and b == 2 and c == 3)

print("ok")
