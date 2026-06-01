print("case:utf8_multibyte_offsets")

s := "汉字/漢字"
assert(utf8.len(s) == 5)
assert(utf8.offset(s, 1) == 1)
assert(utf8.offset(s, 2) == 4)
assert(utf8.offset(s, 3) == 7)
assert(utf8.codepoint(s, 1, 1) == 27721)
assert(utf8.char(27721, 23383, 47, 28450, 23383) == s)

a, b, c, d, e := utf8.codepoint(s, 1, #s)
assert(a == 27721 && b == 23383 && c == 47 && d == 28450 && e == 23383)

print("ok")
