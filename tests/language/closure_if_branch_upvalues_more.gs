print("case:closure_if_branch_upvalues_more")

a := {}
for i := 1; i <= 10; i++ {
  if i % 3 == 0 {
    y := 0
    a[i] = func(x) { t := y; y = x; return t }
  } else {
    if i % 3 == 1 {
      y := 1
      a[i] = func(x) { t := y; y = x; return t }
    } else {
      if i % 3 == 2 {
        y := 2
        a[i] = func(x) { t := y; y = x; return t }
      }
    }
  }
}

for i := 1; i <= 10; i++ {
  assert(a[i](i * 10) == i % 3 && a[i]() == i * 10)
}

print("ok")
