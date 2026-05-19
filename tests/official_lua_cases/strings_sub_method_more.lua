print("case:strings_sub_method_more")

assert(("123456789"):sub(8) == "89")
assert(("123456789"):sub(-3, -2) == "78")
assert(("abcdef"):upper() == "ABCDEF")
assert(("ABCdef"):lower() == "abcdef")
assert(("abc"):rep(3, ":") == "abc:abc:abc")

print("ok")
