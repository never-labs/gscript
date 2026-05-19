print("case:tpack_pack_count_more")

local a = table.pack()
assert(a.n == 0 and a[1] == nil)

a = table.pack(nil, nil, 3, nil)
assert(a.n == 4 and a[1] == nil and a[2] == nil and a[3] == 3 and a[4] == nil)

local function f(...)
  local x = table.pack(...)
  return x.n, x[1], x[2], x[3], x[4]
end

local n, a1, a2, a3, a4 = f(10, nil, "x")
assert(n == 3 and a1 == 10 and a2 == nil and a3 == "x" and a4 == nil)

print("ok")
