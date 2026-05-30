print("case:strings_find_plain_more")

assert(string.find("1234567890123456789", ".45", -9) == 13)
assert(not string.find("abcdefg", "\0", 5, true))
assert(string.find("alo(.)alo", "(.)", 1, true) == 4)
local a, b = string.find("abcabc", "bc", -5, true)
assert(a == 2 and b == 3)
a, b = string.find("abcabc", "bc", -3, true)
assert(a == 5 and b == 6)

print("ok")
