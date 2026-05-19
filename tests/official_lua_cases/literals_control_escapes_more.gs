print("case:literals_control_escapes_more")

s := "\a\b\f\n\r\t\v"
assert(#s == 7)
a, b, c, d, e, f, g := string.byte(s, 1, 7)
assert(a == 7 && b == 8 && c == 12 && d == 10 && e == 13 && f == 9 && g == 11)

assert(string.char(99) .. "12" == "c12")
assert(string.char(99) .. "ab" == "cab")
assert(string.char(99) == "c")
assert(string.char(99) .. "\n" == "c\n")
assert("\0\0\0alo" == "\0" .. "\0\0" .. "alo")
assert(string.char(0, 5, 16, 31, 60, 255, 232) == string.char(0, 5, 16, 31, 60, 255, 232))

print("ok")
