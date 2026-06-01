print("case:errors_non_string_messages_more")

t := {}

func error_table() {
  error(t)
}
res, msg := pcall(error_table)
assert(!res && msg == t)

func error_nil() {
  error(nil)
}
res, msg = pcall(error_nil)
assert(!res)

func fail_table() {
  error({msg: "x"})
}
func append_y(r) {
  return {msg: r.msg .. "y"}
}
res, msg = xpcall(fail_table, append_y)
assert(!res && msg.msg == "xy")

res, msg = pcall(assert, false, "X", t)
assert(!res && msg == "X")

res, msg = pcall(assert, false, t)
assert(!res)

res, msg = pcall(assert, nil, nil)
assert(!res)

print("ok")
