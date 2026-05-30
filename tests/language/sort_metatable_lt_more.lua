print("case:sort_metatable_lt_more")

local tt = {
  __lt = function(a, b)
    return a.val < b.val
  end,
}

local a = {}
for i = 1, 10 do
  a[i] = {val = 11 - i}
  setmetatable(a[i], tt)
end

table.sort(a)
for i = 2, #a do
  assert(not (a[i] < a[i - 1]))
end
for i = 1, 10 do
  assert(a[i].val == i)
end

print("ok")
