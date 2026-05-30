print("case:vararg_argument_adjust_more")

local function f (a, ...)
  local x = table.pack(...)
  for i = 1, x.n do assert(a[i] == x[i]) end
  return x.n, x[1], x[2], x[3], x[4]
end

local n, a, b, c, d = f({1,2,3}, 1, 2, 3)
assert(n == 3 and a == 1 and b == 2 and c == 3 and d == nil)
n, a, b, c, d = f({"alo", nil, 45, f, nil}, "alo", nil, 45, f, nil)
assert(n == 5 and a == "alo" and b == nil and c == 45 and d == f)

local function c12 (...)
  local x = table.pack(...)
  local res = (x.n == 2 and x[1] == 1 and x[2] == 2)
  if res then res = 55 end
  return res, 2
end

assert(c12(1,2) == 55)
assert(c12(1,2,3) == false)

print("ok")
