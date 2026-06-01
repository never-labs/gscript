print("case:calls_return_values")

func f() {
  return 1, 2, 3
}

a, b, c := f()
assert(a == 1 && b == 2 && c == 3)

func g() {
  f()
  return
}

assert(g() == nil)

func h() {
  return nil || f()
}

a, b = h()
assert(a == 1 && b == nil)

print("ok")
