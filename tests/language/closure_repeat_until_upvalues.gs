print("case:closure_repeat_until_upvalues")

a := {}
i := 1

for guard := 1; guard <= 20; guard = guard + 1 {
  x := i
  a[i] = func() {
    i = x + 1
    return x
  }
  if i > 10 || a[i]() != x { break }
}

assert(i == 11 && a[1]() == 1 && a[3]() == 3 && i == 4)

print("ok")
