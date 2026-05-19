print("case:strings_format_numeric_flags_more")

assert(string.format("%c", 34) .. string.format("%c", 48) ..
       string.format("%c", 90) .. string.format("%c", 100) ==
       string.format("%1c%-c%-1c%c", 34, 48, 90, 100))

assert(string.format("%%%d %010d", 10, 23) == "%10 0000000023")
assert(tonumber(string.format("%f", 10.3)) == 10.3)
assert(string.format("%#12o", 10) == "         012")
assert(string.format("%#10x", 100) == "      0x64")
assert(string.format("%#-17X", 100) == "0X64             ")
assert(string.format("%013i", -100) == "-000000000100")
assert(string.format("%2.5d", -100) == "-00100")
assert(string.format("%.u", 0) == "")
assert(string.format("%+#014.0f", 100) == "+000000000100.")
assert(string.format("%-16c", 97) == "a               ")
assert(string.format("%+.3G", 1.5) == "+1.5")
assert(string.format("%.0s", "alo") == "")
assert(string.format("%.s", "alo") == "")

print("ok")
