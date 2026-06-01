print("case:math_tonumber_invalid_more2")

f := func(...) {
  if select("#", ...) == 1 {
    return ...
  }
  return "***"
}

assert(!f(tonumber("fFfa", 15)))
assert(!f(tonumber("099", 8)))
assert(!f(tonumber("", 8)))
assert(!f(tonumber("  ", 9)))
assert(!f(tonumber("0xf", 10)))
assert(!f(tonumber("inf")))
assert(!f(tonumber(" INF ")))
assert(!f(tonumber("Nan")))
assert(!f(tonumber("nan")))
assert(!f(tonumber("  ")))
assert(!f(tonumber("")))
assert(!f(tonumber("1  a")))
assert(!f(tonumber("1  a", 2)))
assert(!f(tonumber("e1")))
assert(!f(tonumber("e  1")))
assert(!f(tonumber(" 3.4.5 ")))

print("ok")
