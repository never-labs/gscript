print("case:strings_format_numbers_flags")

assert(string.format("%s %s", nil, true) == "nil true")
assert(string.format("%s %.4s", false, true) == "false true")
assert(string.format("%.3s %.3s", false, true) == "fal tru")
assert(string.format("%x", 0.0) == "0")
assert(string.format("%02x", 0.0) == "00")
assert(string.format("%08X", 0xFFFFFFFF) == "FFFFFFFF")
assert(string.format("%+08d", 31501) == "+0031501")
assert(string.format("%+08d", -30927) == "-0030927")
assert(string.format("%.0s", "alo") == "")
assert(string.format("%.s", "alo") == "")

print("ok")
