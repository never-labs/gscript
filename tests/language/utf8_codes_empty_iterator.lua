print("case:utf8_codes_empty_iterator")

local f = utf8.codes("")
assert(f("", 2) == nil)
assert(f("", -1) == nil)
assert(f("", math.mininteger) == nil)

print("ok")
