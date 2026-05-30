print("case:utf8_codes_iterator")

local s = "a中文b"
local expected_pos = {1, 2, 5, 8}
local expected_code = {97, 20013, 25991, 98}

assert(utf8.offset(s, 0) == 1)
assert(utf8.offset(s, 0, 3) == 2)
assert(utf8.offset(s, 0, 6) == 5)
assert(utf8.offset(s, -1, utf8.offset(s, 3)) == utf8.offset(s, 2))
assert(utf8.offset(s, 2, utf8.offset(s, 3)) == utf8.offset(s, 4))

local i = 0
for p, c in utf8.codes(s) do
  i = i + 1
  assert(p == expected_pos[i])
  assert(c == expected_code[i])
  assert(utf8.codepoint(s, p, p) == c)
end
assert(i == #expected_code)

print("ok")
