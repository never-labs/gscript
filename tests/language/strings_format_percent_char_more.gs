print("case:strings_format_percent_char_more")

assert(string.format("") == "")
assert(string.format("%%") == "%")
assert(string.format("%c%c%c", 65, 66, 67) == "ABC")
assert(string.format("%1c%-c%-1c%c", 34, 48, 90, 100) == "\"0Zd")
assert(string.format("%%%d %010d", 10, 23) == "%10 0000000023")

print("ok")
