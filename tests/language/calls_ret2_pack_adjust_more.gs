print("case:calls_ret2_pack_adjust_more")

func unlpack(t, i) {
  i = i || 1
  if i <= #t {
    return t[i], unlpack(t, i + 1)
  }
}

func equaltab(t1, t2) {
  assert(#t1 == #t2)
  for i := 1; i <= #t1; i++ {
    assert(t1[i] == t2[i])
  }
}

func pack(...) {
  return table.pack(...)
}

a, b, c, d := unlpack({1, 2, 3})
assert(a == 1 && b == 2 && c == 3 && d == nil)

t := {unlpack({1, 2, 3, 4})}
equaltab(t, {1, 2, 3, 4})

p := pack(unlpack({1, 2, 3, 4}))
assert(p.n >= 1 && p[1] == 1)

print("ok")
