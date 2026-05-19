print("case:api_arith_metamethod_chain_more")

mt := {}
mt.__add = func(a, b) {
  return setmetatable({v: a.v + b.v}, mt)
}
mt.__mod = func(a, b) {
  return setmetatable({v: a.v % b.v}, mt)
}
mt.__unm = func(a) {
  return setmetatable({v: a.v * 2}, mt)
}

a := setmetatable({v: 4}, mt)
b := setmetatable({v: 8}, mt)
c := setmetatable({v: -3}, mt)

assert((a + b).v == 12)
assert((a % c).v == 4 % -3)
assert(((-b) + (a % c)).v == 14)

print("ok")
