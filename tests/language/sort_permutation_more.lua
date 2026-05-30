print("case:sort_permutation_more")

local function check(a)
  for n = #a, 2, -1 do
    assert(not (a[n] < a[n - 1]))
  end
end

local seen = 0
local function perm(s, n)
  n = n or #s
  if n == 1 then
    local t = {table.unpack(s)}
    table.sort(t)
    check(t)
    seen = seen + 1
  else
    for i = 1, n do
      s[i], s[n] = s[n], s[i]
      perm(s, n - 1)
      s[i], s[n] = s[n], s[i]
    end
  end
end

perm{1, 2, 3, 4}
assert(seen == 24)
seen = 0
perm{1, 2, 3, 3, 5}
assert(seen == 120)

print("ok")
