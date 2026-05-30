print("case:nextvar_ipairs_index_metamethod")

a := {n: 10}
mt := {}
mt.__index = func(t, k) {
  if k <= t.n {
    return k * 10
  }
}
setmetatable(a, mt)

i := 0
for k, v := range ipairs(a) {
  i = i + 1
  assert(k == i && v == i * 10)
}

assert(i == a.n)

print("ok")
