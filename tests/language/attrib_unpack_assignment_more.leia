print("case:attrib_unpack_assignment_more")

func f(n) {
  x := {}
  for i := 1; i <= n; i = i + 1 {
    x[i] = i
  }
  return table.unpack(x)
}

a, b, c := nil, nil, nil
a, b = 0, f(1)
assert(a == 0 && b == 1)
a, b = 0, f(1)
assert(a == 0 && b == 1)
a, b, c = 0, 5, f(4)
assert(a == 0 && b == 5 && c == 1)
a, b, c = 0, 5, f(0)
assert(a == 0 && b == 5 && c == nil)

print("ok")
