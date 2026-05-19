print("case:strings_format_tostring")

for i := 0; i <= 30; i++ {
    assert(string.len(string.rep("a", i)) == i)
}

assert(type(tostring(nil)) == "string")
assert(type(tostring(12)) == "string")
assert(string.find(tostring({}), "table:"))
assert(string.find(tostring(print), "function:"))
assert(tostring(true) == "true")
assert(tostring(false) == "false")
assert(tostring(-1203) == "-1203")
assert(tostring(1203.125) == "1203.125")
assert(tostring(-0.5) == "-0.5")
assert(tostring(-32767) == "-32767")

assert(string.format("") == "")
assert(string.format("%c", 34) .. string.format("%c", 48) ..
       string.format("%c", 90) .. string.format("%c", 100) == "\"0Zd")
assert(string.format("%c", 34)..string.format("%c", 48)..string.format("%c", 90)..string.format("%c", 100) ==
       string.format("%1c%-c%-1c%c", 34, 48, 90, 100))
assert(string.format("%-16c", 97) == "a               ")
assert(string.format("%%%d %010d", 10, 23) == "%10 0000000023")
assert(tonumber(string.format("%f", 10.3)) == 10.3)
assert(string.format("%s %s", nil, true) == "nil true")
assert(string.format("%s %.4s", false, true) == "false true")
assert(string.format("%.3s %.3s", false, true) == "fal tru")
assert(string.format("%x", 0.0) == "0")
assert(string.format("%02x", 0.0) == "00")
assert(string.format("%08X", 255) == "000000FF")
assert(string.format("%+08d", 31501) == "+0031501")
assert(string.format("%+08d", -30927) == "-0030927")

print("ok")
