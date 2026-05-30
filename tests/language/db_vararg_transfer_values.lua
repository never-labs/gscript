print("case:db_vararg_transfer_values")

local function prefix(...)
  return 3, ...
end

local a = {}
for i = 1, 100 do
  a[i] = i
end

local x, y, z = table.unpack(a, 1, 3)
assert(x == 1 and y == 2 and z == 3)

local p, q, r, s = prefix(1, 2, 3)
assert(p == 3 and q == 1 and r == 2 and s == 3)

local function collect(...)
  local packed = table.pack(...)
  assert(packed.n == select("#", ...))
  return packed
end

local t = collect(20, 10, 0, nil, "x")
assert(t.n == 5 and t[1] == 20 and t[2] == 10 and t[3] == 0 and t[4] == nil and t[5] == "x")

print("ok")
