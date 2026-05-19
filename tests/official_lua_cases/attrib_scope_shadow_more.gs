print("case:attrib_scope_shadow_more")

a := "outer"
func innerBlock() {
  a := "inner"
  f := func() {
    return a
  }
  assert(f() == "inner")
  a = "changed"
  assert(f() == "changed")
}
innerBlock()
assert(a == "outer")

func make() {
  x := 10
  return func(delta) {
    x = x + delta
    return x
  }
}

c1 := make()
c2 := make()
assert(c1(1) == 11)
assert(c1(2) == 13)
assert(c2(5) == 15)

print("ok")
