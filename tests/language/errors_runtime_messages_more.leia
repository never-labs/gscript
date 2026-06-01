print("case:errors_runtime_messages_more")

fails := func(f) {
  ok, err := pcall(f)
  assert(!ok && type(err) == "string")
}

fails(func() { return {} + 1 })
fails(func() { a := nil; return a(13) })
fails(func() { a := {}; return a.bbbb(3) })
fails(func() { return #3 })
fails(func() { return #print })
fails(func() { a, b, c := nil, nil, nil; return (a && b || c)() })
fails(func() { return print < 10 })
fails(func() { return print < print })
fails(func() { return "10" < 10 })
fails(func() { return 10 < "23" })

print("ok")
