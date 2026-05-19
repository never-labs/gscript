print("case:math_random_range_more")

random := math.random
for i := 1; i <= 100; i++ {
  t := random()
  assert(0 <= t && t < 1)
}
for i := 1; i <= 100; i++ {
  r := random(6)
  assert(math.type(r) == "integer" && 1 <= r && r <= 6)
}
for i := 1; i <= 100; i++ {
  r := random(-10, 10)
  assert(math.type(r) == "integer" && -10 <= r && r <= 10)
}

print("ok")
