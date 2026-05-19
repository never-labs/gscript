print("case:sort_reverse_closure_more")

local a = {7, 1, 5, 2, 9, 4, 8, 3, 6}
local comparisons = 0
table.sort(a, function (x, y)
  comparisons = comparisons + 1
  return y < x
end)

for i = 2, #a do
  assert(a[i - 1] >= a[i])
end
assert(a[1] == 9 and a[#a] == 1 and comparisons > 0)

print("ok")
