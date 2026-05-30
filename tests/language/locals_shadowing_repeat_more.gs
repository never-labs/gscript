print("case:locals_shadowing_repeat_more")

func f(a) {
  _1 := nil; _2 := nil; _3 := nil; _4 := nil; _5 := nil
  _6 := nil; _7 := nil; _8 := nil; _9 := nil; _10 := nil
  x := 3
  b := a
  c := a; d := b
  if d == b {
    x := "q"
    x = b
    assert(x == 2)
  } else {
    assert(nil)
  }
  assert(x == 3)
  localf := 10
  assert(localf == 10 && c == 2)
}

b := 10
a := nil
for guard := 1; guard <= 3; guard++ {
  b := nil
  a = 1; b = 2
  assert(a + 1 == b)
  if a + b == 3 { break }
}

x := 1
assert(x == 1)
f(2)
assert(type(f) == "function")

print("ok")
