print("case:constructs_loop_break_count")

for i := 1; i <= 1000; i++ {
  break
}

n := 100
i := 3
t := {}
a := nil

for !a {
  a = 0
  for j := 1; j <= n; j++ {
    for k := j; k >= 1; k-- {
      a = a + 1
      t[k] = 1
    }
  }
}

assert(a == n * (n + 1) / 2 && i == 3)
assert(t[1] && t[n] && !t[n + 1])

print("ok")
