print("case:errors_pcall_basic")

st, err := pcall(error, "hi", 0)
assert(!st && err == "hi")

st, err = pcall(tostring, 1)
assert(st && err == "1")

st, err = pcall(assert, false)
assert(!st && string.find(err, "assertion"))

st, err = pcall(assert, nil)
assert(!st && string.find(err, "assertion"))

print("ok")
