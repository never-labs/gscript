print("case:coroutine_self_resume_status_more")

func co_func() {
  coroutine.yield(10)
  coroutine.yield(23)
  return 10
}

co := coroutine.create(co_func)
assert(coroutine.status(co) == "suspended")

a, b, c := coroutine.resume(co)
assert(a == true && b == 10 && c == nil)
assert(coroutine.status(co) == "suspended")

a, b = coroutine.resume(co)
assert(a == true && b == 23)

a, b = coroutine.resume(co)
assert(a == true && b == 10)
assert(coroutine.status(co) == "dead")

assert(coroutine.resume(co) == false)
assert(coroutine.resume(co) == false)

print("ok")
