print("case:coroutine_tail_yield_more")

x := nil

func foo(i) {
  return coroutine.yield(i)
}

f := coroutine.wrap(func() {
  for i := 1; i <= 10; i++ {
    assert(foo(i) == x)
  }
  return "a"
})

for i := 1; i <= 10; i++ {
  x = i
  assert(f(i) == i)
}

x = "xuxu"
assert(f("xuxu") == "a")

print("ok")
