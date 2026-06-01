print("case:sort_pack_nil_counts_more")

a := table.pack(1, nil, 3, nil, 5)
assert(a.n == 5 && a[1] == 1 && a[2] == nil && a[3] == 3 && a[4] == nil && a[5] == 5)
b := table.pack("a", "b", nil)
assert(b.n == 3 && b[1] == "a" && b[2] == "b" && b[3] == nil)

print("ok")
