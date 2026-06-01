print("case:pm_find_nul_more2")

a, b := string.find("", "")
assert(a == 1 && b == 0)
a, b = string.find("alo", "")
assert(a == 1 && b == 0)
a, b = string.find("a\0o a\0o a\0o", "a", 1)
assert(a == 1 && b == 1)
assert(!string.find("", "\0"))

print("ok")
