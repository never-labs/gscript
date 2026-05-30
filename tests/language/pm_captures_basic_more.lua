print("case:pm_captures_basic_more")

assert(string.match("alo xyzK", "(%w+)K") == "xyz")
assert(string.match("254 K", "(%d*)K") == "")
assert(string.match("alo ", "(%w*)$") == "")
assert(not string.match("alo ", "(%w+)$"))

local a, b = string.find("(alo)", "%(...")
assert(a == 1 and b == 4)

a, b = string.gsub("  alo alo  ", "^%s*(.-)%s*$", "%1")
assert(a == "alo alo" and b == 1)

a, b = string.gsub("abc=xyz", "(%w*)(%p)(%w+)", "%3%2%1-%0")
assert(a == "xyz=abc-abc=xyz" and b == 1)

a, b = string.gsub("alo alo", "()[al]", "%1")
assert(a == "12o 56o" and b == 4)

print("ok")
