print("case:db_pcall_nested_errors")

bad_add := func() {
  return "joao" + 1
}

outer := func(n) {
  if n == 0 {
    return bad_add()
  }
  return outer(n - 1)
}

ok, msg := pcall(outer, 3)
assert(!ok && type(msg) == "string")

guards := func(f) {
  ok1 := pcall(f)
  ok2, val := pcall(func() { return pcall(f) })
  return ok1, ok2, val
}

a, b, c := guards(bad_add)
assert(a == false && b == true && c == false)

print("ok")
