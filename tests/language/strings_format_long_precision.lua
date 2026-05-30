print("case:strings_format_long_precision")

assert(string.format('"%-50s"', "a") == '"a' .. string.rep(" ", 49) .. '"')

local long = string.rep("%", 2000)
assert(string.format("-%.20s.20s", long) == "-" .. string.rep("%", 20) .. ".20s")

local embedded = string.format("%s\0 is not \0%s", "not be", "be")
assert(embedded == "not be\0 is not \0be")
assert(#embedded == 18)

local framed = string.format("\0%s\0", "abc")
assert(#framed == 5)
assert(string.byte(framed, 1) == 0)
assert(string.byte(framed, 2) == string.byte("a"))
assert(string.byte(framed, 5) == 0)

print("ok")
