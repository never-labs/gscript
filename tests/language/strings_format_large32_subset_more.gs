print("case:strings_format_large32_subset_more")

max := 2147483647
min := -2147483648

assert(string.format("%x", max) == "7fffffff")
assert(string.sub(string.format("%x", min), -8) == "80000000")
assert(string.format("%d", max) == "2147483647")
assert(string.format("%d", min) == "-2147483648")
assert(string.format("%u", 4294967295) == "4294967295")
assert(string.format("%o", 43981) == "125715")

print("ok")
