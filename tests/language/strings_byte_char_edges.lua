print("case:strings_byte_char_edges")

assert(string.byte("a") == 97)
assert(string.byte(string.char(255)) == 255)
assert(string.byte(string.char(0)) == 0)
assert(string.byte("ba", 2) == 97)
assert(string.byte("\n\n", 2, 2) == 10)
assert(string.byte("") == nil)
assert(string.byte("hi", -3) == nil)
assert(string.byte("hi", 3) == nil)
assert(string.byte("hi", 9, 10) == nil)
assert(string.byte("hi", 2, 1) == nil)

assert(string.char() == "")
assert(string.char(65, 66, 67) == "ABC")
local a, b, c, d, e = string.byte("hello", 1, 5)
assert(string.char(a, b, c, d, e) == "hello")

print("ok")
