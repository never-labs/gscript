print("case:closure_for_control")

a := {}
for i := 1; i <= 10; i++ {
  current := i
  cell := {}
  cell.set = func(x) {
    current = x
  }
  cell.get = func() {
    return current
  }
  a[i] = cell
  if i == 3 {
    break
  }
}

assert(a[4] == nil)
a[1].set(10)
assert(a[2].get() == 2)
a[2].set("a")
assert(a[3].get() == 3)
assert(a[2].get() == "a")

print("ok")
