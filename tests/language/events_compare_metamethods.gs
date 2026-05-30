print("case:events_compare_metamethods")

cap := {}

func lt(a, b) {
  cap[#cap + 1] = {"lt", a, b}
  return true
}

func le(a, b) {
  cap[#cap + 1] = {"le", a, b}
  return true
}

func eq(a, b) {
  cap[#cap + 1] = {"eq", a, b}
  return true
}

mt := {}
mt.__lt = lt
mt.__le = le
mt.__eq = eq

a := {}
setmetatable(a, mt)
b := {}
setmetatable(b, mt)

assert(5.0 > a)
last := cap[#cap]
assert(last[1] == "lt" && last[2] == a && last[3] == 5.0)

assert(a >= 10)
last = cap[#cap]
assert(last[1] == "le" && last[2] == 10 && last[3] == a)

assert(a <= -10.0)
last = cap[#cap]
assert(last[1] == "le" && last[2] == a && last[3] == -10.0)

assert(a < -10)
last = cap[#cap]
assert(last[1] == "lt" && last[2] == a && last[3] == -10)

assert(a == b)
last = cap[#cap]
assert(last[1] == "eq" && last[2] == a && last[3] == b)

assert(!(a != b))
last = cap[#cap]
assert(last[1] == "eq" && last[2] == a && last[3] == b)

print("ok")
