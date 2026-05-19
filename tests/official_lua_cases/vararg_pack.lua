print("case:vararg_pack")

local function count(...)
  return select("#", ...)
end

local function collect(a, b, ...)
  local rest = table.pack(...)
  return a, b, rest.n, rest[1], rest[2]
end

assert(count() == 0)
assert(count(10, 20, 30) == 3)

local a, b, n, x, y = collect(1, 2, 3, nil, 5)
assert(a == 1 and b == 2 and n == 3 and x == 3 and y == nil)

local function forward(...)
  return collect(...)
end

a, b, n, x, y = forward(7, 8, 9, 10)
assert(a == 7 and b == 8 and n == 2 and x == 9 and y == 10)

print("ok")
