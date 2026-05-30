print("case:constructs_function_branches_more")

f := func(i) {
  if i < 10 { return "a" }
  if i < 20 { return "b" }
  if i < 30 { return "c" }
}
assert(f(3) == "a" && f(12) == "b" && f(26) == "c" && f(100) == nil)

f = func(i) {
  if i < 10 { return "a" }
  if i < 20 { return "b" }
  if i < 30 { return "c" }
  return 8
}
assert(f(3) == "a" && f(12) == "b" && f(26) == "c" && f(100) == 8)

a, b := nil, 23
x := {f(100) * 2 + 3 || a, a || b + 2}
assert(x[1] == 19 && x[2] == 25)
x = {f: 2 + 3 || a, a: b + 2}
assert(x.f == 5 && x.a == 25)

print("ok")
