print("case:strings_sub_method_more")

assert(string.sub("123456789", 8) == "89")
assert(string.sub("123456789", -3, -2) == "78")
assert(string.upper("abcdef") == "ABCDEF")
assert(string.lower("ABCdef") == "abcdef")
assert(string.rep("abc", 3, ":") == "abc:abc:abc")

print("ok")
