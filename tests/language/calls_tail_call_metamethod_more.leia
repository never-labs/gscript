print("case:calls_tail_call_metamethod_more")

func foo(x, ...) {
  a := {...}
  return x, a[1], a[2]
}

t := setmetatable({}, {__call: foo})

func call_table(x) {
  return t(10, x)
}

a, b, c := call_table(100)
assert(a == t && b == 10 && c == 100)

n := 12
done := nil
done = func() {
  if n == 0 {
    return 1023
  }
  n = n - 1
  return done()
}

u := done
for i := 1; i <= 8; i++ {
  u = setmetatable({}, {__call: u})
}

assert(u() == 1023)

print("ok")
