print("case:errors_assert_messages_more")

local t = {}

local res, msg = pcall(assert, false, "X", t)
assert(not res and msg == "X")

res, msg = pcall(assert)
assert(not res and type(msg) == "string")

local a, b, c = assert(1, "kept", t)
assert(a == 1 and b == "kept" and c == t)

print("ok")
