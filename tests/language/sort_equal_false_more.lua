print("case:sort_equal_false_more")

a = {}
for i = 1, 32 do a[i] = false end
table.sort(a, function (x, y) return nil end)
for i, v in pairs(a) do assert(v == false) end
assert(#a == 32)

table.sort{}

print("ok")
