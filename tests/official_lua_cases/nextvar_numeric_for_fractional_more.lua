print("case:nextvar_numeric_for_fractional_more")

local function count(a, b, c)
  local n, last = 0, nil
  for i = a, b, c do n = n + 1; last = i end
  return n, last
end

local n, last = count(0.5, 2.0, 0.5)
assert(n == 4 and last == 2.0)
n, last = count(2.0, 0.5, -0.5)
assert(n == 4 and last == 0.5)
n, last = count(-1.25, -0.25, 0.25)
assert(n == 5 and last == -0.25)
n, last = count(-0.25, -1.25, -0.25)
assert(n == 5 and last == -1.25)

print("ok")
