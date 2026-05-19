print("case:nextvar_pairs_metamethod")

a := {}
func foo(e, i) {
  assert(e == a)
  if i <= 10 {
    return i + 1, i + 2
  }
}

mt := {}
mt.__pairs = func(x) {
  return foo, x, 0
}
setmetatable(a, mt)

i := 0
for k, v := range pairs(a) {
  i = i + 1
  assert(k == i && v == k + 1)
}

assert(i == 11)

print("ok")
