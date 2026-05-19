print("case:closure_loop_mutation_more2")

makers := {}
for i := 1; i <= 5; i++ {
  base := i * 10
  makers[i] = func(delta) {
    base = base + delta
    return base
  }
}

assert(makers[1](1) == 11)
assert(makers[1](4) == 15)
assert(makers[2](2) == 22)
assert(makers[5](-5) == 45)
assert(makers[2](3) == 25)

func outer(x) {
  count := x
  return func() {
    count = count + 1
    return func() {
      return count
    }
  }
}

nextfn := outer(7)
a := nextfn()
b := nextfn()
assert(a() == 9 && b() == 9)

print("ok")
