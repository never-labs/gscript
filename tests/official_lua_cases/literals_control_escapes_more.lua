print("case:literals_control_escapes_more")

local s = "\a\b\f\n\r\t\v"
assert(#s == 7)
local a, b, c, d, e, f, g = string.byte(s, 1, 7)
assert(a == 7 and b == 8 and c == 12 and d == 10 and e == 13 and f == 9 and g == 11)

assert("\09912" == "c12")
assert("\99ab" == "cab")
assert("\099" == "c")
assert("\099\n" == "c\10")
assert("\0\0\0alo" == "\0" .. "\0\0" .. "alo")
assert("\x00\x05\x10\x1f\x3C\xfF\xe8" == string.char(0, 5, 16, 31, 60, 255, 232))

print("ok")
