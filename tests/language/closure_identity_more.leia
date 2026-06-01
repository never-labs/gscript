print("case:closure_identity_more")

a := {}
for i := 1; i <= 5; i++ {
  a[i] = func(x) { return i }
}
assert(a[3] != a[4] && a[4] != a[5])

checkSame := func() {
  a := func(x) { return math.sin(x) }
  f := func() { return a }
  assert(f() == f())
}
checkSame()

print("ok")
