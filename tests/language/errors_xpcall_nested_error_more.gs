print("case:errors_xpcall_nested_error_more")

func f(x) {
  if x == 0 {
    error("a\n")
  } else {
    aux := func() {
      return f(x - 1)
    }
    a, b := xpcall(aux, aux)
    return a, b
  }
}

ok, msg := f(3)
assert(ok && msg == true)

res, handled := xpcall(func() {
  error({code: 41})
}, func(m) {
  return {code: m.code + 1}
})

assert(!res && handled.code == 42)

print("ok")
