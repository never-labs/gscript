print("case:errors_xpcall_handler_more")

res, msg := xpcall(error, func(m) { return "handled:" .. m }, "boom")
assert(!res && msg == "handled:boom")

res, msg = xpcall(error, error, "inner")
assert(!res && type(msg) == "string")

seen := nil
res, msg = xpcall(func() { error({code: 7}) }, func(r) {
  seen = r
  return r.code + 1
})
assert(!res && seen.code == 7 && msg == 8)

print("ok")
