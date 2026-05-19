print("case:events_compare_ordering")

mt := {}

mt.__lt = func(a, b, c) {
  assert(c == nil)
  if type(a) == "table" { a = a.x }
  if type(b) == "table" { b = b.x }
  return a < b, "ignored"
}

mt.__le = func(a, b, c) {
  assert(c == nil)
  if type(a) == "table" { a = a.x }
  if type(b) == "table" { b = b.x }
  return a <= b, "ignored"
}

mt.__eq = func(a, b, c) {
  assert(c == nil)
  if type(a) == "table" { a = a.x }
  if type(b) == "table" { b = b.x }
  return a == b, "ignored"
}

func Op(x) {
  r := {}
  r.x = x
  return setmetatable(r, mt)
}

assert(!(Op(1) < Op(1)))
assert(Op(1) < Op(2))
assert(!(Op(2) < Op(1)))
assert(!(1 < Op(1)))
assert(Op(1) < 2)
assert(!(2 < Op(1)))

assert(Op(1) <= Op(1))
assert(Op(1) <= Op(2))
assert(!(Op(2) <= Op(1)))
assert(Op("a") <= Op("a"))
assert(Op("a") <= Op("b"))
assert(!(Op("b") <= Op("a")))

assert(!(Op(1) > Op(1)))
assert(!(Op(1) > Op(2)))
assert(Op(2) > Op(1))
assert(1 >= Op(1))
assert(!(1 >= Op(2)))
assert(Op(2) >= 1)

assert(Op(10) == Op(10))
assert(!(Op(10) == Op(11)))
assert(Op("x") != Op("y"))
assert(!(Op("x") != Op("x")))

print("ok")
