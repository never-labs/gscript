print("case:calls_recursion_error_more")

err_on_n := func(n) {
  if n == 0 { error("boom") } else { err_on_n(n - 1) }
}

dummy := func(n) {
  if n > 0 {
    assert(!pcall(err_on_n, n))
    dummy(n - 1)
  }
}

dummy(10)

deep := func(n) {
  if n > 0 { return deep(n - 1) }
  return 101
}
assert(deep(500) == 101)

print("ok")
