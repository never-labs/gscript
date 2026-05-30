print("case:strings_format_string_values")

assert(string.format("") == "")
assert(string.format("%s %s", nil, true) == "nil true")
assert(string.format("%s %.4s", false, true) == "false true")
assert(string.format("%.3s %.3s", false, true) == "fal tru")
assert(string.format('"%-10s"', "a") == '"a' .. string.rep(" ", 9) .. '"')

local s = string.format("\0%s\0", "\0\0\1")
assert(#s == 5)
assert(string.byte(s, 1) == 0)
assert(string.byte(s, 2) == 0)
assert(string.byte(s, 3) == 0)
assert(string.byte(s, 4) == 1)
assert(string.byte(s, 5) == 0)

print("ok")
