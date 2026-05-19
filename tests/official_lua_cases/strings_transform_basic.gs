print("case:strings_transform_basic")

assert(string.sub("abcdef", 2, 4) == "bcd")
assert(string.sub("abcdef", -3) == "def")
assert(string.lower("ABC") == "abc")
assert(string.upper("abc") == "ABC")
assert(string.reverse("abc") == "cba")
assert(string.rep("ab", 3, ".") == "ab.ab.ab")
assert(string.rep("", 10, ".") == string.rep(".", 9))

print("ok")
