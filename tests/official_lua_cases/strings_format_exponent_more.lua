print("case:strings_format_exponent_more")

assert(string.match(string.format("% 1.0E", 100), "^ 1E%+0+2$"))
assert(string.match(string.format("% .1g", 2^10), "^ 1e%+0+3$"))
assert(string.format("%+.3G", 1.5) == "+1.5")
assert(string.format("%.0s", "alo") == "")
assert(string.format("%.s", "alo") == "")

print("ok")
