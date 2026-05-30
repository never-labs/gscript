print("case:goto_flow_equivalent_more")

x := nil
func simple() {
  y := 12
  x = y
  x = x + 1
}
simple()
assert(x == 13)

func foo() {
  a := {}
  for pass := 1; pass <= 2; pass++ {
    a[#a + 1] = 3
    a[#a + 1] = 1
    a[#a + 1] = 2
    a[#a + 1] = 5
    a[#a + 1] = 4
    if pass == 1 { a[#a + 1] = true }
  }
  assert(a[1] == 3 && a[2] == 1 && a[3] == 2 && a[4] == 5 && a[5] == 4)
  assert(a[6] == true && a[7] == 3 && a[8] == 1 &&
         a[9] == 2 && a[10] == 5 && a[11] == 4)
}
foo()

print("ok")
