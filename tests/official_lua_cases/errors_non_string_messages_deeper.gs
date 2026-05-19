print("case:errors_non_string_messages_deeper")

t := {msg: "x"}

res, msg := pcall(func() { error(t) })
assert(!res && msg == t)

func f() {
  error({msg: "x"})
}
res, msg = xpcall(f, func(r) { return {msg: r.msg .. "y"} })
assert(!res && msg.msg == "xy")

print("ok")
