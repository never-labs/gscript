print("case:pm_match_repetition")

assert(string.match("aaab", ".*b") == "aaab")
assert(string.match("aaa", ".*a") == "aaa")
assert(string.match("b", ".*b") == "b")
assert(string.match("aaab", ".+b") == "aaab")
assert(string.match("aaa", ".+a") == "aaa")
assert(!string.match("b", ".+b"))
assert(string.match("aaab", "a*") == "aaa")
assert(string.match("aaa", "^.*$") == "aaa")
assert(string.match("aaa", "b*") == "")

print("ok")
