print("case:errors_assert_messages_more")

t := {}

res, msg := pcall(assert, false, "X", t)
assert(!res && msg == "X")

res, msg = pcall(assert)
assert(!res && type(msg) == "string")

a, b, c := assert(1, "kept", t)
assert(a == 1 && b == "kept" && c == t)

print("ok")
