print("case:locals_repeat_shadow_more2")

func nilarg(x) {
  x = nil
  y := nil
  return x, y
}

a, b := nilarg(10)
assert(a == nil && b == nil)

func check_shadow() {
  i := 10
  func inner100() { i := 100; assert(i == 100) }
  func inner1000() { i := 1000; assert(i == 1000) }
  inner100()
  inner1000()
  assert(i == 10)
  if i != 10 {
    i := 20
    assert(i == 20)
  } else {
    i := 30
    assert(i == 30)
  }
  assert(i == 10)
}
check_shadow()

rb := 10
ra := nil
for {
  rb := 2
  ra = 1
  assert(ra + 1 == rb)
  if ra + rb == 3 { break }
}
assert(rb == 10)

print("ok")
