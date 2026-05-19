print("case:nextvar_generic_for_multivalue_more")

func f(n, p) {
  t := {}
  for i := 1; i <= p; i++ {
    t[i] = i * 10
  }
  return func(_, n, ... ) {
    assert(select("#", ...) == 0)
    if n > 0 {
      n = n - 1
      return n, table.unpack(t)
    }
  }, nil, n
}

x := 0
for n, a := range f(5, 3) {
  x = x + 1
  assert(a == 10)
}
assert(x == 5)

print("ok")
