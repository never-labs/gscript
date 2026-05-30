print("case:errors_assert_pcall")

local ok, err = pcall(assert, false, "bad")
assert(not ok and string.find(err, "bad"))

ok = pcall(error, "boom")
assert(not ok)

local a, b = assert(true, "kept")
assert(a == true and b == "kept")

print("ok")
