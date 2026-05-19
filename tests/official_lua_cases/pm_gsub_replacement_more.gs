print("case:pm_gsub_replacement_more")

a, b := string.gsub("abc", "%w", "%1%0")
assert(a == "aabbcc" && b == 3)

a, b = string.gsub("abc", "%w+", "%0%1")
assert(a == "abcabc" && b == 1)

a, b = string.gsub("", "^", "r")
assert(a == "r" && b == 1)

a, b = string.gsub("", "$", "r")
assert(a == "r" && b == 1)

a, b = string.gsub("a b cd", " *", "-")
assert(a == "-a-b-c-d-" && b == 5)

a, b = string.gsub("abc", "%w", "X", 2)
assert(a == "XXc" && b == 2)

print("ok")
