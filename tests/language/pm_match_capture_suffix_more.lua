print("case:pm_match_capture_suffix_more")

assert(string.match("alo xyzK", "(%w+)K") == "xyz")
assert(string.match("254 K", "(%d*)K") == "")
assert(string.match("alo ", "(%w*)$") == "")
assert(not string.match("alo ", "(%w+)$"))

local a, b = string.match("abc123", "(%a+)(%d+)")
assert(a == "abc" and b == "123")

print("ok")
