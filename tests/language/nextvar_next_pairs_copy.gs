print("case:nextvar_next_pairs_copy")

func checknext(a) {
  b := {}
  k, v := next(a)
  for k {
    b[k] = v
    k, v = next(a, k)
  }
  for kk, vv := range pairs(b) {
    assert(a[kk] == vv)
  }
  for kk, vv := range pairs(a) {
    assert(b[kk] == vv)
  }
}

a := {1, x: 1, y: 2, z: 3}
b := {1, 2, x: 1, y: 2, z: 3}
c := {1, 2, 3, x: 1, y: 2, z: 3}
d := {1, 2, 3, 4, x: 1, y: 2, z: 3}
e := {1, 2, 3, 4, 5, x: 1, y: 2, z: 3}

checknext(a)
checknext(b)
checknext(c)
checknext(d)
checknext(e)

print("ok")
