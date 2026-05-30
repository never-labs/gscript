print("case:errors_pcall_basic")

local st, err = pcall(error, "hi", 0)
assert(not st and err == "hi")

st, err = pcall(tostring, 1)
assert(st and err == "1")

st, err = pcall(assert, false)
assert(not st and string.find(err, "assertion"))

st, err = pcall(assert, nil)
assert(not st and string.find(err, "assertion"))

print("ok")
