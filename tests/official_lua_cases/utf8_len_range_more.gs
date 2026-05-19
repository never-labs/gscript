print("case:utf8_len_range_more")

x := "日本語a-4\0éó"
assert(#x == 17)
assert(utf8.len(x) == 9)
assert(utf8.offset(x, 1) == 1)
assert(utf8.offset(x, 4) == 10)
assert(utf8.offset(x, 8) == 14)
assert(utf8.offset(x, -1, #x + 1) == 16)

assert(utf8.len(x, utf8.offset(x, 4), -1) == 6)
assert(utf8.len(x, utf8.offset(x, 8), utf8.offset(x, 9) + 1) == 2)

a, b := utf8.codepoint(x, utf8.offset(x, 8), utf8.offset(x, 9))
assert(a == 233 && b == 243)

print("ok")
