print("case:strings_find_plain_more")

assert(string.find("1234567890123456789", ".45", -9) == 13)
assert(!string.find("abcdefg", "\0", 5, true))
assert(string.find("alo(.)alo", "(.)", 1, true) == 4)
a, b := string.find("abcabc", "bc", -5, true)
assert(a == 2 && b == 3)
a, b = string.find("abcabc", "bc", -3, true)
assert(a == 5 && b == 6)

print("ok")
