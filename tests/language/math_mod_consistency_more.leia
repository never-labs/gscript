print("case:math_mod_consistency_more")

for i := -10; i <= 10; i++ {
  for j := -10; j <= 10; j++ {
    if j != 0 {
      assert((i + 0.0) % j == i % j)
    }
  }
}
assert(math.mininteger % math.mininteger == 0)
assert(math.maxinteger % math.maxinteger == 0)

print("ok")
