print("case:utf8_multibyte_offsets")

local s = "汉字/漢字"
assert(utf8.len(s) == 5)
assert(utf8.offset(s, 1) == 1)
assert(utf8.offset(s, 2) == 4)
assert(utf8.offset(s, 3) == 7)
assert(utf8.codepoint(s, 1, 1) == 27721)
assert(utf8.char(27721, 23383, 47, 28450, 23383) == s)

local a, b, c, d, e = utf8.codepoint(s, 1, #s)
assert(a == 27721 and b == 23383 and c == 47 and d == 28450 and e == 23383)

print("ok")
