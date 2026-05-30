print("case:pm_gsub_capture_reorder_more")

assert(string.gsub("abc=xyz", "(%w*)(%p)(%w+)", "%3%2%1-%0") == "xyz=abc-abc=xyz")
assert(string.gsub("alo alo", "()[al]", "%1") == "12o 56o")
local a, b = string.gsub("abcd", "(.)", "%0@", 2)
assert(a == "a@b@cd" and b == 2)
a, b = string.gsub("abc d", "(.)", "%1@")
assert(a == "a@b@c@ @d@" and b == 5)

print("ok")
