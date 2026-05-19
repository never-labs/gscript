print("case:tpack_pack_count_more")

a := table.pack()
assert(a.n == 0 && a[1] == nil)

a = table.pack(nil, nil, 3, nil)
assert(a.n == 4 && a[1] == nil && a[2] == nil && a[3] == 3 && a[4] == nil)

func f(...) {
  x := table.pack(...)
  return x.n, x[1], x[2], x[3], x[4]
}

n, a1, a2, a3, a4 := f(10, nil, "x")
assert(n == 3 && a1 == 10 && a2 == nil && a3 == "x" && a4 == nil)

print("ok")
