print("case:calls_inline_function_args")

ok, got := pcall(func(x) { return x + 7 }, 5)
assert(ok && got == 12)

seen := 0
values := {4, 1, 3, 2}
table.sort(values, func(a, b) {
  seen = seen + 1
  return a > b
})
assert(seen > 0)
assert(table.concat(values, ",") == "4,3,2,1")

func f(cb, x) {
  return cb(x), cb(x + 1)
}
a, b := f(func(n) { return n * 3 }, 6)
assert(a == 18 && b == 21)

print("ok")
