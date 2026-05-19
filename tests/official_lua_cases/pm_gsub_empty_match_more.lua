print("case:pm_gsub_empty_match_more")

assert(string.gsub("a b cd", " *", "-") == "-a-b-c-d-")
assert(string.gsub("", "^", "r") == "r")
assert(string.gsub("", "$", "r") == "r")
assert(string.gsub("abc", "%w", "%1%0") == "aabbcc")
assert(string.gsub("abc", "%w+", "%0%1") == "abcabc")

print("ok")
