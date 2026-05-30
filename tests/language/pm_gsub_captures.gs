print("case:pm_gsub_captures")

a, b := string.gsub("abc=xyz", "(%w*)(%p)(%w+)", "%3%2%1-%0")
assert(a == "xyz=abc-abc=xyz" && b == 1)

a, b = string.gsub("abc", "%w", "%1%0")
assert(a == "aabbcc" && b == 3)

a, b = string.gsub("abc", "%w+", "%0%1")
assert(a == "abcabc" && b == 1)

a, b = string.gsub("alo alo", "()[al]", "%1")
assert(a == "12o 56o" && b == 4)

print("ok")
