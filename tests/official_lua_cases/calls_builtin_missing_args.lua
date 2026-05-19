print("case:calls_builtin_missing_args")

assert(not pcall(type))
assert(not pcall(tostring))
assert(not pcall(tonumber))
assert(not pcall(assert))
assert(not pcall(pcall))
assert(not pcall(xpcall))
assert(not pcall(xpcall, print))

local ok, err = pcall(type)
assert(not ok and type(err) == "string")

ok, err = pcall(tostring)
assert(not ok and type(err) == "string")

ok, err = pcall(assert)
assert(not ok and type(err) == "string")

print("ok")
