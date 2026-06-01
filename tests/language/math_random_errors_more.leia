print("case:math_random_errors_more")

assert(!pcall(math.random, 1, 2, 3))
assert(!pcall(math.random, 2, 1))
assert(!pcall(math.random, 10, -10))

for i := 1; i <= 20; i++ {
  r := math.random(1, 6)
  assert(1 <= r && r <= 6 && math.type(r) == "integer")
}

print("ok")
