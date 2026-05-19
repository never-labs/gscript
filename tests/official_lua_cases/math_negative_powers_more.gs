print("case:math_negative_powers_more")

func eq(a, b) { return a == b || math.abs(a - b) <= 1e-11 }

assert(math.pow(2, -3) == 1 / math.pow(2, 3))
assert(eq(math.pow(-3, -3), 1 / math.pow(-3, 3)))
for i := -3; i <= 3; i++ {
  for j := -3; j <= 3; j++ {
    if i != 0 || j > 0 {
      assert(eq(math.pow(i, j), 1 / math.pow(i, -j)))
    }
  }
}

print("ok")
