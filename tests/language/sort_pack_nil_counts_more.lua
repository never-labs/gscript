print("case:sort_pack_nil_counts_more")

local a = table.pack(1, nil, 3, nil, 5)
assert(a.n == 5 and a[1] == 1 and a[2] == nil and a[3] == 3 and a[4] == nil and a[5] == 5)
local b = table.pack("a", "b", nil)
assert(b.n == 3 and b[1] == "a" and b[2] == "b" and b[3] == nil)

print("ok")
