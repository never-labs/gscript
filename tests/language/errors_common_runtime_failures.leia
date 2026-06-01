print("case:errors_common_runtime_failures")

fails := func(f) {
  ok, err := pcall(f)
  assert(!ok && type(err) == "string")
}

fails(func() { return math.sin() })
fails(func() { return assert(false) })
fails(func() { return assert(nil) })
fails(func() {
  t := {}
  return t[#t] + 1
})

assert(pcall(tostring, 1))
assert(!pcall(tostring))
assert(!pcall(tonumber))

print("ok")
