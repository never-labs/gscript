print("case:calls_builtin_missing_args")

assert(!pcall(type))
assert(!pcall(tostring))
assert(!pcall(tonumber))
assert(!pcall(assert))
assert(!pcall(pcall))
assert(!pcall(xpcall))
assert(!pcall(xpcall, print))

ok, err := pcall(type)
assert(!ok && type(err) == "string")

ok, err = pcall(tostring)
assert(!ok && type(err) == "string")

ok, err = pcall(assert)
assert(!ok && type(err) == "string")

print("ok")
