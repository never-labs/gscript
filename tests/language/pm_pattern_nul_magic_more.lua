print("case:pm_pattern_nul_magic_more")

assert(string.match("ab\0\1\2c", "[\0-\2]+") == "\0\1\2")
assert(string.match("ab\0\1\2c", "[\0-\0]+") == "\0")
assert(string.find("b$a", "$\0?") == 2)
assert(string.find("abc\0efg", "%\0") == 4)
assert(string.match("abc\0\0\0", "%\0+") == "\0\0\0")
assert(string.match("abc\0\0\0", "%\0%\0?") == "\0\0")
assert(string.find("abc\0\0", "\0.") == 4)
assert(string.find("abcx\0\0abc\0abc", "x\0\0abc\0a.") == 4)

print("ok")
