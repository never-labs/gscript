print("case:nextvar_numeric_for_fractional_more")

func count(a, b, c) {
  n := 0
  last := nil
  if c > 0 {
    for i := a; i <= b; i += c { n = n + 1; last = i }
  } else {
    for i := a; i >= b; i += c { n = n + 1; last = i }
  }
  return n, last
}

n, last := count(0.5, 2.0, 0.5)
assert(n == 4 && last == 2.0)
n, last = count(2.0, 0.5, -0.5)
assert(n == 4 && last == 0.5)
n, last = count(-1.25, -0.25, 0.25)
assert(n == 5 && last == -0.25)
n, last = count(-0.25, -1.25, -0.25)
assert(n == 5 && last == -1.25)

print("ok")
