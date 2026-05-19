print("case:sort_order")

local function check (a, f)
  f = f or function (x, y) return x < y end
  for n = #a, 2, -1 do
    assert(not f(a[n], a[n - 1]))
  end
end

local a = {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep",
           "Oct", "Nov", "Dec"}
table.sort(a)
check(a)

local function perm (s, n)
  n = n or #s
  if n == 1 then
    local t = {table.unpack(s)}
    table.sort(t)
    check(t)
  else
    for i = 1, n do
      s[i], s[n] = s[n], s[i]
      perm(s, n - 1)
      s[i], s[n] = s[n], s[i]
    end
  end
end

perm{}
perm{1}
perm{1,2}
perm{1,2,3}
perm{2,2,3,4}

print("ok")
