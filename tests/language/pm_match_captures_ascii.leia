print("case:pm_match_captures_ascii")

assert(string.match("alo xyzK", "(%w+)K") == "xyz")
assert(string.match("254 K", "(%d*)K") == "")
assert(string.match("alo ", "(%w*)$") == "")
assert(!string.match("alo ", "(%w+)$"))

a, b := string.match("abc123", "([a-z]+)(%d+)")
assert(a == "abc" && b == "123")

a, b = string.match("abc", "(a*)(b*)")
assert(a == "a" && b == "b")

assert(string.match("abc", "(z*)") == "")

print("ok")
