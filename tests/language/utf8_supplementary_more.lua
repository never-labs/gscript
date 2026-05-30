print("case:utf8_supplementary_more")

local s = "𣲷𠜎𠱓𡁻𠵼ab𠺢"
local t = {0x23CB7, 0x2070E, 0x20C53, 0x2107B, 0x20D7C, 0x61, 0x62, 0x20EA2}
assert(utf8.len(s) == #t)
for i = 1, #t do
  local p = utf8.offset(s, i)
  assert(utf8.codepoint(s, p) == t[i])
end
local i = 0
for p, c in utf8.codes(s) do
  i = i + 1
  assert(p == utf8.offset(s, i))
  assert(c == t[i])
end
assert(i == #t)

print("ok")
