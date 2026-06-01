print("case:strings_patterns_basic")

assert(string.match("abc123", "%a+") == "abc")
assert(string.match("abc123", "%d+") == "123")
assert(string.find("123456789", "345") == 3)
assert(string.find("1234567890123456789", "345", 4) == 13)
assert(!string.find("abcdefg", "xyz"))

s, n := string.gsub("hello world", "%w+", "x")
assert(s == "x x" && n == 2)

f := string.gmatch("a b", "%w+")
assert(f() == "a" && f() == "b" && f() == nil)

print("ok")
