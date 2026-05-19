print("case:math_fmod_integer_more")

func eqT(a, b) {
  return a == b && math.type(a) == math.type(b)
}

for i := -6; i <= 6; i++ {
  for j := -6; j <= 6; j++ {
    if j != 0 {
      mi := math.fmod(i, j)
      mf := math.fmod(i + 0.0, j)
      assert(mi == mf)
      assert(math.type(mi) == "integer" && math.type(mf) == "float")
      if (i >= 0 && j >= 0) || (i <= 0 && j <= 0) || mi == 0 {
        assert(eqT(mi, i % j))
      }
    }
  }
}

s, err := pcall(math.fmod, 3, 0)
assert(!s && string.find(err, "zero"))

print("ok")
