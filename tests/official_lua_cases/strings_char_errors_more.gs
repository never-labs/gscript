print("case:strings_char_errors_more")

assert(string.char() == "")
assert(!pcall(string.char, 256))
assert(!pcall(string.char, -1))
assert(!pcall(string.char, math.maxinteger))
assert(!pcall(string.char, math.mininteger))

print("ok")
