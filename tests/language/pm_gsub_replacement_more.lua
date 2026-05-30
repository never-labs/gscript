print("case:pm_gsub_replacement_more")

local a, b = string.gsub("abc", "%w", "%1%0")
assert(a == "aabbcc" and b == 3)

a, b = string.gsub("abc", "%w+", "%0%1")
assert(a == "abcabc" and b == 1)

a, b = string.gsub("", "^", "r")
assert(a == "r" and b == 1)

a, b = string.gsub("", "$", "r")
assert(a == "r" and b == 1)

a, b = string.gsub("a b cd", " *", "-")
assert(a == "-a-b-c-d-" and b == 5)

a, b = string.gsub("abc", "%w", "X", 2)
assert(a == "XXc" and b == 2)

print("ok")
