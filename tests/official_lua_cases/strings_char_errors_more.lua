print("case:strings_char_errors_more")

assert(string.char() == "")
assert(not pcall(string.char, 256))
assert(not pcall(string.char, -1))
assert(not pcall(string.char, math.maxinteger))
assert(not pcall(string.char, math.mininteger))

print("ok")
