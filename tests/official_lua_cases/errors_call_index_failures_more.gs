print("case:errors_call_index_failures_more")

fails := func(f) {
  ok, err := pcall(f)
  assert(!ok && type(err) == "string")
}

fails(func() { a := nil; return a(13) })
fails(func() { a := {}; return a.bbbb(3) })
fails(func() { aaa := {bbb: 1}; return aaa.bbb:ddd(9) })
fails(func() {
  a, b, c := nil, nil, nil
  return func() { a = b + 1.1 }()
})

print("ok")
