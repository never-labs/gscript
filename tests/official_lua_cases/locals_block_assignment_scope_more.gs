print("case:locals_block_assignment_scope_more")

outer := 10
a := nil
if true {
  outer := nil
  a, outer = 1, 2
  assert(a + 1 == outer)
}
assert(a == 1 && outer == 10)

if true {
  i := 2
  p := 4
  for j := -3; j <= 3; j++ {
    a := j
    a = a + (p - j)
    assert(a == 2 ** i)
    b := -j
    c := b - (p - j)
    assert(c == -(2 ** i))
  }
}

print("ok")
