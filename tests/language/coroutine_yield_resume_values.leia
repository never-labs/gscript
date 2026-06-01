print("case:coroutine_yield_resume_values")

co := coroutine.wrap(func() {
  a := coroutine.yield("first")
  return "done", a
})

assert(co() == "first")
a, b := co("arg")
assert(a == "done" && b == "arg")

count := 0
gen := coroutine.wrap(func() {
  for i := 1; i <= 3; i++ {
    count = count + i
    coroutine.yield(count)
  }
})

assert(gen() == 1)
assert(gen() == 3)
assert(gen() == 6)

print("ok")
