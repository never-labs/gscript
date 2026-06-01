print("case:coroutine_wrap_sieve_more")

func gen(n) {
  return coroutine.wrap(func() {
    for i := 2; i <= n; i++ {
      coroutine.yield(i)
    }
  })
}

func filter(p, g) {
  return coroutine.wrap(func() {
    for {
      n := g()
      if n == nil { return }
      if math.fmod(n, p) != 0 { coroutine.yield(n) }
    }
  })
}

x := gen(80)
a := {}
for {
  n := x()
  if n == nil { break }
  table.insert(a, n)
  x = filter(n, x)
}

assert(#a == 22 && a[#a] == 79)

print("ok")
