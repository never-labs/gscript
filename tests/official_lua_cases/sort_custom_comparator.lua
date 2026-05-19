print("case:sort_custom_comparator")

local function check(a, f)
  f = f or function(x, y) return x < y end
  for n = #a, 2, -1 do
    assert(not f(a[n], a[n - 1]))
  end
end

local a = {5, 1, 4, 2, 3}
local count = 0
table.sort(a, function(x, y)
  count = count + 1
  return y < x
end)
assert(count > 0)
check(a, function(x, y) return y < x end)
assert(table.concat(a, ",") == "5,4,3,2,1")

table.sort({})

a = {false, false, false, false}
table.sort(a, function(x, y) return nil end)
for i, v in pairs(a) do
  assert(v == false)
end

local words = {"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep",
               "Oct", "Nov", "Dec"}
table.sort(words)
check(words)

print("ok")
