print("case:nextvar_next_sparse_tail")

a := {}
for i := 1; i <= 1000; i = i + 1 {
  a[i] = i
  a[i - 1] = nil
}

k, v := next(a, nil)
assert(k == 1000 && v == 1000)
assert(next(a, 1000) == nil)

assert(next({}) == nil)
empty := {}
assert(next(empty, nil) == nil)

print("ok")
