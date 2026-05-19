print("case:vararg_call_unpack_more")

local function c12(...)
  local x = table.pack(...)
  local res = (x.n == 2 and x[1] == 1 and x[2] == 2)
  if res then res = 55 end
  return res, 2
end

local call = function(f, args) return f(table.unpack(args, 1, args.n)) end

local a, b = assert(call(c12, {1, 2, n = 2}))
assert(a == 55 and b == 2)
a = call(c12, {1, 2, n = 1})
assert(not a)

local lim = 20
local vals = {}
for i = 1, lim do vals[i] = i + 0.3 end

local function fixed(a, b, c, d, ...)
  local more = table.pack(...)
  assert(a == 1.3 and b == 2.3 and c == 3.3 and d == 4.3)
  assert(more[1] == 5.3 and more[lim - 4] == lim + 0.3 and more[lim - 3] == nil)
end

call(fixed, vals)

for i = 1, lim do vals[i] = i end
assert(call(math.max, vals) == lim)

print("ok")
