print("case:pm_gsub_trim")

local a, b = string.gsub("alo alo", "alo", "x")
assert(a == "x x" and b == 2)

a, b = string.gsub("alo úlo  ", " +$", "")
assert(a == "alo úlo" and b == 1)

a, b = string.gsub("alo  alo  \n 123\n ", "%s+", " ")
assert(a == "alo alo 123 " and b == 3)

a, b = string.gsub("", "^", "r")
assert(a == "r" and b == 1)

a, b = string.gsub("", "$", "r")
assert(a == "r" and b == 1)

print("ok")
