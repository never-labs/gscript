print("case:errors_xpcall_args_more")

a, b, c := xpcall(string.find, error, "alo", "al")
assert(a && b == 1 && c == 2)
a, b, c = xpcall(string.find, func(x) { return {} }, true, "al")
assert(!a && type(b) == "table" && c == nil)

print("ok")
