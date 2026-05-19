print("case:math_fmod_more2")

for i := -6; i <= 6; i++ {
  for j := -6; j <= 6; j++ {
    if j != 0 {
      mi := math.fmod(i, j)
      mf := math.fmod(i + 0.0, j)
      assert(mi == mf)
      assert(math.type(mi) == "integer" && math.type(mf) == "float")
    }
  }
}
assert(math.fmod(math.mininteger, math.mininteger) == 0)
assert(math.fmod(math.maxinteger, math.maxinteger) == 0)

print("ok")
