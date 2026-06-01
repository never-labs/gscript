print("case:db_vararg_transfer_values")

prefix := func(...) {
  return 3, ...
}

a := {}
for i := 1; i <= 100; i++ {
  a[i] = i
}

x, y, z := table.unpack(a, 1, 3)
assert(x == 1 && y == 2 && z == 3)

p, q, r, s := prefix(1, 2, 3)
assert(p == 3 && q == 1 && r == 2 && s == 3)

collect := func(...) {
  packed := table.pack(...)
  assert(packed.n == select("#", ...))
  return packed
}

t := collect(20, 10, 0, nil, "x")
assert(t.n == 5 && t[1] == 20 && t[2] == 10 && t[3] == 0 && t[4] == nil && t[5] == "x")

print("ok")
