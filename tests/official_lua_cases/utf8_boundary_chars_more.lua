print("case:utf8_boundary_chars_more")

assert(utf8.char() == "")
assert(utf8.codepoint(utf8.char(0x10FFFF)) == 0x10FFFF)
assert(not pcall(utf8.char, 0x7FFFFFFF + 1))
assert(not pcall(utf8.char, -1))
assert(utf8.codepoint("\u{D7FF}") == 0xD800 - 1)
assert(utf8.codepoint("\u{E000}") == 0xDFFF + 1)

print("ok")
