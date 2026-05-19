print("case:strings_find_empty_more")

assert(string.find("alo(.)alo", "(.)", 1, true) == 4)
local a, b = string.find("1234567890123456789", "345", 4)
assert(a == 13 and b == 15)
assert(string.find("", "") == 1)
assert(string.find("", "", 1) == 1)
assert(string.find("", "", 2) == nil)
assert(string.find("", "aaa", 1) == nil)

print("ok")
