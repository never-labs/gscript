print("case:sort_pack_unpack")

a := table.pack()
assert(a[1] == nil && a.n == 0)
a = table.pack(table)
assert(a[1] == table && a.n == 1)
a = table.pack(nil, nil, nil, nil)
assert(a[1] == nil && a.n == 4)

print("ok")
