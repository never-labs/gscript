print("case:closure_break_upvalues")

X := nil
Y := nil
a := math.sin(0)

for guard := 1; guard <= 1; guard = guard + 1 {
  if a {
    b := 10
    X = func() { return b }
    if a { break }
  }
}

if true {
  b := 20
  Y = func() { return b }
}

assert(X() == 10 && Y() == 20)

print("ok")
