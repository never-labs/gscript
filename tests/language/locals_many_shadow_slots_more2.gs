print("case:locals_many_shadow_slots_more2")

total := 0
for i := 1; i <= 12; i++ {
  a, b, c := i, i + 1, i + 2
  if true {
    a, d := b + c, i * 2
    total = total + a + d
  }
  total = total + a + b + c
}

assert(total == 618)

func pack(a, b, c, d) {
  b := b || 0
  d := d || 0
  return a + b + c + d
}

assert(pack(1, nil, 3, nil) == 4)
assert(pack(4, 5, 6, 7) == 22)

print("ok")
