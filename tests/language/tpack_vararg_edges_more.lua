print("case:tpack_vararg_edges_more")

local function check(...)
  local p = table.pack(...)
  assert(p.n == 6)
  assert(p[1] == "a" and p[2] == nil and p[3] == false)
  assert(p[4] == 4 and p[5] == nil and p[6] == "z")

  assert(select("#", ...) == 6)
  assert(select(1, ...) == "a")
  assert(select(3, ...) == false)
  assert(select(-1, ...) == "z")
  assert(select(-3, ...) == 4)

  local a, b, c, d = table.unpack(p, 3, 6)
  assert(a == false and b == 4 and c == nil and d == "z")
end

check("a", nil, false, 4, nil, "z")

print("ok")
