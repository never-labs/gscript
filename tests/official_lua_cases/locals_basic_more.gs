print("case:locals_basic_more")

func f1(x) { x = nil; return x }
assert(f1(10) == nil)
func f2() { x := nil; return x }
assert(f2(10) == nil)
func f3(x) { x = nil; y := nil; return x, y }
assert(f3(10) == nil && select(2, f3(20)) == nil)

func scope() {
  i := 10
  func inner1() { i := 100; assert(i == 100) }
  func inner2() { i := 1000; assert(i == 1000) }
  inner1()
  inner2()
  assert(i == 10)
  if i != 10 {
    i := 20
  } else {
    i := 30
    assert(i == 30)
  }
}
scope()

print("ok")
