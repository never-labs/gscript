print("case:nextvar_next_pairs_copy")

local function checknext(a)
  local b = {}
  local k, v = next(a)
  while k do
    b[k] = v
    k, v = next(a, k)
  end
  for kk, vv in pairs(b) do
    assert(a[kk] == vv)
  end
  for kk, vv in pairs(a) do
    assert(b[kk] == vv)
  end
end

local a = {1, x = 1, y = 2, z = 3}
local b = {1, 2, x = 1, y = 2, z = 3}
local c = {1, 2, 3, x = 1, y = 2, z = 3}
local d = {1, 2, 3, 4, x = 1, y = 2, z = 3}
local e = {1, 2, 3, 4, 5, x = 1, y = 2, z = 3}

checknext(a)
checknext(b)
checknext(c)
checknext(d)
checknext(e)

print("ok")
