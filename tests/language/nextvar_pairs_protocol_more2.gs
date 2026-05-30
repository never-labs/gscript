print("case:nextvar_pairs_protocol_more2")

a := {}
x, y, z := pairs(a)
assert(type(x) == "function" && y == a && z == nil)
func foo(e, i) {
  assert(e == a)
  if i <= 10 { return i + 1, i + 2 }
}
setmetatable(a, {__pairs: func(x) { return foo, x, 0 }})
i := 0
for k, v := range pairs(a) {
  i = i + 1
  assert(k == i && v == k + 1)
}

print("ok")
