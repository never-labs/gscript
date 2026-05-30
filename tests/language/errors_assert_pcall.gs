print("case:errors_assert_pcall")

ok, err := pcall(assert, false, "bad")
assert(!ok && string.find(err, "bad"))

ok = pcall(error, "boom")
assert(!ok)

a, b := assert(true, "kept")
assert(a == true && b == "kept")

print("ok")
