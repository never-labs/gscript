print("case:events_partial_order_more")

t := {}
func rawSet(x) {
  y := {}
  for _, k := range pairs(x) { y[k] = 1 }
  return y
}
func Set(x) { return setmetatable(rawSet(x), t) }
t.__lt = func(a, b) {
  for k := range pairs(a) {
    if !b[k] { return false }
    b[k] = nil
  }
  return next(b) != nil
}
t.__le = func(a, b) {
  for k := range pairs(a) {
    if !b[k] { return false }
  }
  return true
}
assert(Set({1,2,3}) < Set({1,2,3,4}))
assert(!(Set({1,2,3,4}) < Set({1,2,3,4})))
assert(Set({1,2,3,4}) <= Set({1,2,3,4}))
assert(Set({1,2,3,4}) >= Set({1,2,3,4}))
assert(!(Set({1,3}) <= Set({3,5})))
assert(!(Set({1,3}) >= Set({3,5})))

print("ok")
