print("case:nextvar_checknext_more")

func checknext(a) {
  b := {}
  k, v := next(a)
  for k {
    b[k] = v
    k, v = next(a, k)
  }
  for k, v := range pairs(b) { assert(a[k] == v) }
  for k, v := range pairs(a) { assert(b[k] == v) }
}

checknext({1, x: 1, y: 2, z: 3})
checknext({1, 2, x: 1, y: 2, z: 3})
checknext({1, 2, 3, x: 1, y: 2, z: 3})
checknext({1, 2, 3, 4, x: 1, y: 2, z: 3})
checknext({1, 2, 3, 4, 5, x: 1, y: 2, z: 3})

print("ok")
