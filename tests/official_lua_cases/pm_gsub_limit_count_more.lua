print("case:pm_gsub_limit_count_more")

local a, b = string.gsub("banana", "a", "A", 2)
assert(a == "bAnAna" and b == 2)

a, b = string.gsub("banana", "a", "A", 0)
assert(a == "banana" and b == 0)

a, b = string.gsub("banana", "z", "Z")
assert(a == "banana" and b == 0)

a, b = string.gsub("banana", "a+", "A")
assert(a == "bAnAnA" and b == 3)

print("ok")
