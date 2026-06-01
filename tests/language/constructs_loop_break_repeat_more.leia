print("case:constructs_loop_break_repeat_more")

n := 100
i := 3
t := {}
a := nil
for guard := 1; guard <= 2; guard++ {
  if a { break }
  a = 0
  for i := 1; i <= n; i++ {
    for j := i; j >= 1; j-- {
      a = a + 1
      t[j] = 1
    }
  }
}
assert(a == n * (n + 1) / 2 && i == 3)
assert(t[1] && t[n] && !t[0] && !t[n + 1])

func f(b) {
  x := 1
  for guard := 1; guard <= 20; guard++ {
    if b == 1 { x = 10; break }
    if b == 2 { x = 20; break }
    if b == 3 { x = 30 } else { x = x + 1 }
    if x >= 12 { break }
  }
  return x
}
assert(f(1) == 10 && f(2) == 20 && f(3) == 30 && f(4) == 12)

print("ok")
