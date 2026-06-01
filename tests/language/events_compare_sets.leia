print("case:events_compare_sets")

mt := {}

func rawSet(x) {
  y := {}
  for _, k := range pairs(x) {
    y[k] = 1
  }
  return y
}

func Set(x) {
  return setmetatable(rawSet(x), mt)
}

mt.__lt = func(a, b) {
  for k, _ := range pairs(a) {
    if !b[k] { return false }
    b[k] = nil
  }
  return next(b) != nil
}

mt.__le = func(a, b) {
  for k, _ := range pairs(a) {
    if !b[k] { return false }
  }
  return true
}

assert(Set({1, 2, 3}) < Set({1, 2, 3, 4}))
assert(!(Set({1, 2, 3, 4}) < Set({1, 2, 3, 4})))
assert(Set({1, 2, 3, 4}) <= Set({1, 2, 3, 4}))
assert(Set({1, 2, 3, 4}) >= Set({1, 2, 3, 4}))
assert(!(Set({1, 3}) <= Set({3, 5})))
assert(!(Set({1, 3}) >= Set({3, 5})))

mt.__eq = func(a, b) {
  for k, _ := range pairs(a) {
    if !b[k] { return false }
    b[k] = nil
  }
  return next(b) == nil
}

s := Set({1, 3, 5})
assert(s == Set({3, 5, 1}))
assert(!rawequal(s, Set({3, 5, 1})))
assert(rawequal(s, s))
assert(Set({1, 3, 5, 1}) == rawSet({3, 5, 1}))
assert(rawSet({1, 3, 5, 1}) == Set({3, 5, 1}))
assert(Set({1, 3, 5}) != Set({3, 5, 1, 6}))

mt[Set({1, 3, 5})] = 1
assert(mt[Set({1, 3, 5})] == nil)

print("ok")
