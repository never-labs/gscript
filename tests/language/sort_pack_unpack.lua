print("case:sort_pack_unpack")

a = table.pack()
assert(a[1] == nil and a.n == 0)
a = table.pack(table)
assert(a[1] == table and a.n == 1)
a = table.pack(nil, nil, nil, nil)
assert(a[1] == nil and a.n == 4)

print("ok")
