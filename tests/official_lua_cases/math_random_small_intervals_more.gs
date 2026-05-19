print("case:math_random_small_intervals_more")

random := math.random
func aux(x1, x2) {
  mark := {}
  count := 0
  for i := 1; i <= 1000; i++ {
    t := random(x1, x2)
    assert(x1 <= t && t <= x2)
    if !mark[t] {
      mark[t] = true
      count = count + 1
      if count == x2 - x1 + 1 { return }
    }
  }
  assert(false)
}
aux(-10, 0)
aux(1, 6)
aux(1, 2)
aux(-10, -10)
aux(math.mininteger, math.mininteger)
aux(math.maxinteger, math.maxinteger)

print("ok")
