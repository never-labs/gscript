print("case:vararg_call_unpack_more")

func c12(...) {
  x := table.pack(...)
  res := (x.n == 2 && x[1] == 1 && x[2] == 2)
  if res { res = 55 }
  return res, 2
}

call := func(f, args) {
  if args.n != nil { return f(table.spread(args, 1, args.n)) }
  return f(table.spread(args))
}

a, b := call(c12, {1, 2, n: 2})
assert(a == 55 && b == 2)
a = call(c12, {1, 2, n: 1})
assert(!a)

lim := 20
vals := {}
for i := 1; i <= lim; i++ { vals[i] = i + 0.3 }

func fixed(a, b, c, d, ...) {
  more := table.pack(...)
  assert(a == 1.3 && b == 2.3 && c == 3.3 && d == 4.3)
  assert(more[1] == 5.3 && more[lim - 4] == lim + 0.3 && more[lim - 3] == nil)
}

call(fixed, vals)

for i := 1; i <= lim; i++ { vals[i] = i }
assert(call(math.max, vals) == lim)

print("ok")
