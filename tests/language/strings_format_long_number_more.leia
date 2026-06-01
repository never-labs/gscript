print("case:strings_format_long_number_more")

assert(10 ** 38 < math.huge)
s := string.format("%.99f", -(10 ** 38))
assert(string.len(s) >= 38 + 101)
assert(tonumber(s) == -(10 ** 38))

print("ok")
