print("case:pm_escape_compat_more")

assert(string.match("0alo alo", "%x*") == "0a")
assert(string.match("0alo alo", "%X+") == "lo ")

local nul = string.char(0)
assert(string.match("a" .. nul .. "b", "%z") == nul)
assert(string.match("a" .. nul .. "b", "%Z+") == "a")

assert(string.match("-", "[%W]") == "-")
assert(string.match("abc-123", "[^%W]+") == "abc")
assert(string.match("a\nb", "a.b") == "a\nb")

assert(string.match("a (b (c) d) z", "%b()") == "(b (c) d)")
local s, n = string.gsub("alo 'oi' alo", "%b''", '"')
assert(s == 'alo " alo' and n == 1)
local t, m = string.gsub("a (b (c) d) z", "%b()", "")
assert(t == "a  z" and m == 1)

print("ok")
