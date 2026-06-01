print("case:coroutine_recursive_yield_more")

pf := nil
pf = func(n, i) {
  coroutine.yield(n)
  pf(n * i, i + 1)
}

f := coroutine.wrap(pf)
s := 1
for i := 1; i <= 10; i++ {
  assert(f(1, 1) == s)
  s = s * i
}

print("ok")
