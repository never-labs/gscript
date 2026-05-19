print("case:utf8_boundary_chars_more")

assert(utf8.char() == "")
assert(utf8.codepoint(utf8.char(1114111)) == 1114111)
assert(!pcall(utf8.char, 2147483648))
assert(!pcall(utf8.char, -1))
assert(utf8.codepoint("퟿") == 55295)
assert(utf8.codepoint("") == 57344)

print("ok")
