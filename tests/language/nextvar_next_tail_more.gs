print("case:nextvar_next_tail_more")

a := {}
for i := 1; i <= 1000; i++ {
  a[i] = i
  a[i - 1] = nil
}
assert(next(a, nil) == 1000 && next(a, 1000) == nil)

print("ok")
