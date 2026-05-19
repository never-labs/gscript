print("case:calls_incorrect_args_more")

rawget({}, "x", 1)
rawset({}, "x", 1, 2)
assert(math.sin(1, 2) == math.sin(1))
a := {10, 9, 8, 4, 19, 23, 0, 0}
table.sort(a, func(x, y) { return x < y }, "extra arg")
for i := #a; i >= 2; i-- { assert(!(a[i] < a[i - 1])) }

assert(func() { return nil }(4) == nil)
assert(func() { a := nil; return a }(4) == nil)
assert(func(a) { return a }() == nil)

print("ok")
