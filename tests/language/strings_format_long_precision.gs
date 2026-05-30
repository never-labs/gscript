print("case:strings_format_long_precision")

assert(string.format("\"%-50s\"", "a") == "\"a" .. string.rep(" ", 49) .. "\"")

long := string.rep("%", 2000)
assert(string.format("-%.20s.20s", long) == "-" .. string.rep("%", 20) .. ".20s")

embedded := string.format("%s" .. string.char(0) .. " is not " .. string.char(0) .. "%s", "not be", "be")
assert(embedded == "not be" .. string.char(0) .. " is not " .. string.char(0) .. "be")
assert(#embedded == 18)

framed := string.format(string.char(0) .. "%s" .. string.char(0), "abc")
assert(#framed == 5)
assert(string.byte(framed, 1) == 0)
assert(string.byte(framed, 2) == string.byte("a"))
assert(string.byte(framed, 5) == 0)

print("ok")
