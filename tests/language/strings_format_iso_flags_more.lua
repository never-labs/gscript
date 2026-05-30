print("case:strings_format_iso_flags_more")

assert(string.format("%#12o", 10) == "         012")
assert(string.format("%#10x", 100) == "      0x64")
assert(string.format("%#-17X", 100) == "0X64             ")
assert(string.format("%013i", -100) == "-000000000100")
assert(string.format("%2.5d", -100) == "-00100")
assert(string.format("%.u", 0) == "")
assert(string.format("%-16c", 97) == "a               ")
assert(string.format("%+.3G", 1.5) == "+1.5")
assert(string.format("%.0s", "alo") == "")
assert(string.format("%.s", "alo") == "")

print("ok")
