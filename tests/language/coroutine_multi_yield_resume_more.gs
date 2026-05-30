print("case:coroutine_multi_yield_resume_more")

co := coroutine.create(func() {
  a, b := coroutine.yield("a", 10, nil, "z")
  return "done", a, b
})

ok, a, b, c, d := coroutine.resume(co)
assert(ok && a == "a" && b == 10 && c == nil && d == "z")

ok, a, b, c = coroutine.resume(co, 20, "resume")
assert(ok && a == "done" && b == 20 && c == "resume")

print("ok")
