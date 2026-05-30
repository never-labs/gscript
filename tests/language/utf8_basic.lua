print("case:utf8_basic")

assert(not utf8.offset("alo", 5))
assert(not utf8.offset("alo", -4))

local s = "hello World"
for i = 1, utf8.len(s) do
  assert(string.byte(s, i) == string.byte(s, i))
end

assert(utf8.char() == "")
assert(utf8.char(97, 98, 99) == "abc")
assert(utf8.codepoint("abc", 1, 1) == 97)
assert(utf8.offset("alo", 2) == 2)
assert(utf8.len("汉字/漢字") == 5)

print("ok")
