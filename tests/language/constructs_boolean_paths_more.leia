print("case:constructs_boolean_paths_more")

func f(a, b, c, d, e) {
  x := a >= b || c || (d && e) || nil
  return x
}

func g(a, b, c, d, e) {
  if !(a >= b || c || d && e || nil) { return 0 } else { return 1 }
}

func h(a, b, c, d, e) {
  if a >= b || c || (d && e) || nil { return 1 }
  return 0
}

assert(f(2, 1) == true && g(2, 1) == 1 && h(2, 1) == 1)
assert(f(1, 2, "a") == "a" && g(1, 2, "a") == 1 && h(1, 2, "a") == 1)
assert(f(1, 2, nil, 1, "x") == "x" && g(1, 2, nil, 1, "x") == 1 && h(1, 2, nil, 1, "x") == 1)
assert(f(1, 2, nil, nil, "x") == nil && g(1, 2, nil, nil, "x") == 0 && h(1, 2, nil, nil, "x") == 0)
assert(f(1, 2, nil, 1, nil) == nil && g(1, 2, nil, 1, nil) == 0 && h(1, 2, nil, 1, nil) == 0)

assert(1 && 2 < 3 == true && 2 < 3 && "a" < "b" == true)
x := 2 < 3 && !3; assert(x == false)
x = 2 < 1 || (2 > 1 && "a"); assert(x == "a")

func inner() {
  a := nil
  if nil { a = 1 } else { a = 2 }
  assert(a == 2)
}
inner()

print("ok")
