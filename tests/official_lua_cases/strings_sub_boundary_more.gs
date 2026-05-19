print("case:strings_sub_boundary_more")

maxi := math.maxinteger
mini := math.mininteger
assert(string.sub("123456789", 7) == "789")
assert(string.sub("123456789", 7, 6) == "")
assert(string.sub("123456789", 0, 0) == "")
assert(string.sub("123456789", -10, 10) == "123456789")
assert(string.sub("123456789", -10, -20) == "")
assert(string.sub("123456789", -6, -4) == "456")
assert(string.sub("123456789", mini, -4) == "123456")
assert(string.sub("123456789", mini, maxi) == "123456789")
assert(string.sub("123456789", mini, mini) == "")

print("ok")
