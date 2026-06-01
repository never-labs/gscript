print("case:pm_gsub_limit_count_more")

a, b := string.gsub("banana", "a", "A", 2)
assert(a == "bAnAna" && b == 2)

a, b = string.gsub("banana", "a", "A", 0)
assert(a == "banana" && b == 0)

a, b = string.gsub("banana", "z", "Z")
assert(a == "banana" && b == 0)

a, b = string.gsub("banana", "a+", "A")
assert(a == "bAnAnA" && b == 3)

print("ok")
