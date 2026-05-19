print("case:pm_find_nul_strings")

z := string.char(0)
s := "a" .. z .. "o a" .. z .. "o a" .. z .. "o"

a, b := string.find(s, "a", 1)
assert(a == 1 && b == 1)

a, b = string.find(s, "a" .. z .. "o", 2)
assert(a == 5 && b == 7)

a, b = string.find(s, "a" .. z .. "o", 9)
assert(a == 9 && b == 11)

e := "a" .. z .. "a" .. z .. "a" .. z .. "a" .. z .. z .. "ab"
a, b = string.find(e, z .. "ab", 2)
assert(a == 9 && b == 11)

a, b = string.find(e, "b")
assert(a == 11 && b == 11)
assert(!string.find(e, "b" .. z))
assert(!string.find("", z))

print("ok")
