print("case:constructs_silly_loop_scope_more")

for guard := 1; guard <= 1; guard++ { if true { break } }
if false { error("not here") }

func shadow(x) {
  a := nil
  x = {a: 1}
  x = {x: 1}
  x = {G: 1}
  return a, x.G
}
aa, gg := shadow({})
assert(aa == nil && gg == 1)

func f(i) {
  for guard := 1; guard <= 20; guard++ {
    if i > 0 { i = i - 1 } else { return }
  }
  error("loop guard")
}

func g(i) {
  for guard := 1; guard <= 20; guard++ {
    if i > 0 { i = i - 1 } else { return }
  }
  error("loop guard")
}

f(10); g(10)

print("ok")
