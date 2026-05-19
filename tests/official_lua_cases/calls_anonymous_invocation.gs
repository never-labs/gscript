print("case:calls_anonymous_invocation")

a := nil
func(x) { a = x }(23)
assert(a == 23 && func(x) { return x * 2 }(20) == 40)

f := func(x, y) {
  return x + y, x - y
}

p, q := f(9, 4)
assert(p == 13 && q == 5)

print("ok")
