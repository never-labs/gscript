print("case:pm_gsub_captures")

local a, b = string.gsub("abc=xyz", "(%w*)(%p)(%w+)", "%3%2%1-%0")
assert(a == "xyz=abc-abc=xyz" and b == 1)

a, b = string.gsub("abc", "%w", "%1%0")
assert(a == "aabbcc" and b == 3)

a, b = string.gsub("abc", "%w+", "%0%1")
assert(a == "abcabc" and b == 1)

a, b = string.gsub("alo alo", "()[al]", "%1")
assert(a == "12o 56o" and b == 4)

print("ok")
