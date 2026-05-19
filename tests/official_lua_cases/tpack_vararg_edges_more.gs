print("case:tpack_vararg_edges_more")

func check(...) {
  p := table.pack(...)
  assert(p.n == 6)
  assert(p[1] == "a" && p[2] == nil && p[3] == false)
  assert(p[4] == 4 && p[5] == nil && p[6] == "z")

  assert(select("#", ...) == 6)
  assert(select(1, ...) == "a")
  assert(select(3, ...) == false)
  assert(select(-1, ...) == "z")
  assert(select(-3, ...) == 4)

  a, b, c, d := table.unpack(p, 3, 6)
  assert(a == false && b == 4 && c == nil && d == "z")
}

check("a", nil, false, 4, nil, "z")

print("ok")
